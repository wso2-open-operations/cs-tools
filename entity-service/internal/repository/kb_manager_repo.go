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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// KBManagerRepository defines the persistence operations for the kb_managers table.
type KBManagerRepository interface {
	// SearchKBManagers returns rows matching the given optional filters.
	SearchKBManagers(ctx context.Context, req domain.SearchKBManagersRequest) ([]domain.KBManager, error)
	// CreateKBManager grants a user approver access to a knowledge base. A
	// ValidationError is returned if this pair already exists (UNIQUE
	// constraint) or references a knowledge base/user that does not exist.
	CreateKBManager(ctx context.Context, req domain.CreateKBManagerRequest) (domain.KBManager, error)
	// DeleteKBManager revokes a user's approver access to a knowledge base.
	DeleteKBManager(ctx context.Context, knowledgeBaseID, userID string) error
}

type kbManagerRepo struct {
	db *pgxpool.Pool
}

// NewKBManagerRepository constructs a KBManagerRepository backed by the given connection pool.
func NewKBManagerRepository(db *pgxpool.Pool) KBManagerRepository {
	return &kbManagerRepo{db: db}
}

// SearchKBManagers implements KBManagerRepository.
func (r *kbManagerRepo) SearchKBManagers(ctx context.Context, req domain.SearchKBManagersRequest) ([]domain.KBManager, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1

	if req.KnowledgeBaseID != "" {
		where += fmt.Sprintf(" AND knowledge_base_id = $%d", argIdx)
		args = append(args, req.KnowledgeBaseID)
		argIdx++
	}
	if req.UserID != "" {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, req.UserID)
		argIdx++
	}

	query := fmt.Sprintf(
		`SELECT id, knowledge_base_id, user_id, created_at FROM kb_managers %s ORDER BY created_at`,
		where,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search kb managers: %w", err)
	}
	defer rows.Close()

	managers := make([]domain.KBManager, 0)
	for rows.Next() {
		var m domain.KBManager
		if err := rows.Scan(&m.ID, &m.KnowledgeBaseID, &m.UserID, &m.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan kb manager: %w", err)
		}
		managers = append(managers, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kb managers: %w", err)
	}
	return managers, nil
}

// CreateKBManager implements KBManagerRepository.
func (r *kbManagerRepo) CreateKBManager(ctx context.Context, req domain.CreateKBManagerRequest) (domain.KBManager, error) {
	const query = `
		INSERT INTO kb_managers (knowledge_base_id, user_id)
		VALUES ($1, $2)
		RETURNING id, knowledge_base_id, user_id, created_at`

	var m domain.KBManager
	err := r.db.QueryRow(ctx, query, req.KnowledgeBaseID, req.UserID).Scan(
		&m.ID, &m.KnowledgeBaseID, &m.UserID, &m.CreatedOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return domain.KBManager{}, &apierror.ValidationError{Msg: "knowledge base or user does not exist: " + pgErr.Detail}
			case "23505":
				return domain.KBManager{}, &apierror.ValidationError{Msg: "this user is already an approver for this knowledge base"}
			}
		}
		return domain.KBManager{}, fmt.Errorf("create kb manager: %w", err)
	}
	return m, nil
}

// DeleteKBManager implements KBManagerRepository.
func (r *kbManagerRepo) DeleteKBManager(ctx context.Context, knowledgeBaseID, userID string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM kb_managers WHERE knowledge_base_id = $1 AND user_id = $2`,
		knowledgeBaseID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete kb manager: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ValidationError{Msg: "this user is not currently an approver for this knowledge base"}
	}
	return nil
}
