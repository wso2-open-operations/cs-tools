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

// KBArticleRepository defines the persistence operations for the real
// knowledge_article table (see kb-tables.sql from Sajith, Sep 22 -- this
// replaces the original kb_articles/kb_article_history design). Two real
// behavior changes from the original design, both called out where they
// matter below: (1) there is no submitted_at equivalent at all in the real
// table; (2) the real table has no DB-level state-transition-validation
// trigger, unlike the original design's trg_kb_article_valid_transition --
// that validation needs to live in the service layer now, not here.
type KBArticleRepository interface {
	// CreateKBArticle inserts a new article in the draft state.
	CreateKBArticle(ctx context.Context, req domain.CreateKBArticleRequest) (domain.KBArticle, error)

	// GetKBArticleByID returns a single article by id.
	GetKBArticleByID(ctx context.Context, id string) (domain.KBArticle, error)

	// SearchKBArticles returns a filtered, paginated slice of articles together
	// with the total count of matching rows before pagination. COUNT and SELECT
	// are executed concurrently on separate pool connections. Only returns
	// latest=true rows -- older versions in a lineage are excluded from
	// search results by design; see ListKBArticleHistory for those.
	SearchKBArticles(ctx context.Context, req domain.SearchKBArticlesRequest) ([]domain.KBArticle, int, error)

	// UpdateKBArticleState transitions an article's state via a plain
	// UPDATE. Legal-transition validation is NOT enforced by the database
	// on this table (no trigger exists on the real schema) -- the CALLER
	// (service layer) must validate the transition before invoking this.
	UpdateKBArticleState(ctx context.Context, id string, req domain.UpdateKBArticleStateRequest) (domain.KBArticle, error)

	// UpdateKBArticleContent edits an existing draft's title/body IN PLACE
	// -- does NOT create a new versioned row. Callers are expected to only
	// invoke this while state == draft; the DB layer does not itself
	// enforce that. OPEN QUESTION (not yet settled with Sajith): should
	// editing published content instead create a new lineage row (via
	// base_version_id/latest) rather than updating in place? Deliberately
	// NOT implemented here until that's confirmed, to avoid guessing at
	// unstated business logic.
	UpdateKBArticleContent(ctx context.Context, id string, req domain.UpdateKBArticleContentRequest) (domain.KBArticle, error)

	// DeleteKBArticle removes an article only if it is currently in draft or
	// pending_review -- published/retired articles are never deleted.
	// Returns an apierror.ValidationError if the article is in neither state.
	DeleteKBArticle(ctx context.Context, id string) error

	// ListKBArticleHistory returns every version in the given article's
	// lineage (itself plus every row sharing its base_version_id chain),
	// newest first -- there is no separate history table in the real
	// schema; "history" IS other knowledge_article rows. Returns just the
	// one current row until something actually creates additional lineage
	// rows (see UpdateKBArticleContent's open question above).
	ListKBArticleHistory(ctx context.Context, kbArticleID string) ([]domain.KBArticleHistoryEntry, error)
}

type kbArticleRepo struct {
	db *pgxpool.Pool
}

// NewKBArticleRepository constructs a KBArticleRepository backed by the given connection pool.
func NewKBArticleRepository(db *pgxpool.Pool) KBArticleRepository {
	return &kbArticleRepo{db: db}
}

const kbArticleColumns = `id, knowledge_base_id, title, body, state, author_id,
	revised_by_id, source_case_id, rejection_comment, updated_by, base_version_id, latest,
	created_on, updated_on, published_on, retired_on`

func scanKBArticle(row interface {
	Scan(dest ...any) error
}, a *domain.KBArticle) error {
	return row.Scan(
		&a.ID, &a.KnowledgeBaseID, &a.Title, &a.Body, &a.State, &a.AuthorID,
		&a.RevisedByID, &a.SourceCaseID, &a.RejectionComment, &a.UpdatedBy, &a.BaseVersionID, &a.Latest,
		&a.CreatedOn, &a.UpdatedOn, &a.PublishedOn, &a.RetiredOn,
	)
}

// CreateKBArticle implements KBArticleRepository.
func (r *kbArticleRepo) CreateKBArticle(ctx context.Context, req domain.CreateKBArticleRequest) (domain.KBArticle, error) {
	query := fmt.Sprintf(`
		INSERT INTO knowledge_article (id, knowledge_base_id, title, body, state, author_id, created_by, updated_by, created_on, updated_on, latest)
		VALUES (gen_random_uuid(), $1, $2, $3, 'draft', $4::uuid, $4::text, $4::text, NOW(), NOW(), true)
		RETURNING %s`, kbArticleColumns)

	var a domain.KBArticle
	err := scanKBArticle(r.db.QueryRow(ctx, query, req.KnowledgeBaseID, req.Title, req.Body, req.AuthorID), &a)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return domain.KBArticle{}, &apierror.ValidationError{Msg: "one or more referenced IDs do not exist: " + pgErr.Detail}
			case "P0001":
				return domain.KBArticle{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.KBArticle{}, fmt.Errorf("create kb article: %w", err)
	}
	return a, nil
}

