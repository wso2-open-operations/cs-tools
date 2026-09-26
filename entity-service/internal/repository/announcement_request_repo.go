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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// AnnouncementRequestRepository defines the persistence operations for the
// announcement_requests table — see domain.AnnouncementRequest's doc comment
// for what it's for. Every method here is a plain data operation; the state
// machine and its gating rules live one layer up, in the service.
type AnnouncementRequestRepository interface {
	Create(ctx context.Context, req domain.CreateAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	Get(ctx context.Context, id string) (domain.AnnouncementRequest, error)
	Search(ctx context.Context, req domain.SearchAnnouncementRequestsRequest) ([]domain.AnnouncementRequest, int, error)
	// Update applies a plain content update (subject/description/security
	// flag/audience definition — whichever fields are non-nil) with no
	// state change, atomically conditioned on the row still being in
	// expectedState (see the implementation's own doc comment for why).
	// The service layer is responsible for deciding whether a
	// revert-to-draft side effect applies instead before calling this.
	Update(ctx context.Context, id string, expectedState domain.AnnouncementRequestState, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// RecordDryRun sets dry_run_case_id/dry_run_on/dry_run_by.
	RecordDryRun(ctx context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error)
	// Submit moves state to pending_approval, freezes resolved_project_ids/
	// resolved_project_count, and sets submitted_by/submitted_on. The
	// caller (service layer) has already validated the current state and
	// the dry-run precondition before this is called.
	Submit(ctx context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// Approve moves state to approved and sets approved_by/approved_on.
	Approve(ctx context.Context, id, actorID, actorEmail string) (domain.AnnouncementRequest, error)
	// SetSchedule sets or clears scheduled_on (nil clears it) — only
	// callable while the row is approved. The service layer has already
	// validated the current state, the actor, and (if non-nil) that
	// scheduledFor is in the future.
	SetSchedule(ctx context.Context, id string, scheduledFor *time.Time) (domain.AnnouncementRequest, error)
	// ClaimForAutoPublish atomically marks a due, approved row as "being
	// auto-published right now" (publish_claimed_on = NOW()), so a second,
	// overlapping AutoPublish attempt for the same row can't also start
	// fanning out to the same projects. Only succeeds when the row is
	// approved, due (scheduled_on <= NOW()), and not already claimed within
	// staleAfter -- an older claim is assumed abandoned (the process that
	// held it crashed or was killed) and can be reclaimed. Callers must
	// release the claim (ReleaseAutoPublishClaim) once their attempt ends,
	// success or failure.
	ClaimForAutoPublish(ctx context.Context, id string, staleAfter time.Duration) (domain.AnnouncementRequest, error)
	// ReleaseAutoPublishClaim clears publish_claimed_on unconditionally --
	// safe to call even if the row has since moved to published, or was
	// never claimed at all.
	ReleaseAutoPublishClaim(ctx context.Context, id string) error
	// RevertToDraft moves state back to draft, clearing
	// resolved_project_ids/resolved_project_count/dry_run_case_id/
	// dry_run_on/dry_run_by/submitted_by/submitted_on, and — in the same
	// statement — applies whatever content fields the caller also supplied
	// (the "edit while pending_approval" path is one atomic operation, not
	// a revert followed by a separate update).
	RevertToDraft(ctx context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// MarkPublished moves state to published, sets published_by/published_on,
	// and stores caseIDs as published_case_ids.
	MarkPublished(ctx context.Context, id, actorID, actorEmail string, caseIDs []string) (domain.AnnouncementRequest, error)
	// CreateUpdate inserts a new announcement_request_updates row.
	CreateUpdate(ctx context.Context, announcementRequestID, content, createdBy, createdByEmail string) (domain.AnnouncementRequestUpdate, error)
	// ListUpdates returns every update for announcementRequestID, newest first.
	ListUpdates(ctx context.Context, announcementRequestID string) ([]domain.AnnouncementRequestUpdate, error)
	// UpsertDeliveries records (inserts or overwrites) one delivery row per
	// input, keyed on (announcementRequestID, projectID) — see
	// announcement_request_deliveries' own migration doc comment for why
	// this is an upsert rather than a plain insert.
	UpsertDeliveries(ctx context.Context, announcementRequestID string, deliveries []domain.RecordAnnouncementRequestDeliveryInput) ([]domain.AnnouncementRequestDelivery, error)
	// ListDeliveries returns every delivery recorded for announcementRequestID.
	ListDeliveries(ctx context.Context, announcementRequestID string) ([]domain.AnnouncementRequestDelivery, error)
}

type announcementRequestRepo struct {
	db *pgxpool.Pool
}

// NewAnnouncementRequestRepository constructs an AnnouncementRequestRepository
// backed by the given connection pool.
func NewAnnouncementRequestRepository(db *pgxpool.Pool) AnnouncementRequestRepository {
	return &announcementRequestRepo{db: db}
}

// announcementRequestColumns is the column list shared by every query that
// returns a full row, kept in one place so it can't drift out of sync with
// scanAnnouncementRequest's field order.
const announcementRequestColumns = `
	id, kind, state, subject, description, is_security_announcement,
	audience_definition, resolved_project_ids, resolved_project_count,
	dry_run_case_id, dry_run_on, dry_run_by,
	created_by, created_by_email, created_on, updated_on,
	submitted_by, submitted_by_email, submitted_on,
	approved_by, approved_by_email, approved_on,
	published_by, published_by_email, published_on,
	published_case_ids, due_on, scheduled_on`

func scanAnnouncementRequest(row pgx.Row) (domain.AnnouncementRequest, error) {
	var r domain.AnnouncementRequest
	var resolvedProjectIDsRaw, publishedCaseIDsRaw []byte
	if err := row.Scan(
		&r.ID, &r.Kind, &r.State, &r.Subject, &r.Description, &r.IsSecurityAnnouncement,
		&r.AudienceDefinition, &resolvedProjectIDsRaw, &r.ResolvedProjectCount,
		&r.DryRunCaseID, &r.DryRunAt, &r.DryRunBy,
		&r.CreatedBy, &r.CreatedByEmail, &r.CreatedAt, &r.UpdatedAt,
		&r.SubmittedBy, &r.SubmittedByEmail, &r.SubmittedAt,
		&r.ApprovedBy, &r.ApprovedByEmail, &r.ApprovedAt,
		&r.PublishedBy, &r.PublishedByEmail, &r.PublishedAt,
		&publishedCaseIDsRaw, &r.DueOn, &r.ScheduledFor,
	); err != nil {
		return domain.AnnouncementRequest{}, err
	}
	// resolved_project_ids/published_case_ids are JSONB and nil until
	// Submit/MarkPublished respectively — decoded explicitly (not left to
	// pgx's default codec) so a NULL column reads back as a nil slice
	// rather than requiring a *[]string juggling act at every call site.
	if len(resolvedProjectIDsRaw) > 0 {
		if err := json.Unmarshal(resolvedProjectIDsRaw, &r.ResolvedProjectIDs); err != nil {
			return domain.AnnouncementRequest{}, fmt.Errorf("decode resolved_project_ids: %w", err)
		}
	}
	if len(publishedCaseIDsRaw) > 0 {
		if err := json.Unmarshal(publishedCaseIDsRaw, &r.PublishedCaseIDs); err != nil {
			return domain.AnnouncementRequest{}, fmt.Errorf("decode published_case_ids: %w", err)
		}
	}
	return r, nil
}

// Create implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) Create(ctx context.Context, req domain.CreateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	audience := req.AudienceDefinition
	if len(audience) == 0 {
		audience = json.RawMessage(`{}`)
	}
	query := `
		INSERT INTO announcement_requests (kind, subject, description, is_security_announcement, audience_definition, created_by, created_by_email)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query,
		req.Kind, req.Subject, req.Description, req.IsSecurityAnnouncement, audience, req.CreatedBy, req.CreatedByEmail,
	))
	if err != nil {
		return domain.AnnouncementRequest{}, fmt.Errorf("create announcement_request: %w", err)
	}
	return ar, nil
}

// Get implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) Get(ctx context.Context, id string) (domain.AnnouncementRequest, error) {
	query := `SELECT ` + announcementRequestColumns + ` FROM announcement_requests WHERE id = $1`

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, &apierror.NotFoundError{Msg: "announcement request not found: " + id}
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("get announcement_request: %w", err)
	}
	return ar, nil
}

// Search implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) Search(ctx context.Context, req domain.SearchAnnouncementRequestsRequest) ([]domain.AnnouncementRequest, int, error) {
	// state and created_by are both optional filters; NULL::text on the
	// unused side of each OR makes an unset filter match every row without
	// needing to build the WHERE clause dynamically. readyForScheduledPublish
	// works the same way via NOT $3::boolean: when false the whole OR branch
	// is unconditionally true (no extra restriction), when true it requires
	// state = 'approved' and a scheduled_on that has already arrived — the
	// one query operations/csm-scheduled-tasks' publish_scheduled_announcements
	// sub-cron needs (the service layer validates this is never combined
	// with an explicit State).
	const where = `WHERE ($1::text IS NULL OR state = $1)
		AND ($2::text IS NULL OR created_by = $2)
		AND (NOT $3::boolean OR (state = 'approved' AND scheduled_on IS NOT NULL AND scheduled_on <= NOW()))`
	countQuery := `SELECT COUNT(*) FROM announcement_requests ` + where
	dataQuery := `SELECT ` + announcementRequestColumns + ` FROM announcement_requests ` + where + `
		ORDER BY created_on DESC, id
		LIMIT $4 OFFSET $5`

	var state *string
	if req.State != nil {
		s := string(*req.State)
		state = &s
	}

	var total int
	var requests []domain.AnnouncementRequest
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return r.db.QueryRow(egCtx, countQuery, state, req.CreatedBy, req.ReadyForScheduledPublish).Scan(&total)
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, state, req.CreatedBy, req.ReadyForScheduledPublish, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			ar, err := scanAnnouncementRequest(rows)
			if err != nil {
				return err
			}
			requests = append(requests, ar)
		}
		return rows.Err()
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, fmt.Errorf("search announcement_requests: %w", err)
	}
	return requests, total, nil
}

// Update implements AnnouncementRequestRepository. COALESCE keeps whichever
// column a nil pointer didn't touch; audienceSet/audience mirror that same
// "only overwrite if actually provided" behavior for the JSONB column,
// which COALESCE alone can't express cleanly for a byte-slice parameter.
//
// expectedState is part of the WHERE clause, not just a service-layer
// precondition check beforehand — the service calls this for two different
// states (draft's plain edit, approved's in-place edit), and without the
// state repeated here atomically, a concurrent transition racing this call
// (e.g. someone submitting the draft, or publishing the approved request,
// in the gap between the service's own Get and this UPDATE) could silently
// apply a draft-shaped or approved-shaped edit to a row that's moved on to
// a different state by the time this actually runs.
func (r *announcementRequestRepo) Update(ctx context.Context, id string, expectedState domain.AnnouncementRequestState, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			subject = COALESCE($2, subject),
			description = COALESCE($3, description),
			is_security_announcement = COALESCE($4, is_security_announcement),
			audience_definition = CASE WHEN $5::boolean THEN $6 ELSE audience_definition END,
			updated_on = NOW()
		WHERE id = $1 AND state = $7
		RETURNING ` + announcementRequestColumns

	audienceSet := len(req.AudienceDefinition) > 0
	var audience json.RawMessage
	if audienceSet {
		audience = req.AudienceDefinition
	} else {
		audience = json.RawMessage(`null`)
	}

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query,
		id, req.Subject, req.Description, req.IsSecurityAnnouncement, audienceSet, audience, expectedState,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "update")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("update announcement_request: %w", err)
	}
	return ar, nil
}

// onConflictOrNotFound turns a pgx.ErrNoRows from a conditional UPDATE
// (WHERE id = $1 AND state = '<expected>', possibly with more preconditions)
// into the right error. The service layer already confirmed the row exists
// via its own Get immediately before calling into one of these methods, so
// a 0-rows result here means the precondition it checked no longer holds —
// a genuine race between two concurrent transitions on the same request,
// which the WHERE clause exists specifically to close atomically rather
// than silently letting the second writer clobber the first. Re-checking
// via a second Get (rather than assuming ConflictError outright) keeps this
// correct in the one case that isn't a race: the row was deleted entirely
// between the two calls.
func (r *announcementRequestRepo) onConflictOrNotFound(ctx context.Context, id, action string) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	return &apierror.ConflictError{Msg: action + ": the request's state changed before this could be applied — reload and try again"}
}

