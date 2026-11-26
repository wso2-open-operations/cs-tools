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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// The two product usage reports read deployed products from Postgres, so they
// are registered only when deployed products are read from Postgres
// (cfg.UsageReadRoute). Under dual-write with USAGE_READ_MODE=legacy the
// routes are absent (404): the monthly task fails loudly instead of mailing
// a report that misses customers. Anything else means the route exists; an
// anonymous caller is then refused (401) before any query.
func TestProductUsageReportRoutesFollowTheUsageReadRoute(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://entity:entity@127.0.0.1:1/entity?sslmode=disable")
	if err != nil {
		t.Fatalf("build pool: %v", err)
	}
	t.Cleanup(pool.Close)

	for _, tc := range []struct {
		name    string
		mode    config.UsageReadMode
		present bool
	}{
		{"postgres reads", config.UsageReadModePostgres, true},
		{"legacy reads", config.UsageReadModeLegacy, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				DataSource:                               config.DataSourcePostgresServiceNowDualWrite,
				UsageReadMode:                            tc.mode,
				ServiceNowIntegrationServiceBaseURL:      "https://example.invalid",
				ServiceNowIntegrationServiceTokenURL:     "https://example.invalid/oauth2/token",
				ServiceNowIntegrationServiceClientID:     "test-client",
				ServiceNowIntegrationServiceClientSecret: "test-secret",
			}
			withTestAuth(t, cfg)
			router, _ := NewRouter(pool, cfg)
			for _, path := range []string{"/reports/eol-product-usage", "/reports/deprecated-product-usage"} {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				if absent := rec.Code == http.StatusNotFound; absent == tc.present {
					t.Errorf("%s: status %d, want present=%v", path, rec.Code, tc.present)
				}
			}
		})
	}
}
