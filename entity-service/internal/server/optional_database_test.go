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

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// A DATA_SOURCE=servicenow deployment may legitimately have no database at
// all: every entity read and write goes to the SN integration service, and
// nothing in that path touches Postgres. The two Postgres-only feature sets
// (event_publish_failures, sla-status) are the exception, and they are
// skipped rather than allowed to panic on a nil pool.
//
// These tests exist because a nil *pgxpool.Pool does not fail at construction
// — a repository built around one is perfectly happy until the first query,
// which is a request-time panic rather than a startup error. Registering the
// handlers unconditionally would therefore look fine in every test that never
// calls them.
func newDBLessServiceNowRouter(t *testing.T) http.Handler {
	t.Helper()
	cfg := &config.Config{
		DataSource:                               config.DataSourceServiceNow,
		ServiceNowIntegrationServiceBaseURL:      "https://example.invalid",
		ServiceNowIntegrationServiceTokenURL:     "https://example.invalid/oauth2/token",
		ServiceNowIntegrationServiceClientID:     "test-client",
		ServiceNowIntegrationServiceClientSecret: "test-secret",
	}
	withTestAuth(t, cfg)
	router, _ := NewRouter(nil, cfg)
	return router
}

// TestNewRouter_SurvivesANilPool is the direct regression guard for the
// startup crash-loop: building the full dependency graph with no database
// must not panic.
func TestNewRouter_SurvivesANilPool(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewRouter panicked with a nil pool: %v", r)
		}
	}()
	if got := newDBLessServiceNowRouter(t); got == nil {
		t.Fatal("NewRouter returned a nil handler")
	}
}

// TestPostgresOnlyRoutesAreUnregisteredWithoutADatabase asserts the routes
// 404 rather than reaching a repository holding a nil pool. A 500 or a panic
// here would mean the handler was registered anyway.
func TestPostgresOnlyRoutesAreUnregisteredWithoutADatabase(t *testing.T) {
	router := newDBLessServiceNowRouter(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"create event publish failure", http.MethodPost, "/event-publish-failures", `{}`},
		{"search event publish failures", http.MethodPost, "/event-publish-failures/search", `{}`},
		{"resolve event publish failure", http.MethodPost, "/event-publish-failures/some-id/resolve", `{}`},
		{"register sla clock", http.MethodPost, "/cases/case-1/sla-clocks", `{}`},
		{"get sla clock", http.MethodGet, "/cases/case-1/sla-clocks/response", ""},
		{"set sla clock tier", http.MethodPatch, "/cases/case-1/sla-clocks/response/tiers/50", `{"status":"reached"}`},
		{"attempt scheduled task run", http.MethodPost, "/scheduled-tasks/attempts", `{}`},
		{"update scheduled task attempt", http.MethodPatch, "/scheduled-tasks/attempts/some-id", `{}`},
		{"list scheduled task runs", http.MethodGet, "/scheduled-tasks/attempts", ""},
		{"delete scheduled task runs", http.MethodDelete, "/scheduled-tasks/attempts?resolvedBefore=2026-01-01T00:00:00Z", ""},
		{"list saved filter views", http.MethodGet, "/users/me/saved-filter-views?listKey=cases", ""},
		{"save saved filter views", http.MethodPatch, "/users/me/saved-filter-views", `{}`},
		{"unsupported put saved filter views", http.MethodPut, "/users/me/saved-filter-views", `{}`},
		{"delete saved filter views", http.MethodDelete, "/users/me/saved-filter-views?listKey=cases&name=Mine", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s %s panicked with a nil pool: %v", tt.method, tt.path, r)
				}
			}()

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s = %d, want %d — the handler should not be registered without a database",
					tt.method, tt.path, rec.Code, http.StatusNotFound)
			}
		})
	}
}