// RecordDryRun implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) RecordDryRun(ctx context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			dry_run_case_id = $2, dry_run_on = NOW(), dry_run_by = $3, updated_on = NOW()
		WHERE id = $1 AND state = 'draft'
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, req.CaseID, req.ActorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "record dry run")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("record announcement_request dry run: %w", err)
	}
	return ar, nil
}

// Submit implements AnnouncementRequestRepository. The WHERE clause repeats
// both preconditions the service layer already checked (state = draft, a
// dry run recorded) so a concurrent RevertToDraft racing this call can't
// leave a pending_approval row with no valid dry run behind it.
func (r *announcementRequestRepo) Submit(ctx context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	projectIDs, err := json.Marshal(req.ResolvedProjectIDs)
	if err != nil {
		return domain.AnnouncementRequest{}, fmt.Errorf("marshal resolved project ids: %w", err)
	}
	query := `
		UPDATE announcement_requests SET
			state = 'pending_approval',
			resolved_project_ids = $2,
			resolved_project_count = $3,
			submitted_by = $4, submitted_by_email = NULLIF($5, ''), submitted_on = NOW(),
			due_on = NOW() + INTERVAL '1 month',
			updated_on = NOW()
		WHERE id = $1 AND state = 'draft' AND dry_run_case_id IS NOT NULL
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, projectIDs, len(req.ResolvedProjectIDs), req.ActorID, req.ActorEmail))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "submit for approval")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("submit announcement_request: %w", err)
	}
	return ar, nil
}

// Approve implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) Approve(ctx context.Context, id, actorID, actorEmail string) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			state = 'approved', approved_by = $2, approved_by_email = NULLIF($3, ''), approved_on = NOW(), updated_on = NOW()
		WHERE id = $1 AND state = 'pending_approval'
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, actorID, actorEmail))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "approve")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("approve announcement_request: %w", err)
	}
	return ar, nil
}

// SetSchedule implements AnnouncementRequestRepository. Only callable from
// approved — the state condition below matches Approve's own race-closing
// shape (a concurrent Publish racing this call can't leave a published row
// with a stale schedule silently reapplied).
func (r *announcementRequestRepo) SetSchedule(ctx context.Context, id string, scheduledFor *time.Time) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			scheduled_on = $2, updated_on = NOW()
		WHERE id = $1 AND state = 'approved'
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, scheduledFor))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "schedule")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("schedule announcement_request: %w", err)
	}
	return ar, nil
}

// ClaimForAutoPublish implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) ClaimForAutoPublish(ctx context.Context, id string, staleAfter time.Duration) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			publish_claimed_on = NOW()
		WHERE id = $1 AND state = 'approved'
			AND scheduled_on IS NOT NULL AND scheduled_on <= NOW()
			AND (publish_claimed_on IS NULL OR publish_claimed_on <= NOW() - ($2 * INTERVAL '1 second'))
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, staleAfter.Seconds()))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "claim for auto-publish")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("claim announcement_request for auto-publish: %w", err)
	}
	return ar, nil
}

// ReleaseAutoPublishClaim implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) ReleaseAutoPublishClaim(ctx context.Context, id string) error {
	if _, err := r.db.Exec(ctx, `UPDATE announcement_requests SET publish_claimed_on = NULL WHERE id = $1`, id); err != nil {
		return fmt.Errorf("release announcement_request auto-publish claim: %w", err)
	}
	return nil
}

// RevertToDraft implements AnnouncementRequestRepository. Only callable from
// pending_approval — the state condition below prevents this racing a
// concurrent Approve on the same request.
func (r *announcementRequestRepo) RevertToDraft(ctx context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			state = 'draft',
			subject = COALESCE($2, subject),
			description = COALESCE($3, description),
			is_security_announcement = COALESCE($4, is_security_announcement),
			audience_definition = CASE WHEN $5::boolean THEN $6 ELSE audience_definition END,
			resolved_project_ids = NULL, resolved_project_count = NULL,
			dry_run_case_id = NULL, dry_run_on = NULL, dry_run_by = NULL,
			submitted_by = NULL, submitted_on = NULL,
			updated_on = NOW()
		WHERE id = $1 AND state = 'pending_approval'
		RETURNING ` + announcementRequestColumns

	audienceSet := len(req.AudienceDefinition) > 0
	var audience json.RawMessage
	if audienceSet {
		audience = req.AudienceDefinition
	} else {
		audience = json.RawMessage(`null`)
	}

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query,
		id, req.Subject, req.Description, req.IsSecurityAnnouncement, audienceSet, audience,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "revert to draft")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("revert announcement_request to draft: %w", err)
	}
	return ar, nil
}

