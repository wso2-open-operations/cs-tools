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
)

// The inbound half of the product issue sync: what a GitHub webhook (and, later,
// reconciliation and backfill) writes to a product issue's GitHub-side rows.
// Every write runs under the system identity in ONE transaction and never touches
// work_item.github_* (migration 0206's trigger would push the comments of a work
// item that has them to a customer repository).
//
// InsertStatusEvent, InsertGitHubComment and ResolveBoardStatus take the caller's
// transaction and carry no webhook assumptions (source and dedupe key are
// arguments), so reconciliation and backfill reuse them with their own.

// Statuses and sources of work_item_status_event (migration 0304).
const (
	StatusOther             = "OTHER"
	StatusClosed            = "CLOSED"
	StatusReopened          = "REOPENED"
	WaitsOnProductTeam      = "PRODUCT_TEAM"
	WaitsOnNobody           = "NOBODY"
	StatusSourceWebhook     = "WEBHOOK"
	StatusSourceReconcile   = "RECONCILE"
	StatusSourceBackfill    = "BACKFILL"
	CommentOriginGitHub     = "GITHUB"
	CommentOriginPortal     = "PORTAL"
	LinkStateActive         = "ACTIVE"
	LinkStateDeleted        = "DELETED"
	LinkStateInaccessible   = "INACCESSIBLE"
	LinkStateTransferred    = "TRANSFERRED"
	githubCommentNoteType   = "WORK_NOTE"
	githubCommentAuthorSize = 255
)

// ErrProductIssueLinkNotFound means the work item has no github_issue_link row.
var ErrProductIssueLinkNotFound = errors.New("product issue: no github issue link for the work item")

// DeliveryClaim is the webhook delivery a write belongs to.
type DeliveryClaim struct{ ID, Event, Action string }

type deliveryClaimKey struct{}

// WithDeliveryClaim makes every write method below insert the delivery's
// github_webhook_delivery row (the table and semantics of
// GithubSyncRepository.ClaimDelivery) as the first statement of its own
// transaction. The claim then commits or rolls back with the write: a failed or
// interrupted write leaves no claim, so GitHub's retry is processed, and a
// replay of a committed delivery gets ErrDeliverySeen.
func WithDeliveryClaim(ctx context.Context, c DeliveryClaim) context.Context {
	return context.WithValue(ctx, deliveryClaimKey{}, c)
}

