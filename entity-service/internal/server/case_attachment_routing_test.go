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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// TestCaseAttachmentRoutes_UsePostgresUnderPlainPostgres is the control case:
// under plain DataSource=postgres (no database in this test, same
// nil-pool-safe pattern as optional_database_test.go), POST /attachments
// must NOT reach any ServiceNow integration service call -- there is none
// configured -- and must instead reach the Postgres-backed path, which fails
// against a nil pool rather than a network call. This is what
// TestCaseAttachmentRoutes_UseServiceNowUnderPostgresServiceNowDualWrite below
// is contrasted against: that test proves ONLY the fallback DataSource
// redirects attachments to ServiceNow, not that plain postgres started doing
// so too.
func TestCaseAttachmentRoutes_UsePostgresUnderPlainPostgres(t *testing.T) {
	cfg := &config.Config{DataSource: config.DataSourcePostgres}
	withTestAuth(t, cfg)
	router, _ := NewRouter(nil, cfg)

	rec := postAttachment(t, router)

	// "storageKey is required" is the CSM-native (Postgres) data source's
	// own validation message (domain.CreateAttachmentRequest.StorageKey's
	// doc comment: required for Postgres, ignored for ServiceNow) -- seeing
	// it here proves this request reached the Postgres-backed
	// CreateCaseAttachment path specifically, not merely "some" response.
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "storageKey is required") {
		t.Fatalf("expected a 400 citing storageKey (proof this reached the Postgres path), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCaseAttachmentRoutes_UseServiceNowUnderPostgresServiceNowDualWrite is the
// regression guard for the routing fix: under
// DataSource=postgres-servicenow-dual-write, case attachment routes must reach
// ServiceNow (via the same snCaseService already used for case CREATE/
// UPDATE's mirror), never the Postgres-backed case_attachment path -- the
// sftpgo-backed Postgres attachment implementation is not production-ready,
// so this mode must never route a request to it, regardless of how case
// metadata itself is wired in this mode.
func TestCaseAttachmentRoutes_UseServiceNowUnderPostgresServiceNowDualWrite(t *testing.T) {
	var snAttachmentCallReceived bool
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	mux.HandleFunc("/attachments", func(w http.ResponseWriter, r *http.Request) {
		snAttachmentCallReceived = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Attachment created successfully.",
			"attachment": map[string]any{
				"id": "11111111111111111111111111111111", "sizeBytes": 5,
				"createdOn": "2026-09-21 12:00:00", "createdBy": "jane.doe@example.com",
				"downloadUrl": "https://example.invalid/download",
			},
		})
	})
	snServer := httptest.NewServer(mux)
	defer snServer.Close()

	cfg := &config.Config{
		DataSource:                               config.DataSourcePostgresServiceNowDualWrite,
		ServiceNowIntegrationServiceBaseURL:      snServer.URL,
		ServiceNowIntegrationServiceTokenURL:     snServer.URL + "/oauth2/token",
		ServiceNowIntegrationServiceClientID:     "test-client",
		ServiceNowIntegrationServiceClientSecret: "test-secret",
	}
	withTestAuth(t, cfg)
	// NewRouter itself doesn't call Validate (only cmd/api/main.go does), and
	// this test needs no real Postgres pool for the same reason
	// optional_database_test.go's tests don't: attachment routes must reach
	// ServiceNow here, never a repository query, so a nil pool never gets
	// touched.
	router, _ := NewRouter(nil, cfg)

	rec := postAttachment(t, router)

	if !snAttachmentCallReceived {
		t.Fatalf("expected POST /attachments to reach the ServiceNow integration service; it did not (response: %d %s)", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("POST /attachments = %d, want 201 — body: %s", rec.Code, rec.Body.String())
	}
}

// postAttachment issues a minimal, otherwise-valid POST /attachments request
// for a "case" reference and returns the recorded response. The file is a
// tiny base64 data URI ("hello"), well under CreateCaseAttachment's size
// checks in either backend.
func postAttachment(t *testing.T, router http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	body := `{
		"referenceId": "22222222-2222-2222-2222-222222222222",
		"referenceType": "case",
		"name": "notes.txt",
		"type": "text/plain",
		"file": "data:text/plain;base64,aGVsbG8="
	}`
	req := httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
