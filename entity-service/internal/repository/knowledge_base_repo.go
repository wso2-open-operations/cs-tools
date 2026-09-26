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

// knowledgeBaseColumns matches the real "knowledge_base" table -- title/
// active (not name/is_active), no product_id column at all (a knowledge
// base is no longer tied to a specific product in the real schema).
const knowledgeBaseColumns = "id, title, active, created_on, updated_on"

// KnowledgeBaseRepository defines the persistence operations for the real
// knowledge_base table. NOTE: the request structs below (Create/Update...)
// still use their original field names (Name, ProductID, IsActive) --
// deliberately NOT renamed in this pass to keep this fix scoped; this
// repository translates them to the real title/active columns internally.
// ProductID is accepted but silently ignored -- there is nowhere to store
// it, since the real table has no such column at all.
type KnowledgeBaseRepository interface {
	// ListKnowledgeBases returns every knowledge base, ordered by title.
	// Includes deactivated ones -- the Admin screen needs to see and
	// reactivate them; article-creation pickers filter active themselves
	// on the frontend/consumer side.
	ListKnowledgeBases(ctx context.Context) ([]domain.KnowledgeBase, error)

	// CreateKnowledgeBase creates a new, active knowledge base.
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
	rows, err := r.db.Query(ctx, "SELECT "+knowledgeBaseColumns+" FROM knowledge_base ORDER BY title")
	if err != nil {
		return nil, fmt.Errorf("list knowledge bases: %w", err)
	}
	defer rows.Close()

	kbs := make([]domain.KnowledgeBase, 0)
	for rows.Next() {
		var kb domain.KnowledgeBase
		if err := rows.Scan(&kb.ID, &kb.Title, &kb.Active, &kb.CreatedOn, &kb.UpdatedOn); err != nil {
			return nil, fmt.Errorf("scan knowledge base: %w", err)
		}
		kbs = append(kbs, kb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge bases: %w", err)
	}
	return kbs, nil
}

// CreateKnowledgeBase implements KnowledgeBaseRepository. Neither id nor
// created_on/updated_on has a DB-level default on the real table, so all
// three are generated explicitly. created_by/updated_by are NOT NULL on
// the real table -- req has no caller-identity field to source this from
// yet, so a placeholder "system" is used until that's wired through from
// the handler layer (same open question as KB articles' author identity).
func (r *knowledgeBaseRepo) CreateKnowledgeBase(ctx context.Context, req domain.CreateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	const query = `
		INSERT INTO knowledge_base (id, title, active, created_on, updated_on, created_by, updated_by)
		VALUES (gen_random_uuid(), $1, true, NOW(), NOW(), 'system', 'system')
		RETURNING ` + knowledgeBaseColumns

	var kb domain.KnowledgeBase
	err := r.db.QueryRow(ctx, query, req.Name).Scan(
		&kb.ID, &kb.Title, &kb.Active, &kb.CreatedOn, &kb.UpdatedOn,
	)
	if err != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("create knowledge base: %w", err)
	}
	return kb, nil
}

// UpdateKnowledgeBaseName implements KnowledgeBaseRepository.
func (r *knowledgeBaseRepo) UpdateKnowledgeBaseName(ctx context.Context, id string, req domain.UpdateKnowledgeBaseRequest) (domain.KnowledgeBase, error) {
	const query = `
		UPDATE knowledge_base SET title = $2, updated_on = NOW()
		WHERE id = $1
		RETURNING ` + knowledgeBaseColumns

	var kb domain.KnowledgeBase
	err := r.db.QueryRow(ctx, query, id, req.Name).Scan(
		&kb.ID, &kb.Title, &kb.Active, &kb.CreatedOn, &kb.UpdatedOn,
	)
	if err != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("update knowledge base name: %w", err)
	}
	return kb, nil
}

// SetKnowledgeBaseActive implements KnowledgeBaseRepository.
func (r *knowledgeBaseRepo) SetKnowledgeBaseActive(ctx context.Context, id string, req domain.UpdateKnowledgeBaseActiveRequest) (domain.KnowledgeBase, error) {
	const query = `
		UPDATE knowledge_base SET active = $2, updated_on = NOW()
		WHERE id = $1
		RETURNING ` + knowledgeBaseColumns

	var kb domain.KnowledgeBase
	err := r.db.QueryRow(ctx, query, id, req.IsActive).Scan(
		&kb.ID, &kb.Title, &kb.Active, &kb.CreatedOn, &kb.UpdatedOn,
	)
	if err != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("set knowledge base active: %w", err)
	}
	return kb, nil
}