// GetKBArticleByID implements KBArticleRepository.
func (r *kbArticleRepo) GetKBArticleByID(ctx context.Context, id string) (domain.KBArticle, error) {
	query := fmt.Sprintf(`SELECT %s FROM knowledge_article WHERE id = $1`, kbArticleColumns)

	var a domain.KBArticle
	if err := scanKBArticle(r.db.QueryRow(ctx, query, id), &a); err != nil {
		return domain.KBArticle{}, fmt.Errorf("get kb article: %w", err)
	}
	return a, nil
}

// SearchKBArticles implements KBArticleRepository.
func (r *kbArticleRepo) SearchKBArticles(ctx context.Context, req domain.SearchKBArticlesRequest) ([]domain.KBArticle, int, error) {
	filterArgs := []any{}
	argIdx := 1

	where := "WHERE latest = true"

	if req.KnowledgeBaseID != "" {
		where += fmt.Sprintf(" AND knowledge_base_id = $%d", argIdx)
		filterArgs = append(filterArgs, req.KnowledgeBaseID)
		argIdx++
	}

	if len(req.States) > 0 {
		stateStrings := make([]string, len(req.States))
		for i, s := range req.States {
			stateStrings[i] = string(s)
		}
		where += fmt.Sprintf(" AND state = ANY($%d::text[])", argIdx)
		filterArgs = append(filterArgs, stateStrings)
		argIdx++
	}

	if req.AuthorID != "" {
		where += fmt.Sprintf(" AND author_id = $%d", argIdx)
		filterArgs = append(filterArgs, req.AuthorID)
		argIdx++
	}

	if req.SearchQuery != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(req.SearchQuery)
		pattern := "%" + escaped + "%"
		where += fmt.Sprintf(" AND (title ILIKE $%d ESCAPE '\\')", argIdx)
		filterArgs = append(filterArgs, pattern)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM knowledge_article " + where

	dataQuery := fmt.Sprintf(
		`SELECT %s FROM knowledge_article %s ORDER BY created_on DESC, id LIMIT $%d OFFSET $%d`,
		kbArticleColumns, where, argIdx, argIdx+1,
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
			if err := scanKBArticle(rows, &a); err != nil {
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
		return domain.KBArticle{}, fmt.Errorf("update kb article state: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := fmt.Sprintf(`
		UPDATE knowledge_article
		SET state = $2::text,
		    rejection_comment = $3,
		    updated_by = $4,
		    updated_on = NOW(),
		    published_on = CASE WHEN $2::text = 'published' THEN NOW() ELSE published_on END,
		    retired_on   = CASE WHEN $2::text = 'retired'   THEN NOW() ELSE retired_on   END
		WHERE id = $1
		RETURNING %s`, kbArticleColumns)

	var a domain.KBArticle
	err = scanKBArticle(tx.QueryRow(ctx, query, id, string(req.State), req.RejectionComment, req.UpdatedBy), &a)
	if err != nil {
		if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "P0001", "23514":
				return domain.KBArticle{}, &apierror.ValidationError{Msg: pgErr.Message}
			}
		}
		return domain.KBArticle{}, fmt.Errorf("update kb article state: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO knowledge_article_history (id, knowledge_article_id, title, body, state, changed_by)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)`,
		a.ID, a.Title, a.Body, string(a.State), req.UpdatedBy,
	); err != nil {
		return domain.KBArticle{}, fmt.Errorf("update kb article state: insert history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.KBArticle{}, fmt.Errorf("update kb article state: commit tx: %w", err)
	}
	return a, nil
}

// UpdateKBArticleContent implements KBArticleRepository.
func (r *kbArticleRepo) UpdateKBArticleContent(ctx context.Context, id string, req domain.UpdateKBArticleContentRequest) (domain.KBArticle, error) {
	query := fmt.Sprintf(`
		UPDATE knowledge_article
		SET title = $2, body = $3, updated_by = $4, updated_on = NOW()
		WHERE id = $1 AND state = 'draft'
		RETURNING %s`, kbArticleColumns)

	var a domain.KBArticle
	if err := scanKBArticle(r.db.QueryRow(ctx, query, id, req.Title, req.Body, req.UpdatedBy), &a); err != nil {
		return domain.KBArticle{}, fmt.Errorf("update kb article content: %w", err)
	}
	return a, nil
}

// DeleteKBArticle implements KBArticleRepository.
func (r *kbArticleRepo) DeleteKBArticle(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM knowledge_article WHERE id = $1 AND state IN ('draft', 'pending_review')`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete kb article: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.ValidationError{Msg: "article not found, or not in a deletable state"}
	}
	return nil
}

// ListKBArticleHistory implements KBArticleRepository.
func (r *kbArticleRepo) ListKBArticleHistory(ctx context.Context, kbArticleID string) ([]domain.KBArticleHistoryEntry, error) {
	const query = `
		SELECT id, knowledge_article_id, title, body, state, changed_by, created_on
		FROM knowledge_article_history
		WHERE knowledge_article_id = $1
		ORDER BY created_on DESC`

	rows, err := r.db.Query(ctx, query, kbArticleID)
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
