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
	// RecordDryRun sets dry_run_case_id/dry_run_at/dry_run_by.
	RecordDryRun(ctx context.Context, id string, req domain.RecordAnnouncementDryRunRequest) (domain.AnnouncementRequest, error)
	// Submit moves state to pending_approval, freezes resolved_project_ids/
	// resolved_project_count, and sets submitted_by/submitted_at. The
	// caller (service layer) has already validated the current state and
	// the dry-run precondition before this is called.
	Submit(ctx context.Context, id string, req domain.SubmitAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// Approve moves state to approved and sets approved_by/approved_at.
	Approve(ctx context.Context, id, actorID string) (domain.AnnouncementRequest, error)
	// RevertToDraft moves state back to draft, clearing
	// resolved_project_ids/resolved_project_count/dry_run_case_id/
	// dry_run_at/dry_run_by/submitted_by/submitted_at, and — in the same
	// statement — applies whatever content fields the caller also supplied
	// (the "edit while pending_approval" path is one atomic operation, not
	// a revert followed by a separate update).
	RevertToDraft(ctx context.Context, id string, req domain.UpdateAnnouncementRequestRequest) (domain.AnnouncementRequest, error)
	// MarkPublished moves state to published and sets published_by/published_at.
	MarkPublished(ctx context.Context, id, actorID string) (domain.AnnouncementRequest, error)
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
	dry_run_case_id, dry_run_at, dry_run_by,
	created_by, created_at, updated_at,
	submitted_by, submitted_at, approved_by, approved_at, published_by, published_at`

func scanAnnouncementRequest(row pgx.Row) (domain.AnnouncementRequest, error) {
	var r domain.AnnouncementRequest
	var resolvedProjectIDsRaw []byte
	if err := row.Scan(
		&r.ID, &r.Kind, &r.State, &r.Subject, &r.Description, &r.IsSecurityAnnouncement,
		&r.AudienceDefinition, &resolvedProjectIDsRaw, &r.ResolvedProjectCount,
		&r.DryRunCaseID, &r.DryRunAt, &r.DryRunBy,
		&r.CreatedBy, &r.CreatedAt, &r.UpdatedAt,
		&r.SubmittedBy, &r.SubmittedAt, &r.ApprovedBy, &r.ApprovedAt, &r.PublishedBy, &r.PublishedAt,
	); err != nil {
		return domain.AnnouncementRequest{}, err
	}
	// resolved_project_ids is JSONB and nil until Submit — decoded
	// explicitly (not left to pgx's default codec) so a NULL column reads
	// back as a nil slice rather than requiring a *[]string juggling act at
	// every call site.
	if len(resolvedProjectIDsRaw) > 0 {
		if err := json.Unmarshal(resolvedProjectIDsRaw, &r.ResolvedProjectIDs); err != nil {
			return domain.AnnouncementRequest{}, fmt.Errorf("decode resolved_project_ids: %w", err)
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
		INSERT INTO announcement_requests (kind, subject, description, is_security_announcement, audience_definition, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query,
		req.Kind, req.Subject, req.Description, req.IsSecurityAnnouncement, audience, req.CreatedBy,
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
	// needing to build the WHERE clause dynamically.
	const where = `WHERE ($1::text IS NULL OR state = $1) AND ($2::text IS NULL OR created_by = $2)`
	countQuery := `SELECT COUNT(*) FROM announcement_requests ` + where
	dataQuery := `SELECT ` + announcementRequestColumns + ` FROM announcement_requests ` + where + `
		ORDER BY created_at DESC, id
		LIMIT $3 OFFSET $4`

	var state *string
	if req.State != nil {
		s := string(*req.State)
		state = &s
	}

	var total int
	var requests []domain.AnnouncementRequest
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return r.db.QueryRow(egCtx, countQuery, state, req.CreatedBy).Scan(&total)
	})
	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, state, req.CreatedBy, req.Pagination.Limit, req.Pagination.Offset)
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
			updated_at = NOW()
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
			dry_run_case_id = $2, dry_run_at = NOW(), dry_run_by = $3, updated_at = NOW()
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
			submitted_by = $4, submitted_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND state = 'draft' AND dry_run_case_id IS NOT NULL
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, projectIDs, len(req.ResolvedProjectIDs), req.ActorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "submit for approval")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("submit announcement_request: %w", err)
	}
	return ar, nil
}

// Approve implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) Approve(ctx context.Context, id, actorID string) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			state = 'approved', approved_by = $2, approved_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND state = 'pending_approval'
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, actorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "approve")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("approve announcement_request: %w", err)
	}
	return ar, nil
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
			dry_run_case_id = NULL, dry_run_at = NULL, dry_run_by = NULL,
			submitted_by = NULL, submitted_at = NULL,
			updated_at = NOW()
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
func (r *announcementRequestRepo) MarkPublished(ctx context.Context, id, actorID string) (domain.AnnouncementRequest, error) {
	query := `
		UPDATE announcement_requests SET
			state = 'published', published_by = $2, published_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND state = 'approved'
		RETURNING ` + announcementRequestColumns

	ar, err := scanAnnouncementRequest(r.db.QueryRow(ctx, query, id, actorID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequest{}, r.onConflictOrNotFound(ctx, id, "publish")
		}
		return domain.AnnouncementRequest{}, fmt.Errorf("mark announcement_request published: %w", err)
	}
	return ar, nil
}
