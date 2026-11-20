// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/validate"
)

// maxProductIssueConversation caps the comments GetDetail returns (the latest).
const maxProductIssueConversation = 200

// ProductIssueSelectSQL selects the columns of one ProductIssueSummary row, one
// row per PRODUCT_ISSUE work item with its active AFFECTS link (if any). It
// ends before WHERE, so every reader adds its own conditions. Exported so the
// integration test can run it under a customer session and see that row-level
// security hides every row.
const ProductIssueSelectSQL = `
	SELECT wi.id::TEXT, wi.number, wi.subject,
	       g.owner, g.repository, g.issue_number, g.html_url, g.github_state, g.state_reason, g.labels,
	       g.board_status, g.board_status_raw, pi.priority, pi.category,
	       l.to_id::TEXT, cs.number, l.source, l.created_on,
	       GREATEST(g.github_updated_on,
	                (SELECT MAX(c.created_on) FROM comment c WHERE c.work_item_id = wi.id AND c.deleted_at IS NULL),
	                (SELECT MAX(e.occurred_at) FROM work_item_status_event e WHERE e.work_item_id = wi.id)),
	       g.link_state
	  FROM work_item wi
	  JOIN product_issue pi ON pi.id = wi.id
	  JOIN github_issue_link g ON g.work_item_id = wi.id
	  LEFT JOIN work_item_link l ON l.from_id = wi.id AND l.kind = 'AFFECTS' AND l.is_active
	  LEFT JOIN work_item cs ON cs.id = l.to_id`

func scanProductIssueSummary(row pgx.Row) (domain.ProductIssueSummary, error) {
	var s domain.ProductIssueSummary
	var labels []byte
	var caseID, caseNumber *string
	err := row.Scan(&s.ID, &s.Number, &s.Title,
		&s.GitHub.Owner, &s.GitHub.Repository, &s.GitHub.IssueNumber, &s.GitHub.HTMLURL, &s.GitHub.State, &s.GitHub.StateReason, &labels,
		&s.BoardStatus, &s.BoardStatusRaw, &s.Priority, &s.Category,
		&caseID, &caseNumber, &s.LinkSource, &s.LinkedOn, &s.LastActivityOn, &s.LinkState)
	if err != nil {
		return s, err
	}
	s.GitHub.Labels = []string{}
	if len(labels) > 0 {
		if err := json.Unmarshal(labels, &s.GitHub.Labels); err != nil {
			return s, fmt.Errorf("decode labels: %w", err)
		}
	}
	s.NonStandardColumn = s.BoardStatus != nil && *s.BoardStatus == "OTHER"
	if caseID != nil {
		s.Case = &domain.ProductIssueCaseRef{ID: *caseID, Number: stringOrEmpty(caseNumber)}
	}
	return s, nil
}

// ManualRegisterParams are the arguments of RegisterManualProductIssue.
type ManualRegisterParams struct {
	CaseID, Actor, Title, Priority, Category string
	OpenedByEmail, ABTTeam, PublicIssueURL   string
	Issue                                    GitHubIssueRef
	State, StateReason                       string
	Labels                                   []string
	GitHubCreatedOn, GitHubUpdatedOn         *time.Time
	GitHubClosedOn                           *time.Time
}

