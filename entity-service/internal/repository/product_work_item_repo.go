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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Creation request statuses and kinds (work_item_creation_request, migration 0304).
const (
	CreationStatusPending       = "PENDING"
	CreationStatusGitHubCreated = "GITHUB_CREATED"
	CreationStatusCompleted     = "COMPLETED"
	CreationStatusFailed        = "FAILED"
)

// ProductWorkItemCase is what creating a product work item needs to know about
// its case, read in one statement. The footer fields are computed here from the
// database, the same values the portal backend derived from GetCase.
type ProductWorkItemCase struct {
	ID         string
	Number     string
	InternalID string
	State      domain.CaseState
	ProjectID  *string
	AccountID  *string
	// ProductName is the deployed product's catalogue name, DisplayName its
	// versioned display name; both "" when the case has no deployed product.
	ProductName string
	DisplayName string
	// ProductSlug is "<product code>-<version>" when the case names a deployment
	// and its deployed product has both, else "".
	ProductSlug string
	// Team is the account's CRE team name, "" when the account has none (the
	// footer then says undefined, as it always has on Postgres).
	Team string
}

// CreationRequest is one work_item_creation_request row.
type CreationRequest struct {
	ID               string
	CaseID           string
	IdempotencyKey   string
	RequestHash      string
	Kind             string
	Status           string
	GitHubOwner      *string
	GitHubRepository *string
	GitHubNumber     *int
	GitHubNodeID     *string
	GitHubHTMLURL    *string
	WorkItemID       *string
	Response         json.RawMessage
	Error            *string
	CreatedOn        time.Time
	UpdatedOn        time.Time

	// Inserted is true when this call created the row.
	Inserted bool
	// RetryOfFailed is true when the row was FAILED (a definite failure) and this
	// call has claimed it for a retry.
	RetryOfFailed bool
	// InFlight is true when another attempt holds the row: it is PENDING and was
	// touched less than the lease ago (Lease when it has no recorded outcome,
	// AmbiguousLease when it carries an ambiguity note).
	InFlight bool
}

// BeginCreationParams are the arguments of BeginCreationRequest.
type BeginCreationParams struct {
	CaseID, Key, Hash, Kind, Actor string
	// Lease is how long a PENDING attempt without a recorded outcome counts as
	// in flight.
	Lease time.Duration
	// AmbiguousLease is the least time since its last write before a PENDING row
	// that carries an ambiguity note (Error set) may be resumed by another
	// attempt; shorter than that it counts as in flight. Zero disables the gate.
	AmbiguousLease time.Duration
}

// GitHubIssueRef is the GitHub issue a creation request recorded.
type GitHubIssueRef struct {
	Owner, Repository string
	Number            int
	NodeID, HTMLURL   string
}

// RegisterParams are the arguments of RegisterProductIssue and RegisterUntrackedPatch.
type RegisterParams struct {
	RequestID string
	CaseID    string
	CaseKey   string // idempotency key, for the advisory lock
	Actor     string
	// ActorEmail is the caller's email (comment author, opened_by_email).
	ActorEmail string

	Issue GitHubIssueRef
	// GitHubCreatedOn is the issue's created_at as GitHub reports it; nil when
	// the detail read failed.
	GitHubCreatedOn *time.Time

	// Product issue only.
	Title          string
	Priority       string
	Category       string
	ABTTeam        string
	PublicIssueURL string
	Labels         []string

	// Respond builds the stored and returned response. workItemID and number are
	// "" for an untracked patch.
	Respond func(workItemID, number string) any
}

// RegisterOutcome is what a registration or completion produced.
type RegisterOutcome struct {
	Response json.RawMessage
	// Replayed is true when the request was already COMPLETED (a concurrent or
	// earlier attempt won) and nothing was written; Response is then the stored one.
	Replayed bool
	// WorkItemID and Number name the registered product issue (RegisterProductIssue).
	WorkItemID, Number string
}

