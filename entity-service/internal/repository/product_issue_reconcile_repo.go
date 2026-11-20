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
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Reconciliation and backfill of product issues (git internals): the database
// half of POST /product-work-items/reconcile and /backfill. They reuse the
// inbound webhook branch's building blocks (InsertStatusEvent, InsertGitHubComment,
// ResolveBoardStatus, lockLink, insertProductIssueRows) with the sources
// RECONCILE and BACKFILL, so the rows are the ones a webhook would have written.
// Every write runs under the system identity, one transaction per issue, and a
// pass that finds nothing to change writes only the link's last_synced_on.

// statusEventTolerance is how far apart two board events of the same issue,
// column and kind may be and still count as one move: GitHub stamps the same move
// a second apart in a webhook and in the timeline history, so events keyed by
// different ids (a delivery id, a timeline node id) would otherwise be recorded
// twice.
const statusEventTolerance = time.Minute

// BoardCurrentMode says when BoardSync moves the link's current board status.
type BoardCurrentMode int

const (
	// BoardCurrentIfDifferent sets the link's board columns whenever the board's
	// current column (or project number) differs from what is stored.
	BoardCurrentIfDifferent BoardCurrentMode = iota
	// BoardCurrentIfUnset sets them only while the link has no board status yet,
	// so a fill never overrides what a webhook or reconciliation wrote.
	BoardCurrentIfUnset
)

// BoardHistoryChange is one Status change from the board's timeline.
type BoardHistoryChange struct {
	// EventKey is the dedupe key: the timeline event's node id.
	EventKey string
	// FromRaw is the column before ("" for the first placement), ToRaw after.
	FromRaw, ToRaw string
	OccurredAt     time.Time
	Actor          string
}

// BoardSync is what one board says about one issue.
type BoardSync struct {
	ProjectNumber int
	// CurrentRaw is the issue's current Status column; "" when it has none.
	CurrentRaw string
	// Changes are recorded as events (deduped by key and by tolerance).
	Changes []BoardHistoryChange
	Source  string
	Mode    BoardCurrentMode
	// Synthetic records one event stamped Now (key reconcile:<work item>:<to>:<UTC
	// date>) when the link's current column is changed and no recorded board event
	// ends in that column: the change happened but the history does not show it.
	Synthetic bool
	Now       time.Time
}

// ReconcileParams are one linked issue's refresh.
type ReconcileParams struct {
	WorkItemID string
	// Issue refreshes title, labels, priority, category and state; nil leaves them.
	Issue *IssueSnapshot
	// Board brings the board status up to date; nil leaves it.
	Board *BoardSync
}

// ReconcileResult says what a refresh wrote.
type ReconcileResult struct {
	// Changed is true when any field of the issue, link or board status moved.
	Changed bool
	// Stale is true when the issue read was older than what is stored and its
	// issue fields were left alone.
	Stale         bool
	EventsWritten int
}

// ReconcileBoard is one tracking board: its organisation and project number.
type ReconcileBoard struct {
	Owner         string
	ProjectNumber int
}

// BackfillCreateParams register one GitHub issue that has no work item yet.
type BackfillCreateParams struct {
	// CaseID is the matched case. Unmatched registers the issue with no case (no
	// account, project or AFFECTS link) and requires CaseID to be "".
	CaseID    string
	Unmatched bool
	Actor     string

	Title, Priority, Category, OpenedByEmail, ABTTeam, PublicIssueURL string
	Issue                                                             GitHubIssueRef
	State, StateReason                                                string
	Labels                                                            []string
	CreatedOn, UpdatedOn, ClosedOn                                    *time.Time

	// Comments are every comment of the issue (WorkItemID is filled in here).
	Comments []GitHubCommentInput
	// Board is the issue's board history on its product's tracking board, or nil.
	Board *BoardSync
}

// BackfillCreateResult is what registering an issue wrote.
type BackfillCreateResult struct {
	WorkItemID, Number string
	// AlreadyLinked is true when another writer linked the issue first; nothing
	// was written.
	AlreadyLinked    bool
	CommentsInserted int
	EventsWritten    int
}

// BackfillFillResult is what filling a known issue's gaps wrote.
type BackfillFillResult struct {
	CommentsInserted int
	EventsWritten    int
}

