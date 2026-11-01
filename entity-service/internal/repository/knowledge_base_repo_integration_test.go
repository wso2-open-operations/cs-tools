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

// Integration test for KnowledgeBaseRepository against a live PostgreSQL
// instance holding the real knowledge_base table (migration 0044), where
// active is nullable. Same DSN and skip-when-unset pattern as
// kb_article_repo_integration_test.go (package repository_test, reuses
// caseStatsPool):
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run KnowledgeBase

package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	kbActiveNullID = "3b111111-0000-0000-0000-000000000001" // active NULL
	kbActiveTrueID = "3b111111-0000-0000-0000-000000000002" // active true
)

// TestKnowledgeBaseListNullActive: one knowledge_base row with NULL active
// must not fail the whole list call; it reads as inactive, and the other rows
// are unaffected.
func TestKnowledgeBaseListNullActive(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM knowledge_base WHERE id IN ($1, $2)`, kbActiveNullID, kbActiveTrueID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx, `INSERT INTO knowledge_base (id, created_on, updated_on, created_by, updated_by, title, active)
	          VALUES ($1, now(), now(), 'kb-active-test', 'kb-active-test', 'KB active NULL', NULL),
	                 ($2, now(), now(), 'kb-active-test', 'kb-active-test', 'KB active true', true)`, kbActiveNullID, kbActiveTrueID); err != nil {
		t.Fatalf("seed: %v", err)
	}

	repo := repository.NewKnowledgeBaseRepository(pool)
	kbs, err := repo.ListKnowledgeBases(ctx)
	if err != nil {
		t.Fatalf("ListKnowledgeBases with a NULL active row: %v", err)
	}
	byID := map[string]domain.KnowledgeBase{}
	for _, kb := range kbs {
		byID[kb.ID] = kb
	}
	if kb, ok := byID[kbActiveNullID]; !ok || kb.Active {
		t.Errorf("NULL-active row = %+v (present %v), want present with Active false", kb, ok)
	}
	if kb, ok := byID[kbActiveTrueID]; !ok || !kb.Active {
		t.Errorf("active row = %+v (present %v), want present with Active true", kb, ok)
	}

	// The by-id writes share the same column list: a NULL-active row must
	// come back from RETURNING too.
	renamed, err := repo.UpdateKnowledgeBaseName(ctx, kbActiveNullID, domain.UpdateKnowledgeBaseRequest{Name: "KB active NULL (renamed)"})
	if err != nil || renamed.Active || renamed.Title != "KB active NULL (renamed)" {
		t.Errorf("UpdateKnowledgeBaseName on NULL-active row: %+v, err %v", renamed, err)
	}
}

func TestKnowledgeBaseByIDMissingAndMalformed(t *testing.T) {
	pool := caseStatsPool(t)
	repo := repository.NewKnowledgeBaseRepository(pool)
	ctx := context.Background()

	var nfe *apierror.NotFoundError
	var ve *apierror.ValidationError

	missing := "3b999999-0000-0000-0000-000000000009"
	if _, err := repo.UpdateKnowledgeBaseName(ctx, missing, domain.UpdateKnowledgeBaseRequest{Name: "x"}); !errors.As(err, &nfe) {
		t.Errorf("rename missing id: err = %#v, want NotFoundError", err)
	}
	if _, err := repo.SetKnowledgeBaseActive(ctx, missing, domain.UpdateKnowledgeBaseActiveRequest{IsActive: true}); !errors.As(err, &nfe) {
		t.Errorf("set active missing id: err = %#v, want NotFoundError", err)
	}
	if _, err := repo.UpdateKnowledgeBaseName(ctx, "not-a-uuid", domain.UpdateKnowledgeBaseRequest{Name: "x"}); !errors.As(err, &ve) {
		t.Errorf("rename malformed id: err = %#v, want ValidationError", err)
	}
	if _, err := repo.SetKnowledgeBaseActive(ctx, "not-a-uuid", domain.UpdateKnowledgeBaseActiveRequest{IsActive: true}); !errors.As(err, &ve) {
		t.Errorf("set active malformed id: err = %#v, want ValidationError", err)
	}
}
