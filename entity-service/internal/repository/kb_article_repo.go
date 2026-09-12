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
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// KBArticleRepository defines the persistence operations for the kb_articles table.
type KBArticleRepository interface {
	// CreateKBArticle inserts a new article in the draft state.
	CreateKBArticle(ctx context.Context, req domain.CreateKBArticleRequest) (domain.KBArticle, error)

	// GetKBArticleByID returns a single article by id.
	GetKBArticleByID(ctx context.Context, id string) (domain.KBArticle, error)

	// SearchKBArticles returns a filtered, paginated slice of articles together
	// with the total count of matching rows before pagination. COUNT and SELECT
	// are executed concurrently on separate pool connections.
	SearchKBArticles(ctx context.Context, req domain.SearchKBArticlesRequest) ([]domain.KBArticle, int, error)

	// UpdateKBArticleState transitions an article's state. Illegal transitions
	// are rejected by the trg_kb_article_valid_transition trigger and surfaced
	// here as an apierror.ValidationError, not a raw DB error.
	UpdateKBArticleState(ctx context.Context, id string, req domain.UpdateKBArticleStateRequest) (domain.KBArticle, error)

	// UpdateKBArticleContent edits an existing draft's title/body. Callers
	// are expected to only invoke this while state == draft; the DB layer
	// does not itself enforce that (unlike state transitions, which the
	// trigger enforces) -- the CSM Portal Backend enforces it before
	// forwarding, same as the author-only rule on submit.
	UpdateKBArticleContent(ctx context.Context, id string, req domain.UpdateKBArticleContentRequest) (domain.KBArticle, error)

	// DeleteKBArticle removes an article only if it is currently in draft or
	// pending_review -- published/retired articles are never deleted.
	// Returns an apierror.ValidationError if the article is in neither state.
	DeleteKBArticle(ctx context.Context, id string) error

	// ListKBArticleHistory returns snapshots for an article, newest first.
	ListKBArticleHistory(ctx context.Context, kbArticleID string) ([]domain.KBArticleHistoryEntry, error)
}

type kbArticleRepo struct {
	db *pgxpool.Pool
}

// NewKBArticleRepository constructs a KBArticleRepository backed by the given connection pool.
func NewKBArticleRepository(db *pgxpool.Pool) KBArticleRepository {
	return &kbArticleRepo{db: db}
}

