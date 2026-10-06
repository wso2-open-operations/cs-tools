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

// This is an integration test: it runs KBArticleRepository against a live
// PostgreSQL instance holding the real knowledge_article table (migration
// 0044), regression-testing a read-side crash where one row with a NULL body,
// state, author_id, knowledge_base_id or latest failed a whole
// SearchKBArticles page with "cannot scan NULL into *string". All of those
// columns are nullable in the table and NULL on many real rows, which only a
// real row can show. Same DSN and skip-when-unset pattern as
// project_case_stats_repo_integration_test.go (package repository_test,
// reuses caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run KBArticleNullColumns

package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	kbNullBaseID   = "3a111111-0000-0000-0000-000000000001"
	kbNullUserID   = "3a222222-0000-0000-0000-000000000001"
	kbNullBareID   = "3a333333-0000-0000-0000-000000000001" // body, state and author_id NULL
	kbNullFullID   = "3a333333-0000-0000-0000-000000000002" // every column populated
	kbNullDraftID  = "3a333333-0000-0000-0000-000000000003" // knowledge_base_id, author_id and latest NULL; draft
	kbNullBodyText = "Reset it from the account settings page."
)

// seedKBArticleNullFixture creates one knowledge base and three
// knowledge_article rows covering the NULL shapes found on real data.
func seedKBArticleNullFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge_article WHERE id IN ($1, $2, $3)`, kbNullBareID, kbNullFullID, kbNullDraftID)
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge_base WHERE id = $1`, kbNullBaseID)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, kbNullUserID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name)
	          VALUES ($1, now(), now(), 'kb-null-test-author')`, kbNullUserID)

	mustExec(`INSERT INTO knowledge_base (id, created_on, updated_on, created_by, updated_by, title, active)
	          VALUES ($1, now(), now(), 'kb-null-test', 'kb-null-test', 'KB null test', true)`, kbNullBaseID)

	// Newest first in the search's created_on DESC order: bare, full, draft.
	mustExec(`INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, body, state, knowledge_base_id, author_id, latest)
	          VALUES ($1, now(), now(), 'kb-null-test', 'kb-null-test', 'Bare article', NULL, NULL, $2, NULL, true)`,
		kbNullBareID, kbNullBaseID)

	mustExec(`INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, body, state, knowledge_base_id, author_id, latest)
	          VALUES ($1, now() - interval '1 minute', now(), 'kb-null-test', 'kb-null-test', 'Full article', $4, 'published', $2, $3, true)`,
		kbNullFullID, kbNullBaseID, kbNullUserID, kbNullBodyText)

	mustExec(`INSERT INTO knowledge_article (id, created_on, updated_on, created_by, updated_by, title, body, state, knowledge_base_id, author_id, latest)
	          VALUES ($1, now() - interval '2 minutes', now(), 'kb-null-test', 'kb-null-test', 'Draft without base', 'old body', 'draft', NULL, NULL, NULL)`,
		kbNullDraftID)
}

// TestKBArticleNullColumnsSearch is the original failure: a page
// containing a row with NULL body/state/author_id must come back whole, with
// those fields as "" rather than failing the request.
func TestKBArticleNullColumnsSearch(t *testing.T) {
	pool := caseStatsPool(t)
	seedKBArticleNullFixture(t, pool)

	repo := repository.NewKBArticleRepository(pool)

	articles, total, err := repo.SearchKBArticles(context.Background(), domain.SearchKBArticlesRequest{
		KnowledgeBaseID: kbNullBaseID,
		Pagination:      domain.Pagination{Limit: 50, Offset: 0},
	})
	if err != nil {
		t.Fatalf("SearchKBArticles: %v", err)
	}
	if total != 2 || len(articles) != 2 {
		t.Fatalf("total = %d, len = %d, want 2 and 2 (the bare and full articles; the draft has no knowledge base)", total, len(articles))
	}

	byID := map[string]domain.KBArticle{}
	for _, a := range articles {
		byID[a.ID] = a
	}

	bare, ok := byID[kbNullBareID]
	if !ok {
		t.Fatalf("bare article %s missing from results: %+v", kbNullBareID, articles)
	}
	if bare.Body != "" || bare.State != "" || bare.AuthorID != "" {
		t.Errorf("bare article body/state/authorId = %q/%q/%q, want all empty", bare.Body, bare.State, bare.AuthorID)
	}
	if bare.KnowledgeBaseID != kbNullBaseID || bare.Title != "Bare article" || !bare.Latest {
		t.Errorf("bare article = %+v, want knowledgeBaseId %s, title \"Bare article\", latest true", bare, kbNullBaseID)
	}

	full, ok := byID[kbNullFullID]
	if !ok {
		t.Fatalf("full article %s missing from results: %+v", kbNullFullID, articles)
	}
	if full.Body != kbNullBodyText || full.State != domain.KBArticleStatePublished || full.AuthorID != kbNullUserID {
		t.Errorf("full article body/state/authorId = %q/%q/%q, want %q/published/%s", full.Body, full.State, full.AuthorID, kbNullBodyText, kbNullUserID)
	}
}

// TestKBArticleNullColumnsGetByID covers the single-row read, which
// shares scanKBArticle with search, for the rows search itself never returns
// (latest NULL) as well as the NULL body/state/author_id shape.
func TestKBArticleNullColumnsGetByID(t *testing.T) {
	pool := caseStatsPool(t)
	seedKBArticleNullFixture(t, pool)

	repo := repository.NewKBArticleRepository(pool)
	ctx := context.Background()

	bare, err := repo.GetKBArticleByID(ctx, kbNullBareID)
	if err != nil {
		t.Fatalf("GetKBArticleByID(bare): %v", err)
	}
	if bare.Body != "" || bare.State != "" || bare.AuthorID != "" {
		t.Errorf("bare body/state/authorId = %q/%q/%q, want all empty", bare.Body, bare.State, bare.AuthorID)
	}

	draft, err := repo.GetKBArticleByID(ctx, kbNullDraftID)
	if err != nil {
		t.Fatalf("GetKBArticleByID(draft): %v", err)
	}
	if draft.KnowledgeBaseID != "" || draft.AuthorID != "" || draft.Latest {
		t.Errorf("draft knowledgeBaseId/authorId/latest = %q/%q/%v, want empty/empty/false", draft.KnowledgeBaseID, draft.AuthorID, draft.Latest)
	}
	if draft.Body != "old body" || draft.State != domain.KBArticleStateDraft {
		t.Errorf("draft body/state = %q/%q, want \"old body\"/draft", draft.Body, draft.State)
	}
}

// TestKBArticleNullColumnsUpdateContent covers the write path's
// RETURNING scan: editing a draft whose knowledge_base_id/author_id/latest
// are NULL must return the updated row rather than fail after the UPDATE.
func TestKBArticleNullColumnsUpdateContent(t *testing.T) {
	pool := caseStatsPool(t)
	seedKBArticleNullFixture(t, pool)

	repo := repository.NewKBArticleRepository(pool)

	got, err := repo.UpdateKBArticleContent(context.Background(), kbNullDraftID, domain.UpdateKBArticleContentRequest{
		Title:     "Draft without base (edited)",
		Body:      "new body",
		UpdatedBy: "kb-null-test",
	})
	if err != nil {
		t.Fatalf("UpdateKBArticleContent: %v", err)
	}
	if got.Title != "Draft without base (edited)" || got.Body != "new body" {
		t.Errorf("title/body = %q/%q, want the edited values", got.Title, got.Body)
	}
	if got.KnowledgeBaseID != "" || got.AuthorID != "" || got.Latest {
		t.Errorf("knowledgeBaseId/authorId/latest = %q/%q/%v, want empty/empty/false", got.KnowledgeBaseID, got.AuthorID, got.Latest)
	}
}
