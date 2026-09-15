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

// This file holds the entity-service DB writes/reads this backfill needs.
// Deliberately raw SQL, not the existing internal/repository package: those
// repositories are shaped for the live API's request/response DTOs
// (domain.CreateKBArticleRequest etc.) and don't expose the full column set
// (state, timestamps, source_sys_id, usage counters) a historical backfill
// must set directly.
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// kbStore performs the idempotent inserts/lookups this backfill needs
// against entity-service's own Postgres database.
type kbStore struct {
	pool *pgxpool.Pool
}

func newKBStore(pool *pgxpool.Pool) *kbStore {
	return &kbStore{pool: pool}
}

// upsertKnowledgeBase looks up a knowledge_bases row by name (the best
// available dedupe key -- knowledge_bases has no source_sys_id column; see
// this tool's return notes for why one wasn't added) and returns its id if
// found, otherwise inserts a new row and returns the new id. product_id is
// always left NULL (no SN field maps to it, per the spec).
func (s *kbStore) upsertKnowledgeBase(ctx context.Context, kb snKnowledgeBase) (id string, created bool, err error) {
	err = s.pool.QueryRow(ctx, `SELECT id FROM knowledge_bases WHERE name = $1`, kb.Title).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("look up knowledge_base by name %q: %w", kb.Title, err)
	}

	err = s.pool.QueryRow(ctx,
		`INSERT INTO knowledge_bases (name, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		kb.Title, kb.Active, kb.CreatedOn, kb.UpdatedOn,
	).Scan(&id)
	if err != nil {
		return "", false, fmt.Errorf("insert knowledge_base %q: %w", kb.Title, err)
	}
	return id, true, nil
}

// insertKBManager grants userID manager access to knowledgeBaseID,
// idempotently: the (knowledge_base_id, user_id) UNIQUE constraint means a
// rerun simply no-ops via ON CONFLICT DO NOTHING rather than erroring.
// Returns whether a new row was actually inserted.
func (s *kbStore) insertKBManager(ctx context.Context, knowledgeBaseID, userID string) (inserted bool, err error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO kb_managers (knowledge_base_id, user_id) VALUES ($1, $2)
		 ON CONFLICT (knowledge_base_id, user_id) DO NOTHING`,
		knowledgeBaseID, userID,
	)
	if err != nil {
		return false, fmt.Errorf("insert kb_manager (kb=%s, user=%s): %w", knowledgeBaseID, userID, err)
	}
	return tag.RowsAffected() > 0, nil
}

// articleExistsBySourceSysID implements the kb_articles idempotency check.
func (s *kbStore) articleExistsBySourceSysID(ctx context.Context, sourceSysID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kb_articles WHERE source_sys_id = $1)`, sourceSysID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check kb_articles idempotency for %q: %w", sourceSysID, err)
	}
	return exists, nil
}

// historyExistsBySourceSysID implements the kb_article_history idempotency check.
func (s *kbStore) historyExistsBySourceSysID(ctx context.Context, sourceSysID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM kb_article_history WHERE source_sys_id = $1)`, sourceSysID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check kb_article_history idempotency for %q: %w", sourceSysID, err)
	}
	return exists, nil
}

// insertArticle inserts one kb_articles row from a fully-resolved
// articlePlan and returns its new id. visibility/reviewer_id/team_key are
// never set (left at their column defaults / NULL) per the spec.
func (s *kbStore) insertArticle(ctx context.Context, p articlePlan) (id string, err error) {
	err = s.pool.QueryRow(ctx,
		`INSERT INTO kb_articles (
			knowledge_base_id, title, body, state, author_id, updated_by,
			source_case_id, rejection_comment, generated_with_ai, ai_generated_by,
			helpful_count, rating, use_count, view_count,
			source_sys_id, submitted_at, published_at, retired_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id`,
		p.KnowledgeBaseID, p.Title, p.Body, p.State, p.AuthorID, p.UpdatedBy,
		p.SourceCaseID, p.RejectionComment, p.GeneratedWithAI, p.AIGeneratedBy,
		p.HelpfulCount, p.Rating, p.UseCount, p.ViewCount,
		p.SourceSysID, p.SubmittedAt, p.PublishedAt, p.RetiredAt,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert kb_article (source_sys_id=%s): %w", p.SourceSysID, err)
	}
	return id, nil
}

// insertHistory inserts one kb_article_history row from a fully-resolved
// historyPlan, pointed at the given kb_articles id (the current/latest
// row's id -- every history row for one chain shares this same value).
func (s *kbStore) insertHistory(ctx context.Context, kbArticleID string, p historyPlan) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO kb_article_history (
			kb_article_id, title, body, state, changed_by,
			generated_with_ai, ai_generated_by, source_sys_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		kbArticleID, p.Title, p.Body, p.State, p.ChangedBy,
		p.GeneratedWithAI, p.AIGeneratedBy, p.SourceSysID,
	)
	if err != nil {
		return fmt.Errorf("insert kb_article_history (source_sys_id=%s): %w", p.SourceSysID, err)
	}
	return nil
}