// ProductIssueReconcileRepository is the database side of reconciliation and
// backfill. It also carries the link lookups of the webhook branch.
type ProductIssueReconcileRepository interface {
	LinkByIssue(ctx context.Context, owner, repository string, number int) (*LinkedIssue, error)
	LinkByNodeID(ctx context.Context, nodeID string) (*LinkedIssue, error)
	IsTrackingBoard(ctx context.Context, owner, repository string, projectNumber int) (bool, error)
	SetLinkState(ctx context.Context, workItemID, state string) error

	// DatabaseNow is the database's clock. A reconciliation run stamps its start from
	// it: links are marked synced with the database's NOW(), so the two must share a
	// clock (the application's may differ by seconds).
	DatabaseNow(ctx context.Context) (time.Time, error)
	// ListBoards returns the distinct (owner, board_project_number) pairs of
	// product_repo_mapping, rows without a board skipped, ordered by owner
	// (case-insensitive) then number.
	ListBoards(ctx context.Context) ([]ReconcileBoard, error)
	// ListLinksNotSeen returns ACTIVE links of repositories tracked on the board
	// that no pass has synced since startedBefore, after the work item id afterID,
	// by id. A link that sits on another board of a shared repository is left out,
	// and a CLOSED link is only returned when it was last synced over a week before
	// startedBefore.
	ListLinksNotSeen(ctx context.Context, owner string, board int, startedBefore time.Time, afterID string, limit int) ([]LinkedIssue, error)
	// ReconcileLink refreshes one linked issue in one transaction.
	ReconcileLink(ctx context.Context, p ReconcileParams) (ReconcileResult, error)

	// MappedRepository returns the owner and repository as spelled by the first
	// matching product_repo_mapping row, and whether one exists.
	MappedRepository(ctx context.Context, owner, repository string) (owner2, repository2 string, ok bool, err error)
	// BoardsForRepository returns the tracking board numbers of the repository's mapping rows.
	BoardsForRepository(ctx context.Context, owner, repository string) ([]int, error)
	// CaseIDByWSO2ID and CaseIDByNumber resolve a case-like work item; "" when
	// there is none or the value is ambiguous.
	CaseIDByWSO2ID(ctx context.Context, wso2ID string) (string, error)
	CaseIDByNumber(ctx context.Context, number string) (string, error)
	// BackfillCreate registers an issue, its comments and its board history in one transaction.
	BackfillCreate(ctx context.Context, p BackfillCreateParams) (BackfillCreateResult, error)
	// HasActiveCaseLink reports whether the product issue has an active AFFECTS link.
	HasActiveCaseLink(ctx context.Context, workItemID string) (bool, error)
	// BackfillAttachCase gives an unmatched product issue its case: an AFFECTS link
	// (source BACKFILL) and the case's account and project on the work item, as the
	// create path writes them. It does nothing (false) when the issue got an active
	// link meanwhile. The other rows of a product issue carry neither column.
	BackfillAttachCase(ctx context.Context, workItemID, caseID, actor string) (bool, error)
	// BackfillFill adds the comments and board events a known issue lacks.
	BackfillFill(ctx context.Context, workItemID string, comments []GitHubCommentInput, board *BoardSync) (BackfillFillResult, error)
}

type productIssueReconcileRepo struct {
	*productIssueSyncRepo
}

// NewProductIssueReconcileRepository returns the Postgres implementation.
func NewProductIssueReconcileRepository(db *Scoped) ProductIssueReconcileRepository {
	return &productIssueReconcileRepo{productIssueSyncRepo: &productIssueSyncRepo{db: db}}
}

// DatabaseNow implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) DatabaseNow(ctx context.Context) (time.Time, error) {
	ctx = WithSystemIdentity(ctx)
	var now time.Time
	if err := r.db.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return time.Time{}, fmt.Errorf("read database time: %w", err)
	}
	return now.UTC(), nil
}

