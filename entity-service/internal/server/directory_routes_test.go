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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/config"
)

// newDirectoryRouter builds the real router against a stubbed upstream, so the
// directory searches are exercised over HTTP exactly as a caller hits them.
func newDirectoryRouter(t *testing.T) http.Handler {
	t.Helper()

	upstream := http.NewServeMux()
	upstream.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-token", "expires_in": 3600})
	})
	// Groups are a live query against the backing data source. This is the
	// membership half of team resolution, and the half that stayed here when
	// the team registry moved out to the caller.
	upstream.HandleFunc("/groups/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"groups":[{"id":"11111111111111111111111111111111","name":"Some Group","active":true,"parent":null}],` +
			`"totalRecords":1,"offset":0,"limit":20}`))
	})
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)

	router, _ := NewRouter(nil, &config.Config{
		DataSource:                               config.DataSourceServiceNow,
		ServiceNowIntegrationServiceBaseURL:      srv.URL,
		ServiceNowIntegrationServiceTokenURL:     srv.URL + "/oauth2/token",
		ServiceNowIntegrationServiceClientID:     "test-client",
		ServiceNowIntegrationServiceClientSecret: "test-secret",
	})
	return router
}

func postDirectory(t *testing.T, router http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestGroupsSearch_IsStillALiveQuery: group membership is live state and cannot
// be answered from configuration, so it stayed here.
func TestGroupsSearch_IsStillALiveQuery(t *testing.T) {
	rec := postDirectory(t, newDirectoryRouter(t), "/groups/search", `{"pagination":{"limit":20}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /groups/search = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Some Group") {
		t.Fatalf("groups response = %s, want the upstream row", rec.Body.String())
	}
}

// TestCuratedCataloguesAreNoLongerServedHere locks in the move: the team
// registry and the role allow-list are the caller's configuration now, and this
// service no longer reads either. If someone reinstates a route here, the
// registry has two owners again and they will silently disagree.
// TestPostgresOnlyRoutesAreAbsentWithoutAPool pins that SN-mode startup
// without a reachable database must not register the side-table routes.
func TestPostgresOnlyRoutesAreAbsentWithoutAPool(t *testing.T) {
	router := newDirectoryRouter(t)
	for _, path := range []string{
		"/event-publish-failures/search",
		"/scheduled-tasks/attempts",
		"/salesforce/events",
	} {
		rec := postDirectory(t, router, path, `{}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s = %d, want 404 when the pool is nil", path, rec.Code)
		}
	}
}

func TestCuratedCataloguesAreNoLongerServedHere(t *testing.T) {
	router := newDirectoryRouter(t)
	for _, path := range []string{"/teams/search", "/roles/search"} {
		rec := postDirectory(t, router, path, `{}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s = %d, want 404 (the catalogue moved to the portal backend)", path, rec.Code)
		}
	}
}
