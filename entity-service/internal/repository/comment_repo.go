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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// CommentRow is the raw shape of one row read from the comment table
// (migration 000037, extended by migration 000086 with DeletedAt/DeletedBy/
// LastEditedAt).
type CommentRow struct {
	ID         string
	WorkItemID string
	Content    string
	Type       *string
	CreatedBy  string
	CreatedOn  time.Time
	// CreatedByName is the resolved display name for CreatedBy (an email --
	// see CreatedBy's own doc comment in comment_service.go), matched
	// case-insensitively against "user".email/first_name/last_name/name the
	// same way case_repo.go's SearchCaseActivities already resolves a
	// comment's author for the case activity timeline -- see its own doc
	// comment for the DISTINCT ON reasoning (email has no unique constraint).
	// Only SearchComments populates this; CreateComment's RETURNing has no
	// join to resolve it from and leaves it "".
	CreatedByName string
	// DeletedAt/DeletedBy are non-nil once SoftDeleteComment has run. Content
	// is left untouched in the database either way -- see SoftDeleteComment's
	// own doc comment; it is the service layer's job to decide what a given
	// caller should actually see in its place.
	DeletedAt *time.Time
	DeletedBy *string
	// LastEditedAt is non-nil once UpdateComment has run at least once.
	LastEditedAt *time.Time
}

// CommentEditHistoryRow is one prior version of a comment's body, recorded by
// UpdateComment before it overwrites comment.content (migration 000086's
// comment_edit_history table). Body is always the PRE-edit content.
type CommentEditHistoryRow struct {
	ID        string
	CommentID string
	Body      string
	EditedBy  string
	EditedAt  time.Time
}

// ReferenceTypeToWorkItemType maps a domain.ReferenceType to the
// work_item_type_enum value(s) it corresponds to. comment.work_item_id is a
// foreign key into work_item(id), so only reference types that are
// themselves work_item subtypes can be commented on through this data
// source. "deployment" has no entry: deployment (migration 000013) is its
// own standalone table with its own primary key space, not a work_item
// subtype, so a comment can never point at one here.
//
// ReferenceTypeCase maps to all five case-like work_item types (the same
// set case_repo.go's own caseLikeWorkItemTypes names), not just literal
// "CASE" -- found live as a real bug (a CS-numbered work_item whose real
// type is SERVICE_REQUEST returned zero comments through this path, even
// though case_repo.go's own GetCaseByID/SearchCases have served all five
// case-like types since "Case-like work_item types" landed; this file's own
// comment search/create was never updated to match).
var ReferenceTypeToWorkItemType = map[domain.ReferenceType][]string{
	domain.ReferenceTypeCase:          {"CASE", "ENGAGEMENT", "SERVICE_REQUEST", "SECURITY_REPORT_ANALYSIS", "ANNOUNCEMENT"},
	domain.ReferenceTypeConversation:  {"CONVERSATION"},
	domain.ReferenceTypeChangeRequest: {"CHANGE_REQUEST"},
	domain.ReferenceTypeIncident:      {"INCIDENT"},
}