// ListBoards implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) ListBoards(ctx context.Context) ([]ReconcileBoard, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT MIN(btrim(owner)), board_project_number
		  FROM product_repo_mapping
		 WHERE board_project_number IS NOT NULL
		 GROUP BY lower(btrim(owner)), board_project_number
		 ORDER BY lower(btrim(owner)), board_project_number`)
	if err != nil {
		return nil, fmt.Errorf("list tracking boards: %w", err)
	}
	defer rows.Close()
	var out []ReconcileBoard
	for rows.Next() {
		var b ReconcileBoard
		if err := rows.Scan(&b.Owner, &b.ProjectNumber); err != nil {
			return nil, fmt.Errorf("scan tracking board: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListLinksNotSeen implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) ListLinksNotSeen(ctx context.Context, owner string, board int, startedBefore time.Time, afterID string, limit int) ([]LinkedIssue, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT l.work_item_id::TEXT, l.owner, l.repository, l.issue_number, l.node_id, l.link_state, l.github_state,
		       l.board_status, l.board_status_raw, l.board_project_number, l.github_updated_on
		  FROM github_issue_link l
		 WHERE l.link_state = 'ACTIVE'
		   AND lower(l.owner) = lower($1)
		   AND EXISTS (SELECT 1 FROM product_repo_mapping m
		                WHERE lower(m.owner) = lower(l.owner) AND lower(m.repository) = lower(l.repository)
		                  AND m.board_project_number = $2)
		   AND (l.board_project_number IS NULL OR l.board_project_number = $2)
		   AND (l.last_synced_on IS NULL OR l.last_synced_on < $3)
		   AND (l.github_state = 'OPEN' OR l.last_synced_on IS NULL OR l.last_synced_on < $3 - interval '7 days')
		   AND ($4 = '' OR l.work_item_id > $4::uuid)
		 ORDER BY l.work_item_id
		 LIMIT $5`, strings.TrimSpace(owner), board, startedBefore, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list links not seen on the board: %w", err)
	}
	defer rows.Close()
	var out []LinkedIssue
	for rows.Next() {
		var l LinkedIssue
		if err := rows.Scan(&l.WorkItemID, &l.Owner, &l.Repository, &l.IssueNumber, &l.NodeID, &l.LinkState, &l.GitHubState,
			&l.BoardStatus, &l.BoardStatusRaw, &l.BoardProjectNumber, &l.GitHubUpdatedOn); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ReconcileLink implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) ReconcileLink(ctx context.Context, p ReconcileParams) (ReconcileResult, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (ReconcileResult, error) {
		var out ReconcileResult
		if _, _, _, err := lockLink(ctx, tx, p.WorkItemID); err != nil {
			return out, err
		}
		var linkState string
		if err := tx.QueryRow(ctx, `SELECT link_state FROM github_issue_link WHERE work_item_id = $1::uuid`, p.WorkItemID).Scan(&linkState); err != nil {
			return out, fmt.Errorf("read link state: %w", err)
		}
		if linkState != LinkStateActive {
			return out, nil
		}
		if p.Issue != nil {
			changed, stale, err := reconcileIssueInTx(ctx, tx, p.WorkItemID, *p.Issue)
			if err != nil {
				return out, err
			}
			out.Changed, out.Stale = changed, stale
		}
		if p.Board != nil {
			events, changed, err := applyBoardSync(ctx, tx, p.WorkItemID, *p.Board)
			if err != nil {
				return out, err
			}
			out.EventsWritten = events
			out.Changed = out.Changed || changed || events > 0
		}
		// Seen by this pass, changed or not: what ListLinksNotSeen leaves out.
		if _, err := tx.Exec(ctx, `UPDATE github_issue_link SET last_synced_on = NOW() WHERE work_item_id = $1::uuid`, p.WorkItemID); err != nil {
			return out, fmt.Errorf("mark github issue link synced: %w", err)
		}
		return out, nil
	})
}