// ProductIssueRepository is the read side and the manual link of product issues.
// Every method runs under the system identity: the routes are internal-only.
type ProductIssueRepository interface {
	// LoadCase is the case a link is made to (404 unless case-like).
	LoadCase(ctx context.Context, caseID string) (ProductWorkItemCase, error)
	ListForCase(ctx context.Context, caseID string) ([]domain.ProductIssueSummary, error)
	// GetSummary returns one product issue's summary row (404 for another type).
	GetSummary(ctx context.Context, id string) (domain.ProductIssueSummary, error)
	GetDetail(ctx context.Context, id string) (domain.ProductIssueDetail, error)
	Search(ctx context.Context, f domain.SearchProductIssuesFilters, p domain.Pagination) ([]domain.ProductIssueSummary, int, error)
	// FindByGitHubIssue returns the work item that has a github_issue_link for the
	// issue (owner and repository compared case-insensitively), "" if none.
	FindByGitHubIssue(ctx context.Context, owner, repository string, number int) (string, error)
	// MappedRepository returns the owner and repository, as spelled by the first
	// matching product_repo_mapping row (any row, active or not), and whether one
	// exists.
	MappedRepository(ctx context.Context, owner, repository string) (owner2, repository2 string, ok bool, err error)
	// IssueBeingCreated reports whether a case's create request (PENDING or
	// GITHUB_CREATED, not yet COMPLETED or FAILED) claims the GitHub issue: it is
	// mid-registration, and a manual link must not register it first.
	IssueBeingCreated(ctx context.Context, owner, repository string, number int) (bool, error)
	// LinkExisting links a product issue to a case with source MANUAL. created is
	// false when it already had the active link to this case. 404 when the work
	// item is not a product issue; 409 already_linked when it is linked to another
	// case. It rests on the unique index, so concurrent calls cannot both link.
	LinkExisting(ctx context.Context, workItemID, caseID, actor string) (created bool, err error)
	// RegisterManualProductIssue writes a product issue, its GitHub link and a MANUAL
	// case link in one transaction. ErrGitHubIssueAlreadyLinked when the issue got a
	// link first.
	RegisterManualProductIssue(ctx context.Context, p ManualRegisterParams) (workItemID string, err error)
	// Unlink deactivates the active AFFECTS link of the product issue to the case;
	// false when there was none.
	Unlink(ctx context.Context, caseID, workItemID, actor string) (bool, error)
}

type productIssueRepo struct {
	db    *Scoped
	cases *productWorkItemRepo
}

// NewProductIssueRepository returns the Postgres implementation.
func NewProductIssueRepository(db *Scoped) ProductIssueRepository {
	return &productIssueRepo{db: db, cases: &productWorkItemRepo{db: db}}
}

func (r *productIssueRepo) LoadCase(ctx context.Context, caseID string) (ProductWorkItemCase, error) {
	return r.cases.LoadCase(ctx, caseID)
}

func (r *productIssueRepo) ListForCase(ctx context.Context, caseID string) ([]domain.ProductIssueSummary, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, ProductIssueSelectSQL+`
	 WHERE wi.type = 'PRODUCT_ISSUE' AND l.to_id = $1::uuid
	 ORDER BY wi.created_on DESC, wi.id DESC`, caseID)
	if err != nil {
		return nil, fmt.Errorf("list product issues of case: %w", err)
	}
	defer rows.Close()
	out := []domain.ProductIssueSummary{}
	for rows.Next() {
		s, err := scanProductIssueSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan product issue: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return out, attachProductIssueClocks(ctx, r.db, out)
}