// CreateKBArticle implements KBArticleRepository.
func (r *kbArticleRepo) CreateKBArticle(ctx context.Context, req domain.CreateKBArticleRequest) (domain.KBArticle, error) {
	const query = `
		INSERT INTO kb_articles (knowledge_base_id, title, body, author_id, updated_by, team_key)
		VALUES ($1, $2, $3, $4, $4, $5)
		RETURNING id, knowledge_base_id, title, body, state, author_id,
		          reviewer_id, source_case_id, rejection_comment, updated_by, team_key, created_at, updated_at, submitted_at, published_at, retired_at`

	var a domain.KBArticle
	err := r.db.QueryRow(ctx, query,
		req.KnowledgeBaseID, req.Title, req.Body, req.AuthorID, req.TeamKey,
	).Scan(
		&a.ID, &a.KnowledgeBaseID, &a.Title, &a.Body, &a.State, &a.AuthorID,
		&a.ReviewerID, &a.SourceCaseID, &a.RejectionComment, &a.UpdatedBy, &a.TeamKey, &a.CreatedOn, &a.UpdatedOn, &a.SubmittedOn, &a.PublishedOn, &a.RetiredOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation — knowledge_base_id or author_id does not exist
				return domain.KBArticle{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001": // raise_exception from trg_kb_article_knowledge_base_active
				return domain.KBArticle{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.KBArticle{}, fmt.Errorf("create kb article: %w", err)
	}
	return a, nil
}

// GetKBArticleByID implements KBArticleRepository.
func (r *kbArticleRepo) GetKBArticleByID(ctx context.Context, id string) (domain.KBArticle, error) {
	const query = `
		SELECT id, knowledge_base_id, title, body, state, author_id,
		       reviewer_id, source_case_id, rejection_comment, updated_by, team_key, created_at, updated_at, submitted_at, published_at, retired_at
		FROM kb_articles
		WHERE id = $1`

	var a domain.KBArticle
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.KnowledgeBaseID, &a.Title, &a.Body, &a.State, &a.AuthorID,
		&a.ReviewerID, &a.SourceCaseID, &a.RejectionComment, &a.UpdatedBy, &a.TeamKey, &a.CreatedOn, &a.UpdatedOn, &a.SubmittedOn, &a.PublishedOn, &a.RetiredOn,
	)
	if err != nil {
		return domain.KBArticle{}, fmt.Errorf("get kb article: %w", err)
	}
	return a, nil
}

// SearchKBArticles implements KBArticleRepository.
func (r *kbArticleRepo) SearchKBArticles(ctx context.Context, req domain.SearchKBArticlesRequest) ([]domain.KBArticle, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE 1=1"

	if req.KnowledgeBaseID != "" {
		where += fmt.Sprintf(" AND knowledge_base_id = $%d", argIdx)
		filterArgs = append(filterArgs, req.KnowledgeBaseID)
		argIdx++
	}

	if len(req.States) > 0 {
		// Convert []KBArticleState to []string — pgx has no codec for named string types.
		stateStrings := make([]string, len(req.States))
		for i, s := range req.States {
			stateStrings[i] = string(s)
		}
		where += fmt.Sprintf(" AND state = ANY($%d::kb_article_state_enum[])", argIdx)
		filterArgs = append(filterArgs, stateStrings)
		argIdx++
	}

	if req.AuthorID != "" {
		where += fmt.Sprintf(" AND author_id = $%d", argIdx)
		filterArgs = append(filterArgs, req.AuthorID)
		argIdx++
	}

	if len(req.TeamKeys) > 0 {
		where += fmt.Sprintf(" AND team_key = ANY($%d)", argIdx)
		filterArgs = append(filterArgs, req.TeamKeys)
		argIdx++
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (title ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM kb_articles " + where

	dataQuery := fmt.Sprintf(
		`SELECT id, knowledge_base_id, title, body, state, author_id,
		        reviewer_id, source_case_id, rejection_comment, updated_by, team_key, created_at, updated_at, submitted_at, published_at, retired_at
		 FROM kb_articles
		 %s
		 ORDER BY created_at DESC, id
		 LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	dataArgs := append(append([]any{}, filterArgs...), req.Pagination.Limit, req.Pagination.Offset)

	var total int
	var articles []domain.KBArticle

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, filterArgs...).Scan(&total); err != nil {
			return fmt.Errorf("count kb articles: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query kb articles: %w", err)
		}
		defer rows.Close()

		result := make([]domain.KBArticle, 0, req.Pagination.Limit)
		for rows.Next() {
			var a domain.KBArticle
			if err := rows.Scan(
				&a.ID, &a.KnowledgeBaseID, &a.Title, &a.Body, &a.State, &a.AuthorID,
				&a.ReviewerID, &a.SourceCaseID, &a.RejectionComment, &a.UpdatedBy, &a.TeamKey, &a.CreatedOn, &a.UpdatedOn, &a.SubmittedOn, &a.PublishedOn, &a.RetiredOn,
			); err != nil {
				return fmt.Errorf("scan kb article: %w", err)
			}
			result = append(result, a)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate kb articles: %w", err)
		}
		articles = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

// UpdateKBArticleState implements KBArticleRepository.
func (r *kbArticleRepo) UpdateKBArticleState(ctx context.Context, id string, req domain.UpdateKBArticleStateRequest) (domain.KBArticle, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.KBArticle{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Capture the state before the update so we only record a version
	// snapshot when a genuine transition happens -- a retried or duplicated
	// request that resubmits the *same* target state must not inflate the
	// version count.
	var oldState string
	if err := tx.QueryRow(ctx, `SELECT state FROM kb_articles WHERE id = $1`, id).Scan(&oldState); err != nil {
		return domain.KBArticle{}, fmt.Errorf("read current kb article state: %w", err)
	}

	const query = `
 UPDATE kb_articles
 SET state = $2::kb_article_state_enum,
     rejection_comment = $3,
     updated_by = $4,
     submitted_at = CASE WHEN $2 = 'pending_review' THEN NOW() ELSE submitted_at END,
     published_at = CASE WHEN $2 = 'published' THEN NOW() ELSE published_at END,
     retired_at   = CASE WHEN $2 = 'retired'   THEN NOW() ELSE retired_at   END
 WHERE id = $1
 RETURNING id, knowledge_base_id, title, body, state, author_id,
           reviewer_id, source_case_id, rejection_comment, updated_by, team_key, created_at, updated_at, submitted_at, published_at, retired_at`

	var a domain.KBArticle
	err = tx.QueryRow(ctx, query, id, string(req.State), req.RejectionComment, req.UpdatedBy).Scan(
		&a.ID, &a.KnowledgeBaseID, &a.Title, &a.Body, &a.State, &a.AuthorID,
		&a.ReviewerID, &a.SourceCaseID, &a.RejectionComment, &a.UpdatedBy, &a.TeamKey, &a.CreatedOn, &a.UpdatedOn, &a.SubmittedOn, &a.PublishedOn, &a.RetiredOn,
	)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "P0001":
				return domain.KBArticle{}, &apierror.ValidationError{Msg: pgErr.Message}
			case "23514":
				return domain.KBArticle{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.KBArticle{}, fmt.Errorf("update kb article state: %w", err)
	}

	if oldState != string(a.State) {
		changedBy := a.AuthorID
		if a.UpdatedBy != nil && *a.UpdatedBy != "" {
			changedBy = *a.UpdatedBy
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO kb_article_history (kb_article_id, title, body, state, changed_by)
  VALUES ($1, $2, $3, $4, $5)`,
			a.ID, a.Title, a.Body, string(a.State), changedBy,
		); err != nil {
			return domain.KBArticle{}, fmt.Errorf("insert kb article history: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.KBArticle{}, fmt.Errorf("commit tx: %w", err)
	}
	return a, nil
}

// UpdateKBArticleContent implements KBArticleRepository.
func (r *kbArticleRepo) UpdateKBArticleContent(ctx context.Context, id string, req domain.UpdateKBArticleContentRequest) (domain.KBArticle, error) {
	const query = `
		UPDATE kb_articles
		SET title = $2, body = $3, updated_by = $4, updated_at = NOW()
		WHERE id = $1 AND state = 'draft'
		RETURNING id, knowledge_base_id, title, body, state, author_id,
		          reviewer_id, source_case_id, rejection_comment, updated_by, team_key, created_at, updated_at, submitted_at, published_at, retired_at`

	var a domain.KBArticle
	err := r.db.QueryRow(ctx, query, id, req.Title, req.Body, req.UpdatedBy).Scan(
		&a.ID, &a.KnowledgeBaseID, &a.Title, &a.Body, &a.State, &a.AuthorID,
		&a.ReviewerID, &a.SourceCaseID, &a.RejectionComment, &a.UpdatedBy, &a.TeamKey, &a.CreatedOn, &a.UpdatedOn, &a.SubmittedOn, &a.PublishedOn, &a.RetiredOn,
	)
	if err != nil {
		return domain.KBArticle{}, fmt.Errorf("update kb article content: %w", err)
	}
	return a, nil
}

// DeleteKBArticle implements KBArticleRepository.
func (r *kbArticleRepo) DeleteKBArticle(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM kb_article_history WHERE kb_article_id = $1`, id); err != nil {
		return fmt.Errorf("delete kb article history: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM kb_articles WHERE id = $1 AND state IN ('draft', 'pending_review')`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete kb article: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ValidationError{Msg: "article not found, or not in a deletable state"}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// ListKBArticleHistory implements KBArticleRepository.
func (r *kbArticleRepo) ListKBArticleHistory(ctx context.Context, kbArticleID string) ([]domain.KBArticleHistoryEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, kb_article_id, title, body, state, changed_by, created_at
		 FROM kb_article_history WHERE kb_article_id = $1 ORDER BY created_at DESC`,
		kbArticleID,
	)
	if err != nil {
		return nil, fmt.Errorf("list kb article history: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.KBArticleHistoryEntry, 0)
	for rows.Next() {
		var e domain.KBArticleHistoryEntry
		if err := rows.Scan(&e.ID, &e.KBArticleID, &e.Title, &e.Body, &e.State, &e.ChangedBy, &e.CreatedOn); err != nil {
			return nil, fmt.Errorf("scan kb article history: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