// reconcileIssueInTx brings the issue fields of a locked link up to s, writing
// only what differs. An issue read older than the stored update time is stale.
func reconcileIssueInTx(ctx context.Context, tx pgx.Tx, workItemID string, s IssueSnapshot) (changed, stale bool, err error) {
	var prevState string
	var storedUpdated *time.Time
	if err = tx.QueryRow(ctx, `SELECT github_state, github_updated_on FROM github_issue_link WHERE work_item_id = $1::uuid`, workItemID).
		Scan(&prevState, &storedUpdated); err != nil {
		return false, false, fmt.Errorf("read github issue link: %w", err)
	}
	if s.UpdatedOn != nil && storedUpdated != nil && s.UpdatedOn.Before(*storedUpdated) {
		return false, true, nil
	}
	if s.Title != "" {
		tag, err := tx.Exec(ctx, `
			UPDATE work_item SET subject = $2, updated_on = NOW(), updated_by = $3
			 WHERE id = $1::uuid AND subject IS DISTINCT FROM $2`, workItemID, s.Title, githubSyncActor)
		if err != nil {
			return false, false, fmt.Errorf("update product issue subject: %w", err)
		}
		changed = changed || tag.RowsAffected() > 0
	}
	labels := s.Labels
	if labels == nil {
		labels = []string{}
	}
	labelsJSON, err := marshalLabels(labels)
	if err != nil {
		return false, false, err
	}
	state := s.State
	if state != "OPEN" && state != "CLOSED" {
		state = prevState
	}
	tag, err := tx.Exec(ctx, `
		UPDATE github_issue_link l
		   SET labels = n.labels, github_state = n.state, state_reason = n.reason, github_closed_on = n.closed,
		       github_updated_on = n.updated, last_synced_on = NOW(), updated_on = NOW()
		  FROM (SELECT $2::jsonb AS labels, $3::text AS state, NULLIF($4::text, '') AS reason,
		               CASE WHEN $3::text = 'OPEN' THEN NULL::timestamptz
		                    ELSE COALESCE($5::timestamptz, (SELECT github_closed_on FROM github_issue_link WHERE work_item_id = $1::uuid)) END AS closed,
		               COALESCE($6::timestamptz, (SELECT github_updated_on FROM github_issue_link WHERE work_item_id = $1::uuid)) AS updated) n
		 WHERE l.work_item_id = $1::uuid
		   AND (l.labels IS DISTINCT FROM n.labels OR l.github_state IS DISTINCT FROM n.state
		        OR l.state_reason IS DISTINCT FROM n.reason OR l.github_closed_on IS DISTINCT FROM n.closed
		        OR l.github_updated_on IS DISTINCT FROM n.updated)`,
		workItemID, labelsJSON, state, s.StateReason, s.ClosedOn, s.UpdatedOn)
	if err != nil {
		return false, false, fmt.Errorf("update github issue link: %w", err)
	}
	changed = changed || tag.RowsAffected() > 0
	tag, err = tx.Exec(ctx, `
		UPDATE product_issue SET priority = NULLIF($2, ''), category = NULLIF($3, '')
		 WHERE id = $1::uuid AND (priority IS DISTINCT FROM NULLIF($2, '') OR category IS DISTINCT FROM NULLIF($3, ''))`,
		workItemID, truncateColumn(s.Priority, 50), truncateColumn(s.Category, 100))
	if err != nil {
		return false, false, fmt.Errorf("update product issue labels: %w", err)
	}
	return changed || tag.RowsAffected() > 0, false, nil
}

// boardEventRecorded reports whether the issue already has an event for the
// same column within statusEventTolerance of at: the same move recorded under
// another source's key.
func boardEventRecorded(ctx context.Context, tx pgx.Tx, workItemID, raw string, at time.Time) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM work_item_status_event
		                WHERE work_item_id = $1::uuid AND raw_status IS NOT NULL
		                  AND lower(btrim(raw_status)) = lower(btrim($2))
		                  AND occurred_at BETWEEN $3::timestamptz - make_interval(secs => $4::float8)
		                                      AND $3::timestamptz + make_interval(secs => $4::float8))`,
		workItemID, raw, at, statusEventTolerance.Seconds()).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check recorded board event: %w", err)
	}
	return ok, nil
}

// applyBoardSync records the board's changes as status events and, per b.Mode,
// moves the link's current board status. The link row must be locked by the caller.
func applyBoardSync(ctx context.Context, tx pgx.Tx, workItemID string, b BoardSync) (events int, linkChanged bool, err error) {
	var storedRaw, storedStatus *string
	var storedProject *int
	if err = tx.QueryRow(ctx, `SELECT board_status_raw, board_status, board_project_number FROM github_issue_link WHERE work_item_id = $1::uuid`, workItemID).
		Scan(&storedRaw, &storedStatus, &storedProject); err != nil {
		return 0, false, fmt.Errorf("read link board status: %w", err)
	}
	changes := append([]BoardHistoryChange(nil), b.Changes...)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].OccurredAt.Before(changes[j].OccurredAt) })
	for _, ch := range changes {
		to := strings.TrimSpace(ch.ToRaw)
		if to == "" || strings.TrimSpace(ch.EventKey) == "" {
			continue // a cleared Status is not a column; a change with no key cannot be deduped
		}
		if seen, err := boardEventRecorded(ctx, tx, workItemID, to, ch.OccurredAt); err != nil {
			return events, false, err
		} else if seen {
			continue
		}
		toStatus, err := ResolveBoardStatus(ctx, tx, to)
		if err != nil {
			return events, false, err
		}
		var from *string
		if prev := strings.TrimSpace(ch.FromRaw); prev != "" {
			fromStatus, err := ResolveBoardStatus(ctx, tx, prev)
			if err != nil {
				return events, false, err
			}
			from = &fromStatus.Status
		}
		inserted, err := InsertStatusEvent(ctx, tx, StatusEventInput{
			WorkItemID: workItemID, From: from, To: toStatus.Status, WaitsOn: toStatus.WaitsOn, OccurredAt: ch.OccurredAt,
			Actor: ch.Actor, Source: b.Source, DedupeKey: ch.EventKey, RawStatus: to,
		})
		if err != nil {
			return events, false, err
		}
		if inserted {
			events++
		}
	}

	current := strings.TrimSpace(b.CurrentRaw)
	if current == "" {
		return events, false, nil
	}
	differs := storedRaw == nil || !strings.EqualFold(strings.TrimSpace(*storedRaw), current) ||
		storedProject == nil || *storedProject != b.ProjectNumber
	if b.Mode == BoardCurrentIfUnset {
		differs = storedRaw == nil
	}
	if !differs {
		return events, false, nil
	}
	mapped, err := ResolveBoardStatus(ctx, tx, current)
	if err != nil {
		return events, false, err
	}
	if b.Synthetic {
		var latest *string
		err := tx.QueryRow(ctx, `
			SELECT raw_status FROM work_item_status_event
			 WHERE work_item_id = $1::uuid AND raw_status IS NOT NULL
			 ORDER BY occurred_at DESC, created_on DESC LIMIT 1`, workItemID).Scan(&latest)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return events, false, fmt.Errorf("read latest board event: %w", err)
		}
		if latest == nil || !strings.EqualFold(strings.TrimSpace(*latest), current) {
			inserted, err := InsertStatusEvent(ctx, tx, StatusEventInput{
				WorkItemID: workItemID, From: storedStatus, To: mapped.Status, WaitsOn: mapped.WaitsOn, OccurredAt: b.Now.UTC(),
				Source: b.Source, RawStatus: current,
				DedupeKey: fmt.Sprintf("reconcile:%s:%s:%s", workItemID, mapped.Status, b.Now.UTC().Format("2006-01-02")),
			})
			if err != nil {
				return events, false, err
			}
			if inserted {
				events++
			}
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE github_issue_link
		   SET board_status_raw = $2, board_status = $3, board_project_number = $4, last_synced_on = NOW(), updated_on = NOW()
		 WHERE work_item_id = $1::uuid`, workItemID, current, mapped.Status, b.ProjectNumber); err != nil {
		return events, false, fmt.Errorf("update github issue link board status: %w", err)
	}
	return events, true, nil
}

