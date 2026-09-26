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
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// testAccessConfig names each portal role with a dummy token role, so the tests
// never depend on real role names.
func testAccessConfig() AccessConfig {
	return AccessConfig{
		Viewer:               []string{"test-viewer"},
		Escalator:            []string{"test-escalator"},
		AttachmentDownloader: []string{"test-attachment-downloader"},
		UsageMetricsViewer:   []string{"test-usage-metrics-viewer"},
		CsEngineer:      []string{"test-cs-engineer"},
		Admin:                []string{"test-admin"},
		TimecardApprover:     []string{"test-timecard-approver"},
		DashboardDesigner:    []string{"test-dashboard-designer"},
	}
}

// serveWithRoles runs one request through a guard-wrapped handler as a caller
// whose token carries roles, and reports the status plus whether the wrapped
// handler ran.
func serveWithRoles(g *AccessGuard, perm Permission, roles []string) (status int, reached bool) {
	h := g.Require(perm, func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})
	user := &middleware.UserInfo{Email: "staff@example.com", UserID: "user-1", Roles: roles}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h(w, req.WithContext(middleware.WithUserInfo(req.Context(), user)))
	return w.Code, reached
}

func TestAccessGuard_PermissionMatrix(t *testing.T) {
	// allExceptAdmin is every route permission cs_engineer also holds;
	// PermAdmin is deliberately excluded from it and tested separately below
	// -- it is the one permission admin does not share with cs_engineer.
	allExceptAdmin := []Permission{PermView, PermViewOperations, PermTimeCardsAndUpdates, PermEscalate, PermDownloadAttachment, PermWrite, PermViewSecurityCenter}
	all := append(append([]Permission{}, allExceptAdmin...), PermAdmin)
	tests := []struct {
		name  string
		roles []string
		allow []Permission
	}{
		{"viewer reads only", []string{"test-viewer"}, []Permission{PermView}},
		{"escalator can view and escalate", []string{"test-escalator"}, []Permission{PermView, PermEscalate}},
		{"downloader can view and download", []string{"test-attachment-downloader"}, []Permission{PermView, PermDownloadAttachment}},
		{"CS engineer can do every route permission except admin-only ones", []string{"test-cs-engineer"}, allExceptAdmin},
		{"admin can do every route permission, including admin-only ones", []string{"test-admin"}, all},
		{"usage metrics viewer can view only", []string{"test-usage-metrics-viewer"}, []Permission{PermView}},
		{"timecard approver can view and use time cards and updates", []string{"test-timecard-approver"}, []Permission{PermView, PermTimeCardsAndUpdates}},
		{"dashboard designer can view only", []string{"test-dashboard-designer"}, []Permission{PermView}},
		{"roles combine", []string{"test-viewer", "test-escalator", "test-attachment-downloader"}, []Permission{PermView, PermEscalate, PermDownloadAttachment}},
		{"unrelated roles grant nothing", []string{"wso2-everyone", "admin", "agent", "customer"}, nil},
		{"no roles", nil, nil},
		{"role names are case sensitive", []string{"TEST-ADMIN"}, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewAccessGuard(testAccessConfig())
			for _, perm := range all {
				status, reached := serveWithRoles(g, perm, tc.roles)
				want := slices.Contains(tc.allow, perm)
				wantStatus := http.StatusForbidden
				if want {
					wantStatus = http.StatusNoContent
				}
				if status != wantStatus || reached != want {
					t.Errorf("permission %d: status = %d reached = %v, want status %d reached %v", perm, status, reached, wantStatus, want)
				}
			}
		})
	}
}