// MarkPublished implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) MarkPublished(ctx context.Context, id, actorID, actorEmail string, caseIDs []string) (domain.AnnouncementRequest, error) {
	caseIDsJSON, err := json.Marshal(caseIDs)
	if err != nil {
		return domain.AnnouncementRequest{}, fmt.Errorf("marshal published case ids: %w", err)
	}
	query := `
		UPDATE announcement_requests SET
			state = 'published', published_by = $2, published_by_email = NULLIF($3, ''), published_on = NOW(),
			published_case_ids = $4, updated_on = NOW()
		WHERE id = $1 AND state = 'approved'
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, actorID, actorEmail, caseIDsJSON))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "publish")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("mark announcement_request published: %w", err)
	}
	return ar, nil
}

// CreateUpdate implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) CreateUpdate(ctx context.Context, announcementRequestID, content, createdBy, createdByEmail string) (domain.AnnouncementRequestUpdate, error) {
	query := `
		INSERT INTO announcement_request_updates (announcement_request_id, content, created_by, created_by_email)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, announcement_request_id, content, created_by, created_by_email, created_on`

	var u domain.AnnouncementRequestUpdate
	err := r.db.QueryRow(ctx, query, announcementRequestID, content, createdBy, createdByEmail).Scan(
		&u.ID, &u.AnnouncementRequestID, &u.Content, &u.CreatedBy, &u.CreatedByEmail, &u.CreatedOn,
	)
	if err != nil {
		return domain.AnnouncementRequestUpdate{}, fmt.Errorf("create announcement_request_update: %w", err)
	}
	return u, nil
}

// ListUpdates implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) ListUpdates(ctx context.Context, announcementRequestID string) ([]domain.AnnouncementRequestUpdate, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, announcement_request_id, content, created_by, created_by_email, created_on
		 FROM announcement_request_updates
		 WHERE announcement_request_id = $1
		 ORDER BY created_on DESC`, announcementRequestID)
	if err != nil {
		return nil, fmt.Errorf("list announcement_request_updates: %w", err)
	}
	defer rows.Close()

	updates := []domain.AnnouncementRequestUpdate{}
	for rows.Next() {
		var u domain.AnnouncementRequestUpdate
		if err := rows.Scan(&u.ID, &u.AnnouncementRequestID, &u.Content, &u.CreatedBy, &u.CreatedByEmail, &u.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan announcement_request_update: %w", err)
		}
		updates = append(updates, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate announcement_request_updates: %w", err)
	}
	return updates, nil
}