func (r *productIssueRepo) GetSummary(ctx context.Context, id string) (domain.ProductIssueSummary, error) {
	ctx = WithSystemIdentity(ctx)
	if !validate.IsUUID(id) {
		return domain.ProductIssueSummary{}, &apierror.NotFoundError{Msg: "product issue not found"}
	}
	s, err := scanProductIssueSummary(r.db.QueryRow(ctx, ProductIssueSelectSQL+` WHERE wi.type = 'PRODUCT_ISSUE' AND wi.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return s, &apierror.NotFoundError{Msg: "product issue not found"}
	}
	if err != nil {
		return s, fmt.Errorf("get product issue: %w", err)
	}
	one := []domain.ProductIssueSummary{s}
	if err := attachProductIssueClocks(ctx, r.db, one); err != nil {
		return s, err
	}
	return one[0], nil
}

func (r *productIssueRepo) GetDetail(ctx context.Context, id string) (domain.ProductIssueDetail, error) {
	ctx = WithSystemIdentity(ctx)
	var d domain.ProductIssueDetail
	var err error
	d.ProductIssueSummary, err = r.GetSummary(ctx, id)
	if err != nil {
		return d, err
	}
	err = r.db.QueryRow(ctx, `
		SELECT pi.opened_by_email, pi.abt_team, pi.public_issue_url, wi.created_on, g.last_synced_on
		  FROM work_item wi
		  JOIN product_issue pi ON pi.id = wi.id
		  JOIN github_issue_link g ON g.work_item_id = wi.id
		 WHERE wi.id = $1::uuid`, id).Scan(&d.OpenedByEmail, &d.ABTTeam, &d.PublicIssueURL, &d.CreatedOn, &d.LastSyncedOn)
	if err != nil {
		return d, fmt.Errorf("get product issue extension: %w", err)
	}

	// The latest comments, oldest first. A comment deleted in the portal or on
	// GitHub is not part of the conversation.
	crows, err := r.db.Query(ctx, `
		SELECT id, type, author, content, created_on, origin, github_comment_id, author_login, delivery, command_id FROM (
		  SELECT c.id::TEXT AS id, c.type::TEXT AS type, c.created_by AS author, COALESCE(c.content, '') AS content, c.created_on,
		         ref.origin, ref.github_comment_id, ref.author_login,
		         CASE WHEN ref.origin = 'PORTAL' AND gc.status IN ('PENDING', 'DELIVERED', 'FAILED', 'CANCELLED') THEN gc.status END AS delivery,
		         CASE WHEN ref.origin = 'PORTAL' THEN gc.id::TEXT END AS command_id
		    FROM comment c
		    LEFT JOIN comment_github_ref ref ON ref.comment_id = c.id
		    LEFT JOIN LATERAL (SELECT g2.id, g2.status FROM github_command g2
		                        WHERE g2.comment_id = c.id AND g2.kind = 'ADD_COMMENT'
		                        ORDER BY g2.created_on DESC, g2.id DESC LIMIT 1) gc ON TRUE
		   WHERE c.work_item_id = $1::uuid AND c.deleted_at IS NULL AND ref.deleted_on IS NULL
		   ORDER BY c.created_on DESC, c.id DESC
		   LIMIT $2) latest
		ORDER BY created_on ASC, id ASC`, id, maxProductIssueConversation+1)
	if err != nil {
		return d, fmt.Errorf("list product issue conversation: %w", err)
	}
	defer crows.Close()
	d.Conversation = []domain.ProductIssueComment{}
	for crows.Next() {
		var c domain.ProductIssueComment
		if err := crows.Scan(&c.ID, &c.Type, &c.Author, &c.Body, &c.CreatedOn, &c.Origin, &c.GitHubCommentID, &c.AuthorLogin, &c.Delivery, &c.CommandID); err != nil {
			return d, fmt.Errorf("scan product issue comment: %w", err)
		}
		d.Conversation = append(d.Conversation, c)
	}
	if err := crows.Err(); err != nil {
		return d, err
	}
	if len(d.Conversation) > maxProductIssueConversation {
		d.ConversationTruncated = true
		d.Conversation = d.Conversation[1:] // the extra, oldest row
	}

	erows, err := r.db.Query(ctx, `
		SELECT id::TEXT, from_status, to_status, waits_on, occurred_at, actor, source, raw_status
		  FROM work_item_status_event WHERE work_item_id = $1::uuid ORDER BY occurred_at ASC, id ASC`, id)
	if err != nil {
		return d, fmt.Errorf("list product issue status history: %w", err)
	}
	defer erows.Close()
	d.StatusHistory = []domain.ProductIssueStatusEvent{}
	for erows.Next() {
		var e domain.ProductIssueStatusEvent
		if err := erows.Scan(&e.ID, &e.FromStatus, &e.ToStatus, &e.WaitsOn, &e.OccurredOn, &e.Actor, &e.Source, &e.RawStatus); err != nil {
			return d, fmt.Errorf("scan product issue status event: %w", err)
		}
		d.StatusHistory = append(d.StatusHistory, e)
	}
	if err := erows.Err(); err != nil {
		return d, err
	}
	erows.Close()

	cmds, err := listGitHubCommands(ctx, r.db, id)
	if err != nil {
		return d, err
	}
	d.Commands = make([]domain.GitHubCommand, 0, len(cmds))
	for _, c := range cmds {
		d.Commands = append(d.Commands, c.Domain())
	}
	return d, nil
}

func (r *productIssueRepo) Search(ctx context.Context, f domain.SearchProductIssuesFilters, p domain.Pagination) ([]domain.ProductIssueSummary, int, error) {
	ctx = WithSystemIdentity(ctx)
	conds := []string{"wi.type = 'PRODUCT_ISSUE'"}
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "$$", fmt.Sprintf("$%d", len(args))))
	}
	if f.CaseID != "" {
		add("l.to_id = $$::uuid", f.CaseID)
	}
	if f.Unmatched {
		conds = append(conds, "l.id IS NULL")
	}
	if f.GitHubState != "" {
		add("g.github_state = $$", f.GitHubState)
	}
	if len(f.BoardStatuses) > 0 {
		add("g.board_status = ANY($$::text[])", f.BoardStatuses)
	}
	if f.Owner != "" {
		add("lower(g.owner) = lower($$)", f.Owner)
	}
	if f.Repository != "" {
		add("lower(g.repository) = lower($$)", f.Repository)
	}
	if f.Query != "" {
		args = append(args, "%"+escapeLikeWildcards(f.Query)+"%")
		cond := fmt.Sprintf(`(wi.number ILIKE $%[1]d ESCAPE '\' OR wi.subject ILIKE $%[1]d ESCAPE '\'`, len(args))
		if n, ok := digitsToInt(f.Query); ok {
			args = append(args, n)
			cond += fmt.Sprintf(" OR g.issue_number = $%d", len(args))
		}
		conds = append(conds, cond+")")
	}
	if f.Mine {
		args = append(args, f.MineEmail)
		n := len(args)
		// created_by / opened_by_email are emails; the case's assignee is a user row.
		conds = append(conds, fmt.Sprintf(`(lower(wi.created_by) = lower($%[1]d) OR lower(pi.opened_by_email) = lower($%[1]d)
		    OR EXISTS (SELECT 1 FROM work_item_link ml
		                 JOIN work_item mc ON mc.id = ml.to_id
		                 JOIN "user" mu ON mu.id = mc.assigned_to_id
		                WHERE ml.from_id = wi.id AND ml.kind = 'AFFECTS' AND ml.is_active AND lower(mu.email) = lower($%[1]d)))`, n))
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM work_item wi
		  JOIN product_issue pi ON pi.id = wi.id
		  JOIN github_issue_link g ON g.work_item_id = wi.id
		  LEFT JOIN work_item_link l ON l.from_id = wi.id AND l.kind = 'AFFECTS' AND l.is_active`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count product issues: %w", err)
	}
	pageArgs := append(append([]any{}, args...), p.Limit, p.Offset)
	from, order := ProductIssueSelectSQL, "wi.created_on DESC, wi.id DESC"
	if f.Sort == domain.ProductIssueSortPausedSince || f.Sort == domain.ProductIssueSortSLAPercentage {
		// The newest resolution clock row decides the order: the same row the clock read
		// attaches (productIssueClockWhere, productIssueClockRank). The percentage is the
		// stored column, which is as of the last engine write.
		from += ` LEFT JOIN LATERAL (
		    SELECT s.paused_on, s.business_elapsed_percentage AS pct
		      FROM sla s JOIN sla_policy sp ON sp.id = s.sla_policy_id
		     WHERE s.work_item_id = wi.id AND ` + productIssueClockWhere + `
		     ORDER BY ` + productIssueClockRank + ` LIMIT 1) ck ON TRUE`
		if f.Sort == domain.ProductIssueSortPausedSince {
			order = "ck.paused_on ASC NULLS LAST, " + order
		} else {
			order = "ck.pct DESC NULLS LAST, " + order
		}
	}
	rows, err := r.db.Query(ctx, from+where+fmt.Sprintf(
		" ORDER BY %s LIMIT $%d OFFSET $%d", order, len(args)+1, len(args)+2), pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search product issues: %w", err)
	}
	defer rows.Close()
	out := []domain.ProductIssueSummary{}
	for rows.Next() {
		s, err := scanProductIssueSummary(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan product issue: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()
	if err := attachProductIssueClocks(ctx, r.db, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// digitsToInt parses a query made only of 1 to 9 digits.
func digitsToInt(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func (r *productIssueRepo) FindByGitHubIssue(ctx context.Context, owner, repository string, number int) (string, error) {
	ctx = WithSystemIdentity(ctx)
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT work_item_id::TEXT FROM github_issue_link
		 WHERE lower(owner) = lower($1) AND lower(repository) = lower($2) AND issue_number = $3`, owner, repository, number).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find github issue link: %w", err)
	}
	return id, nil
}

func (r *productIssueRepo) MappedRepository(ctx context.Context, owner, repository string) (string, string, bool, error) {
	ctx = WithSystemIdentity(ctx)
	var o, rp string
	err := r.db.QueryRow(ctx, `
		SELECT owner, repository FROM product_repo_mapping
		 WHERE lower(btrim(owner)) = lower($1) AND lower(btrim(repository)) = lower($2)
		 ORDER BY is_active DESC, created_on LIMIT 1`, strings.TrimSpace(owner), strings.TrimSpace(repository)).Scan(&o, &rp)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("find product repo mapping: %w", err)
	}
	return strings.TrimSpace(o), strings.TrimSpace(rp), true, nil
}