func TestAccessGuard_UsesConfiguredRoleNames(t *testing.T) {
	cfg := testAccessConfig()
	cfg.Escalator = []string{"corp-support-notes", "corp-interns"}
	g := NewAccessGuard(cfg)

	for _, role := range []string{"corp-support-notes", "corp-interns"} {
		if status, _ := serveWithRoles(g, PermEscalate, []string{role}); status != http.StatusNoContent {
			t.Errorf("configured role %q: status = %d, want 204", role, status)
		}
	}
	if status, _ := serveWithRoles(g, PermEscalate, []string{"test-escalator"}); status != http.StatusForbidden {
		t.Errorf("default name after override: status = %d, want 403 (the configured names replace the previous ones)", status)
	}
}

func TestAccessGuard_AuthenticatedNeedsNoRole(t *testing.T) {
	status, reached := serveWithRoles(NewAccessGuard(testAccessConfig()), PermAuthenticated, nil)
	if status != http.StatusNoContent || !reached {
		t.Errorf("status = %d reached = %v, want 204 and reached", status, reached)
	}
}

func TestAccessGuard_NoUserIs401(t *testing.T) {
	reached := false
	h := NewAccessGuard(testAccessConfig()).Require(PermAuthenticated, func(http.ResponseWriter, *http.Request) { reached = true })
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assertStatus(t, w, http.StatusUnauthorized)
	if reached {
		t.Error("handler ran for a request with no authenticated user")
	}
}

func TestAccessGuard_UnknownPermissionIsDenied(t *testing.T) {
	status, reached := serveWithRoles(NewAccessGuard(testAccessConfig()), Permission(999), []string{"test-admin"})
	if status != http.StatusForbidden || reached {
		t.Errorf("status = %d reached = %v, want 403 and not reached", status, reached)
	}
}

func TestAccessGuard_RolesFor(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	tests := []struct {
		name string
		held []string
		want []string
	}{
		{"none", nil, []string{}},
		{"one role", []string{"test-viewer"}, []string{"viewer"}},
		{"several roles come back in a fixed order", []string{"test-admin", "test-escalator", "test-viewer"}, []string{"viewer", "escalator", "admin"}},
		{"CS engineer", []string{"test-cs-engineer"}, []string{"cs_engineer"}},
		{"every role", []string{
			"test-viewer", "test-escalator", "test-attachment-downloader",
			"test-cs-engineer", "test-usage-metrics-viewer", "test-timecard-approver",
			"test-dashboard-designer", "test-admin",
		}, []string{"viewer", "escalator", "attachment_downloader", "cs_engineer", "usage_metrics_viewer", "timecard_approver", "dashboard_designer", "admin"}},
		{"unrelated roles are ignored", []string{"wso2-everyone", "agent"}, []string{}},
		{"a duplicated held role is reported once", []string{"test-viewer", "test-viewer"}, []string{"viewer"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := g.RolesFor(tc.held)
			if got == nil || !slices.Equal(got, tc.want) {
				t.Errorf("RolesFor = %#v, want %#v (non-nil)", got, tc.want)
			}
		})
	}
}

func TestAccessGuard_RolesForUsesConfiguredNames(t *testing.T) {
	cfg := testAccessConfig()
	cfg.CsEngineer = []string{"corp-se-a", "corp-se-b"}
	g := NewAccessGuard(cfg)
	if got := g.RolesFor([]string{"corp-se-b"}); !slices.Equal(got, []string{"cs_engineer"}) {
		t.Errorf("RolesFor = %v, want [cs_engineer]", got)
	}
	if got := g.RolesFor([]string{"test-cs-engineer"}); len(got) != 0 {
		t.Errorf("RolesFor = %v, want none: the configured names replace the default", got)
	}
}

func TestAccessGuard_OperationsAreForCsEngineersAndAdmins(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{
		"test-viewer", "test-escalator",
		"test-attachment-downloader", "test-usage-metrics-viewer",
		"test-timecard-approver", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermViewOperations, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s reading operations: status = %d, want 403", role, status)
		}
		if status, _ := serveWithRoles(g, PermView, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading cases and customers: status = %d, want 204", role, status)
		}
	}
	for _, role := range []string{"test-cs-engineer", "test-admin"} {
		if status, _ := serveWithRoles(g, PermViewOperations, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading operations: status = %d, want 204", role, status)
		}
	}
}