// MappedRepository implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) MappedRepository(ctx context.Context, owner, repository string) (string, string, bool, error) {
	ctx = WithSystemIdentity(ctx)
	var o, rp string
	err := r.db.QueryRow(ctx, `
		SELECT btrim(owner), btrim(repository) FROM product_repo_mapping
		 WHERE lower(btrim(owner)) = lower($1) AND lower(btrim(repository)) = lower($2)
		 ORDER BY is_active DESC, created_on LIMIT 1`, strings.TrimSpace(owner), strings.TrimSpace(repository)).Scan(&o, &rp)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("find product repo mapping: %w", err)
	}
	return o, rp, true, nil
}

// BoardsForRepository implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) BoardsForRepository(ctx context.Context, owner, repository string) ([]int, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT board_project_number FROM product_repo_mapping
		 WHERE board_project_number IS NOT NULL
		   AND lower(btrim(owner)) = lower($1) AND lower(btrim(repository)) = lower($2)
		 ORDER BY board_project_number`, strings.TrimSpace(owner), strings.TrimSpace(repository))
	if err != nil {
		return nil, fmt.Errorf("list boards of repository: %w", err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan board number: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *productIssueReconcileRepo) caseID(ctx context.Context, column, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `SELECT id::TEXT FROM work_item WHERE `+column+` = $1 AND type = ANY(`+caseLikeWorkItemTypes+`) LIMIT 2`, value)
	if err != nil {
		return "", fmt.Errorf("find case by %s: %w", column, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", fmt.Errorf("scan case id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(ids) != 1 {
		return "", nil // none, or ambiguous: not a match
	}
	return ids[0], nil
}

// CaseIDByWSO2ID implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) CaseIDByWSO2ID(ctx context.Context, wso2ID string) (string, error) {
	return r.caseID(ctx, "wso2_id", wso2ID)
}

// CaseIDByNumber implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) CaseIDByNumber(ctx context.Context, number string) (string, error) {
	return r.caseID(ctx, "number", number)
}

// BackfillCreate implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) BackfillCreate(ctx context.Context, p BackfillCreateParams) (BackfillCreateResult, error) {
	ctx = WithSystemIdentity(ctx)
	res, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (BackfillCreateResult, error) {
		var out BackfillCreateResult
		id, number, err := insertProductIssueRows(ctx, tx, productIssueRows{
			CaseID: p.CaseID, Unmatched: p.Unmatched, Actor: p.Actor, Title: p.Title, Priority: p.Priority, Category: p.Category,
			OpenedByEmail: p.OpenedByEmail, ABTTeam: p.ABTTeam, PublicIssueURL: p.PublicIssueURL,
			Issue: p.Issue, State: p.State, StateReason: p.StateReason, Labels: p.Labels,
			GitHubCreatedOn: p.CreatedOn, GitHubUpdatedOn: p.UpdatedOn, GitHubClosedOn: p.ClosedOn,
			// The footer's email is read from a user-editable body: stored for display,
			// never resolved to a user id (as the manual link does).
			LinkSource: "BACKFILL", ResolveOpenedByUser: false,
		})
		if err != nil {
			return out, err
		}
		out.WorkItemID, out.Number = id, number
		for _, c := range p.Comments {
			c.WorkItemID = id
			_, inserted, err := InsertGitHubComment(ctx, tx, c)
			if err != nil {
				return out, err
			}
			if inserted {
				out.CommentsInserted++
			}
		}
		if p.Board != nil {
			b := *p.Board
			b.Mode, b.Synthetic = BoardCurrentIfUnset, false
			events, _, err := applyBoardSync(ctx, tx, id, b)
			if err != nil {
				return out, err
			}
			out.EventsWritten = events
		}
		return out, nil
	})
	if errors.Is(err, ErrGitHubIssueAlreadyLinked) {
		return BackfillCreateResult{AlreadyLinked: true}, nil
	}
	return res, err
}

// HasActiveCaseLink implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) HasActiveCaseLink(ctx context.Context, workItemID string) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var ok bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM work_item_link WHERE from_id = $1::uuid AND kind = 'AFFECTS' AND is_active)`, workItemID).Scan(&ok); err != nil {
		return false, fmt.Errorf("check active case link: %w", err)
	}
	return ok, nil
}