// claimDeliveryInTx inserts the claim carried by ctx, if any.
func claimDeliveryInTx(ctx context.Context, tx pgx.Tx) error {
	c, ok := ctx.Value(deliveryClaimKey{}).(DeliveryClaim)
	if !ok {
		return nil
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO github_webhook_delivery (delivery_id, event, action)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (delivery_id) DO NOTHING`, c.ID, c.Event, c.Action)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDeliverySeen
		}
		return fmt.Errorf("github: claim delivery %s: %w", c.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrDeliverySeen
	}
	return nil
}

// LinkedIssue is the github_issue_link row a delivery was matched to.
type LinkedIssue struct {
	WorkItemID        string
	Owner, Repository string
	IssueNumber       int
	NodeID            *string
	LinkState         string
	GitHubState       string
	BoardStatus       *string
	// BoardStatusRaw is the column name as GitHub spells it.
	BoardStatusRaw     *string
	BoardProjectNumber *int
	GitHubUpdatedOn    *time.Time
}

// IssueSnapshot is the issue as an issues delivery reports it.
type IssueSnapshot struct {
	WorkItemID  string
	Title       string // already cut to the work_item.subject column; "" leaves the subject alone
	State       string // OPEN or CLOSED
	StateReason string
	Labels      []string
	// Priority and Category are derived from Labels by the caller; "" clears them.
	Priority, Category string
	UpdatedOn          *time.Time
	ClosedOn           *time.Time

	// RecordStateEvent (a closed or reopened delivery) records a status event when
	// the open/closed state changed, keyed by DedupeKey.
	RecordStateEvent bool
	DedupeKey, Actor string
	OccurredAt       time.Time
}

// SnapshotOutcome says what ApplyIssueSnapshot did.
type SnapshotOutcome struct {
	// Stale is true when the delivery is older than what is stored: only
	// last_synced_on moved.
	Stale        bool
	StateChanged bool
	EventWritten bool
}

// GitHubCommentInput is one GitHub issue comment to mirror.
type GitHubCommentInput struct {
	WorkItemID      string
	GitHubCommentID int64
	Author          string // GitHub login
	// Editor is the GitHub login that made an edit (the delivery's sender); it
	// falls back to Author.
	Editor    string
	Body      string
	CreatedOn time.Time
	UpdatedOn *time.Time
	Origin    string // COMMENT_ORIGIN_*; "" means GITHUB
	// IssueUpdatedOn moves github_updated_on forward (never back) when set.
	IssueUpdatedOn *time.Time
}

// CommentOutcome says what a comment write did.
type CommentOutcome struct {
	CommentID string
	// Inserted is true when a new comment row was written, Updated when an
	// existing one changed, Skipped when nothing was (already recorded, deleted,
	// unchanged or stale).
	Inserted, Updated, Skipped bool
}

// StatusEventInput is one work_item_status_event row.
type StatusEventInput struct {
	WorkItemID string
	From       *string
	To         string
	WaitsOn    string
	OccurredAt time.Time
	Actor      string
	Source     string
	// DedupeKey is unique across all sources (a webhook's delivery id; a timeline
	// event node id for reconciliation and backfill).
	DedupeKey string
	RawStatus string
}

// BoardStatusParams is a board Status change to record.
type BoardStatusParams struct {
	WorkItemID    string
	ProjectNumber int
	ItemID        string
	RawStatus     string
	DedupeKey     string
	Source        string
	Actor         string
	OccurredAt    time.Time
}

// BoardStatusOutcome says what RecordBoardStatus did.
type BoardStatusOutcome struct {
	// Inserted is true when the event row was new, Applied when the link's board
	// columns moved (false for an out-of-order event older than the latest).
	Inserted, Applied bool
	Status            string
}

// BoardStatus is board_status_map's reading of a raw column name.
type BoardStatus struct {
	Status  string
	WaitsOn string
	// Mapped is false for a column the map does not know (status OTHER).
	Mapped bool
}

// ProductIssueSyncRepository is what the inbound webhook branch reads and writes.
type ProductIssueSyncRepository interface {
	// LinkByIssue and LinkByNodeID find the link a delivery concerns; (nil, nil)
	// when there is none, which sends the delivery down the customer-repository path.
	LinkByIssue(ctx context.Context, owner, repository string, number int) (*LinkedIssue, error)
	LinkByNodeID(ctx context.Context, nodeID string) (*LinkedIssue, error)
	// IsTrackingBoard reports whether projectNumber is the board of a
	// product_repo_mapping row for owner/repository.
	IsTrackingBoard(ctx context.Context, owner, repository string, projectNumber int) (bool, error)

	ApplyIssueSnapshot(ctx context.Context, s IssueSnapshot) (SnapshotOutcome, error)
	// SetLinkState marks the link DELETED or TRANSFERRED (history is kept).
	SetLinkState(ctx context.Context, workItemID, state string) error

	// MirrorGitHubComment inserts the comment unless its GitHub id is recorded.
	MirrorGitHubComment(ctx context.Context, in GitHubCommentInput) (CommentOutcome, error)
	// UpdateGitHubComment changes the content of the recorded comment (inserting it
	// when it was never seen).
	UpdateGitHubComment(ctx context.Context, in GitHubCommentInput) (CommentOutcome, error)
	// DeleteGitHubComment soft-deletes the recorded comment. A comment never seen
	// is recorded already deleted (in carries its content), so a created delivery
	// that arrives after its deletion is skipped.
	DeleteGitHubComment(ctx context.Context, in GitHubCommentInput, by string) (CommentOutcome, error)

	// RecordBoardStatus maps the raw status, writes the status event and moves the
	// link's board columns.
	RecordBoardStatus(ctx context.Context, p BoardStatusParams) (BoardStatusOutcome, error)
	// RecordBoardItem stores the board item id and project number only.
	RecordBoardItem(ctx context.Context, workItemID, itemID string, projectNumber int) error
}

type productIssueSyncRepo struct {
	db *Scoped
}

// NewProductIssueSyncRepository returns the Postgres implementation.
func NewProductIssueSyncRepository(db *Scoped) ProductIssueSyncRepository {
	return &productIssueSyncRepo{db: db}
}

const linkedIssueColumns = `work_item_id::TEXT, owner, repository, issue_number, node_id, link_state, github_state,
		board_status, board_status_raw, board_project_number, github_updated_on`

func scanLinkedIssue(row pgx.Row) (*LinkedIssue, error) {
	var l LinkedIssue
	err := row.Scan(&l.WorkItemID, &l.Owner, &l.Repository, &l.IssueNumber, &l.NodeID, &l.LinkState, &l.GitHubState,
		&l.BoardStatus, &l.BoardStatusRaw, &l.BoardProjectNumber, &l.GitHubUpdatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan github issue link: %w", err)
	}
	return &l, nil
}

// LinkByIssue implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) LinkByIssue(ctx context.Context, owner, repository string, number int) (*LinkedIssue, error) {
	ctx = WithSystemIdentity(ctx)
	return scanLinkedIssue(r.db.QueryRow(ctx, `
		SELECT `+linkedIssueColumns+` FROM github_issue_link
		 WHERE lower(owner) = lower($1) AND lower(repository) = lower($2) AND issue_number = $3`, owner, repository, number))
}

// LinkByNodeID implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) LinkByNodeID(ctx context.Context, nodeID string) (*LinkedIssue, error) {
	if nodeID == "" {
		return nil, nil
	}
	ctx = WithSystemIdentity(ctx)
	return scanLinkedIssue(r.db.QueryRow(ctx, `SELECT `+linkedIssueColumns+` FROM github_issue_link WHERE node_id = $1`, nodeID))
}

// IsTrackingBoard implements ProductIssueSyncRepository. Any mapping row for the
// repository counts, active or not, as the manual link does.
func (r *productIssueSyncRepo) IsTrackingBoard(ctx context.Context, owner, repository string, projectNumber int) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM product_repo_mapping
		                WHERE lower(owner) = lower($1) AND lower(repository) = lower($2) AND board_project_number = $3)`,
		owner, repository, projectNumber).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check tracking board: %w", err)
	}
	return ok, nil
}

