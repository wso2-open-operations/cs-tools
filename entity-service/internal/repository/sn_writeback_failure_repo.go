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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SNWritebackFailureRepository defines the persistence operations for the
// sn_writeback_failures table (see service.SNWritebackDispatcher). A row is
// one mirror write the previous system is missing; it lives until a replay
// of it succeeds (Delete) -- the table has no "resolved" column and none is
// added, the row's absence is the resolution. Read back by entity so the
// record's own detail can show what of it is not mirrored (see
// domain.ChangeRequest.MirrorFailures).
type SNWritebackFailureRepository interface {
	// Create inserts a new failure row.
	Create(ctx context.Context, req domain.CreateSNWritebackFailureRequest) (domain.SNWritebackFailure, error)
	// GetByID returns one row, or a NotFoundError.
	GetByID(ctx context.Context, id string) (domain.SNWritebackFailure, error)
	// ListByEntity returns the rows of one entity, oldest first (the order
	// the writes were made in, which is the order to replay them in).
	ListByEntity(ctx context.Context, entityType, entityID string) ([]domain.SNWritebackFailure, error)
	// List returns rows newest first, optionally filtered by entity type and
	// id, at most limit of them.
	List(ctx context.Context, entityType, entityID string, limit int) ([]domain.SNWritebackFailure, error)
	// UpdateError replaces a row's error with the reason its latest replay
	// failed. A NotFoundError when the row is gone.
	UpdateError(ctx context.Context, id, errMsg string) error
	// Delete removes a row -- a replay of it succeeded. A NotFoundError when
	// the row is already gone.
	Delete(ctx context.Context, id string) error
}

type snWritebackFailureRepo struct {
	db *pgxpool.Pool
}

// NewSNWritebackFailureRepository constructs an SNWritebackFailureRepository
// backed by the given connection pool.
func NewSNWritebackFailureRepository(db *pgxpool.Pool) SNWritebackFailureRepository {
	return &snWritebackFailureRepo{db: db}
}

// snWritebackFailureColumns is the column list shared by every query that
// returns a full row, kept in one place so it can't drift out of sync with
// scanSNWritebackFailure's field order.
const snWritebackFailureColumns = `id, entity_type, entity_id, operation, payload, error, created_on`

func scanSNWritebackFailure(row interface{ Scan(...any) error }) (domain.SNWritebackFailure, error) {
	var f domain.SNWritebackFailure
	if err := row.Scan(
		&f.ID, &f.EntityType, &f.EntityID, &f.Operation, &f.Payload, &f.Error, &f.CreatedOn,
	); err != nil {
		return domain.SNWritebackFailure{}, err
	}
	return f, nil
}

// Create implements SNWritebackFailureRepository.
func (r *snWritebackFailureRepo) Create(ctx context.Context, req domain.CreateSNWritebackFailureRequest) (domain.SNWritebackFailure, error) {
	query := `
		INSERT INTO sn_writeback_failures (entity_type, entity_id, operation, payload, error)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + snWritebackFailureColumns

	f, err := scanSNWritebackFailure(r.db.QueryRow(ctx, query, req.EntityType, req.EntityID, req.Operation, req.Payload, req.Error))
	if err != nil {
		return domain.SNWritebackFailure{}, fmt.Errorf("create sn_writeback_failure: %w", err)
	}
	return f, nil
}

// GetByID implements SNWritebackFailureRepository.
func (r *snWritebackFailureRepo) GetByID(ctx context.Context, id string) (domain.SNWritebackFailure, error) {
	f, err := scanSNWritebackFailure(r.db.QueryRow(ctx,
		`SELECT `+snWritebackFailureColumns+` FROM sn_writeback_failures WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SNWritebackFailure{}, &apierror.NotFoundError{Msg: "mirror write failure not found"}
	}
	if err != nil {
		return domain.SNWritebackFailure{}, fmt.Errorf("get sn_writeback_failure: %w", err)
	}
	return f, nil
}

// ListByEntity implements SNWritebackFailureRepository.
func (r *snWritebackFailureRepo) ListByEntity(ctx context.Context, entityType, entityID string) ([]domain.SNWritebackFailure, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+snWritebackFailureColumns+` FROM sn_writeback_failures
		 WHERE entity_type = $1 AND entity_id = $2::uuid ORDER BY created_on ASC, id ASC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list sn_writeback_failures by entity: %w", err)
	}
	defer rows.Close()
	return collectSNWritebackFailures(rows)
}

// snWritebackFailureListMax caps List: an operator page, not a dump.
const snWritebackFailureListMax = 200

// List implements SNWritebackFailureRepository.
func (r *snWritebackFailureRepo) List(ctx context.Context, entityType, entityID string, limit int) ([]domain.SNWritebackFailure, error) {
	if limit <= 0 || limit > snWritebackFailureListMax {
		limit = snWritebackFailureListMax
	}
	// NULLIF-ed text parameters: an empty filter matches everything. entity_id
	// is a UUID column, so the comparison is on its text form.
	rows, err := r.db.Query(ctx,
		`SELECT `+snWritebackFailureColumns+` FROM sn_writeback_failures
		 WHERE ($1 = '' OR entity_type = $1) AND ($2 = '' OR entity_id::text = $2)
		 ORDER BY created_on DESC, id DESC LIMIT $3`, entityType, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sn_writeback_failures: %w", err)
	}
	defer rows.Close()
	return collectSNWritebackFailures(rows)
}

func collectSNWritebackFailures(rows pgx.Rows) ([]domain.SNWritebackFailure, error) {
	out := []domain.SNWritebackFailure{}
	for rows.Next() {
		f, err := scanSNWritebackFailure(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sn_writeback_failure: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read sn_writeback_failures: %w", err)
	}
	return out, nil
}

// UpdateError implements SNWritebackFailureRepository.
func (r *snWritebackFailureRepo) UpdateError(ctx context.Context, id, errMsg string) error {
	ct, err := r.db.Exec(ctx, `UPDATE sn_writeback_failures SET error = $2 WHERE id = $1::uuid`, id, errMsg)
	if err != nil {
		return fmt.Errorf("update sn_writeback_failure error: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "mirror write failure not found"}
	}
	return nil
}

// Delete implements SNWritebackFailureRepository.
func (r *snWritebackFailureRepo) Delete(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `DELETE FROM sn_writeback_failures WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete sn_writeback_failure: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "mirror write failure not found"}
	}
	return nil
}