// ProductWorkItemRepository holds the database steps of POST
// /cases/{id}/product-work-items.
type ProductWorkItemRepository interface {
	LoadCase(ctx context.Context, caseID string) (ProductWorkItemCase, error)
	// BeginCreationRequest inserts the PENDING row (or finds the existing one)
	// and claims it for this attempt, in one short transaction under an advisory
	// lock on (case, key).
	BeginCreationRequest(ctx context.Context, p BeginCreationParams) (CreationRequest, error)
	// IssueClaimed reports whether a GitHub issue is already recorded against any
	// creation request or product issue, so adoption never takes another's.
	IssueClaimed(ctx context.Context, owner, repository string, number int) (bool, error)
	RecordGitHubCreated(ctx context.Context, requestID, actor string, ref GitHubIssueRef) error
	// RecordOutcome records a failure: definite true marks the request FAILED (the
	// issue certainly was not created); false leaves it PENDING with the note, so
	// the next attempt looks for the issue before creating again.
	RecordOutcome(ctx context.Context, requestID, actor, note string, definite bool) error
	// RegisterProductIssue writes the product issue, its GitHub link and its case
	// link in one transaction and records work_item_id on the request (status stays
	// GITHUB_CREATED). If the request already has a work item it writes nothing and
	// returns that one, so a retry never inserts a second.
	RegisterProductIssue(ctx context.Context, p RegisterParams) (RegisterOutcome, error)
	// CompleteRequest marks the request COMPLETED with the stored response, after the
	// case's work note (and patch tag) were written through the case service.
	CompleteRequest(ctx context.Context, p RegisterParams) (RegisterOutcome, error)
}

type productWorkItemRepo struct {
	db *Scoped
}

// NewProductWorkItemRepository returns the Postgres implementation.
func NewProductWorkItemRepository(db *Scoped) ProductWorkItemRepository {
	return &productWorkItemRepo{db: db}
}

// LoadCase implements ProductWorkItemRepository. Only a case-like work item
// answers, as GetCaseByID does; any other id is "case not found".
func (r *productWorkItemRepo) LoadCase(ctx context.Context, caseID string) (ProductWorkItemCase, error) {
	ctx = WithSystemIdentity(ctx)
	var c ProductWorkItemCase
	var internalID, state, prodName, dispName, slug, team *string
	err := r.db.QueryRow(ctx, `
		SELECT wi.id::TEXT, wi.number, wi.wso2_id, `+caseLikeStateColumn+`,
		       wi.project_id::TEXT, wi.account_id::TEXT,
		       prod.name, prod.name || COALESCE(' ' || pv.version, ''),
		       CASE WHEN wi.deployment_id IS NOT NULL
		             AND NULLIF(BTRIM(prod.code), '') IS NOT NULL AND NULLIF(BTRIM(pv.version), '') IS NOT NULL
		            THEN BTRIM(prod.code) || '-' || BTRIM(pv.version) END,
		       NULLIF(BTRIM(cre.name), '')
		FROM work_item wi
		LEFT JOIN "case" c ON c.id = wi.id
		`+caseLikeJoins+`
		LEFT JOIN account a ON a.id = wi.account_id
		LEFT JOIN "group" cre ON cre.id = a.cre_team_id
		LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
		LEFT JOIN product prod ON prod.id = dp.product_id
		LEFT JOIN product_version pv ON pv.id = dp.version_id
		WHERE wi.id = $1::uuid AND wi.type = ANY(`+caseLikeWorkItemTypes+`)`, caseID,
	).Scan(&c.ID, &c.Number, &internalID, &state, &c.ProjectID, &c.AccountID, &prodName, &dispName, &slug, &team)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductWorkItemCase{}, &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return ProductWorkItemCase{}, fmt.Errorf("load case for product work item: %w", err)
	}
	c.InternalID = stringOrEmpty(internalID)
	c.ProductName = strings.TrimSpace(stringOrEmpty(prodName))
	c.DisplayName = strings.TrimSpace(stringOrEmpty(dispName))
	c.ProductSlug = stringOrEmpty(slug)
	c.Team = stringOrEmpty(team)
	if state != nil {
		c.State = domain.CaseState(strings.ToLower(*state))
	}
	return c, nil
}

