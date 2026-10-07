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
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
)

// wantReadOnlyRoutes is the explicit allowlist of routes served from the read
// pool. It is deliberately a second copy of readOnlyRoutes: adding or removing
// a route is then a two-place, reviewable change, and the reviewer is asked
// the one question that matters (does anything on this route's handler ->
// service -> repository path write, for any data source?). See "Read-pool
// route opt-in" in entity-service/CLAUDE.md.
var wantReadOnlyRoutes = []string{
	"GET /accounts/{id}",
	"GET /alerts/{id}",
	"GET /announcement-requests/{id}",
	"GET /announcement-requests/{id}/deliveries",
	"GET /announcement-requests/{id}/updates",
	"GET /attachments/{id}",
	"GET /attachments/{id}/content",
	"GET /cases/{id}",
	"GET /cases/{id}/escalations",
	"GET /cases/{id}/feedback",
	"GET /catalogs/{catalogId}/items/{catalogItemId}/variables",
	"GET /change-requests/{id}",
	"GET /change-requests/{id}/approvals",
	"GET /cloud-status/availabilities",
	"GET /cloud-status/availability-history",
	"GET /cloud-status/incidents",
	"GET /cloud-status/incidents/{id}",
	"GET /cloud-status/monitors",
	"GET /comments/{id}/history",
	"GET /conversations/{id}",
	"GET /groups/{id}",
	"GET /incident-tasks/{id}",
	"GET /incidents/{id}",
	"GET /invoices/{id}",
	"GET /kb-articles/{id}",
	"GET /kb-articles/{id}/history",
	"GET /knowledge-bases",
	"GET /metadata",
	"GET /onboarding-steps/{membershipSfId}",
	"GET /opportunities/{id}",
	"GET /outages/metadata",
	"GET /outages/{id}",
	"GET /outages/{id}/notification-state",
	"GET /outages/{number}/communication-log",
	"GET /plg/analytics/dashboard",
	"GET /plg/lifecycle",
	"GET /plg/organizations/{organizationId}",
	"GET /plg/organizations/{organizationId}/products/{productCode}",
	"GET /plg/pairings/{orgPlatformId}/location",
	"GET /plg/playbook-run-tasks/{taskId}/shape",
	"GET /plg/playbooks",
	"GET /plg/playbooks/{playbookId}",
	"GET /plg/products",
	"GET /problems/{id}",
	"GET /products/github-repo",
	"GET /products/vulnerabilities/meta",
	"GET /products/vulnerabilities/{id}",
	"GET /projects/{id}",
	"GET /projects/{id}/cases/stats",
	"GET /projects/{id}/change-requests/stats",
	"GET /projects/{id}/contacts/{contactId}",
	"GET /projects/{id}/conversations/stats",
	"GET /projects/{id}/deployments/stats",
	"GET /projects/{id}/metadata",
	"GET /projects/{id}/stats",
	"GET /projects/{id}/time-cards/stats",
	"GET /scheduled-tasks/attempts",
	"GET /sla-duration-policy",
	"GET /sla-status",
	"GET /slas/{id}",
	"GET /smart-alerts/{id}",
	"GET /tags/search",
	"GET /tasks/{id}",
	"GET /team-schedule/activity",
	"GET /team-schedule/catalogue",
	"GET /team-schedule/edit-markers",
	"GET /team-schedule/members",
	"GET /team-schedule/my-lead-teams",
	"GET /team-schedule/on-duty",
	"GET /teams/{id}/members",
	"GET /users/me",
	"GET /users/me/saved-filter-views",
	"GET /users/{id}",
	"POST /accounts/search",
	"POST /accounts/{id}/contacts/search",
	"POST /announcement-requests/search",
	"POST /attachments/search",
	"POST /call-requests/search",
	"POST /call-requests/search-all",
	"POST /cases/aggregate",
	"POST /cases/feedback/aggregate",
	"POST /cases/feedback/search",
	"POST /cases/search",
	"POST /cases/time-cards/search",
	"POST /cases/{id}/activities/search",
	"POST /cases/{id}/comments/search",
	"POST /cases/{id}/tasks/search",
	"POST /catalogs/search",
	"POST /change-requests/aggregate",
	"POST /change-requests/search",
	"POST /comments/search",
	"POST /configuration-items/search",
	"POST /conversations/search",
	"POST /deployed-products/projects/search",
	"POST /deployed-products/search",
	"POST /deployed-products/{id}/metrics/search",
	"POST /deployed-products/{id}/metrics/usage-counts/search",
	"POST /deployments/search",
	"POST /escalations/search",
	"POST /event-publish-failures/search",
	"POST /groups/search",
	"POST /incident-tasks/aggregate",
	"POST /incident-tasks/search",
	"POST /incidents/aggregate",
	"POST /incidents/search",
	"POST /incidents/{id}/activities/search",
	"POST /instances/metrics/search",
	"POST /instances/metrics/stats/search",
	"POST /instances/search",
	"POST /instances/usages/search",
	"POST /instances/usages/stats/search",
	"POST /invoices/search",
	"POST /kb-articles/search",
	"POST /kb-manager-groups/search",
	"POST /kb-manager-users/search",
	"POST /onboarding-steps/search",
	"POST /opportunities/search",
	"POST /outages/search",
	"POST /outages/{id}/communications/search",
	"POST /plg/organizations/search",
	"POST /plg/registrations/search",
	"POST /plg/users/search",
	"POST /plg/work-queue/search",
	"POST /problems/aggregate",
	"POST /problems/search",
	"POST /products/search",
	"POST /products/vulnerabilities/search",
	"POST /products/{id}/versions/search",
	"POST /project-opportunity-links/search",
	"POST /projects/search",
	"POST /projects/{id}/contacts/search",
	"POST /search",
	"POST /service-offerings/search",
	"POST /services/search",
	"POST /slas/search",
	"POST /tags/search",
	"POST /tasks/search",
	"POST /team-schedule/absences/search",
	"POST /team-schedule/assignments/search",
	"POST /time-cards/search",
	"POST /users/search",
}

