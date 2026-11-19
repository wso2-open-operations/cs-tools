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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The outbound half of the product issue write-back (migration 0309): portal
// actions queued for GitHub in github_command, and what the worker records about
// each attempt. Identity: the methods that serve a portal request (Insert*, Get,
// ListForWorkItem, Retry) run under the caller's identity; the worker's (ClaimDue,
// Mark*, Reschedule, SetPortalCommentGitHubID, LinkForCommand, SetBoardItemID) run
// under the system identity, as the GitHub sync does. Nothing here changes
// github_issue_link's stored state except board_item_id.

// Command kinds, statuses and the pending limits of github_command.
const (
	CommandAddComment     = "ADD_COMMENT"
	CommandSetBoardStatus = "SET_BOARD_STATUS"
	CommandSetState       = "SET_STATE"

	CommandPending   = "PENDING"
	CommandDelivered = "DELIVERED"
	CommandFailed    = "FAILED"
	CommandCancelled = "CANCELLED"

	commandLease           = 5 * time.Minute
	commandRecentOthers    = 10
	commandMaxErrorChars   = 300
	pendingKindIndexName   = "github_command_one_pending_per_kind_key"
	githubCommentIDKeyName = "comment_github_ref_github_comment_id_key"
)

var (
	// ErrCommandPending means the work item already has an open (PENDING) move or
	// close command; the new one was not written.
	ErrCommandPending = errors.New("github command: a pending command of this kind exists")
	// ErrIdempotencyKeyReused means the Idempotency-Key was already used on the work
	// item for another kind of command.
	ErrIdempotencyKeyReused = errors.New("github command: the idempotency key was used for another request")
	// ErrCommandNotFound means no command with that id on that work item.
	ErrCommandNotFound = errors.New("github command: not found")
	// ErrCommandNotFailed means Retry was asked for a command that is not FAILED.
	ErrCommandNotFailed = errors.New("github command: not failed")
	// ErrGitHubCommentIDTaken means the GitHub comment id is already recorded on
	// another comment (typically a mirrored copy the webhook inserted first).
	ErrGitHubCommentIDTaken = errors.New("github command: the github comment id is already recorded")
)

// GitHubCommand is one github_command row.
type GitHubCommand struct {
	ID, WorkItemID, Kind, Status string
	Payload                      json.RawMessage
	CommentID                    *string
	Attempts                     int
	// DeliveryAttempted is set before the first call to the engineering entity and
	// never reset: the action may already be on GitHub.
	DeliveryAttempted    bool
	NextAttemptOn        time.Time
	LastError            *string
	GitHubStatus         *int
	IdempotencyKey       *string
	RequestedBy          string
	CreatedOn, UpdatedOn time.Time
	DeliveredOn          *time.Time
	WindowStartedOn      time.Time
}

// NewGitHubCommand is a command to queue.
type NewGitHubCommand struct {
	WorkItemID string
	Kind       string
	// Payload is the JSON document stored as-is (see the 0309 column comment).
	Payload        json.RawMessage
	CommentID      string // ADD_COMMENT only
	IdempotencyKey string // "" for none
	RequestedBy    string
}

// PortalCommentInput is a comment written in the portal and queued for GitHub.
type PortalCommentInput struct {
	WorkItemID string
	// Content is what the portal shows (the user's text, created_by = RequestedBy).
	Content string
	// GitHubBody is the exact text sent to GitHub (the payload body).
	GitHubBody     string
	IdempotencyKey string
	RequestedBy    string
}

// PortalComment is the comment row a portal comment wrote.
type PortalComment struct {
	ID, Author, Body string
	CreatedOn        time.Time
}

// PortalCommentResult is what InsertPortalComment wrote (or found).
type PortalCommentResult struct {
	Comment PortalComment
	Command GitHubCommand
	// Existed is true when the Idempotency-Key matched an earlier request: nothing
	// new was written.
	Existed bool
}