// lockLink reads the link row FOR UPDATE; the writers of one issue serialise on it.
func lockLink(ctx context.Context, tx pgx.Tx, workItemID string) (state string, boardStatus *string, updatedOn *time.Time, err error) {
	err = tx.QueryRow(ctx, `
		SELECT github_state, board_status, github_updated_on FROM github_issue_link WHERE work_item_id = $1::uuid FOR UPDATE`,
		workItemID).Scan(&state, &boardStatus, &updatedOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, nil, ErrProductIssueLinkNotFound
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("lock github issue link: %w", err)
	}
	return state, boardStatus, updatedOn, nil
}

// touchLink records that the link was synced now, and moves github_updated_on
// forward (never back) when updatedOn is set.
func touchLink(ctx context.Context, tx pgx.Tx, workItemID string, updatedOn *time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE github_issue_link
		   SET last_synced_on = NOW(), updated_on = NOW(),
		       github_updated_on = GREATEST(github_updated_on, $2::timestamptz)
		 WHERE work_item_id = $1::uuid`, workItemID, updatedOn); err != nil {
		return fmt.Errorf("touch github issue link: %w", err)
	}
	return nil
}

// ApplyIssueSnapshot implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) ApplyIssueSnapshot(ctx context.Context, s IssueSnapshot) (SnapshotOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (SnapshotOutcome, error) {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return SnapshotOutcome{}, err
		}
		var out SnapshotOutcome
		prevState, prevBoard, storedUpdated, err := lockLink(ctx, tx, s.WorkItemID)
		if err != nil {
			return out, err
		}
		// Deliveries can arrive out of order; every one carries the whole issue, so an
		// older one must not overwrite a newer.
		if s.UpdatedOn != nil && storedUpdated != nil && s.UpdatedOn.Before(*storedUpdated) {
			out.Stale = true
			return out, touchLink(ctx, tx, s.WorkItemID, nil)
		}
		// GitHub stamps updated_at to the second, so a closed issue that is labeled in
		// the same second carries the same instant, and the order of the two deliveries
		// is unknown. A delivery at the stored instant may still update title, labels
		// and priority, but only a closed or reopened one may change the open/closed
		// state, its reason or the close date.
		stateMayChange := s.RecordStateEvent || s.UpdatedOn == nil || storedUpdated == nil || s.UpdatedOn.After(*storedUpdated)
		if s.Title != "" {
			if _, err := tx.Exec(ctx, `
				UPDATE work_item SET subject = $2, updated_on = NOW(), updated_by = $3
				 WHERE id = $1::uuid AND subject IS DISTINCT FROM $2`, s.WorkItemID, s.Title, githubSyncActor); err != nil {
				return out, fmt.Errorf("update product issue subject: %w", err)
			}
		}
		labels := s.Labels
		if labels == nil {
			labels = []string{}
		}
		labelsJSON, err := json.Marshal(labels)
		if err != nil {
			return out, fmt.Errorf("marshal labels: %w", err)
		}
		state := s.State
		if (state != "OPEN" && state != "CLOSED") || !stateMayChange {
			state = prevState
		}
		if _, err := tx.Exec(ctx, `
			UPDATE github_issue_link
			   SET labels = $2::jsonb, github_state = $3::text,
			       state_reason = CASE WHEN $7::boolean THEN NULLIF($4, '') ELSE state_reason END,
			       github_closed_on = CASE WHEN NOT $7::boolean THEN github_closed_on
			                               WHEN $3::text = 'OPEN' THEN NULL
			                               ELSE COALESCE($5::timestamptz, github_closed_on) END,
			       github_updated_on = COALESCE($6::timestamptz, github_updated_on),
			       last_synced_on = NOW(), updated_on = NOW()
			 WHERE work_item_id = $1::uuid`,
			s.WorkItemID, labelsJSON, state, s.StateReason, s.ClosedOn, s.UpdatedOn, stateMayChange); err != nil {
			return out, fmt.Errorf("update github issue link: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE product_issue SET priority = NULLIF($2, ''), category = NULLIF($3, '') WHERE id = $1::uuid`,
			s.WorkItemID, truncateColumn(s.Priority, 50), truncateColumn(s.Category, 100)); err != nil {
			return out, fmt.Errorf("update product issue labels: %w", err)
		}
		out.StateChanged = state != prevState
		if out.StateChanged && s.RecordStateEvent {
			ev := StatusEventInput{
				WorkItemID: s.WorkItemID, OccurredAt: s.OccurredAt, Actor: s.Actor,
				Source: StatusSourceWebhook, DedupeKey: s.DedupeKey,
			}
			if state == "CLOSED" {
				ev.From, ev.To, ev.WaitsOn = prevBoard, StatusClosed, WaitsOnNobody
			} else {
				closed := StatusClosed
				ev.From, ev.To, ev.WaitsOn = &closed, StatusReopened, WaitsOnProductTeam
			}
			if out.EventWritten, err = InsertStatusEvent(ctx, tx, ev); err != nil {
				return out, err
			}
		}
		return out, nil
	})
}

