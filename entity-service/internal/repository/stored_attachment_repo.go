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
// live in Azure Blob Storage: work_item_attachment, comment_attachment and
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
	// UpdateStorageKey changes the stored key of a COMPLETE attachment from `from`
	// to `to`, and reports whether it did (false when the row no longer has the key
	// `from`, so a repeated call is harmless). Meant for repository.WithSystemIdentity.
	UpdateStorageKey(ctx context.Context, kind domain.StoredAttachmentKind, id, from, to string) (bool, error)
	// ListAbandoned returns up to limit uploads, across the three tables, that
	// are still PENDING and were created before olderThan. Meant for
	// repository.WithSystemIdentity.
	ListAbandoned(ctx context.Context, olderThan time.Time, limit int) ([]domain.AbandonedUpload, error)
	// CountPendingStagedUploads returns how many uploads by email, across the three
	// tables, are still PENDING and were created after since, among the uploads
	// that may be sent straight to storage.
	CountPendingStagedUploads(ctx context.Context, email string, since time.Time) (int, error)
	// LinkInlineImages attaches to a comment the images in ids that are stored,
	// unlinked, on that work item and uploaded by email, and returns the ids it
	// linked (repeating the call for an already linked image returns it again).
	// Meant for repository.WithSystemIdentity: UPDATE is internal-only.
	LinkInlineImages(ctx context.Context, commentID, workItemID, email string, ids []string) ([]string, error)
	// LinkedImageIDs returns those of ids that are already linked to the comment.
	LinkedImageIDs(ctx context.Context, commentID string, ids []string) ([]string, error)
	// GetProject returns the project a draft description image is uploaded to,
	// as the caller may see it: an external caller must be a REGISTERED member,
	// an internal one is unrestricted. A project that does not exist or that the
	// caller may not see is a *apierror.NotFoundError. Only ID, Number and
	// ProjectNumber of the result are set (the last two are the project's number).
	GetProject(ctx context.Context, projectID string) (domain.StoredAttachmentParent, error)
	// ListDescriptionDrafts returns the COMPLETE draft description images among
	// ids: uploaded to that project by email, not linked to a comment or work
	// item. Meant for repository.WithSystemIdentity (the filters are explicit).
	ListDescriptionDrafts(ctx context.Context, projectID, email string, ids []string) ([]domain.StoredAttachmentRow, error)
	// LinkDescriptionImages links the drafts in ids to the work item whose
	// description shows them: work_item_id set and description_image true, in one
	// guarded UPDATE that only touches a row that is still a draft of that project
	// uploaded by email. It returns the ids now linked to that work item (repeating
	// the call returns them again). Meant for repository.WithSystemIdentity: UPDATE
	// is internal-only.
	LinkDescriptionImages(ctx context.Context, workItemID, projectID, email string, ids []string) ([]string, error)
	// ListReportDrafts returns the COMPLETE draft work item attachments (reports
	// uploaded before their case exists) among ids: uploaded to that project by
	// email, not linked to a work item, and uploaded at or after notBefore (an
	// older one is on its way to the cleanup). Meant for repository.WithSystemIdentity
	// (the filters are explicit).
	ListReportDrafts(ctx context.Context, projectID, email string, ids []string, notBefore time.Time) ([]domain.StoredAttachmentRow, error)
	// ListUnlinkedReportDrafts returns up to limit stored report drafts, created
	// before olderThan, that no case was ever linked to. Meant for WithSystemIdentity.
	ListUnlinkedReportDrafts(ctx context.Context, olderThan time.Time, limit int) ([]domain.StoredAttachmentRow, error)
	// LinkReportDrafts links the drafts in ids to the work item that was just
	// created: work_item_id set, in one guarded UPDATE that only touches a row that
	// is still a draft of that project, stored, and uploaded by email. It returns
	// the ids now linked to that work item (repeating the call returns them again).
	// Meant for repository.WithSystemIdentity: UPDATE is internal-only.
	LinkReportDrafts(ctx context.Context, workItemID, projectID, email string, ids []string) ([]string, error)
	// ListUnlinkedImages returns up to limit stored comment images, created before
	// olderThan, that no comment was ever linked to. Draft description images
	// (no work item yet) count; an image linked to a work item's description
	// (description_image = true) never does. Meant for WithSystemIdentity.
	ListUnlinkedImages(ctx context.Context, olderThan time.Time, limit int) ([]domain.StoredAttachmentRow, error)
	// RecordCleanupAttempt moves a row to the back of the cleanup queue (updated_on =
	// now) after a cleanup of it failed, so rows that keep failing cannot fill every
	// run's limited batch and starve the ones behind them. Meant for WithSystemIdentity.
	RecordCleanupAttempt(ctx context.Context, kind domain.StoredAttachmentKind, id string) error
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
	// parentExpr is the SQL that yields the parent id as text (parentCol, except
	// for a comment image that is still a draft: its project) and draftExpr the SQL
	// boolean "this is a draft description image".
	parentExpr string
	draftExpr  string
}

