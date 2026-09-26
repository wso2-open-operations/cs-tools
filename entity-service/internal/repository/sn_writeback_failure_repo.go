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
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SNWritebackFailureRepository defines the persistence operations for the
// sn_writeback_failures table. This is a pilot mechanism (see
// service.SNWritebackDispatcher): a single insert method is enough today —
// nothing reads this table back through the API yet, the same starting
// shape EventPublishFailureRepository had before Search/MarkResolved were
// added for its own operator-facing endpoints.
type SNWritebackFailureRepository interface {
	// Create inserts a new failure row.
	Create(ctx context.Context, req domain.CreateSNWritebackFailureRequest) (domain.SNWritebackFailure, error)
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
