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

// TestLegacyAttachmentRoutes_AreGone pins that the ServiceNow/base64 attachment
// routes are no longer served by any data source. Files are held by the portal
// backends in Azure Blob Storage; entity-service only keeps their metadata.
func TestLegacyAttachmentRoutes_AreGone(t *testing.T) {
	for _, ds := range []config.DataSource{config.DataSourcePostgres, config.DataSourcePostgresServiceNowDualWrite} {
		cfg := &config.Config{DataSource: ds}
		if ds == config.DataSourcePostgresServiceNowDualWrite {
			cfg.ServiceNowIntegrationServiceBaseURL = "https://sn.invalid"
			cfg.ServiceNowIntegrationServiceTokenURL = "https://sn.invalid/oauth2/token"
			cfg.ServiceNowIntegrationServiceClientID = "test-client"
			cfg.ServiceNowIntegrationServiceClientSecret = "test-secret"
		}
		withTestAuth(t, cfg)
		router, _ := NewRouter(nil, cfg)

		const id = "11111111-1111-1111-1111-111111111111"
		for _, c := range []struct{ method, path string }{
			{http.MethodPost, "/attachments"},
			{http.MethodPost, "/attachments/" + id + "/confirm"},
			{http.MethodGet, "/attachments/" + id},
			{http.MethodGet, "/attachments/" + id + "/content"},
			{http.MethodGet, "/attachments/" + id + "/case"},
			{http.MethodPatch, "/attachments/" + id},
			{http.MethodDelete, "/attachments/" + id},
		} {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s under %s = %d, want 404 or 405 (route removed)", c.method, c.path, ds, rec.Code)
			}
		}
	}
}

// TestAttachmentSearchRoute_IsStillServed pins that listing attachment metadata
// from PostgreSQL stays registered.
func TestAttachmentSearchRoute_IsStillServed(t *testing.T) {
	cfg := &config.Config{DataSource: config.DataSourcePostgres}
	withTestAuth(t, cfg)
	router, _ := NewRouter(nil, cfg)

	req := httptest.NewRequest(http.MethodPost, "/attachments/search", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
		t.Fatalf("POST /attachments/search = %d, want a handled response", rec.Code)
	}
}