const creationRequestColumns = `id::TEXT, case_id::TEXT, idempotency_key, request_hash, kind, status,
	github_owner, github_repository, github_issue_number, github_node_id, github_html_url,
	work_item_id::TEXT, response, error, created_on, updated_on`

func scanCreationRequest(row pgx.Row) (CreationRequest, error) {
	var cr CreationRequest
	var resp []byte
	err := row.Scan(&cr.ID, &cr.CaseID, &cr.IdempotencyKey, &cr.RequestHash, &cr.Kind, &cr.Status,
		&cr.GitHubOwner, &cr.GitHubRepository, &cr.GitHubNumber, &cr.GitHubNodeID, &cr.GitHubHTMLURL,
		&cr.WorkItemID, &resp, &cr.Error, &cr.CreatedOn, &cr.UpdatedOn)
	if len(resp) > 0 {
		cr.Response = json.RawMessage(resp)
	}
	return cr, err
}

// lockCreationKey takes the transaction-scoped advisory lock that serialises the
// database steps of every attempt for one (case, key). It is held to the end of
// the transaction and never across the GitHub call.
func lockCreationKey(ctx context.Context, tx pgx.Tx, caseID, key string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`, caseID, key); err != nil {
		return fmt.Errorf("lock creation request: %w", err)
	}
	return nil
}

// BeginCreationRequest implements ProductWorkItemRepository.
func (r *productWorkItemRepo) BeginCreationRequest(ctx context.Context, p BeginCreationParams) (CreationRequest, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (CreationRequest, error) {
		if err := lockCreationKey(ctx, tx, p.CaseID, p.Key); err != nil {
			return CreationRequest{}, err
		}
		cr, err := scanCreationRequest(tx.QueryRow(ctx, `
			INSERT INTO work_item_creation_request (idempotency_key, case_id, request_hash, kind, created_by, updated_by)
			VALUES ($1, $2::uuid, $3, $4, $5, $5)
			ON CONFLICT (case_id, idempotency_key) DO NOTHING
			RETURNING `+creationRequestColumns, p.Key, p.CaseID, p.Hash, p.Kind, p.Actor))
		if err == nil {
			cr.Inserted = true
			return cr, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return CreationRequest{}, fmt.Errorf("insert creation request: %w", err)
		}
		// Fresh read under the lock: another attempt may have moved the row on.
		cr, err = scanCreationRequest(tx.QueryRow(ctx,
			`SELECT `+creationRequestColumns+` FROM work_item_creation_request WHERE case_id = $1::uuid AND idempotency_key = $2 FOR UPDATE`,
			p.CaseID, p.Key))
		if err != nil {
			return CreationRequest{}, fmt.Errorf("read creation request: %w", err)
		}
		if cr.RequestHash != p.Hash {
			return cr, nil // the caller answers 409 idempotency_key_reused; nothing is claimed
		}
		switch cr.Status {
		case CreationStatusPending:
			age := time.Since(cr.UpdatedOn)
			if (cr.Error == nil && age < p.Lease) || (cr.Error != nil && age < p.AmbiguousLease) {
				cr.InFlight = true
				return cr, nil
			}
		case CreationStatusFailed:
			// A definite failure: this attempt retries the create.
			cr.Status = CreationStatusPending
			cr.RetryOfFailed = true
		default:
			return cr, nil // GITHUB_CREATED / COMPLETED need no claim
		}
		// Claim: a fresh lease for this attempt and no recorded outcome.
		if _, err := tx.Exec(ctx, `
			UPDATE work_item_creation_request SET status = 'PENDING', error = NULL, updated_on = NOW(), updated_by = $2
			WHERE id = $1::uuid`, cr.ID, p.Actor); err != nil {
			return CreationRequest{}, fmt.Errorf("claim creation request: %w", err)
		}
		cr.Error = nil
		return cr, nil
	})
}

// IssueClaimed implements ProductWorkItemRepository.
func (r *productWorkItemRepo) IssueClaimed(ctx context.Context, owner, repository string, number int) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var claimed bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM github_issue_link
		                WHERE lower(owner) = lower($1) AND lower(repository) = lower($2) AND issue_number = $3)
		    OR EXISTS (SELECT 1 FROM work_item_creation_request
		                WHERE lower(github_owner) = lower($1) AND lower(github_repository) = lower($2) AND github_issue_number = $3)`,
		owner, repository, number).Scan(&claimed)
	if err != nil {
		return false, fmt.Errorf("check issue claimed: %w", err)
	}
	return claimed, nil
}