// TestAccessGuard_ViewAllDashboardsIsForCsEngineersAndAdmins covers
// PermViewAllDashboards directly through Permits rather than serveWithRoles:
// unlike every other permission here, no route is ever registered with it —
// DashboardHandler checks it itself, per dashboard, alongside the caller's
// unconditional PermView access to the (unrestricted) dashboard list.
func TestAccessGuard_ViewAllDashboardsIsForCsEngineersAndAdmins(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{
		"test-viewer", "test-escalator",
		"test-attachment-downloader", "test-usage-metrics-viewer",
		"test-timecard-approver", "test-dashboard-designer",
	} {
		if g.Permits(PermViewAllDashboards, []string{role}) {
			t.Errorf("%s: Permits(PermViewAllDashboards) = true, want false", role)
		}
	}
	for _, role := range []string{"test-cs-engineer", "test-admin"} {
		if !g.Permits(PermViewAllDashboards, []string{role}) {
			t.Errorf("%s: Permits(PermViewAllDashboards) = false, want true", role)
		}
	}
	if g.Permits(PermViewAllDashboards, nil) {
		t.Error("no roles: Permits(PermViewAllDashboards) = true, want false")
	}
}

// TestAccessGuard_SecurityCenterIsForCsEngineersAndAdmins covers
// PermViewSecurityCenter, which — unlike PermView — plain viewer/escalator/
// attachment_downloader/usage_metrics_viewer/timecard_approver/
// dashboard_designer do NOT hold, even though every one of them holds
// PermView itself.
func TestAccessGuard_SecurityCenterIsForCsEngineersAndAdmins(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{
		"test-viewer", "test-escalator",
		"test-attachment-downloader", "test-usage-metrics-viewer",
		"test-timecard-approver", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermViewSecurityCenter, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s reading Security Center: status = %d, want 403", role, status)
		}
		if status, _ := serveWithRoles(g, PermView, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading cases and customers: status = %d, want 204", role, status)
		}
	}
	for _, role := range []string{"test-cs-engineer", "test-admin"} {
		if status, _ := serveWithRoles(g, PermViewSecurityCenter, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading Security Center: status = %d, want 204", role, status)
		}
	}
}

func TestAccessGuard_UnconfiguredRolesAreHeldByNobody(t *testing.T) {
	g := NewAccessGuard(AccessConfig{})
	for _, perm := range []Permission{PermView, PermViewOperations, PermTimeCardsAndUpdates, PermEscalate, PermDownloadAttachment, PermWrite, PermAdmin, PermViewSecurityCenter} {
		if status, _ := serveWithRoles(g, perm, []string{"test-admin", "test-viewer", ""}); status != http.StatusForbidden {
			t.Errorf("permission %d with no roles configured: status = %d, want 403", perm, status)
		}
	}
	if got := g.RolesFor([]string{"test-admin", ""}); len(got) != 0 {
		t.Errorf("RolesFor = %v, want none when no roles are configured", got)
	}
}

func TestAccessGuard_TimeCardsAndUpdatesAreForCsEngineersAdminsAndApprovers(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{"test-cs-engineer", "test-admin", "test-timecard-approver"} {
		if status, _ := serveWithRoles(g, PermTimeCardsAndUpdates, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", role, status)
		}
	}
	for _, role := range []string{
		"test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermTimeCardsAndUpdates, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", role, status)
		}
		if status, _ := serveWithRoles(g, PermView, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s reading cases and customers: status = %d, want 204", role, status)
		}
	}
	if status, _ := serveWithRoles(g, PermTimeCardsAndUpdates, nil); status != http.StatusForbidden {
		t.Errorf("no roles: status = %d, want 403", status)
	}
}