func (r *productIssueRepo) IssueBeingCreated(ctx context.Context, owner, repository string, number int) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var claimed bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM work_item_creation_request
		                WHERE status IN ('PENDING', 'GITHUB_CREATED')
		                  AND lower(github_owner) = lower($1) AND lower(github_repository) = lower($2) AND github_issue_number = $3)`,
		owner, repository, number).Scan(&claimed)
	if err != nil {
		return false, fmt.Errorf("check issue being created: %w", err)
	}
	return claimed, nil
}

func (r *productIssueRepo) LinkExisting(ctx context.Context, workItemID, caseID, actor string) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	if !validate.IsUUID(workItemID) {
		return false, &apierror.NotFoundError{Msg: "product issue not found"}
	}
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (bool, error) {
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM work_item wi JOIN product_issue pi ON pi.id = wi.id WHERE wi.id = $1::uuid AND wi.type = 'PRODUCT_ISSUE'`, workItemID).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, &apierror.NotFoundError{Msg: "product issue not found"}
		}
		if err != nil {
			return false, fmt.Errorf("check product issue: %w", err)
		}
		// Two passes: the active link can vanish (an unlink) between the insert
		// finding it taken and the read; the second insert then succeeds.
		for range 2 {
			var linkID string
			err = tx.QueryRow(ctx, `
				INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
				VALUES ($1::uuid, $2::uuid, 'AFFECTS', 'MANUAL', $3, $3)
				ON CONFLICT (from_id) WHERE kind = 'AFFECTS' AND is_active DO NOTHING
				RETURNING id::TEXT`, workItemID, caseID, actor).Scan(&linkID)
			if err == nil {
				return true, nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return false, fmt.Errorf("insert work item link: %w", err)
			}
			var toID, toNumber string
			err = tx.QueryRow(ctx, `
				SELECT l.to_id::TEXT, cs.number FROM work_item_link l JOIN work_item cs ON cs.id = l.to_id
				 WHERE l.from_id = $1::uuid AND l.kind = 'AFFECTS' AND l.is_active`, workItemID).Scan(&toID, &toNumber)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return false, fmt.Errorf("read active link: %w", err)
			}
			if toID == caseID {
				return false, nil
			}
			return false, &apierror.ConflictError{
				Msg: "This product issue is already linked to case " + toNumber + ".", Code: apierror.CodeAlreadyLinked}
		}
		return false, fmt.Errorf("link product issue: the active link kept changing")
	})
}