func TestReadOnlyRoutesAllowlist(t *testing.T) {
	want := map[string]bool{}
	for _, p := range wantReadOnlyRoutes {
		if want[p] {
			t.Errorf("wantReadOnlyRoutes lists %q twice", p)
		}
		want[p] = true
	}

	var extra, missing []string
	for p := range readOnlyRoutes {
		if !want[p] {
			extra = append(extra, p)
		}
	}
	for p := range want {
		if _, ok := readOnlyRoutes[p]; !ok {
			missing = append(missing, p)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	if len(extra) > 0 {
		t.Errorf("marked read-only in routes.go but not in the test allowlist:\n  %s", strings.Join(extra, "\n  "))
	}
	if len(missing) > 0 {
		t.Errorf("in the test allowlist but not marked read-only in routes.go:\n  %s", strings.Join(missing, "\n  "))
	}
}

// TestReadOnlyRoutesShape keeps the allowlist to the route classes that can be
// pure reads. It does not prove a route is read-only (only reading its call
// chain does); it stops a write route, a health probe or a Salesforce ingest
// route from being added by a slip.
func TestReadOnlyRoutesShape(t *testing.T) {
	for p := range readOnlyRoutes {
		method, path, ok := strings.Cut(p, " ")
		if !ok {
			t.Errorf("%q is not a method-prefixed pattern", p)
			continue
		}
		switch method {
		case "GET":
		case "POST":
			if !strings.HasSuffix(path, "/search") && !strings.HasSuffix(path, "/search-all") && !strings.HasSuffix(path, "/aggregate") {
				t.Errorf("%q: only POST .../search, .../search-all and .../aggregate may be read-only", p)
			}
		default:
			t.Errorf("%q: %s routes are never read-only", p, method)
		}
		for _, banned := range []string{"/health", "/salesforce"} {
			if strings.Contains(path, banned) {
				t.Errorf("%q: %s routes must not be read-only", p, banned)
			}
		}
	}
}

// TestReadOnlyRoutesAreRegistered fails when an allowlisted pattern is not
// registered anywhere, which would mean the mark silently wraps nothing (a typo
// or a route that was renamed).
func TestReadOnlyRoutesAreRegistered(t *testing.T) {
	var src strings.Builder
	for _, f := range []string{"routes.go", "plg_routes.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src.Write(b)
	}
	registered := map[string]bool{}
	for _, m := range regexp.MustCompile(`mux\.HandleFunc\(\s*"([^"]+)"`).FindAllStringSubmatch(src.String(), -1) {
		registered[m[1]] = true
	}
	if len(registered) == 0 {
		t.Fatal("parsed no registrations; the parser is broken, not the routes")
	}
	for p := range readOnlyRoutes {
		if !registered[p] {
			t.Errorf("%q is marked read-only but no mux.HandleFunc registers it", p)
		}
	}
}

// TestReadPoolMuxMarksOnlyListedRoutes drives the real readPoolMux with stub
// handlers: a listed pattern's handler sees a read-marked context, an unlisted
// one does not, and path values still resolve through the wrapper.
func TestReadPoolMuxMarksOnlyListedRoutes(t *testing.T) {
	var seenReadOnly bool
	var seenID string
	stub := func(w http.ResponseWriter, r *http.Request) {
		seenReadOnly = db.IsReadOnly(r.Context())
		seenID = r.PathValue("id")
		w.WriteHeader(http.StatusNoContent)
	}

	mux := newReadPoolMux()
	mux.HandleFunc("POST /cases/search", stub)          // listed
	mux.HandleFunc("GET /cases/{id}", stub)             // listed, has a path value
	mux.Handle("GET /users/me", http.HandlerFunc(stub)) // listed, via Handle
	mux.HandleFunc("POST /cases", stub)                 // a create: never listed
	mux.HandleFunc("PATCH /cases/{id}", stub)           // an update: never listed
	mux.HandleFunc("GET /health", stub)                 // probe: never listed

	tests := []struct {
		name, method, target string
		wantReadOnly         bool
		wantID               string
	}{
		{"listed search", http.MethodPost, "/cases/search", true, ""},
		{"listed by-id keeps path value", http.MethodGet, "/cases/abc", true, "abc"},
		{"listed via Handle", http.MethodGet, "/users/me", true, ""},
		{"create is unmarked", http.MethodPost, "/cases", false, ""},
		{"update is unmarked", http.MethodPatch, "/cases/abc", false, "abc"},
		{"health is unmarked", http.MethodGet, "/health", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seenReadOnly, seenID = !tc.wantReadOnly, "unset"
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.target, nil))
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204 (route not matched?)", rec.Code)
			}
			if seenReadOnly != tc.wantReadOnly {
				t.Errorf("db.IsReadOnly(ctx) = %v, want %v", seenReadOnly, tc.wantReadOnly)
			}
			if seenID != tc.wantID {
				t.Errorf("PathValue(id) = %q, want %q", seenID, tc.wantID)
			}
		})
	}
}
