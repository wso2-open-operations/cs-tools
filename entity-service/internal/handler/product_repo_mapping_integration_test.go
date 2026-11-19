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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// product_repo_mapping.board_project_number and is_internal (migration 0304)
// round-trip through the repository and GET /products/github-repo. Skipped
// without ENTITY_TEST_DATABASE_URL.

package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

func TestProductRepoMapping_BoardAndInternalFlagsRoundTrip(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	const withBoard, withoutBoard = "PRM Test Product With Board", "PRM Test Product Without Board"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM product_repo_mapping WHERE product_name IN ($1, $2)`, withBoard, withoutBoard)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(ctx, `
		INSERT INTO product_repo_mapping (created_by, updated_by, product_name, owner, repository, github_label, board_project_number, is_internal)
		VALUES ('test', 'test', $1, 'acme-org', 'prm-internal', 'prm-board', 42, true)`, withBoard); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Defaults: no board, not internal.
	if _, err := pool.Exec(ctx, `
		INSERT INTO product_repo_mapping (created_by, updated_by, product_name, owner, repository, github_label)
		VALUES ('test', 'test', $1, 'acme-org', 'prm-public', 'prm-plain')`, withoutBoard); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Lead emails (migration 0310) are set by SQL and must never reach this route's answer.
	if _, err := pool.Exec(ctx, `UPDATE product_repo_mapping SET lead_emails = ARRAY['lead@example.com'] WHERE product_name = $1`, withBoard); err != nil {
		t.Fatalf("set lead_emails: %v", err)
	}

	repo := repository.NewProductRepoMappingRepository(pool)
	rows, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	found := 0
	for _, m := range rows {
		switch m.ProductName {
		case withBoard:
			found++
			if m.BoardProjectNumber == nil || *m.BoardProjectNumber != 42 || !m.IsInternal {
				t.Errorf("repository: %+v, want board 42 and internal", m)
			}
		case withoutBoard:
			found++
			if m.BoardProjectNumber != nil || m.IsInternal {
				t.Errorf("repository: %+v, want no board and not internal", m)
			}
		}
	}
	if found != 2 {
		t.Fatalf("ListActive returned %d of the 2 seeded rows", found)
	}

	h := handler.NewProductRepoMappingHandler(service.NewProductRepoMappingService(repo))
	get := func(name string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		h.Get(rec, httptest.NewRequest(http.MethodGet, "/products/github-repo?name="+url.QueryEscape(name), nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %q = %d: %s", name, rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	b := get(withBoard + " 1.2.3")
	if b["boardProjectNumber"] != float64(42) || b["isInternal"] != true || b["githubLabel"] != "prm-board" {
		t.Errorf("handler body = %v, want boardProjectNumber 42, isInternal true", b)
	}
	for k, v := range b {
		if strings.Contains(strings.ToLower(k), "lead") || strings.Contains(strings.ToLower(fmt.Sprint(v)), "lead@example.com") {
			t.Errorf("handler body %v exposes lead emails", b)
		}
	}
	p := get(withoutBoard)
	if _, has := p["boardProjectNumber"]; has {
		t.Errorf("handler body %v carries boardProjectNumber, want it omitted when unset", p)
	}
	if p["isInternal"] != false || p["githubLabel"] != "prm-plain" {
		t.Errorf("handler body = %v, want isInternal false", p)
	}
}
