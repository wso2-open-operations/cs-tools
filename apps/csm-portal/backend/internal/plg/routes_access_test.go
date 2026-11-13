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

package plg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	csmhandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/handler"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	plghandler "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/handler"
)

// The four routes that author a playbook template. Everything else PLG serves
// is PermUsePlg. Kept as data rather than derived from the path, because
// "starts with /plg/playbooks" is exactly the wrong rule: GET /plg/playbooks
// and GET /plg/playbooks/{id} are reads a CS engineer must keep.
var playbookManagementRoutes = []struct{ method, path string }{
	{http.MethodPost, "/plg/products/choreo/playbooks"},
	{http.MethodPatch, "/plg/playbooks/pb-1"},
	{http.MethodPut, "/plg/playbooks/pb-1/tasks"},
	{http.MethodDelete, "/plg/playbooks/pb-1"},
}

// Every other PLG route, one per registered pattern.
var everydayRoutes = []struct{ method, path string }{
	{http.MethodGet, "/plg/me"},
	{http.MethodGet, "/plg/products"},
	{http.MethodGet, "/plg/cs-users"},
	{http.MethodGet, "/plg/lifecycle"},
	{http.MethodPost, "/plg/organizations/search"},
	{http.MethodPost, "/plg/organizations/aggregate"},
	{http.MethodGet, "/plg/organizations/org-1"},
	{http.MethodPatch, "/plg/organizations/org-1"},
	{http.MethodGet, "/plg/organizations/org-1/products/choreo"},
	{http.MethodPatch, "/plg/organizations/org-1/products/choreo"},
	{http.MethodPost, "/plg/organizations/org-1/products/choreo/playbook-runs"},
	{http.MethodPost, "/plg/organizations/org-1/products/choreo/notes"},
	{http.MethodDelete, "/plg/playbook-runs/run-1"},
	{http.MethodPatch, "/plg/playbook-run-tasks/task-1"},
	{http.MethodPatch, "/plg/notes/note-1"},
	{http.MethodPost, "/plg/registrations/search"},
	{http.MethodPost, "/plg/registrations/volume-trend"},
	{http.MethodPost, "/plg/registrations/aggregate"},
	{http.MethodPost, "/plg/registrations/op-1/acknowledge"},
	{http.MethodGet, "/plg/playbooks"},
	{http.MethodGet, "/plg/playbooks/pb-1"},
	{http.MethodGet, "/plg/analytics/dashboard"},
	{http.MethodGet, "/plg/work-queue"},
}

// testAccessConfig mirrors the csm-portal handler package's own fixture: dummy
// role names, never the real ones, since those are organisation vocabulary that
// must not be committed.
func testAccessConfig() csmhandler.AccessConfig {
	return csmhandler.AccessConfig{
		Viewer:               []string{"test-viewer"},
		Escalator:            []string{"test-escalator"},
		AttachmentDownloader: []string{"test-attachment-downloader"},
		UsageMetricsViewer:   []string{"test-usage-metrics-viewer"},
		CsEngineer:           []string{"test-cs-engineer"},
		Admin:                []string{"test-admin"},
		TimecardApprover:     []string{"test-timecard-approver"},
		DashboardDesigner:    []string{"test-dashboard-designer"},
	}
}

// plgMux registers PLG's real route table against a stub identity middleware.
//
// The route callback is deliberately IDENTICAL to csm-portal's own route() in
// cmd/server/main.go — `mux.HandleFunc(pattern, guard.Require(perm, h))` — so
// what this exercises is the composition production uses, not a test-only
// arrangement that happens to agree with it.
//
// The stub identity answers 204 INSTEAD OF calling the handler, which is what
// makes this test possible without a running entity-service: a request that
// reaches it has passed the guard, and no PLG handler ever runs. It also pins
// the middleware ORDER — the guard is outside identity, so a caller whose roles
// are wrong is rejected before the (upstream-calling) identity resolver is
// entered. If those two were ever swapped, every denied case below would come
// back 204.
func plgMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	guard := csmhandler.NewAccessGuard(testAccessConfig())
	route := func(pattern string, perm csmhandler.Permission, h http.HandlerFunc) {
		mux.HandleFunc(pattern, guard.Require(perm, h))
	}
	stubIdentity := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}
	// Handlers is never invoked — the stub above stands in for it — so the zero
	// value is enough, and avoids building five services with no upstream.
	register(&plghandler.Handlers{}, stubIdentity, route)
	return mux
}

// statusAs issues one request as a caller whose token carries roles, and
// returns the status. The body is "{}" so a handler that reads one is not the
// thing that fails — though with the stub identity in plgMux, none ever runs.
func statusAs(t *testing.T, mux *http.ServeMux, method, path string, roles []string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	user := &middleware.UserInfo{Email: "staff@example.com", UserID: "sub-1", Roles: roles}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req.WithContext(middleware.WithUserInfo(req.Context(), user)))
	return w.Code
}

// The main split: a CS engineer reaches all 23 everyday routes and is refused
// on the four that author a playbook template.
func TestRoutes_CsEngineerWorksTheQueueButCannotAuthorPlaybooks(t *testing.T) {
	mux := plgMux(t)
	engineer := []string{"test-cs-engineer"}

	for _, r := range everydayRoutes {
		if got := statusAs(t, mux, r.method, r.path, engineer); got != http.StatusNoContent {
			t.Errorf("%s %s as cs engineer: status = %d, want 204", r.method, r.path, got)
		}
	}
	for _, r := range playbookManagementRoutes {
		if got := statusAs(t, mux, r.method, r.path, engineer); got != http.StatusForbidden {
			t.Errorf("%s %s as cs engineer: status = %d, want 403", r.method, r.path, got)
		}
	}
}