var storedAttachmentTables = map[domain.StoredAttachmentKind]storedAttachmentTable{
	// A draft report (migration 0280) has only a project_id until the case it
	// belongs to is created: its parent reads as that project until it is linked.
	domain.StoredAttachmentWorkItem: {"work_item_attachment", "work_item_id",
		"COALESCE(work_item_id, project_id)::TEXT",
		"(work_item_id IS NULL AND project_id IS NOT NULL)"},
	// A comment image is linked to its work item at upload and to its comment
	// when the comment is saved (migration 0150).
	// A draft description image (migration 0250) has only a project_id: its parent
	// reads as that project until it is linked to the work item.
	domain.StoredAttachmentComment: {"comment_attachment", "work_item_id",
		"COALESCE(work_item_id, project_id)::TEXT",
		"(comment_id IS NULL AND work_item_id IS NULL AND project_id IS NOT NULL)"},
	domain.StoredAttachmentDeployment: {"deployment_attachment", "deployment_id", "deployment_id::TEXT", "false"},
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
	return `id::TEXT, number, name, content_type, size_bytes, status::TEXT, storage_key, created_by, ` + t.parentExpr + `, ` + t.draftExpr + `, staged`
}

func scanStoredAttachment(row pgx.Row) (domain.StoredAttachmentRow, error) {
	var a domain.StoredAttachmentRow
	err := row.Scan(&a.ID, &a.Number, &a.Name, &a.ContentType, &a.SizeBytes, &a.Status, &a.StorageKey, &a.CreatedBy, &a.ParentID, &a.Draft, &a.Staged)
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
	parentCol := t.parentCol
	if in.Draft {
		if kind != domain.StoredAttachmentComment && kind != domain.StoredAttachmentWorkItem {
			return domain.StoredAttachmentRow{}, &apierror.ValidationError{Msg: "only a comment image or a work item attachment can be a draft"}
		}
		// A draft description image or report has a project and no work item yet.
		parentCol = "project_id"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO `+t.table+` (id, created_on, updated_on, created_by, updated_by, name, content_type, size_bytes, `+parentCol+`, staged)
		VALUES (gen_random_uuid(), now(), now(), $1, $1, $2, $3, $4, $5, $6)
		RETURNING `+storedAttachmentColumns(t),
		in.CreatedBy, in.Name, in.ContentType, in.SizeBytes, in.ParentID, in.Staged)
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

// GetProject implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) GetProject(ctx context.Context, projectID string) (domain.StoredAttachmentParent, error) {
	var p domain.StoredAttachmentParent
	// project itself has no row-level security, so the caller's membership is
	// checked here with the same function the attachment policies use.
	err := r.db.QueryRow(ctx, `
		SELECT p.id::TEXT, COALESCE(p.number, '')
		FROM project p
		WHERE p.id = $1
		  AND ((SELECT current_setting('app.is_internal', true) = 'true') OR is_project_member(p.id))`,
		projectID).Scan(&p.ID, &p.ProjectNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StoredAttachmentParent{}, &apierror.NotFoundError{Msg: "project not found"}
		}
		return domain.StoredAttachmentParent{}, callerError(fmt.Errorf("get attachment project: %w", err))
	}
	p.Number = p.ProjectNumber
	return p, nil
}

// ListDescriptionDrafts implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) ListDescriptionDrafts(ctx context.Context, projectID, email string, ids []string) ([]domain.StoredAttachmentRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	t, err := storedAttachmentTableFor(domain.StoredAttachmentComment)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+storedAttachmentColumns(t)+` FROM comment_attachment
		WHERE id = ANY($1::uuid[])
		  AND project_id = $2
		  AND comment_id IS NULL AND work_item_id IS NULL AND description_image = false
		  AND status = 'COMPLETE'
		  AND lower(created_by) = lower($3)`, ids, projectID, email)
	if err != nil {
		return nil, callerError(fmt.Errorf("list description drafts: %w", err))
	}
	defer rows.Close()
	var out []domain.StoredAttachmentRow
	for rows.Next() {
		a, err := scanStoredAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan description draft: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate description drafts: %w", err)
	}
	return out, nil
}

// LinkDescriptionImages implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) LinkDescriptionImages(ctx context.Context, workItemID, projectID, email string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// The WHERE is the guard: a row is only taken while it is still a draft of
	// this project, stored, and the author's. An image already linked to this same
	// work item is returned again, so a repeated call is harmless. The storage key
	// (built from the project number and "drafts") is left as it is.
	rows, err := r.db.Query(ctx, `
		UPDATE comment_attachment
		SET work_item_id = $1, description_image = true, updated_on = now()
		WHERE id = ANY($4::uuid[])
		  AND project_id = $2
		  AND status = 'COMPLETE'
		  AND lower(created_by) = lower($3)
		  AND comment_id IS NULL
		  AND ((work_item_id IS NULL AND description_image = false)
		       OR (work_item_id = $1 AND description_image = true))
		RETURNING id::TEXT`, workItemID, projectID, email, ids)
	if err != nil {
		return nil, fmt.Errorf("link description images: %w", err)
	}
	defer rows.Close()
	var linked []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan linked description image: %w", err)
		}
		linked = append(linked, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate linked description images: %w", err)
	}
	return linked, nil
}

// ListReportDrafts implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) ListReportDrafts(ctx context.Context, projectID, email string, ids []string, notBefore time.Time) ([]domain.StoredAttachmentRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	t, err := storedAttachmentTableFor(domain.StoredAttachmentWorkItem)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+storedAttachmentColumns(t)+` FROM work_item_attachment
		WHERE id = ANY($1::uuid[])
		  AND project_id = $2
		  AND work_item_id IS NULL
		  AND status = 'COMPLETE'
		  AND lower(created_by) = lower($3)
		  AND created_on >= $4`, ids, projectID, email, notBefore)
	if err != nil {
		return nil, callerError(fmt.Errorf("list report drafts: %w", err))
	}
	defer rows.Close()
	var out []domain.StoredAttachmentRow
	for rows.Next() {
		a, err := scanStoredAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report draft: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report drafts: %w", err)
	}
	return out, nil
}

// LinkReportDrafts implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) LinkReportDrafts(ctx context.Context, workItemID, projectID, email string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// The WHERE is the guard: a row is only taken while it is still a draft of this
	// project, stored, and the author's. A file already linked to this same work
	// item is returned again, so a repeated call is harmless. The storage key (built
	// from the project number and "drafts") is left as it is.
	rows, err := r.db.Query(ctx, `
		UPDATE work_item_attachment
		SET work_item_id = $1, updated_on = now()
		WHERE id = ANY($4::uuid[])
		  AND project_id = $2
		  AND status = 'COMPLETE'
		  AND lower(created_by) = lower($3)
		  AND (work_item_id IS NULL OR work_item_id = $1)
		RETURNING id::TEXT`, workItemID, projectID, email, ids)
	if err != nil {
		return nil, fmt.Errorf("link report drafts: %w", err)
	}
	defer rows.Close()
	var linked []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan linked report: %w", err)
		}
		linked = append(linked, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate linked reports: %w", err)
	}
	return linked, nil
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

// UpdateStorageKey implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) UpdateStorageKey(ctx context.Context, kind domain.StoredAttachmentKind, id, from, to string) (bool, error) {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return false, err
	}
	ct, err := r.db.Exec(ctx, `
		UPDATE `+t.table+`
		SET storage_key = $3, updated_on = now()
		WHERE id = $1 AND status = 'COMPLETE' AND storage_key = $2`, id, from, to)
	if err != nil {
		return false, fmt.Errorf("update storage key: %w", err)
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
		rows, err := r.db.Query(ctx, `SELECT id::text FROM `+t.table+` WHERE status = 'PENDING' AND created_on < $1 ORDER BY updated_on, created_on LIMIT $2`, olderThan, limit-len(out))
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

// CountPendingStagedUploads implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) CountPendingStagedUploads(ctx context.Context, email string, since time.Time) (int, error) {
	total := 0
	for _, kind := range []domain.StoredAttachmentKind{domain.StoredAttachmentWorkItem, domain.StoredAttachmentComment, domain.StoredAttachmentDeployment} {
		t, err := storedAttachmentTableFor(kind)
		if err != nil {
			return 0, err
		}
		var n int
		if err := r.db.QueryRow(ctx, `SELECT count(*) FROM `+t.table+`
			WHERE status = 'PENDING' AND staged AND lower(created_by) = lower($1) AND created_on > $2`,
			email, since).Scan(&n); err != nil {
			return 0, fmt.Errorf("count pending staged uploads: %w", err)
		}
		total += n
	}
	return total, nil
}

// LinkInlineImages implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) LinkInlineImages(ctx context.Context, commentID, workItemID, email string, ids []string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE comment_attachment
		SET comment_id = $1, updated_on = now()
		WHERE id = ANY($4::uuid[])
		  AND work_item_id = $2
		  AND status = 'COMPLETE'
		  AND description_image = false
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

// LinkedImageIDs implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) LinkedImageIDs(ctx context.Context, commentID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id::TEXT FROM comment_attachment
		WHERE comment_id = $1 AND id = ANY($2::uuid[])`, commentID, ids)
	if err != nil {
		return nil, callerError(fmt.Errorf("get linked images: %w", err))
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan linked image id: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate linked image ids: %w", err)
	}
	return out, nil
}