// CommandLink is what the worker needs of a product issue's GitHub link.
type CommandLink struct {
	WorkItemID, Owner, Repository string
	IssueNumber                   int
	NodeID, BoardItemID           *string
	BoardStatus                   *string
	GitHubState, LinkState        string
	// Org is the organisation the engineering entity is called with: the link's
	// owner, the one the webhook branch requires a board event to name.
	Org string
	// BoardProjectNumber is the product's tracking board: a board_project_number of
	// a product_repo_mapping row for the link's repository (as the webhook branch's
	// IsTrackingBoard reads it). When several rows differ, the link's own last known
	// board wins, then active mappings, then the lowest number. nil: none mapped.
	BoardProjectNumber *int
}

// GitHubCommandRepository is the queue the write-back worker drains.
type GitHubCommandRepository interface {
	// InsertCommand queues a command under the caller's identity in its own
	// transaction. existed is true when the Idempotency-Key matched (the first row
	// is returned). ErrCommandPending for a second open move/close.
	InsertCommand(ctx context.Context, in NewGitHubCommand) (cmd GitHubCommand, existed bool, err error)
	// InsertPortalComment writes comment, comment_github_ref (PORTAL, no GitHub id)
	// and the ADD_COMMENT command in one transaction.
	InsertPortalComment(ctx context.Context, in PortalCommentInput) (PortalCommentResult, error)

	// ClaimDue takes up to limit due PENDING commands, moving next_attempt_on 5
	// minutes ahead as the lease (FOR UPDATE SKIP LOCKED).
	ClaimDue(ctx context.Context, limit int) ([]GitHubCommand, error)
	// MarkDelivered, MarkFailed, MarkCancelled and Reschedule record an attempt's
	// outcome. They act on PENDING rows only (a command cancelled or retried
	// meanwhile is left alone). reason is cut to 300 characters; attempts is the
	// number of calls made so far. githubStatus is X-GitHub-Status, else the
	// engineering entity's status (nil: no answer).
	MarkDelivered(ctx context.Context, id string, attempts int, githubStatus *int) error
	MarkFailed(ctx context.Context, id string, attempts int, reason string, githubStatus *int) error
	MarkCancelled(ctx context.Context, id string, reason string) error
	Reschedule(ctx context.Context, id string, attempts int, next time.Time, reason string, githubStatus *int) error
	// BeginAttempt records, before the first call of a run, that the action may now
	// be on GitHub (delivery_attempted, never reset) and the attempt number, and
	// renews the lease for a full 5 minutes. ok is false (nothing written) when the
	// command is no longer PENDING or its claim lease has already run out: the run
	// must then not call out.
	BeginAttempt(ctx context.Context, id string, attempts int) (ok bool, err error)
	// Retry puts a FAILED command of the work item back to PENDING (attempts 0,
	// due now, a fresh 24 h window). ErrCommandNotFound, ErrCommandNotFailed,
	// ErrCommandPending (another open command of the kind).
	Retry(ctx context.Context, workItemID, id string) (GitHubCommand, error)

	// Get returns (nil, nil) for an unknown id.
	Get(ctx context.Context, id string) (*GitHubCommand, error)
	// ListForWorkItem returns every PENDING and FAILED command plus the 10 newest
	// others, newest first.
	ListForWorkItem(ctx context.Context, workItemID string) ([]GitHubCommand, error)

	// SetPortalCommentGitHubID records the GitHub id on a portal comment's ref only
	// while it is still NULL; set reports whether it did. ErrGitHubCommentIDTaken
	// when another comment already has that id.
	SetPortalCommentGitHubID(ctx context.Context, commentID string, githubCommentID int64, authorLogin string, githubUpdatedOn *time.Time) (set bool, err error)

	// FindCommandByKey returns the command an Idempotency-Key was used for on the
	// work item, nil when none.
	FindCommandByKey(ctx context.Context, workItemID, key string) (*GitHubCommand, error)
	// PortalCommentGitHubID returns the GitHub comment id recorded on a portal
	// comment's ref, nil while it has none.
	PortalCommentGitHubID(ctx context.Context, commentID string) (*int64, error)
	// GitHubCommentRecorded reports whether any comment already has this GitHub id.
	GitHubCommentRecorded(ctx context.Context, githubCommentID int64) (bool, error)
	// UserDisplayName is the portal's name for the person with this email: the
	// user's name, else first and last name; "" when unknown.
	UserDisplayName(ctx context.Context, email string) (string, error)

	// LinkForCommand reads the link fields the worker needs; ErrProductIssueLinkNotFound.
	LinkForCommand(ctx context.Context, workItemID string) (*CommandLink, error)
	// SetBoardItemID stores the board item id found for the issue (that column
	// only), under the link lock the webhook paths take.
	SetBoardItemID(ctx context.Context, workItemID, itemID string) error
}

