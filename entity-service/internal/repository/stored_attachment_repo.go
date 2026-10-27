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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// StoredAttachmentRepository persists the three attachment tables whose files
// live in SFTPGo: work_item_attachment, comment_attachment and
// deployment_attachment.
//
// All three are row-level-security protected (migration 100026), so every read
// and the INSERT run as the caller: a row the caller may not see is simply not
// returned, and an external caller can only INSERT a row that is not stored
// yet. UPDATE is internal-only by design, so MarkComplete and MarkFailed are
// meant to be called with repository.WithSystemIdentity on the context, after
// the service has checked the caller itself.
type StoredAttachmentRepository interface {
	// GetParent returns the work item (kinds work_item and comment) or
	// deployment an attachment hangs from, as the caller may see it. A parent
	// that does not exist, or that the caller may not see, is a
	// *apierror.NotFoundError.
	GetParent(ctx context.Context, kind domain.StoredAttachmentKind, parentID string) (domain.StoredAttachmentParent, error)
	// InsertPending adds a not-yet-stored attachment (status PENDING, no
	// storage_key) and returns it with its generated id and number. A parent
	// the caller may not write to is a *apierror.NotFoundError.
	InsertPending(ctx context.Context, kind domain.StoredAttachmentKind, in domain.StoredAttachmentInsert) (domain.StoredAttachmentRow, error)
	// GetByIDs returns the rows with those ids that the caller may see.
	GetByIDs(ctx context.Context, kind domain.StoredAttachmentKind, ids []string) ([]domain.StoredAttachmentRow, error)
	// MarkComplete records where a PENDING attachment's file now is. It reports
	// false when the row is no longer PENDING (another request confirmed it).
	MarkComplete(ctx context.Context, kind domain.StoredAttachmentKind, id, storageKey string, sizeBytes int64) (bool, error)
	// MarkFailed records why a PENDING attachment's upload was rejected.
	MarkFailed(ctx context.Context, kind domain.StoredAttachmentKind, id, detail string) error
	// MarkDeleted soft-deletes a COMPLETE attachment: status DELETED, with who
	// and when. It reports false when the row is no longer COMPLETE (already
	// deleted by another request). Meant for repository.WithSystemIdentity.
	MarkDeleted(ctx context.Context, kind domain.StoredAttachmentKind, id, deletedBy string) (bool, error)
	// ListAbandoned returns up to limit uploads, across the three tables, that
	// are still PENDING and were created before olderThan. Meant for
	// repository.WithSystemIdentity.
	ListAbandoned(ctx context.Context, olderThan time.Time, limit int) ([]domain.AbandonedUpload, error)
	// LinkInlineImages attaches to a comment the images in ids that are stored,
	// unlinked, on that work item and uploaded by email, and returns the ids it
	// linked (repeating the call for an already linked image returns it again).
	// Meant for repository.WithSystemIdentity: UPDATE is internal-only.
	LinkInlineImages(ctx context.Context, commentID, workItemID, email string, ids []string) ([]string, error)
	// ListUnlinkedImages returns up to limit stored comment images, created before
	// olderThan, that no comment was ever linked to. Meant for WithSystemIdentity.
	ListUnlinkedImages(ctx context.Context, olderThan time.Time, limit int) ([]domain.StoredAttachmentRow, error)
}

// callerError turns "the request carried no caller identity" into a 401: a
// request with no verified token is the caller's to fix, not a server fault.
func callerError(err error) error {
	if errors.Is(err, ErrNoCallerIdentity) {
		return &apierror.UnauthorizedError{Msg: "a verified user token (x-user-id-token) is required"}
	}
	return err
}

type storedAttachmentRepo struct {
	db *Scoped
}

// NewStoredAttachmentRepository constructs a StoredAttachmentRepository. It
// holds a *Scoped, not a raw pool: every table it touches is RLS-protected.
func NewStoredAttachmentRepository(db *Scoped) StoredAttachmentRepository {
	return &storedAttachmentRepo{db: db}
}

// storedAttachmentTable names an attachment table and the column that links a
// row to its parent. The values come from a closed map keyed by kind, never
// from the request, which is what makes interpolating them into SQL safe.
type storedAttachmentTable struct {
	table     string
	parentCol string
}

var storedAttachmentTables = map[domain.StoredAttachmentKind]storedAttachmentTable{
	domain.StoredAttachmentWorkItem: {"work_item_attachment", "work_item_id"},
	// A comment image is linked to its work item at upload and to its comment
	// when the comment is saved (migration 0150).
	domain.StoredAttachmentComment:    {"comment_attachment", "work_item_id"},
	domain.StoredAttachmentDeployment: {"deployment_attachment", "deployment_id"},
}

