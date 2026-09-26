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

// membershipWriteRoutes is every route CSM_MIGRATION_PORTAL_WRITES_ENABLED
// gates, with a body where one is needed.
var membershipWriteRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"invite", http.MethodPost, "/projects/3f1e8d6a-3b4c-4d5e-8f90-123456789abc/contacts", `{"email":"jane@acme.com","roles":["Portal user"]}`},
	{"change roles", http.MethodPatch, "/projects/3f1e8d6a-3b4c-4d5e-8f90-123456789abc/contacts/jane%40acme.com", `{"roles":["Portal user"]}`},
	{"deactivate", http.MethodDelete, "/projects/3f1e8d6a-3b4c-4d5e-8f90-123456789abc/contacts/jane%40acme.com", ""},
	{"resend invitation", http.MethodPost, "/projects/3f1e8d6a-3b4c-4d5e-8f90-123456789abc/contacts/jane%40acme.com/resend-invitation", ""},
}

// newPortalWriteRouter builds a Postgres-mode router with a lazily-connected
// pool (pgxpool.New does not dial, so no database is needed) and the flag set
// as given.
func newPortalWriteRouter(t *testing.T, flagOn bool) http.Handler {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatalf("construct pool: %v", err)
	}
	t.Cleanup(pool.Close)

	cfg := &config.Config{
		DataSource:                      config.DataSourcePostgres,
		DBUser:                          "unused",
		DBPassword:                      "unused",
		DBName:                          "unused",
		SalesEntityBaseURL:              "https://example.invalid",
		SalesEntityTokenURL:             "https://example.invalid/oauth2/token",
		SalesEntityClientID:             "test-client",
		SalesEntityClientSecret:         "test-secret",
		CSMMigrationPortalWritesEnabled: flagOn,
	}
	withTestAuth(t, cfg)
	router, _ := NewRouter(pool, cfg)
	return router
}

// TestPortalMembershipWriteRoutesAreAbsentWhenTheFlagIsOff is the guard on the
// kill switch: off, the four routes do not exist at all. Not registering them
// (rather than registering a handler that refuses) is deliberate — until the
// Sales Entity create endpoints these depend on are deployed, a portal built
// against them must fail loudly instead of writing one system and not the
// other.
func TestPortalMembershipWriteRoutesAreAbsentWhenTheFlagIsOff(t *testing.T) {
	router := newPortalWriteRouter(t, false)
	for _, tc := range membershipWriteRoutes {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)
			// 404 for a path nothing matches, or 405 for PATCH/DELETE on
			// /projects/{id}/contacts/{contactId}, whose GET is registered
			// unconditionally — either way the request never reaches a
			// membership-write handler, which is what this asserts.
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 404/405 with the flag off", tc.method, tc.path, rec.Code)
			}
		})
	}
}

// TestPortalMembershipWriteRoutesArePresentWhenTheFlagIsOn is the other half:
// with the flag on the same requests reach a handler. They do not succeed —
// there is no database behind the pool — but anything other than a 404 proves
// the route exists, which is what this asserts.
func TestPortalMembershipWriteRoutesArePresentWhenTheFlagIsOn(t *testing.T) {
	router := newPortalWriteRouter(t, true)
	for _, tc := range membershipWriteRoutes {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want the route to be registered with the flag on", tc.method, tc.path, rec.Code)
			}
		})
	}
}

// TestPortalMembershipWriteRoutesNeedSalesEntity pins the rest of the gate:
// the flag alone is not enough, because half of every one of these writes
// goes to Salesforce.
func TestPortalMembershipWriteRoutesNeedSalesEntity(t *testing.T) {
	cfg := &config.Config{
		DataSource:                      config.DataSourcePostgres,
		CSMMigrationPortalWritesEnabled: true,
	}
	if cfg.HasPortalMembershipWrites() {
		t.Error("the writes must not be enabled without a sales-entity-service connection")
	}
	cfg.SalesEntityBaseURL = "https://example.invalid"
	cfg.SalesEntityTokenURL = "https://example.invalid/oauth2/token"
	cfg.SalesEntityClientID = "id"
	cfg.SalesEntityClientSecret = "secret"
	if !cfg.HasPortalMembershipWrites() {
		t.Error("a complete configuration with the flag on must enable the writes")
	}
	cfg.DataSource = config.DataSourceServiceNow
	if cfg.HasPortalMembershipWrites() {
		t.Error("the writes are a Postgres transaction; they must not be enabled on the ServiceNow data source")
	}
}