// CommentRepository defines the persistence operations for the comment table.
type CommentRepository interface {
	// CreateComment inserts a new comment row pointing at referenceID, after
	// confirming in the same round trip that a work_item with that id exists
	// and is of the type referenceType maps to (work_item's primary key
	// space is shared across every subtype, so the id alone doesn't
	// guarantee that). typeEnum must already be a valid comment_type_enum
	// label (e.g. "COMMENT", "WORK_NOTE") -- the caller (service layer)
	// decides which CommentType values are writable here. Returns a
	// ValidationError if referenceType has no work_item mapping; a
	// NotFoundError if no matching work_item exists.
	CreateComment(ctx context.Context, referenceID string, referenceType domain.ReferenceType, typeEnum, content, createdBy string) (CommentRow, error)
	// SearchComments returns a paginated, newest-first slice of comments for
	// referenceID together with the total matching count, optionally
	// filtered to one comment_type_enum label. Returns a ValidationError if
	// referenceType has no work_item mapping. excludeDeleted, when true,
	// filters out soft-deleted rows (deleted_at IS NOT NULL) at the SQL
	// level -- not in the caller -- so total/pagination stay consistent for a
	// customer caller, who must never see a soft-deleted comment at all (see
	// commentService.SearchComments's own visibility-rule doc comment).
	SearchComments(ctx context.Context, referenceID string, referenceType domain.ReferenceType, typeEnumFilter *string, excludeDeleted bool, pagination domain.Pagination) ([]CommentRow, int, error)
	// UpdateComment edits a comment's content, recording the pre-edit body as
	// a new comment_edit_history row in the same transaction. Returns a
	// NotFoundError if id does not exist, or a ValidationError if the
	// comment is already soft-deleted (a deleted comment cannot be edited).
	UpdateComment(ctx context.Context, id string, newContent string, editorEmail string) (CommentRow, error)
	// SoftDeleteComment marks a comment deleted (deleted_at/deleted_by) without
	// touching its content. Returns a NotFoundError if id does not exist, or a
	// ConflictError if it is already deleted.
	SoftDeleteComment(ctx context.Context, id string, deletedByEmail string) error
	// GetCommentEditHistory returns commentID's prior versions, newest first.
	GetCommentEditHistory(ctx context.Context, commentID string) ([]CommentEditHistoryRow, error)
	// GetCommentByID returns a single comment row by id, or a NotFoundError.
	// Used by the service layer to resolve the original author for the
	// UpdateComment/DeleteComment authorization check before mutating.
	GetCommentByID(ctx context.Context, id string) (CommentRow, error)
}

type commentRepo struct {
	db *pgxpool.Pool
}

// NewCommentRepository constructs a CommentRepository backed by the given connection pool.
func NewCommentRepository(db *pgxpool.Pool) CommentRepository {
	return &commentRepo{db: db}
}

const commentColumns = `id, work_item_id, content, type, created_by, created_on, deleted_at, deleted_by, last_edited_at`

func scanComment(row interface{ Scan(...any) error }) (CommentRow, error) {
	var c CommentRow
	err := row.Scan(&c.ID, &c.WorkItemID, &c.Content, &c.Type, &c.CreatedBy, &c.CreatedOn, &c.DeletedAt, &c.DeletedBy, &c.LastEditedAt)
	return c, err
}

// scanCommentWithName scans one row of SearchComments' own query, which adds
// a resolved_name column (see that method's own doc comment) beyond
// scanComment's plain commentColumns shape.
func scanCommentWithName(row interface{ Scan(...any) error }) (CommentRow, error) {
	var c CommentRow
	err := row.Scan(&c.ID, &c.WorkItemID, &c.Content, &c.Type, &c.CreatedBy, &c.CreatedOn, &c.DeletedAt, &c.DeletedBy, &c.LastEditedAt, &c.CreatedByName)
	return c, err
}

// CreateComment implements CommentRepository.
func (r *commentRepo) CreateComment(ctx context.Context, referenceID string, referenceType domain.ReferenceType, typeEnum, content, createdBy string) (CommentRow, error) {
	workItemTypes, ok := ReferenceTypeToWorkItemType[referenceType]
	if !ok {
		return CommentRow{}, &apierror.ValidationError{Msg: "referenceType is not supported by the Postgres data source: " + string(referenceType)}
	}

	// INSERT ... SELECT ... WHERE EXISTS rather than a plain INSERT, so the
	// work_item's type is checked in the same round trip as the insert.
	// ::text[] before ::work_item_type_enum[]: this repository never
	// registers work_item_type_enum/_work_item_type_enum with pgx, so
	// binding workItemTypes ([]string) directly to the enum array type has
	// no encode plan -- same fix as every other enum array bind in this
	// codebase (e.g. time_card_repo.go's state filter).
	const query = `
		INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		SELECT gen_random_uuid(), NOW(), $1, $2::comment_type_enum, wi.id, $3
		FROM work_item wi
		WHERE wi.id = $4 AND wi.type = ANY($5::text[]::work_item_type_enum[])
		RETURNING ` + commentColumns

	c, err := scanComment(r.db.QueryRow(ctx, query, createdBy, typeEnum, content, referenceID, workItemTypes))
	if errors.Is(err, pgx.ErrNoRows) {
		return CommentRow{}, &apierror.NotFoundError{Msg: "no " + string(referenceType) + " found with id " + referenceID}
	}
	if err != nil {
		return CommentRow{}, fmt.Errorf("create comment: %w", err)
	}
	return c, nil
}

