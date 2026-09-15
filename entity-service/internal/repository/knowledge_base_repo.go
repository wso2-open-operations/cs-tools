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

const knowledgeBaseColumns = "id, product_id, name, is_active, created_at, updated_at"

// KnowledgeBaseRepository defines the persistence operations for the knowledge_bases table.
type KnowledgeBaseRepository interface {
	// ListKnowledgeBases returns every knowledge base, ordered by name.
	// Includes deactivated ones -- the Admin screen needs to see and
	// reactivate them; article-creation pickers filter is_active
	// themselves on the frontend/consumer side.
	ListKnowledgeBases(ctx context.Context) ([]domain.KnowledgeBase, error)

	// CreateKnowledgeBase creates a new, active knowledge base for a
	// product. A ValidationError is returned if the product already has one
	// (product_id is UNIQUE).
	CreateKnowledgeBase(ctx context.Context, req domain.CreateKnowledgeBaseRequest) (domain.KnowledgeBase, error)

	// UpdateKnowledgeBaseName renames an existing knowledge base.
	UpdateKnowledgeBaseName(ctx context.Context, id string, req domain.UpdateKnowledgeBaseRequest) (domain.KnowledgeBase, error)

	// SetKnowledgeBaseActive activates or deactivates a knowledge base.
	// Deactivating only blocks new article creation (enforced by the
	// caller, not by this column alone) -- it never touches existing
	// articles in any state.
	SetKnowledgeBaseActive(ctx context.Context, id string, req domain.UpdateKnowledgeBaseActiveRequest) (domain.KnowledgeBase, error)
}

type knowledgeBaseRepo struct {
	db *pgxpool.Pool
}

// NewKnowledgeBaseRepository constructs a KnowledgeBaseRepository backed by the given connection pool.
func NewKnowledgeBaseRepository(db *pgxpool.Pool) KnowledgeBaseRepository {
	return &knowledgeBaseRepo{db: db}
}

// ListKnowledgeBases implements KnowledgeBaseRepository.
func (r *knowledgeBaseRepo) ListKnowledgeBases(ctx context.Context) ([]domain.KnowledgeBase, error) {
	rows, err := r.db.Query(ctx, "SELECT "+knowledgeBaseColumns+" FROM knowledge_bases ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list knowledge bases: %w", err)
	}
	defer rows.Close()

	kbs := make([]domain.KnowledgeBase, 0)
	for rows.Next() {
		var kb domain.KnowledgeBase
		if err := rows.Scan(&kb.ID, &kb.ProductID, &kb.Name, &kb.IsActive, &kb.CreatedOn, &kb.UpdatedOn); err != nil {
			return nil, fmt.Errorf("scan knowledge base: %w", err)
		}
		kbs = append(kbs, kb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge bases: %w", err)
	}
	return kbs, nil
}

// CreateKnowledgeBase implements KnowledgeBaseRepository.
func (r *knowledgeBaseRepo) CreateKnowledgeBase(ctx context.Context, req domain.CreateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	const query = `
		INSERT INTO knowledge_bases (product_id, name)
		VALUES ($1, $2)
		RETURNING ` + knowledgeBaseColumns

	var kb domain.KnowledgeBase
	err := r.db.QueryRow(ctx, query, req.ProductID, req.Name).Scan(
		&kb.ID, &kb.ProductID, &kb.Name, &kb.IsActive, &kb.CreatedOn, &kb.UpdatedOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation -- product_id does not exist
				return domain.KnowledgeBase{}, &apierror.ValidationError{Msg: "product does not exist: " + pgErr.Detail}
			case "23505": // unique_violation -- this product already has a KB
				return domain.KnowledgeBase{}, &apierror.ValidationError{Msg: "this product already has a knowledge base"}
			}
		}
		return domain.KnowledgeBase{}, fmt.Errorf("create knowledge base: %w", err)
	}
	return kb, nil
}

// UpdateKnowledgeBaseName implements KnowledgeBaseRepository.
func (r *knowledgeBaseRepo) UpdateKnowledgeBaseName(ctx context.Context, id string, req domain.UpdateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	const query = `
		UPDATE knowledge_bases SET name = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + knowledgeBaseColumns

	var kb domain.KnowledgeBase
	err := r.db.QueryRow(ctx, query, id, req.Name).Scan(
		&kb.ID, &kb.ProductID, &kb.Name, &kb.IsActive, &kb.CreatedOn, &kb.UpdatedOn,
	)
	if err != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("update knowledge base name: %w", err)
	}
	return kb, nil
}

// SetKnowledgeBaseActive implements KnowledgeBaseRepository.
func (r *knowledgeBaseRepo) SetKnowledgeBaseActive(ctx context.Context, id string, req domain.UpdateKnowledgeBaseActiveRequest) (domain.KnowledgeBase, error) {
	const query = `
		UPDATE knowledge_bases SET is_active = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + knowledgeBaseColumns

	var kb domain.KnowledgeBase
	err := r.db.QueryRow(ctx, query, id, req.IsActive).Scan(
		&kb.ID, &kb.ProductID, &kb.Name, &kb.IsActive, &kb.CreatedOn, &kb.UpdatedOn,
	)
	if err != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("set knowledge base active: %w", err)
	}
	return kb, nil
}