// Admin holds both permissions, so no PLG route is closed to it.
func TestRoutes_AdminReachesEverything(t *testing.T) {
	mux := plgMux(t)
	admin := []string{"test-admin"}

	for _, r := range append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...) {
		if got := statusAs(t, mux, r.method, r.path, admin); got != http.StatusNoContent {
			t.Errorf("%s %s as admin: status = %d, want 204", r.method, r.path, got)
		}
	}
}

// PLG is narrower than PermView. Every role below holds PermView and so can read
// cases and customers, but none of them may open PLG: a section where every
// control 403s is worse than one that is not offered.
func TestRoutes_ViewOnlyRolesAreShutOutEntirely(t *testing.T) {
	mux := plgMux(t)
	all := append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...)

	for _, role := range []string{
		"test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-timecard-approver", "test-dashboard-designer",
	} {
		for _, r := range all {
			if got := statusAs(t, mux, r.method, r.path, []string{role}); got != http.StatusForbidden {
				t.Errorf("%s %s as %s: status = %d, want 403", r.method, r.path, role, got)
			}
		}
	}
}

// A token carrying no roles at all reaches nothing — the guard denies before
// identity would have had a chance to resolve the caller.
func TestRoutes_NoRolesReachesNothing(t *testing.T) {
	mux := plgMux(t)
	all := append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...)

	for _, r := range all {
		if got := statusAs(t, mux, r.method, r.path, nil); got != http.StatusForbidden {
			t.Errorf("%s %s with no roles: status = %d, want 403", r.method, r.path, got)
		}
	}
}

// Reading a playbook is not managing one. Pinned separately because the obvious
// wrong implementation — gating on the /plg/playbooks path prefix — passes every
// other test in this file and breaks exactly this: an engineer who cannot list
// templates cannot choose one to run.
func TestRoutes_EngineerCanStillReadPlaybooks(t *testing.T) {
	mux := plgMux(t)
	engineer := []string{"test-cs-engineer"}

	for _, path := range []string{"/plg/playbooks", "/plg/playbooks/pb-1"} {
		if got := statusAs(t, mux, http.MethodGet, path, engineer); got != http.StatusNoContent {
			t.Errorf("GET %s as cs engineer: status = %d, want 204", path, got)
		}
	}
	// ...and assigning one to a pairing is the engineer's job, not admin's.
	if got := statusAs(t, mux, http.MethodPost, "/plg/organizations/org-1/products/choreo/playbook-runs", engineer); got != http.StatusNoContent {
		t.Errorf("attaching a playbook as cs engineer: status = %d, want 204", got)
	}
}

// registeredPattern matches one route registration in plg.go's own source.
var registeredPattern = regexp.MustCompile(`add\("([A-Z]+ /[^"]*)"`)

// TestRoutes_EveryRouteIsCovered pins this file's two lists against the route
// table itself, in both directions.
//
// THE PREVIOUS VERSION OF THIS TEST DID NOT WORK. It asserted that the two
// lists totalled a hand-written constant of 24 — but both sides lived here, so
// they could only disagree if someone edited a list and forgot the constant. A
// route added to plg.go was invisible to it, and the comment claiming otherwise
// was simply wrong. Verified: adding a route to register() and running the
// suite passed.
//
// This version reads the patterns register() actually mounts, and resolves each
// test entry through the real mux — `(*ServeMux).Handler` reports which pattern
// a request matched, which is the enumeration the old comment assumed did not
// exist. Two failures are therefore possible:
//
//   - a pattern in plg.go that no test entry reaches: a new route went in
//     without anyone deciding what its permission tests should say;
//   - a test entry matching no pattern at all: a typo, or a path that moved,
//     leaving an assertion that silently proved nothing.
func TestRoutes_EveryRouteIsCovered(t *testing.T) {
	src, err := os.ReadFile("plg.go")
	if err != nil {
		t.Fatalf("read plg.go: %v", err)
	}
	registered := map[string]bool{}
	for _, m := range registeredPattern.FindAllStringSubmatch(string(src), -1) {
		registered[m[1]] = true
	}
	if len(registered) == 0 {
		t.Fatal("found no routes in plg.go — the regexp is wrong, not the route table")
	}

	mux := plgMux(t)
	covered := map[string]bool{}
	for _, r := range append(append([]struct{ method, path string }{}, everydayRoutes...), playbookManagementRoutes...) {
		req := httptest.NewRequest(r.method, r.path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("%s %s in this file matches no route in plg.go — every assertion using it proves nothing",
				r.method, r.path)
			continue
		}
		covered[pattern] = true
	}

	missing := make([]string, 0, len(registered))
	for pattern := range registered {
		if !covered[pattern] {
			missing = append(missing, pattern)
		}
	}
	sort.Strings(missing)
	for _, pattern := range missing {
		t.Errorf("%s is registered in plg.go but no entry in this file reaches it — add one to everydayRoutes or playbookManagementRoutes", pattern)
	}
}