// RecordGitHubCreated implements ProductWorkItemRepository.
func (r *productWorkItemRepo) RecordGitHubCreated(ctx context.Context, requestID, actor string, ref GitHubIssueRef) error {
	ctx = WithSystemIdentity(ctx)
	tag, err := r.db.Exec(ctx, `
		UPDATE work_item_creation_request
		   SET status = 'GITHUB_CREATED', github_owner = $2, github_repository = $3, github_issue_number = $4,
		       github_node_id = NULLIF($5, ''), github_html_url = NULLIF($6, ''), error = NULL,
		       updated_on = NOW(), updated_by = $7
		 WHERE id = $1::uuid AND status IN ('PENDING', 'GITHUB_CREATED')`,
		requestID, ref.Owner, ref.Repository, ref.Number, ref.NodeID, ref.HTMLURL, actor)
	if err != nil {
		return fmt.Errorf("record github created: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("record github created: creation request %s is not pending", requestID)
	}
	return nil
}

// RecordOutcome implements ProductWorkItemRepository.
func (r *productWorkItemRepo) RecordOutcome(ctx context.Context, requestID, actor, note string, definite bool) error {
	ctx = WithSystemIdentity(ctx)
	status := CreationStatusPending
	if definite {
		status = CreationStatusFailed
	}
	if len(note) > 500 {
		note = note[:500]
	}
	_, err := r.db.Exec(ctx, `
		UPDATE work_item_creation_request SET status = $2, error = $3, updated_on = NOW(), updated_by = $4
		 WHERE id = $1::uuid AND status IN ('PENDING', 'FAILED')`, requestID, status, note, actor)
	if err != nil {
		return fmt.Errorf("record creation outcome: %w", err)
	}
	return nil
}

// lockedRequest re-reads the creation request under the advisory lock and
// reports a COMPLETED one so registration never runs twice. workItemID is the
// work item already registered for the request, "" if none.
func lockedRequest(ctx context.Context, tx pgx.Tx, p RegisterParams) (done *RegisterOutcome, workItemID string, err error) {
	if err := lockCreationKey(ctx, tx, p.CaseID, p.CaseKey); err != nil {
		return nil, "", err
	}
	var status string
	var resp []byte
	var wi *string
	err = tx.QueryRow(ctx, `SELECT status, response, work_item_id::text FROM work_item_creation_request WHERE id = $1::uuid FOR UPDATE`, p.RequestID).Scan(&status, &resp, &wi)
	if err != nil {
		return nil, "", fmt.Errorf("re-read creation request: %w", err)
	}
	if status == CreationStatusCompleted {
		return &RegisterOutcome{Response: json.RawMessage(resp), Replayed: true}, "", nil
	}
	if wi != nil {
		workItemID = *wi
	}
	return nil, workItemID, nil
}

// RegisterProductIssue implements ProductWorkItemRepository: the product issue, its
// GitHub link and its case link in one transaction, and work_item_id on the request
// (status stays GITHUB_CREATED until CompleteRequest). It never sets a
// work_item.github_* column: migration 0206's trigger would forward the case's
// comments to the product repository.
func (r *productWorkItemRepo) RegisterProductIssue(ctx context.Context, p RegisterParams) (RegisterOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (RegisterOutcome, error) {
		done, existing, err := lockedRequest(ctx, tx, p)
		if err != nil {
			return RegisterOutcome{}, err
		}
		if done != nil {
			return *done, nil
		}
		if existing != "" {
			var number string
			if err := tx.QueryRow(ctx, `SELECT number FROM work_item WHERE id = $1::uuid`, existing).Scan(&number); err != nil {
				return RegisterOutcome{}, fmt.Errorf("read registered work item: %w", err)
			}
			return RegisterOutcome{WorkItemID: existing, Number: number}, nil
		}
		workItemID, number, err := insertProductIssueRows(ctx, tx, productIssueRows{
			CaseID: p.CaseID, Actor: p.ActorEmail, Title: p.Title, Priority: p.Priority, Category: p.Category,
			OpenedByEmail: p.ActorEmail, ABTTeam: p.ABTTeam, PublicIssueURL: p.PublicIssueURL,
			Issue: p.Issue, Labels: p.Labels, GitHubCreatedOn: p.GitHubCreatedOn, LinkSource: "CREATION",
			ResolveOpenedByUser: true,
		})
		if errors.Is(err, ErrGitHubIssueAlreadyLinked) {
			return RegisterOutcome{}, &apierror.ConflictError{Msg: "This GitHub issue is already linked to another work item."}
		}
		if err != nil {
			return RegisterOutcome{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE work_item_creation_request
			   SET work_item_id = $2::uuid, github_node_id = COALESCE(NULLIF($3, ''), github_node_id),
			       github_html_url = COALESCE(NULLIF($4, ''), github_html_url), updated_on = NOW(), updated_by = $5
			 WHERE id = $1::uuid`, p.RequestID, workItemID, p.Issue.NodeID, p.Issue.HTMLURL, p.Actor); err != nil {
			return RegisterOutcome{}, fmt.Errorf("record work item on creation request: %w", err)
		}
		return RegisterOutcome{WorkItemID: workItemID, Number: number}, nil
	})
}

// CompleteRequest implements ProductWorkItemRepository.
func (r *productWorkItemRepo) CompleteRequest(ctx context.Context, p RegisterParams) (RegisterOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (RegisterOutcome, error) {
		done, existing, err := lockedRequest(ctx, tx, p)
		if err != nil {
			return RegisterOutcome{}, err
		}
		if done != nil {
			return *done, nil
		}
		var wi *string
		var number string
		if existing != "" {
			if err := tx.QueryRow(ctx, `SELECT number FROM work_item WHERE id = $1::uuid`, existing).Scan(&number); err != nil {
				return RegisterOutcome{}, fmt.Errorf("read registered work item: %w", err)
			}
			wi = &existing
		}
		response, err := json.Marshal(p.Respond(existing, number))
		if err != nil {
			return RegisterOutcome{}, fmt.Errorf("marshal response: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE work_item_creation_request
			   SET status = 'COMPLETED', work_item_id = $2::uuid, response = $3::jsonb, error = NULL,
			       github_owner = $4, github_repository = $5, github_issue_number = $6,
			       github_node_id = COALESCE(NULLIF($7, ''), github_node_id), github_html_url = COALESCE(NULLIF($8, ''), github_html_url),
			       updated_on = NOW(), updated_by = $9
			 WHERE id = $1::uuid`,
			p.RequestID, wi, response, p.Issue.Owner, p.Issue.Repository, p.Issue.Number, p.Issue.NodeID, p.Issue.HTMLURL, p.Actor); err != nil {
			return RegisterOutcome{}, fmt.Errorf("complete creation request: %w", err)
		}
		return RegisterOutcome{Response: response, WorkItemID: existing, Number: number}, nil
	})
}

// ErrGitHubIssueAlreadyLinked is returned by insertProductIssueRows when the
// GitHub issue (or its node id) already has a github_issue_link row.
var ErrGitHubIssueAlreadyLinked = errors.New("github issue already linked to a work item")

// productIssueRows is everything the shared product issue insert writes: the
// work_item copied from the case, product_issue, github_issue_link and the
// AFFECTS link to the case.
type productIssueRows struct {
	CaseID, Actor, Title, Priority, Category string
	OpenedByEmail, ABTTeam, PublicIssueURL   string
	Issue                                    GitHubIssueRef
	// State is OPEN or CLOSED; "" means OPEN.
	State, StateReason                               string
	Labels                                           []string
	GitHubCreatedOn, GitHubUpdatedOn, GitHubClosedOn *time.Time
	LinkSource                                       string // CREATION or MANUAL
	// ResolveOpenedByUser sets opened_by_user_id from OpenedByEmail. True only
	// where the email is the authenticated caller's (the create path); false
	// where it was read from a user-editable issue body (registered from GitHub).
	ResolveOpenedByUser bool
}

// insertProductIssueRows writes the four rows of a product issue inside tx and
// returns the new work item's id and PI number. It never sets a work_item.github_*
// column (migration 0206's trigger would push the case's comments to the
// repository). A github_issue_link unique violation (23505) is
// ErrGitHubIssueAlreadyLinked; the transaction is then unusable and the caller
// must return the error so it rolls back.
func insertProductIssueRows(ctx context.Context, tx pgx.Tx, in productIssueRows) (workItemID, number string, err error) {
	err = tx.QueryRow(ctx, `
		INSERT INTO work_item (created_on, updated_on, created_by, updated_by, number, subject, type, account_id, project_id)
		SELECT NOW(), NOW(), $2, $2, next_work_item_number('PRODUCT_ISSUE'), $3, 'PRODUCT_ISSUE', c.account_id, c.project_id
		  FROM work_item c WHERE c.id = $1::uuid
		RETURNING id::TEXT, number`, in.CaseID, in.Actor, in.Title).Scan(&workItemID, &number)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", &apierror.NotFoundError{Msg: "case not found"}
	}
	if err != nil {
		return "", "", fmt.Errorf("insert product issue work item: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_issue (id, priority, category, opened_by_user_id, opened_by_email, abt_team, public_issue_url)
		VALUES ($1::uuid, NULLIF($2, ''), NULLIF($3, ''),
		        CASE WHEN $7::boolean THEN (SELECT u.id FROM "user" u WHERE LOWER(u.email) = LOWER($4) ORDER BY u.id LIMIT 1) END,
		        NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))`,
		workItemID, in.Priority, in.Category, in.OpenedByEmail, in.ABTTeam, in.PublicIssueURL, in.ResolveOpenedByUser); err != nil {
		return "", "", fmt.Errorf("insert product issue: %w", err)
	}
	if in.Labels == nil {
		in.Labels = []string{} // labels must be a JSON array, never null
	}
	labels, err := json.Marshal(in.Labels)
	if err != nil {
		return "", "", fmt.Errorf("marshal labels: %w", err)
	}
	state := in.State
	if state == "" {
		state = "OPEN"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO github_issue_link (work_item_id, owner, repository, issue_number, node_id, html_url, github_state, state_reason, labels,
		                               github_created_on, github_updated_on, github_closed_on)
		VALUES ($1::uuid, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, NULLIF($8, ''), $9::jsonb, $10, $11, $12)`,
		workItemID, in.Issue.Owner, in.Issue.Repository, in.Issue.Number, in.Issue.NodeID, in.Issue.HTMLURL, state, in.StateReason,
		labels, in.GitHubCreatedOn, in.GitHubUpdatedOn, in.GitHubClosedOn); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", "", ErrGitHubIssueAlreadyLinked
		}
		return "", "", fmt.Errorf("insert github issue link: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
		VALUES ($1::uuid, $2::uuid, 'AFFECTS', $4, $3, $3)`, workItemID, in.CaseID, in.Actor, in.LinkSource); err != nil {
		return "", "", fmt.Errorf("insert work item link: %w", err)
	}
	return workItemID, number, nil
}