type githubCommandRepo struct{ db *Scoped }

// NewGitHubCommandRepository returns the Postgres implementation.
func NewGitHubCommandRepository(db *Scoped) GitHubCommandRepository {
	return &githubCommandRepo{db: db}
}

const githubCommandColumns = `id::TEXT, work_item_id::TEXT, kind, status, payload, comment_id::TEXT, attempts, delivery_attempted, next_attempt_on,
		last_error, github_status, idempotency_key, requested_by, created_on, updated_on, delivered_on, window_started_on`

func scanGitHubCommand(row pgx.Row) (GitHubCommand, error) {
	var c GitHubCommand
	err := row.Scan(&c.ID, &c.WorkItemID, &c.Kind, &c.Status, &c.Payload, &c.CommentID, &c.Attempts, &c.DeliveryAttempted, &c.NextAttemptOn,
		&c.LastError, &c.GitHubStatus, &c.IdempotencyKey, &c.RequestedBy, &c.CreatedOn, &c.UpdatedOn, &c.DeliveredOn, &c.WindowStartedOn)
	return c, err
}

func isUniqueOn(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// lockIdempotencyKey serialises two requests with the same key on one item, so the
// second sees the first's row.
func lockIdempotencyKey(ctx context.Context, tx pgx.Tx, workItemID, key string) error {
	if key == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "github_command:"+workItemID+":"+key); err != nil {
		return fmt.Errorf("github command: lock idempotency key: %w", err)
	}
	return nil
}

func commandByKey(ctx context.Context, tx pgx.Tx, workItemID, key string) (*GitHubCommand, error) {
	if key == "" {
		return nil, nil
	}
	c, err := scanGitHubCommand(tx.QueryRow(ctx, `SELECT `+githubCommandColumns+` FROM github_command
		WHERE work_item_id = $1::uuid AND idempotency_key = $2`, workItemID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github command: look up idempotency key: %w", err)
	}
	return &c, nil
}

// InsertGitHubCommandTx queues a command inside tx. The insert runs in a savepoint,
// so tx stays usable after ErrCommandPending. existed: see InsertCommand.
func InsertGitHubCommandTx(ctx context.Context, tx pgx.Tx, in NewGitHubCommand) (GitHubCommand, bool, error) {
	if err := lockIdempotencyKey(ctx, tx, in.WorkItemID, in.IdempotencyKey); err != nil {
		return GitHubCommand{}, false, err
	}
	if prev, err := commandByKey(ctx, tx, in.WorkItemID, in.IdempotencyKey); err != nil || prev != nil {
		if err != nil {
			return GitHubCommand{}, false, err
		}
		if prev.Kind != in.Kind {
			return GitHubCommand{}, false, ErrIdempotencyKeyReused
		}
		return *prev, true, nil
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return GitHubCommand{}, false, fmt.Errorf("github command: savepoint: %w", err)
	}
	c, err := scanGitHubCommand(sp.QueryRow(ctx, `
		INSERT INTO github_command (work_item_id, kind, payload, comment_id, idempotency_key, requested_by)
		VALUES ($1::uuid, $2, $3::jsonb, $4::uuid, NULLIF($5, ''), $6)
		RETURNING `+githubCommandColumns, in.WorkItemID, in.Kind, []byte(in.Payload), emptyToNil(in.CommentID), in.IdempotencyKey, in.RequestedBy))
	if err != nil {
		_ = sp.Rollback(ctx)
		if isUniqueOn(err, pendingKindIndexName) {
			return GitHubCommand{}, false, ErrCommandPending
		}
		return GitHubCommand{}, false, fmt.Errorf("github command: insert: %w", err)
	}
	if err := sp.Commit(ctx); err != nil {
		return GitHubCommand{}, false, fmt.Errorf("github command: release savepoint: %w", err)
	}
	return c, false, nil
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// InsertCommand implements GitHubCommandRepository.
func (r *githubCommandRepo) InsertCommand(ctx context.Context, in NewGitHubCommand) (GitHubCommand, bool, error) {
	var (
		c       GitHubCommand
		existed bool
	)
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		c, existed, err = InsertGitHubCommandTx(ctx, tx, in)
		return err
	})
	if err != nil {
		return GitHubCommand{}, false, err
	}
	return c, existed, nil
}