// announcementRequestDeliveryColumns is the column list shared by every
// query returning a full delivery row, kept in one place so it can't drift
// out of sync with scanAnnouncementRequestDelivery's field order.
const announcementRequestDeliveryColumns = `
	id, announcement_request_id, project_id, case_id, status, error_message, created_on, updated_on`

func scanAnnouncementRequestDelivery(row pgx.Row) (domain.AnnouncementRequestDelivery, error) {
	var d domain.AnnouncementRequestDelivery
	if err := row.Scan(
		&d.ID, &d.AnnouncementRequestID, &d.ProjectID, &d.CaseID, &d.Status, &d.ErrorMessage,
		&d.CreatedOn, &d.UpdatedOn,
	); err != nil {
		return domain.AnnouncementRequestDelivery{}, err
	}
	return d, nil
}

// UpsertDeliveries implements AnnouncementRequestRepository. Runs every
// input's upsert inside one transaction — a Publish fan-out pass either
// records completely or not at all, never a partial batch that could leave
// this table disagreeing with what the caller's own in-memory tally says
// happened for that same pass.
func (r *announcementRequestRepo) UpsertDeliveries(ctx context.Context, announcementRequestID string, deliveries []domain.RecordAnnouncementRequestDeliveryInput) ([]domain.AnnouncementRequestDelivery, error) {
	if len(deliveries) == 0 {
		return nil, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("upsert announcement_request_deliveries: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO announcement_request_deliveries
			(announcement_request_id, project_id, case_id, status, error_message)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (announcement_request_id, project_id) DO UPDATE SET
			case_id = EXCLUDED.case_id,
			status = EXCLUDED.status,
			error_message = EXCLUDED.error_message,
			updated_on = NOW()
		RETURNING ` + announcementRequestDeliveryColumns

	results := make([]domain.AnnouncementRequestDelivery, 0, len(deliveries))
	for _, d := range deliveries {
		row, err := scanAnnouncementRequestDelivery(tx.QueryRow(ctx, query,
			announcementRequestID, d.ProjectID, d.CaseID, d.Status, d.ErrorMessage,
		))
		if err != nil {
			return nil, fmt.Errorf("upsert announcement_request_delivery for project %s: %w", d.ProjectID, err)
		}
		results = append(results, row)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("upsert announcement_request_deliveries: commit: %w", err)
	}
	return results, nil
}

// ListDeliveries implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) ListDeliveries(ctx context.Context, announcementRequestID string) ([]domain.AnnouncementRequestDelivery, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+announcementRequestDeliveryColumns+`
		 FROM announcement_request_deliveries
		 WHERE announcement_request_id = $1
		 ORDER BY created_on`, announcementRequestID)
	if err != nil {
		return nil, fmt.Errorf("list announcement_request_deliveries: %w", err)
	}
	defer rows.Close()

	deliveries := []domain.AnnouncementRequestDelivery{}
	for rows.Next() {
		d, err := scanAnnouncementRequestDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan announcement_request_delivery: %w", err)
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate announcement_request_deliveries: %w", err)
	}
	return deliveries, nil
}
