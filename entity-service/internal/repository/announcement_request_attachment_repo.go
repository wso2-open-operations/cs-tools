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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// announcementRequestAttachmentColumns is the metadata column list (never the
// bytes) shared by every query that returns an attachment, kept in one place
// so it cannot drift from scanAnnouncementRequestAttachment's field order.
const announcementRequestAttachmentColumns = `
	id, announcement_request_id, name, mime_type, size_bytes, created_by, created_by_email, created_on`

func scanAnnouncementRequestAttachment(row pgx.Row) (domain.AnnouncementRequestAttachment, error) {
	var a domain.AnnouncementRequestAttachment
	var size int64
	if err := row.Scan(&a.ID, &a.AnnouncementRequestID, &a.Name, &a.Type, &size, &a.CreatedBy, &a.CreatedByEmail, &a.CreatedOn); err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	a.SizeBytes = int(size)
	return a, nil
}

// guardRequestForAttachmentChange is the first statement of every attachment
// write. It is the state-conditioned write of the request row itself, so the
// two things the attachment change depends on hold atomically with it, in the
// same way every state transition repeats its state in its own WHERE clause
// (see the doc comment on Update): the request is still in the state the
// service saw, and its row is locked until commit, which serialises two
// concurrent attachment changes on one request (the file count and the
// name check below cannot be raced past the limit).
//
// With revertToDraft the request is in pending_approval and is sent back to
// draft by the change, like an edit of its content (see RevertToDraft: the
// audience snapshot, the dry run and the submission are cleared, because what
// was sent for review is no longer what is being changed to).
func guardRequestForAttachmentChange(ctx context.Context, tx pgx.Tx, id string, expectedState domain.AnnouncementRequestState, revertToDraft bool) (bool, error) {
	var query string
	var args []any
	if revertToDraft {
		query = `
			UPDATE announcement_requests SET
				state = 'draft',
				resolved_project_ids = NULL, resolved_project_count = NULL,
				dry_run_case_id = NULL, dry_run_on = NULL, dry_run_by = NULL,
				submitted_by = NULL, submitted_on = NULL,
				updated_on = NOW()
			WHERE id = $1 AND state = 'pending_approval'`
		args = []any{id}
	} else {
		query = `UPDATE announcement_requests SET updated_on = NOW() WHERE id = $1 AND state = $2`
		args = []any{id, expectedState}
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("guard announcement_request for attachment change: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// guardNoDeliveries refuses a change to a request's files once a case exists for
// any of its projects (a delivery recorded as succeeded, or as created but short
// of its follow-up). Every announcement case of a request is given the request's
// files as they are when its turn comes, so a change after the first case would
// leave the projects already sent with one set and the ones still to come (or
// being retried after a failed follow-up step) with another, one announcement in
// two versions. A delivery recorded as failed made no case, so it freezes nothing.
// Called inside the change's own transaction, after the request has been guarded,
// so the refusal rolls the guard write back too.
func guardNoDeliveries(ctx context.Context, tx pgx.Tx, id, action string) error {
	var delivered bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM announcement_request_deliveries
			 WHERE announcement_request_id = $1 AND status <> 'failed')`, id,
	).Scan(&delivered); err != nil {
		return fmt.Errorf("check announcement_request_deliveries before %s: %w", action, err)
	}
	if delivered {
		return &apierror.ConflictError{Msg: action + ": some projects have already received this announcement, so its files can no longer change"}
	}
	return nil
}

// AddAttachment implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) AddAttachment(ctx context.Context, id string, expectedState domain.AnnouncementRequestState, revertToDraft bool, att domain.AnnouncementRequestAttachmentContent) (domain.AnnouncementRequestAttachment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, fmt.Errorf("add announcement_request_attachment: begin: %w", err)
	}
	// Rolled back on every path but a successful commit (a no-op after one).
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	guarded, err := guardRequestForAttachmentChange(ctx, tx, id, expectedState, revertToDraft)
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}
	if !guarded {
		return domain.AnnouncementRequestAttachment{}, r.onConflictOrNotFound(ctx, id, "attach file")
	}
	if err := guardNoDeliveries(ctx, tx, id, "attach file"); err != nil {
		return domain.AnnouncementRequestAttachment{}, err
	}

	var count int
	var duplicate bool
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(BOOL_OR(LOWER(name) = LOWER($2)), FALSE)
		   FROM announcement_request_attachment WHERE announcement_request_id = $1`,
		id, att.Name,
	).Scan(&count, &duplicate); err != nil {
		return domain.AnnouncementRequestAttachment{}, fmt.Errorf("add announcement_request_attachment: count: %w", err)
	}
	if count >= domain.MaxAnnouncementRequestAttachments {
		return domain.AnnouncementRequestAttachment{}, &apierror.ValidationError{Msg: fmt.Sprintf("a request can carry at most %d files; remove one to attach another", domain.MaxAnnouncementRequestAttachments)}
	}
	if duplicate {
		return domain.AnnouncementRequestAttachment{}, &apierror.ValidationError{Msg: fmt.Sprintf("a file named %q is already attached to this request", att.Name)}
	}

	saved, err := scanAnnouncementRequestAttachment(tx.QueryRow(ctx,
		`INSERT INTO announcement_request_attachment
			(announcement_request_id, name, mime_type, size_bytes, content, created_by, created_by_email)
		 VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''))
		 RETURNING `+announcementRequestAttachmentColumns,
		id, att.Name, att.Type, int64(att.SizeBytes), att.Content, att.CreatedBy, derefString(att.CreatedByEmail),
	))
	if err != nil {
		return domain.AnnouncementRequestAttachment{}, fmt.Errorf("add announcement_request_attachment: insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AnnouncementRequestAttachment{}, fmt.Errorf("add announcement_request_attachment: commit: %w", err)
	}
	return saved, nil
}

// DeleteAttachment implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) DeleteAttachment(ctx context.Context, id, attachmentID string, expectedState domain.AnnouncementRequestState, revertToDraft bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("delete announcement_request_attachment: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	guarded, err := guardRequestForAttachmentChange(ctx, tx, id, expectedState, revertToDraft)
	if err != nil {
		return err
	}
	if !guarded {
		return r.onConflictOrNotFound(ctx, id, "remove file")
	}
	if err := guardNoDeliveries(ctx, tx, id, "remove file"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`DELETE FROM announcement_request_attachment WHERE id = $1 AND announcement_request_id = $2`,
		attachmentID, id)
	if err != nil {
		return fmt.Errorf("delete announcement_request_attachment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Returning here rolls the guard write back too: removing a file the
		// request never had must not send a request in review back to draft.
		return &apierror.NotFoundError{Msg: "announcement request attachment not found: " + attachmentID}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("delete announcement_request_attachment: commit: %w", err)
	}
	return nil
}

// ListAttachments implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) ListAttachments(ctx context.Context, id string) ([]domain.AnnouncementRequestAttachment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+announcementRequestAttachmentColumns+`
		   FROM announcement_request_attachment
		  WHERE announcement_request_id = $1
		  ORDER BY created_on, id`, id)
	if err != nil {
		return nil, fmt.Errorf("list announcement_request_attachment: %w", err)
	}
	defer rows.Close()

	attachments := []domain.AnnouncementRequestAttachment{}
	for rows.Next() {
		a, err := scanAnnouncementRequestAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan announcement_request_attachment: %w", err)
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate announcement_request_attachment: %w", err)
	}
	return attachments, nil
}

// GetAttachmentContent implements AnnouncementRequestRepository.
func (r *announcementRequestRepo) GetAttachmentContent(ctx context.Context, id, attachmentID string) (domain.AnnouncementRequestAttachmentContent, error) {
	var out domain.AnnouncementRequestAttachmentContent
	var size int64
	err := r.db.QueryRow(ctx,
		`SELECT `+announcementRequestAttachmentColumns+`, content
		   FROM announcement_request_attachment
		  WHERE id = $1 AND announcement_request_id = $2`,
		attachmentID, id,
	).Scan(&out.ID, &out.AnnouncementRequestID, &out.Name, &out.Type, &size, &out.CreatedBy, &out.CreatedByEmail, &out.CreatedOn, &out.Content)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AnnouncementRequestAttachmentContent{}, &apierror.NotFoundError{Msg: "announcement request attachment not found: " + attachmentID}
		}
		return domain.AnnouncementRequestAttachmentContent{}, fmt.Errorf("get announcement_request_attachment content: %w", err)
	}
	out.SizeBytes = int(size)
	return out, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
