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

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func postTitles(h *TitlesHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/issues/titles", bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	h.PostTitles(rec, req)
	return rec
}

func TestPostTitlesValidation400s(t *testing.T) {
	h := NewTitlesHandler(nil, "")

	cases := []struct {
		name string
		body string
	}{
		{"invalid JSON", `not json`},
		{"empty ids", `{"ids": []}`},
		{"too many ids", `{"ids": [` + strings.TrimSuffix(strings.Repeat("1,", 201), ",") + `]}`},
		{"non-positive id", `{"ids": [1, 0]}`},
		{"negative id", `{"ids": [-1]}`},
		{"body too large", `{"ids": [1], "padding": "` + strings.Repeat("x", titlesMaxBodyBytes+1) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postTitles(h, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPostTitlesNullsWhenNoGithubToken(t *testing.T) {
	pool := testPool(t)
	repoID := seedTitlesFixture(t, pool)
	var issueID int32
	if err := pool.QueryRow(context.Background(), `SELECT id FROM issues WHERE repository_id = $1`, repoID).Scan(&issueID); err != nil {
		t.Fatalf("lookup issue id: %v", err)
	}

	h := NewTitlesHandler(pool, "") // no token
	rec := postTitles(h, `{"ids": [`+strconv.Itoa(int(issueID))+`]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var body titlesResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if title, ok := body.Titles[strconv.Itoa(int(issueID))]; !ok || title != nil {
		t.Errorf("expected null title without a GITHUB_TOKEN, got %v (present=%v)", title, ok)
	}
}

func TestPostTitlesNullForUnknownID(t *testing.T) {
	h := NewTitlesHandler(testPool(t), "")
	rec := postTitles(h, `{"ids": [999999999]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body titlesResponseBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if title, ok := body.Titles["999999999"]; !ok || title != nil {
		t.Errorf("expected null title for an unknown id, got %v (present=%v)", title, ok)
	}
}

func seedTitlesFixture(t *testing.T, pool *pgxpool.Pool) (repoID int32) {
	t.Helper()
	ctx := context.Background()
	var projectID int32
	if err := pool.QueryRow(ctx, `INSERT INTO projects (github_project_id, title, enabled) VALUES ($1,$2,true) RETURNING id`,
		"PVT_titles_test", "Titles Test").Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO repositories (owner, name, issue_query, sla_project_id, enabled)
		VALUES ($1,$2,$3,$4,true) RETURNING id
	`, "test-owner", "test-titles", `label:"Origin/CS"`, projectID).Scan(&repoID); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO issues (repository_id, github_number, state, github_created_at, github_updated_at)
		VALUES ($1, 1, 'OPEN', now(), now())
	`, repoID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM issues WHERE repository_id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM repositories WHERE id = $1`, repoID)
		pool.Exec(bg, `DELETE FROM projects WHERE id = $1`, projectID)
	})
	return repoID
}