func storedAttachmentTableFor(kind domain.StoredAttachmentKind) (storedAttachmentTable, error) {
	t, ok := storedAttachmentTables[kind]
	if !ok {
		return storedAttachmentTable{}, &apierror.ValidationError{Msg: "kind must be one of work_item, comment, deployment"}
	}
	return t, nil
}

// maxStatusDetailLen caps what MarkFailed stores in status_detail.
const maxStatusDetailLen = 500

// GetParent implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) GetParent(ctx context.Context, kind domain.StoredAttachmentKind, parentID string) (domain.StoredAttachmentParent, error) {
	if _, err := storedAttachmentTableFor(kind); err != nil {
		return domain.StoredAttachmentParent{}, err
	}
	var p domain.StoredAttachmentParent
	var err error
	if kind == domain.StoredAttachmentDeployment {
		err = r.db.QueryRow(ctx, `
			SELECT d.id::TEXT, d.number, COALESCE(p.number, '')
			FROM deployment d
			LEFT JOIN project p ON p.id = d.project_id
			WHERE d.id = $1`, parentID).Scan(&p.ID, &p.Number, &p.ProjectNumber)
	} else {
		// "Closed" is read from the work item's OWN type's extension table, the
		// same way every closed filter on a case list does (caseLikeOwnStateColumn).
		// A type with no state of its own (a task, say) reads as open.
		err = r.db.QueryRow(ctx, `
			SELECT wi.id::TEXT, wi.number, COALESCE(p.number, ''),
			       COALESCE(`+caseLikeOwnStateColumn+`, '') = 'CLOSED'
			FROM work_item wi
			LEFT JOIN project p ON p.id = wi.project_id
			LEFT JOIN "case" c ON c.id = wi.id`+caseLikeJoins+`
			WHERE wi.id = $1`, parentID).Scan(&p.ID, &p.Number, &p.ProjectNumber, &p.Closed)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredAttachmentParent{}, &apierror.NotFoundError{Msg: "parent record not found"}
		}
		return domain.StoredAttachmentParent{}, callerError(fmt.Errorf("get attachment parent: %w", err))
	}
	return p, nil
}

// storedAttachmentColumns is the column list every query returning a row uses,
// kept next to scanStoredAttachment so the two cannot drift apart.
func storedAttachmentColumns(t storedAttachmentTable) string {
	return `id::TEXT, number, name, content_type, size_bytes, status::TEXT, storage_key, created_by, ` + t.parentCol + `::TEXT`
}

func scanStoredAttachment(row pgx.Row) (domain.StoredAttachmentRow, error) {
	var a domain.StoredAttachmentRow
	err := row.Scan(&a.ID, &a.Number, &a.Name, &a.ContentType, &a.SizeBytes, &a.Status, &a.StorageKey, &a.CreatedBy, &a.ParentID)
	return a, err
}

// InsertPending implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) InsertPending(ctx context.Context, kind domain.StoredAttachmentKind, in domain.StoredAttachmentInsert) (domain.StoredAttachmentRow, error) {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return domain.StoredAttachmentRow{}, err
	}
	// The id is generated here: of the three tables only work_item_attachment has
	// an id default (migration 0179). number comes from its own column default
	// (WIA/CA/DA + a sequence); status defaults to PENDING and storage_key stays
	// NULL, which is exactly what the external INSERT policy requires.
	row := r.db.QueryRow(ctx, `
		INSERT INTO `+t.table+` (id, created_on, updated_on, created_by, updated_by, name, content_type, size_bytes, `+t.parentCol+`)
		VALUES (gen_random_uuid(), now(), now(), $1, $1, $2, $3, $4, $5)
		RETURNING `+storedAttachmentColumns(t),
		in.CreatedBy, in.Name, in.ContentType, in.SizeBytes, in.ParentID)
	a, err := scanStoredAttachment(row)
	if err != nil {
		var pgErr *pgconn.PgError
		// 42501: a row-level-security policy refused the insert, which is what a
		// parent the caller is not a member of looks like. Reported as not found,
		// like every other record the caller may not see.
		if errors.As(err, &pgErr) && pgErr.Code == "42501" {
			return domain.StoredAttachmentRow{}, &apierror.NotFoundError{Msg: "parent record not found"}
		}
		// 23503: the parent id references nothing.
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.StoredAttachmentRow{}, &apierror.NotFoundError{Msg: "parent record not found"}
		}
		return domain.StoredAttachmentRow{}, callerError(fmt.Errorf("insert pending attachment: %w", err))
	}
	return a, nil
}