// SearchComments implements CommentRepository.
func (r *commentRepo) SearchComments(ctx context.Context, referenceID string, referenceType domain.ReferenceType, typeEnumFilter *string, excludeDeleted bool, pagination domain.Pagination) ([]CommentRow, int, error) {
	workItemTypes, ok := ReferenceTypeToWorkItemType[referenceType]
	if !ok {
		return nil, 0, &apierror.ValidationError{Msg: "referenceType is not supported by the Postgres data source: " + string(referenceType)}
	}

	args := []any{referenceID, workItemTypes}
	// ::text[] before ::work_item_type_enum[] -- see CreateComment's own
	// comment on the identical bind above for why.
	where := "WHERE c.work_item_id = $1 AND wi.type = ANY($2::text[]::work_item_type_enum[])"
	if typeEnumFilter != nil {
		args = append(args, *typeEnumFilter)
		where += fmt.Sprintf(" AND c.type = $%d::comment_type_enum", len(args))
	}
	if excludeDeleted {
		where += " AND c.deleted_at IS NULL"
	}

	const fromJoin = "FROM comment c JOIN work_item wi ON wi.id = c.work_item_id"

	countQuery := "SELECT COUNT(*) " + fromJoin + " " + where
	// Resolves the comment author's display name for the response -- see
	// CommentRow.CreatedByName's own doc comment. The email-match join is
	// wrapped in its own DISTINCT ON subquery, same as
	// case_repo.go's SearchCaseActivities: "user".email has no unique
	// constraint, so two user rows sharing an address would otherwise fan a
	// single comment row out into more than one result row, while
	// countQuery above (no "user" join) still counts it once.
	dataQuery := fmt.Sprintf(`
		SELECT c.id, c.work_item_id, c.content, c.type, c.created_by, c.created_on,
			c.deleted_at, c.deleted_by, c.last_edited_at, c.resolved_name
		FROM (
			SELECT DISTINCT ON (c.id)
				c.id, c.work_item_id, c.content, c.type, c.created_by, c.created_on,
				c.deleted_at, c.deleted_by, c.last_edited_at,
				COALESCE(NULLIF(TRIM(u.name), ''), NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), '') AS resolved_name
			%s
			LEFT JOIN "user" u ON LOWER(u.email) = LOWER(c.created_by)
			%s
			ORDER BY c.id, u.id
		) c
		ORDER BY c.created_on DESC, c.id
		LIMIT $%d OFFSET $%d`,
		fromJoin, where, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), pagination.Limit, pagination.Offset)

	var total int
	var rows []CommentRow

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count comments: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		res, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query comments: %w", err)
		}
		defer res.Close()

		out := make([]CommentRow, 0, pagination.Limit)
		for res.Next() {
			c, err := scanCommentWithName(res)
			if err != nil {
				return fmt.Errorf("scan comment: %w", err)
			}
			out = append(out, c)
		}
		if err := res.Err(); err != nil {
			return fmt.Errorf("iterate comments: %w", err)
		}
		rows = out
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