// BackfillAttachCase implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) BackfillAttachCase(ctx context.Context, workItemID, caseID, actor string) (bool, error) {
	if strings.TrimSpace(caseID) == "" {
		return false, errProductIssueCaseRequired
	}
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (bool, error) {
		if _, _, _, err := lockLink(ctx, tx, workItemID); err != nil {
			return false, err
		}
		var linkID string
		err := tx.QueryRow(ctx, `
			INSERT INTO work_item_link (from_id, to_id, kind, source, created_by, updated_by)
			VALUES ($1::uuid, $2::uuid, 'AFFECTS', 'BACKFILL', $3, $3)
			ON CONFLICT (from_id) WHERE kind = 'AFFECTS' AND is_active DO NOTHING
			RETURNING id::TEXT`, workItemID, caseID, actor).Scan(&linkID)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("insert work item link: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE work_item wi SET account_id = c.account_id, project_id = c.project_id, updated_on = NOW(), updated_by = $3
			  FROM work_item c WHERE wi.id = $1::uuid AND c.id = $2::uuid`, workItemID, caseID, actor); err != nil {
			return false, fmt.Errorf("copy account and project from the case: %w", err)
		}
		return true, nil
	})
}

// BackfillFill implements ProductIssueReconcileRepository.
func (r *productIssueReconcileRepo) BackfillFill(ctx context.Context, workItemID string, comments []GitHubCommentInput, board *BoardSync) (BackfillFillResult, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (BackfillFillResult, error) {
		var out BackfillFillResult
		if _, _, _, err := lockLink(ctx, tx, workItemID); err != nil {
			return out, err
		}
		for _, c := range comments {
			c.WorkItemID = workItemID
			_, inserted, err := InsertGitHubComment(ctx, tx, c)
			if err != nil {
				return out, err
			}
			if inserted {
				out.CommentsInserted++
			}
		}
		if board != nil {
			b := *board
			b.Mode, b.Synthetic = BoardCurrentIfUnset, false
			events, _, err := applyBoardSync(ctx, tx, workItemID, b)
			if err != nil {
				return out, err
			}
			out.EventsWritten = events
		}
		return out, nil
	})
}

func marshalLabels(labels []string) ([]byte, error) {
	b, err := json.Marshal(labels)
	if err != nil {
		return nil, fmt.Errorf("marshal labels: %w", err)
	}
	return b, nil
}