// GetByIDs implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) GetByIDs(ctx context.Context, kind domain.StoredAttachmentKind, ids []string) ([]domain.StoredAttachmentRow, error) {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `SELECT `+storedAttachmentColumns(t)+` FROM `+t.table+` WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, callerError(fmt.Errorf("get attachments: %w", err))
	}
	defer rows.Close()
	var out []domain.StoredAttachmentRow
	for rows.Next() {
		a, err := scanStoredAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}
	return out, nil
}

// MarkComplete implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) MarkComplete(ctx context.Context, kind domain.StoredAttachmentKind, id, storageKey string, sizeBytes int64) (bool, error) {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return false, err
	}
	ct, err := r.db.Exec(ctx, `
		UPDATE `+t.table+`
		SET status = 'COMPLETE', storage_key = $2, size_bytes = $3, stored_on = now(), status_detail = NULL, updated_on = now()
		WHERE id = $1 AND status = 'PENDING'`, id, storageKey, sizeBytes)
	if err != nil {
		return false, fmt.Errorf("mark attachment complete: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

// MarkFailed implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) MarkFailed(ctx context.Context, kind domain.StoredAttachmentKind, id, detail string) error {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(detail) > maxStatusDetailLen {
		detail = string([]rune(detail)[:maxStatusDetailLen])
	}
	if _, err := r.db.Exec(ctx, `
		UPDATE `+t.table+`
		SET status = 'FAILED', status_detail = $2, updated_on = now()
		WHERE id = $1 AND status = 'PENDING'`, id, detail); err != nil {
		return fmt.Errorf("mark attachment failed: %w", err)
	}
	return nil
}

// MarkDeleted implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) MarkDeleted(ctx context.Context, kind domain.StoredAttachmentKind, id, deletedBy string) (bool, error) {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return false, err
	}
	ct, err := r.db.Exec(ctx, `
		UPDATE `+t.table+`
		SET status = 'DELETED', deleted_on = now(), deleted_by = $2, updated_on = now(), updated_by = $2
		WHERE id = $1 AND status = 'COMPLETE'`, id, deletedBy)
	if err != nil {
		return false, fmt.Errorf("mark attachment deleted: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

// ListAbandoned implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) ListAbandoned(ctx context.Context, olderThan time.Time, limit int) ([]domain.AbandonedUpload, error) {
	var out []domain.AbandonedUpload
	for _, kind := range []domain.StoredAttachmentKind{domain.StoredAttachmentWorkItem, domain.StoredAttachmentComment, domain.StoredAttachmentDeployment} {
		if len(out) >= limit {
			break
		}
		t, err := storedAttachmentTableFor(kind)
		if err != nil {
			return nil, err
		}
		rows, err := r.db.Query(ctx, `SELECT id::text FROM `+t.table+` WHERE status = 'PENDING' AND created_on < $1 ORDER BY created_on LIMIT $2`, olderThan, limit-len(out))
		if err != nil {
			return nil, fmt.Errorf("list abandoned uploads: %w", err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan abandoned upload: %w", err)
			}
			out = append(out, domain.AbandonedUpload{Kind: kind, ID: id})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate abandoned uploads: %w", err)
		}
	}
	return out, nil
}

// LinkInlineImages implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) LinkInlineImages(ctx context.Context, commentID, workItemID, email string, ids []string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE comment_attachment
		SET comment_id = $1, updated_on = now()
		WHERE id = ANY($4::uuid[])
		  AND work_item_id = $2
		  AND status = 'COMPLETE'
		  AND lower(created_by) = lower($3)
		  AND (comment_id IS NULL OR comment_id = $1)
		RETURNING id::TEXT`, commentID, workItemID, email, ids)
	if err != nil {
		return nil, fmt.Errorf("link inline images: %w", err)
	}
	defer rows.Close()
	var linked []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan linked image: %w", err)
		}
		linked = append(linked, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate linked images: %w", err)
	}
	return linked, nil
}

// ListUnlinkedImages implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) ListUnlinkedImages(ctx context.Context, olderThan time.Time, limit int) ([]domain.StoredAttachmentRow, error) {
	t, err := storedAttachmentTableFor(domain.StoredAttachmentComment)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+storedAttachmentColumns(t)+` FROM comment_attachment
		WHERE comment_id IS NULL AND status = 'COMPLETE' AND created_on < $1
		ORDER BY created_on LIMIT $2`, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("list unlinked images: %w", err)
	}
	defer rows.Close()
	var out []domain.StoredAttachmentRow
	for rows.Next() {
		a, err := scanStoredAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan unlinked image: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unlinked images: %w", err)
	}
	return out, nil
}