// GetCommentByID implements CommentRepository.
func (r *commentRepo) GetCommentByID(ctx context.Context, id string) (CommentRow, error) {
	row, err := scanComment(r.db.QueryRow(ctx, `SELECT `+commentColumns+` FROM comment WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return CommentRow{}, &apierror.NotFoundError{Msg: "comment not found: " + id}
	}
	if err != nil {
		return CommentRow{}, fmt.Errorf("get comment by id: %w", err)
	}
	return row, nil
}

// UpdateComment implements CommentRepository.
func (r *commentRepo) UpdateComment(ctx context.Context, id string, newContent string, editorEmail string) (CommentRow, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CommentRow{}, fmt.Errorf("update comment: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldContent string
	var deletedAt *time.Time
	// SELECT ... FOR UPDATE: row-locked for the duration of the transaction so
	// a concurrent edit or delete can't interleave between this read and the
	// INSERT/UPDATE below.
	err = tx.QueryRow(ctx, `SELECT content, deleted_at FROM comment WHERE id = $1 FOR UPDATE`, id).Scan(&oldContent, &deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CommentRow{}, &apierror.NotFoundError{Msg: "comment not found: " + id}
	}
	if err != nil {
		return CommentRow{}, fmt.Errorf("update comment: read current: %w", err)
	}
	if deletedAt != nil {
		return CommentRow{}, &apierror.ValidationError{Msg: "a deleted comment cannot be edited"}
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO comment_edit_history (comment_id, body, edited_by, edited_at) VALUES ($1, $2, $3, NOW())`,
		id, oldContent, editorEmail,
	); err != nil {
		return CommentRow{}, fmt.Errorf("update comment: insert edit history: %w", err)
	}

	row, err := scanComment(tx.QueryRow(ctx,
		`UPDATE comment SET content = $1, last_edited_at = NOW() WHERE id = $2 RETURNING `+commentColumns,
		newContent, id,
	))
	if err != nil {
		return CommentRow{}, fmt.Errorf("update comment: write new content: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return CommentRow{}, fmt.Errorf("update comment: commit tx: %w", err)
	}
	return row, nil
}

// SoftDeleteComment implements CommentRepository. It never touches content --
// the row is retained verbatim so the service layer can still decide, per
// caller, whether to show it redacted or not at all.
func (r *commentRepo) SoftDeleteComment(ctx context.Context, id string, deletedByEmail string) error {
	const query = `
		UPDATE comment SET deleted_at = NOW(), deleted_by = $1
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING id`
	var returnedID string
	err := r.db.QueryRow(ctx, query, deletedByEmail, id).Scan(&returnedID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("soft delete comment: %w", err)
	}

	// No row matched the UPDATE ... WHERE deleted_at IS NULL -- either the
	// comment doesn't exist, or it's already deleted. Distinguish the two
	// with a follow-up read so the caller gets the right status code.
	var alreadyDeleted bool
	checkErr := r.db.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM comment WHERE id = $1`, id).Scan(&alreadyDeleted)
	if errors.Is(checkErr, pgx.ErrNoRows) {
		return &apierror.NotFoundError{Msg: "comment not found: " + id}
	}
	if checkErr != nil {
		return fmt.Errorf("soft delete comment: check existing: %w", checkErr)
	}
	if alreadyDeleted {
		return &apierror.ConflictError{Msg: "comment is already deleted"}
	}
	// Should be unreachable (the row exists and wasn't deleted, yet the
	// conditional UPDATE above matched nothing), but fail loudly rather than
	// report a false success.
	return fmt.Errorf("soft delete comment: update matched no row for an existing, non-deleted comment %s", id)
}

// GetCommentEditHistory implements CommentRepository.
func (r *commentRepo) GetCommentEditHistory(ctx context.Context, commentID string) ([]CommentEditHistoryRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, comment_id, body, edited_by, edited_at FROM comment_edit_history WHERE comment_id = $1 ORDER BY edited_at DESC`,
		commentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get comment edit history: %w", err)
	}
	defer rows.Close()

	out := []CommentEditHistoryRow{}
	for rows.Next() {
		var h CommentEditHistoryRow
		if err := rows.Scan(&h.ID, &h.CommentID, &h.Body, &h.EditedBy, &h.EditedAt); err != nil {
			return nil, fmt.Errorf("scan comment edit history: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comment edit history: %w", err)
	}
	return out, nil
}