func truncateColumn(v string, n int) string {
	r := []rune(v)
	if len(r) <= n {
		return v
	}
	return string(r[:n])
}

// SetLinkState implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) SetLinkState(ctx context.Context, workItemID, state string) error {
	ctx = WithSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return err
		}
		if _, _, _, err := lockLink(ctx, tx, workItemID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE github_issue_link SET link_state = $2, last_synced_on = NOW(), updated_on = NOW()
			 WHERE work_item_id = $1::uuid`, workItemID, state); err != nil {
			return fmt.Errorf("set github issue link state: %w", err)
		}
		return nil
	})
}

// InsertStatusEvent writes one status event inside tx; false when its dedupe key
// is already recorded (nothing is changed then).
func InsertStatusEvent(ctx context.Context, tx pgx.Tx, in StatusEventInput) (bool, error) {
	if strings.TrimSpace(in.DedupeKey) == "" {
		return false, errors.New("status event needs a dedupe key")
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO work_item_status_event (work_item_id, from_status, to_status, waits_on, occurred_at, actor, source, dedupe_key, raw_status)
		VALUES ($1::uuid, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, NULLIF($9, ''))
		ON CONFLICT (dedupe_key) DO NOTHING`,
		in.WorkItemID, in.From, in.To, in.WaitsOn, in.OccurredAt, in.Actor, in.Source, in.DedupeKey, in.RawStatus)
	if err != nil {
		return false, fmt.Errorf("insert status event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ResolveBoardStatus reads a raw board column through board_status_map (case
// and surrounding spaces ignored). An unknown column is OTHER, waiting on the
// product team, and Mapped is false.
func ResolveBoardStatus(ctx context.Context, tx pgx.Tx, raw string) (BoardStatus, error) {
	var b BoardStatus
	err := tx.QueryRow(ctx, `
		SELECT normalized_status, waits_on FROM board_status_map WHERE lower(btrim(raw_status)) = lower(btrim($1))`,
		raw).Scan(&b.Status, &b.WaitsOn)
	if errors.Is(err, pgx.ErrNoRows) {
		return BoardStatus{Status: StatusOther, WaitsOn: WaitsOnProductTeam}, nil
	}
	if err != nil {
		return BoardStatus{}, fmt.Errorf("read board status map: %w", err)
	}
	b.Mapped = true
	return b, nil
}

// RecordBoardStatus implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) RecordBoardStatus(ctx context.Context, p BoardStatusParams) (BoardStatusOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (BoardStatusOutcome, error) {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return BoardStatusOutcome{}, err
		}
		var out BoardStatusOutcome
		_, prev, _, err := lockLink(ctx, tx, p.WorkItemID)
		if err != nil {
			return out, err
		}
		mapped, err := ResolveBoardStatus(ctx, tx, p.RawStatus)
		if err != nil {
			return out, err
		}
		out.Status = mapped.Status
		// The newest board event already recorded (the GitHub close/reopen events carry
		// no raw status), to tell an out-of-order delivery from a new move.
		var latest *time.Time
		if err := tx.QueryRow(ctx, `
			SELECT MAX(occurred_at) FROM work_item_status_event WHERE work_item_id = $1::uuid AND raw_status IS NOT NULL`,
			p.WorkItemID).Scan(&latest); err != nil {
			return out, fmt.Errorf("read latest board event: %w", err)
		}
		outOfOrder := latest != nil && p.OccurredAt.Before(*latest)
		from := prev
		if outOfOrder {
			// An older move was not made from where the link stands now: it was made from
			// the status the latest board event before it moved to (none: no from).
			from = nil
			var before string
			err := tx.QueryRow(ctx, `
				SELECT to_status FROM work_item_status_event
				 WHERE work_item_id = $1::uuid AND raw_status IS NOT NULL AND occurred_at < $2
				 ORDER BY occurred_at DESC, created_on DESC LIMIT 1`, p.WorkItemID, p.OccurredAt).Scan(&before)
			if err == nil {
				from = &before
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return out, fmt.Errorf("read previous board event: %w", err)
			}
		}
		// A reconciliation that found the link in this column before the delivery
		// arrived recorded the move with its own time (a synthetic event, key
		// reconcile:...). The delivery carries the real time: correct that event
		// instead of recording the move twice.
		if merged, err := adoptSyntheticEvent(ctx, tx, p, mapped.Status, from); err != nil || merged {
			if merged {
				err = touchLink(ctx, tx, p.WorkItemID, nil)
			}
			return out, err
		}
		out.Inserted, err = InsertStatusEvent(ctx, tx, StatusEventInput{
			WorkItemID: p.WorkItemID, From: from, To: mapped.Status, WaitsOn: mapped.WaitsOn,
			OccurredAt: p.OccurredAt, Actor: p.Actor, Source: p.Source, DedupeKey: p.DedupeKey, RawStatus: p.RawStatus,
		})
		if err != nil || !out.Inserted {
			return out, err
		}
		if outOfOrder {
			return out, touchLink(ctx, tx, p.WorkItemID, nil)
		}
		out.Applied = true
		if _, err := tx.Exec(ctx, `
			UPDATE github_issue_link
			   SET board_status_raw = $2, board_status = $3, board_item_id = COALESCE(NULLIF($4, ''), board_item_id),
			       board_project_number = $5, last_synced_on = NOW(), updated_on = NOW()
			 WHERE work_item_id = $1::uuid`,
			p.WorkItemID, p.RawStatus, mapped.Status, p.ItemID, p.ProjectNumber); err != nil {
			return out, fmt.Errorf("update github issue link board status: %w", err)
		}
		return out, nil
	})
}

