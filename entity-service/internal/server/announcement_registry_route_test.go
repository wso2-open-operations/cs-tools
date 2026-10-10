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

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

const announcementRegistryCasesPath = "/announcements/registry/cases"

func newPostgresRouterForRegistry(t *testing.T) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://entity:entity@127.0.0.1:1/entity?sslmode=disable")
	if err != nil {
		t.Fatalf("build pool: %v", err)
	}
	t.Cleanup(pool.Close)
	cfg := &config.Config{DataSource: config.DataSourcePostgres}
	withTestAuth(t, cfg)
	router, _ := NewRouter(pool, cfg)
	return router
}

// The registry's one-shot read is a Postgres-only route: where cases are served
// by ServiceNow it must not exist, so the portal backend gets a 404 and falls
// back to paging /cases/search instead of reaching a repository with no
// database behind it.
func TestAnnouncementRegistryCasesRouteIsAbsentOnTheServiceNowDataSource(t *testing.T) {
	router := newDBLessServiceNowRouter(t)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, announcementRegistryCasesPath, strings.NewReader(`{}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 on the ServiceNow data source", rec.Code)
	}
}

// On Postgres the route exists, is POST only, and an anonymous caller is
// refused before anything is read.
func TestAnnouncementRegistryCasesRouteOnPostgres(t *testing.T) {
	router := newPostgresRouterForRegistry(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, announcementRegistryCasesPath, strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST status = %d, want 401 for an anonymous caller", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, announcementRegistryCasesPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405: the route is POST only", rec.Code)
	}
}

const announcementRegistryRowsPath = "/announcements/registry/rows"

// The grouped registry read is Postgres-only like the case read: absent on any
// other data source, POST only on Postgres, and an anonymous caller is
// refused before anything is read.
func TestAnnouncementRegistryRowsRoute(t *testing.T) {
	sn := newDBLessServiceNowRouter(t)
	rec := httptest.NewRecorder()
	sn.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, announcementRegistryRowsPath, strings.NewReader(`{}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-Postgres data source: status = %d, want 404", rec.Code)
	}

	router := newPostgresRouterForRegistry(t)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, announcementRegistryRowsPath, strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST status = %d, want 401 for an anonymous caller", rec.Code)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, announcementRegistryRowsPath, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405: the route is POST only", rec.Code)
	}
}