// InsertPortalComment implements GitHubCommandRepository. The comment is a
// WORK_NOTE (like the mirrored GitHub comments: the comment trigger that queues to a
// customer repository acts on type COMMENT only) whose created_by is the caller's
// email, the value every portal comment carries.
func (r *githubCommandRepo) InsertPortalComment(ctx context.Context, in PortalCommentInput) (PortalCommentResult, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (PortalCommentResult, error) {
		if err := lockIdempotencyKey(ctx, tx, in.WorkItemID, in.IdempotencyKey); err != nil {
			return PortalCommentResult{}, err
		}
		if prev, err := commandByKey(ctx, tx, in.WorkItemID, in.IdempotencyKey); err != nil {
			return PortalCommentResult{}, err
		} else if prev != nil {
			if prev.Kind != CommandAddComment || prev.CommentID == nil {
				return PortalCommentResult{}, ErrIdempotencyKeyReused
			}
			res := PortalCommentResult{Command: *prev, Existed: true}
			if prev.CommentID != nil {
				err := tx.QueryRow(ctx, `SELECT id::TEXT, created_by, COALESCE(content, ''), created_on FROM comment WHERE id = $1::uuid`, *prev.CommentID).
					Scan(&res.Comment.ID, &res.Comment.Author, &res.Comment.Body, &res.Comment.CreatedOn)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return PortalCommentResult{}, fmt.Errorf("github command: read comment of existing command: %w", err)
				}
			}
			return res, nil
		}
		var cm PortalComment
		if err := tx.QueryRow(ctx, `
			INSERT INTO comment (created_on, created_by, type, work_item_id, content)
			VALUES (NOW(), $1, `+"'"+githubCommentNoteType+"'"+`, $2::uuid, $3)
			RETURNING id::TEXT, created_by, COALESCE(content, ''), created_on`,
			in.RequestedBy, in.WorkItemID, in.Content).Scan(&cm.ID, &cm.Author, &cm.Body, &cm.CreatedOn); err != nil {
			return PortalCommentResult{}, fmt.Errorf("github command: insert portal comment: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO comment_github_ref (comment_id, github_comment_id, origin) VALUES ($1::uuid, NULL, $2)`,
			cm.ID, CommentOriginPortal); err != nil {
			return PortalCommentResult{}, fmt.Errorf("github command: insert portal comment ref: %w", err)
		}
		payload, err := json.Marshal(map[string]string{"body": in.GitHubBody})
		if err != nil {
			return PortalCommentResult{}, err
		}
		cmd, _, err := InsertGitHubCommandTx(ctx, tx, NewGitHubCommand{
			WorkItemID: in.WorkItemID, Kind: CommandAddComment, Payload: payload, CommentID: cm.ID,
			IdempotencyKey: in.IdempotencyKey, RequestedBy: in.RequestedBy,
		})
		if err != nil {
			return PortalCommentResult{}, err
		}
		return PortalCommentResult{Comment: cm, Command: cmd}, nil
	})
}