// syntheticEventWindow is how much later than the real move a reconciliation's
// synthetic event may be stamped and still be taken for that move.
const syntheticEventWindow = 24 * time.Hour

// adoptSyntheticEvent moves a reconciliation's synthetic status event (source
// RECONCILE, key reconcile:...) to the time of the webhook's move when it is the
// same move: the same status and raw column, stamped later than the webhook's
// time and within syntheticEventWindow of it. from, when known, replaces the
// event's from_status. It reports whether an event was adopted.
func adoptSyntheticEvent(ctx context.Context, tx pgx.Tx, p BoardStatusParams, to string, from *string) (bool, error) {
	var id string
	err := tx.QueryRow(ctx, `
		SELECT id::TEXT FROM work_item_status_event
		 WHERE work_item_id = $1::uuid AND source = 'RECONCILE' AND dedupe_key LIKE 'reconcile:%'
		   AND to_status = $2 AND raw_status IS NOT NULL AND lower(btrim(raw_status)) = lower(btrim($3))
		   AND occurred_at > $4::timestamptz AND occurred_at <= $4::timestamptz + make_interval(secs => $5::float8)
		 ORDER BY occurred_at LIMIT 1 FOR UPDATE`,
		p.WorkItemID, to, p.RawStatus, p.OccurredAt, syntheticEventWindow.Seconds()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("find synthetic board event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE work_item_status_event SET occurred_at = $2, from_status = COALESCE($3, from_status) WHERE id = $1::uuid`,
		id, p.OccurredAt, from); err != nil {
		return false, fmt.Errorf("correct synthetic board event: %w", err)
	}
	return true, nil
}

// RecordBoardItem implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) RecordBoardItem(ctx context.Context, workItemID, itemID string, projectNumber int) error {
	ctx = WithSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return err
		}
		if _, _, _, err := lockLink(ctx, tx, workItemID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE github_issue_link
			   SET board_item_id = $2, board_project_number = $3, last_synced_on = NOW(), updated_on = NOW()
			 WHERE work_item_id = $1::uuid`, workItemID, itemID, projectNumber); err != nil {
			return fmt.Errorf("record board item: %w", err)
		}
		return nil
	})
}

// InsertGitHubComment writes a mirrored comment and its comment_github_ref inside
// tx. The comment is a WORK_NOTE: the only comment trigger (0112/0206,
// trg_github_outbound_comment) acts on type COMMENT alone, so a mirrored GitHub
// comment can never be queued back to GitHub. created_by is the GitHub login (the
// conversation read model shows it as the author). When the GitHub comment id is
// already recorded nothing is written and the existing comment id is returned.
func InsertGitHubComment(ctx context.Context, tx pgx.Tx, in GitHubCommentInput) (commentID string, inserted bool, err error) {
	err = tx.QueryRow(ctx, `SELECT comment_id::TEXT FROM comment_github_ref WHERE github_comment_id = $1`, in.GitHubCommentID).Scan(&commentID)
	if err == nil {
		return commentID, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("look up github comment: %w", err)
	}
	author := truncateColumn(in.Author, githubCommentAuthorSize)
	if author == "" {
		author = "github"
	}
	created := in.CreatedOn
	if created.IsZero() {
		created = time.Now().UTC()
	}
	origin := in.Origin
	if origin == "" {
		origin = CommentOriginGitHub
	}
	if err = tx.QueryRow(ctx, `
		INSERT INTO comment (created_on, created_by, type, work_item_id, content)
		VALUES ($1, $2, `+"'"+githubCommentNoteType+"'"+`, $3::uuid, $4) RETURNING id::TEXT`,
		created, author, in.WorkItemID, in.Body).Scan(&commentID); err != nil {
		return "", false, fmt.Errorf("insert github comment: %w", err)
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO comment_github_ref (comment_id, github_comment_id, origin, author_login, github_updated_on)
		VALUES ($1::uuid, $2, $3, NULLIF($4, ''), $5)`,
		commentID, in.GitHubCommentID, origin, in.Author, in.UpdatedOn); err != nil {
		return "", false, fmt.Errorf("insert github comment ref: %w", err)
	}
	return commentID, true, nil
}