func (r *productIssueRepo) RegisterManualProductIssue(ctx context.Context, p ManualRegisterParams) (string, error) {
	if strings.TrimSpace(p.CaseID) == "" {
		return "", &apierror.ValidationError{Msg: "A case is required to register a product issue."}
	}
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (string, error) {
		id, _, err := insertProductIssueRows(ctx, tx, productIssueRows{
			CaseID: p.CaseID, Actor: p.Actor, Title: p.Title, Priority: p.Priority, Category: p.Category,
			OpenedByEmail: p.OpenedByEmail, ABTTeam: p.ABTTeam, PublicIssueURL: p.PublicIssueURL,
			Issue: p.Issue, State: p.State, StateReason: p.StateReason, Labels: p.Labels,
			GitHubCreatedOn: p.GitHubCreatedOn, GitHubUpdatedOn: p.GitHubUpdatedOn, GitHubClosedOn: p.GitHubClosedOn,
			// opened_by_email is read from the issue body, which anyone who can edit the
			// issue can change: stored for display, never resolved to a user id.
			LinkSource: "MANUAL", ResolveOpenedByUser: false,
		})
		return id, err
	})
}

func (r *productIssueRepo) Unlink(ctx context.Context, caseID, workItemID, actor string) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	if !validate.IsUUID(workItemID) || !validate.IsUUID(caseID) {
		return false, nil
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE work_item_link SET is_active = false, updated_on = NOW(), updated_by = $3
		 WHERE from_id = $1::uuid AND to_id = $2::uuid AND kind = 'AFFECTS' AND is_active`, workItemID, caseID, actor)
	if err != nil {
		return false, fmt.Errorf("unlink product issue: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