// ListUnlinkedImages implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) ListUnlinkedImages(ctx context.Context, olderThan time.Time, limit int) ([]domain.StoredAttachmentRow, error) {
	t, err := storedAttachmentTableFor(domain.StoredAttachmentComment)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+storedAttachmentColumns(t)+` FROM comment_attachment
		WHERE comment_id IS NULL AND description_image = false AND status = 'COMPLETE' AND created_on < $1
		ORDER BY updated_on, created_on LIMIT $2`, olderThan, limit)
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

// ListUnlinkedReportDrafts implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) ListUnlinkedReportDrafts(ctx context.Context, olderThan time.Time, limit int) ([]domain.StoredAttachmentRow, error) {
	t, err := storedAttachmentTableFor(domain.StoredAttachmentWorkItem)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT `+storedAttachmentColumns(t)+` FROM work_item_attachment
		WHERE work_item_id IS NULL AND project_id IS NOT NULL AND status = 'COMPLETE' AND created_on < $1
		ORDER BY updated_on, created_on LIMIT $2`, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("list unlinked report drafts: %w", err)
	}
	defer rows.Close()
	var out []domain.StoredAttachmentRow
	for rows.Next() {
		a, err := scanStoredAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan unlinked report draft: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate unlinked report drafts: %w", err)
	}
	return out, nil
}

// RecordCleanupAttempt implements StoredAttachmentRepository.
func (r *storedAttachmentRepo) RecordCleanupAttempt(ctx context.Context, kind domain.StoredAttachmentKind, id string) error {
	t, err := storedAttachmentTableFor(kind)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, `UPDATE `+t.table+` SET updated_on = now() WHERE id = $1 AND status IN ('PENDING', 'COMPLETE')`, id); err != nil {
		return fmt.Errorf("record cleanup attempt: %w", err)
	}
	return nil
}