// MirrorGitHubComment implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) MirrorGitHubComment(ctx context.Context, in GitHubCommentInput) (CommentOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (CommentOutcome, error) {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return CommentOutcome{}, err
		}
		id, inserted, err := InsertGitHubComment(ctx, tx, in)
		if err != nil {
			return CommentOutcome{}, err
		}
		if inserted {
			return CommentOutcome{CommentID: id, Inserted: true}, touchLink(ctx, tx, in.WorkItemID, in.IssueUpdatedOn)
		}
		return CommentOutcome{CommentID: id, Skipped: true}, nil
	})
}

// UpdateGitHubComment implements ProductIssueSyncRepository.
func (r *productIssueSyncRepo) UpdateGitHubComment(ctx context.Context, in GitHubCommentInput) (CommentOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (CommentOutcome, error) {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return CommentOutcome{}, err
		}
		var (
			out        CommentOutcome
			content    *string
			deletedOn  *time.Time
			refUpdated *time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT c.id::TEXT, c.content, ref.deleted_on, ref.github_updated_on
			  FROM comment_github_ref ref JOIN comment c ON c.id = ref.comment_id
			 WHERE ref.github_comment_id = $1 FOR UPDATE OF c`, in.GitHubCommentID).Scan(&out.CommentID, &content, &deletedOn, &refUpdated)
		if errors.Is(err, pgx.ErrNoRows) {
			// An edit of a comment this side never saw (made before the link existed, or a
			// missed delivery): record it as it now reads.
			id, inserted, err := InsertGitHubComment(ctx, tx, in)
			if err != nil {
				return CommentOutcome{}, err
			}
			out = CommentOutcome{CommentID: id, Inserted: inserted, Skipped: !inserted}
			return out, touchLink(ctx, tx, in.WorkItemID, in.IssueUpdatedOn)
		}
		if err != nil {
			return out, fmt.Errorf("read github comment: %w", err)
		}
		if deletedOn != nil || (in.UpdatedOn != nil && refUpdated != nil && in.UpdatedOn.Before(*refUpdated)) {
			out.Skipped = true
			return out, nil
		}
		editedAt := time.Now().UTC()
		if in.UpdatedOn != nil {
			editedAt = *in.UpdatedOn
		}
		editor := in.Editor
		if editor == "" {
			editor = in.Author
		}
		if content == nil || *content != in.Body {
			old := ""
			if content != nil {
				old = *content
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO comment_edit_history (comment_id, body, edited_by, edited_at) VALUES ($1::uuid, $2, $3, $4)`,
				out.CommentID, old, truncateColumn(editor, githubCommentAuthorSize), editedAt); err != nil {
				return out, fmt.Errorf("record github comment edit: %w", err)
			}
			if _, err := tx.Exec(ctx, `UPDATE comment SET content = $2, last_edited_at = $3 WHERE id = $1::uuid`,
				out.CommentID, in.Body, editedAt); err != nil {
				return out, fmt.Errorf("update github comment: %w", err)
			}
			out.Updated = true
		} else {
			out.Skipped = true
		}
		if _, err := tx.Exec(ctx, `UPDATE comment_github_ref SET github_updated_on = $2 WHERE github_comment_id = $1`,
			in.GitHubCommentID, editedAt); err != nil {
			return out, fmt.Errorf("update github comment ref: %w", err)
		}
		return out, touchLink(ctx, tx, in.WorkItemID, in.IssueUpdatedOn)
	})
}