// ClaimDue implements GitHubCommandRepository.
func (r *githubCommandRepo) ClaimDue(ctx context.Context, limit int) ([]GitHubCommand, error) {
	ctx = WithSystemIdentity(ctx)
	// Claimed by moving next_attempt_on forward, not by a status: a worker that dies
	// mid-call leaves the row due again on its own once the lease runs out.
	rows, err := r.db.Query(ctx, `
		WITH due AS (
			SELECT id FROM github_command
			 WHERE status = 'PENDING' AND next_attempt_on <= NOW()
			 ORDER BY next_attempt_on, id
			 LIMIT $1
			 FOR UPDATE SKIP LOCKED
		)
		UPDATE github_command g
		   SET next_attempt_on = NOW() + make_interval(secs => $2::int)
		  FROM due
		 WHERE g.id = due.id
		RETURNING g.id::TEXT, g.work_item_id::TEXT, g.kind, g.status, g.payload, g.comment_id::TEXT, g.attempts, g.delivery_attempted, g.next_attempt_on,
		          g.last_error, g.github_status, g.idempotency_key, g.requested_by, g.created_on, g.updated_on, g.delivered_on, g.window_started_on`,
		limit, int(commandLease.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("github command: claim due: %w", err)
	}
	defer rows.Close()
	var out []GitHubCommand
	for rows.Next() {
		c, err := scanGitHubCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("github command: scan claimed: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkDelivered implements GitHubCommandRepository.
func (r *githubCommandRepo) MarkDelivered(ctx context.Context, id string, attempts int, githubStatus *int) error {
	ctx = WithSystemIdentity(ctx)
	_, err := r.db.Exec(ctx, `
		UPDATE github_command
		   SET status = 'DELIVERED', delivered_on = NOW(), updated_on = NOW(), attempts = $2, last_error = NULL, github_status = $3
		 WHERE id = $1::uuid AND status = 'PENDING'`, id, attempts, githubStatus)
	if err != nil {
		return fmt.Errorf("github command: mark delivered %s: %w", id, err)
	}
	return nil
}

// MarkFailed implements GitHubCommandRepository.
func (r *githubCommandRepo) MarkFailed(ctx context.Context, id string, attempts int, reason string, githubStatus *int) error {
	ctx = WithSystemIdentity(ctx)
	_, err := r.db.Exec(ctx, `
		UPDATE github_command
		   SET status = 'FAILED', updated_on = NOW(), attempts = $2, last_error = $3, github_status = $4
		 WHERE id = $1::uuid AND status = 'PENDING'`, id, attempts, truncateColumn(reason, commandMaxErrorChars), githubStatus)
	if err != nil {
		return fmt.Errorf("github command: mark failed %s: %w", id, err)
	}
	return nil
}

// MarkCancelled implements GitHubCommandRepository.
func (r *githubCommandRepo) MarkCancelled(ctx context.Context, id string, reason string) error {
	ctx = WithSystemIdentity(ctx)
	_, err := r.db.Exec(ctx, `
		UPDATE github_command
		   SET status = 'CANCELLED', updated_on = NOW(), last_error = $2
		 WHERE id = $1::uuid AND status = 'PENDING'`, id, truncateColumn(reason, commandMaxErrorChars))
	if err != nil {
		return fmt.Errorf("github command: mark cancelled %s: %w", id, err)
	}
	return nil
}

// Reschedule implements GitHubCommandRepository.
func (r *githubCommandRepo) Reschedule(ctx context.Context, id string, attempts int, next time.Time, reason string, githubStatus *int) error {
	ctx = WithSystemIdentity(ctx)
	_, err := r.db.Exec(ctx, `
		UPDATE github_command
		   SET attempts = $2, next_attempt_on = $3, last_error = NULLIF($4, ''), github_status = $5, updated_on = NOW()
		 WHERE id = $1::uuid AND status = 'PENDING'`, id, attempts, next, truncateColumn(reason, commandMaxErrorChars), githubStatus)
	if err != nil {
		return fmt.Errorf("github command: reschedule %s: %w", id, err)
	}
	return nil
}

// BeginAttempt implements GitHubCommandRepository.
func (r *githubCommandRepo) BeginAttempt(ctx context.Context, id string, attempts int) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	// Only while the claim's lease is still running: past it, another worker may
	// have claimed the row and must not run it concurrently with this one.
	tag, err := r.db.Exec(ctx, `
		UPDATE github_command
		   SET attempts = $2, delivery_attempted = TRUE, updated_on = NOW(),
		       next_attempt_on = NOW() + make_interval(secs => $3::int)
		 WHERE id = $1::uuid AND status = 'PENDING' AND next_attempt_on > NOW()`, id, attempts, int(commandLease.Seconds()))
	if err != nil {
		return false, fmt.Errorf("github command: begin attempt %s: %w", id, err)
	}
	return tag.RowsAffected() == 1, nil
}

// Retry implements GitHubCommandRepository. last_error stays until the next attempt.
func (r *githubCommandRepo) Retry(ctx context.Context, workItemID, id string) (GitHubCommand, error) {
	return InTxReturning(ctx, r.db, func(tx pgx.Tx) (GitHubCommand, error) {
		sp, err := tx.Begin(ctx)
		if err != nil {
			return GitHubCommand{}, fmt.Errorf("github command: savepoint: %w", err)
		}
		c, err := scanGitHubCommand(sp.QueryRow(ctx, `
			UPDATE github_command
			   SET status = 'PENDING', attempts = 0, next_attempt_on = NOW(), window_started_on = NOW(), updated_on = NOW()
			 WHERE id = $1::uuid AND work_item_id = $2::uuid AND status = 'FAILED'
			RETURNING `+githubCommandColumns, id, workItemID))
		if err == nil {
			return c, sp.Commit(ctx)
		}
		_ = sp.Rollback(ctx)
		if isUniqueOn(err, pendingKindIndexName) {
			return GitHubCommand{}, ErrCommandPending
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return GitHubCommand{}, fmt.Errorf("github command: retry %s: %w", id, err)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM github_command WHERE id = $1::uuid AND work_item_id = $2::uuid)`, id, workItemID).Scan(&exists); err != nil {
			return GitHubCommand{}, fmt.Errorf("github command: retry %s: %w", id, err)
		}
		if !exists {
			return GitHubCommand{}, ErrCommandNotFound
		}
		return GitHubCommand{}, ErrCommandNotFailed
	})
}

// Get implements GitHubCommandRepository.
func (r *githubCommandRepo) Get(ctx context.Context, id string) (*GitHubCommand, error) {
	c, err := scanGitHubCommand(r.db.QueryRow(ctx, `SELECT `+githubCommandColumns+` FROM github_command WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github command: get %s: %w", id, err)
	}
	return &c, nil
}

// ListForWorkItem implements GitHubCommandRepository.
func (r *githubCommandRepo) ListForWorkItem(ctx context.Context, workItemID string) ([]GitHubCommand, error) {
	return listGitHubCommands(ctx, r.db, workItemID)
}

func listGitHubCommands(ctx context.Context, db *Scoped, workItemID string) ([]GitHubCommand, error) {
	rows, err := db.Query(ctx, `
		(SELECT `+githubCommandColumns+` FROM github_command
		  WHERE work_item_id = $1::uuid AND status IN ('PENDING', 'FAILED'))
		UNION ALL
		(SELECT `+githubCommandColumns+` FROM github_command
		  WHERE work_item_id = $1::uuid AND status NOT IN ('PENDING', 'FAILED')
		  ORDER BY created_on DESC, id DESC LIMIT $2)
		ORDER BY created_on DESC, id DESC`, workItemID, commandRecentOthers)
	if err != nil {
		return nil, fmt.Errorf("github command: list for work item: %w", err)
	}
	defer rows.Close()
	out := []GitHubCommand{}
	for rows.Next() {
		c, err := scanGitHubCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("github command: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Domain is the command as the routes show it.
func (c GitHubCommand) Domain() domain.GitHubCommand {
	out := domain.GitHubCommand{
		ID: c.ID, Kind: c.Kind, Status: c.Status, Attempts: c.Attempts, LastError: c.LastError,
		RequestedBy: c.RequestedBy, CreatedOn: c.CreatedOn, DeliveredOn: c.DeliveredOn, CommentID: c.CommentID,
	}
	if c.Status == CommandPending {
		next := c.NextAttemptOn
		out.NextAttemptOn = &next
	}
	if c.Kind == CommandSetBoardStatus {
		var p struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(c.Payload, &p) == nil && p.Status != "" {
			out.TargetStatus = &p.Status
		}
	}
	return out
}

// FindCommandByKey implements GitHubCommandRepository.
func (r *githubCommandRepo) FindCommandByKey(ctx context.Context, workItemID, key string) (*GitHubCommand, error) {
	if key == "" {
		return nil, nil
	}
	c, err := scanGitHubCommand(r.db.QueryRow(ctx, `SELECT `+githubCommandColumns+` FROM github_command
		WHERE work_item_id = $1::uuid AND idempotency_key = $2`, workItemID, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github command: find by key: %w", err)
	}
	return &c, nil
}

// PortalCommentGitHubID implements GitHubCommandRepository.
func (r *githubCommandRepo) PortalCommentGitHubID(ctx context.Context, commentID string) (*int64, error) {
	ctx = WithSystemIdentity(ctx)
	var id *int64
	err := r.db.QueryRow(ctx, `SELECT github_comment_id FROM comment_github_ref WHERE comment_id = $1::uuid`, commentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("github command: read comment ref: %w", err)
	}
	return id, nil
}

// GitHubCommentRecorded implements GitHubCommandRepository.
func (r *githubCommandRepo) GitHubCommentRecorded(ctx context.Context, githubCommentID int64) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM comment_github_ref WHERE github_comment_id = $1)`, githubCommentID).Scan(&ok); err != nil {
		return false, fmt.Errorf("github command: check github comment id: %w", err)
	}
	return ok, nil
}

// UserDisplayName implements GitHubCommandRepository.
func (r *githubCommandRepo) UserDisplayName(ctx context.Context, email string) (string, error) {
	ctx = WithSystemIdentity(ctx)
	var name *string
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(NULLIF(btrim(name), ''), NULLIF(btrim(concat_ws(' ', first_name, last_name)), ''))
		  FROM "user" WHERE lower(email) = lower($1)
		 ORDER BY is_active DESC NULLS LAST, created_on DESC, id LIMIT 1`, email).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && name == nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("github command: read user name: %w", err)
	}
	return *name, nil
}

// SetPortalCommentGitHubID implements GitHubCommandRepository.
func (r *githubCommandRepo) SetPortalCommentGitHubID(ctx context.Context, commentID string, githubCommentID int64, authorLogin string, githubUpdatedOn *time.Time) (bool, error) {
	ctx = WithSystemIdentity(ctx)
	set, err := InTxReturning(ctx, r.db, func(tx pgx.Tx) (bool, error) {
		sp, err := tx.Begin(ctx)
		if err != nil {
			return false, fmt.Errorf("github command: savepoint: %w", err)
		}
		tag, err := sp.Exec(ctx, `
			UPDATE comment_github_ref
			   SET github_comment_id = $2, author_login = NULLIF($3, ''), github_updated_on = $4
			 WHERE comment_id = $1::uuid AND origin = 'PORTAL' AND github_comment_id IS NULL`,
			commentID, githubCommentID, authorLogin, githubUpdatedOn)
		if err != nil {
			_ = sp.Rollback(ctx)
			if isUniqueOn(err, githubCommentIDKeyName) {
				return false, ErrGitHubCommentIDTaken
			}
			return false, fmt.Errorf("github command: set github comment id: %w", err)
		}
		return tag.RowsAffected() == 1, sp.Commit(ctx)
	})
	return set, err
}

// AdoptPendingPortalComment gives a GitHub comment that echoes a portal comment
// the portal comment's ref (contract section 5): the oldest PORTAL ref with no
// GitHub id, on workItemID, whose ADD_COMMENT command carries body (line endings
// compared as LF). It also marks that command DELIVERED when it is PENDING or
// FAILED. It returns the adopted comment id, or "" when nothing matches. It waits
// for a row a worker is setting, then (the id now set) matches nothing.
func AdoptPendingPortalComment(ctx context.Context, tx pgx.Tx, workItemID, body string, githubCommentID int64, authorLogin string, updatedOn *time.Time) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		UPDATE comment_github_ref ref
		   SET github_comment_id = $3, author_login = NULLIF($4, ''), github_updated_on = $5
		 WHERE ref.comment_id = (
		        SELECT r.comment_id
		          FROM comment_github_ref r
		          JOIN comment c ON c.id = r.comment_id
		          JOIN github_command g ON g.comment_id = c.id AND g.kind = 'ADD_COMMENT'
		         WHERE r.origin = 'PORTAL' AND r.github_comment_id IS NULL AND r.deleted_on IS NULL
		           AND c.work_item_id = $1::uuid AND c.deleted_at IS NULL
		           AND replace(g.payload->>'body', E'\r\n', E'\n') = replace($2::text, E'\r\n', E'\n')
		         ORDER BY c.created_on, c.id
		         LIMIT 1
		           FOR UPDATE OF r)
		   AND ref.github_comment_id IS NULL
		RETURNING ref.comment_id::TEXT`, workItemID, body, githubCommentID, authorLogin, updatedOn).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("github command: adopt portal comment: %w", err)
	}
	// GitHub has the comment: a retry of its command would post it twice.
	if _, err := tx.Exec(ctx, `
		UPDATE github_command
		   SET status = 'DELIVERED', delivered_on = NOW(), updated_on = NOW(), last_error = NULL
		 WHERE comment_id = $1::uuid AND kind = 'ADD_COMMENT' AND status IN ('PENDING', 'FAILED')`, id); err != nil {
		return "", fmt.Errorf("github command: mark adopted command delivered: %w", err)
	}
	return id, nil
}

// LinkForCommand implements GitHubCommandRepository.
func (r *githubCommandRepo) LinkForCommand(ctx context.Context, workItemID string) (*CommandLink, error) {
	ctx = WithSystemIdentity(ctx)
	var l CommandLink
	err := r.db.QueryRow(ctx, `
		SELECT l.work_item_id::TEXT, l.owner, l.repository, l.issue_number, l.node_id, l.board_item_id, l.board_status,
		       l.github_state, l.link_state,
		       (SELECT m.board_project_number FROM product_repo_mapping m
		         WHERE lower(m.owner) = lower(l.owner) AND lower(m.repository) = lower(l.repository)
		           AND m.board_project_number IS NOT NULL
		         ORDER BY (m.board_project_number = l.board_project_number) DESC NULLS LAST, m.is_active DESC, m.board_project_number
		         LIMIT 1)
		  FROM github_issue_link l
		  JOIN work_item wi ON wi.id = l.work_item_id AND wi.type = 'PRODUCT_ISSUE'
		 WHERE l.work_item_id = $1::uuid`, workItemID).
		Scan(&l.WorkItemID, &l.Owner, &l.Repository, &l.IssueNumber, &l.NodeID, &l.BoardItemID, &l.BoardStatus,
			&l.GitHubState, &l.LinkState, &l.BoardProjectNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProductIssueLinkNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("github command: read link: %w", err)
	}
	l.Org = l.Owner
	return &l, nil
}

// SetBoardItemID implements GitHubCommandRepository.
func (r *githubCommandRepo) SetBoardItemID(ctx context.Context, workItemID, itemID string) error {
	ctx = WithSystemIdentity(ctx)
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, _, _, err := lockLink(ctx, tx, workItemID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE github_issue_link SET board_item_id = $2 WHERE work_item_id = $1::uuid`, workItemID, itemID); err != nil {
			return fmt.Errorf("github command: set board item id: %w", err)
		}
		return nil
	})
}