// DeleteGitHubComment implements ProductIssueSyncRepository. The comment table's
// own soft delete (deleted_at/deleted_by) and comment_github_ref.deleted_on are
// both set; the conversation read model filters on both. A comment never seen is
// first inserted from in, so the deletion is recorded and a created delivery that
// arrives late finds the id taken.
func (r *productIssueSyncRepo) DeleteGitHubComment(ctx context.Context, in GitHubCommentInput, by string) (CommentOutcome, error) {
	ctx = WithSystemIdentity(ctx)
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (CommentOutcome, error) {
		if err := claimDeliveryInTx(ctx, tx); err != nil {
			return CommentOutcome{}, err
		}
		var out CommentOutcome
		var deletedOn *time.Time
		err := tx.QueryRow(ctx, `
			SELECT c.id::TEXT, ref.deleted_on FROM comment_github_ref ref JOIN comment c ON c.id = ref.comment_id
			 WHERE ref.github_comment_id = $1 FOR UPDATE OF c`, in.GitHubCommentID).Scan(&out.CommentID, &deletedOn)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if out.CommentID, out.Inserted, err = InsertGitHubComment(ctx, tx, in); err != nil {
				return out, err
			}
		case err != nil:
			return out, fmt.Errorf("read github comment: %w", err)
		case deletedOn != nil:
			out.Skipped = true
			return out, nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE comment SET deleted_at = COALESCE(deleted_at, NOW()), deleted_by = COALESCE(deleted_by, $2) WHERE id = $1::uuid`,
			out.CommentID, truncateColumn(by, githubCommentAuthorSize)); err != nil {
			return out, fmt.Errorf("soft-delete github comment: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE comment_github_ref SET deleted_on = NOW() WHERE github_comment_id = $1`, in.GitHubCommentID); err != nil {
			return out, fmt.Errorf("mark github comment ref deleted: %w", err)
		}
		out.Updated = !out.Inserted
		return out, touchLink(ctx, tx, in.WorkItemID, in.IssueUpdatedOn)
	})
}
