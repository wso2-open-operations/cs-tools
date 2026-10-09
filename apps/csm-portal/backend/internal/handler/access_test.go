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
		CsEngineer:           []string{"test-cs-engineer"},
		Admin:                []string{"test-admin"},
		TimecardApprover:     []string{"test-timecard-approver"},
		DashboardDesigner:    []string{"test-dashboard-designer"},
		SalesSolutions:       []string{"test-sales-solutions"},
		WorknoteCreator:      []string{"test-worknote-creator"},
		AnnouncementCreator:  []string{"test-announcement-creator"},
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
	// csEngineerPerms is every route permission cs_engineer holds, escalation
	// included (any internal engineer may escalate, as in ServiceNow).
	// PermAdmin and PermApproveTimeCard are deliberately excluded and tested
	// separately below -- approving a time card is a dedicated responsibility
	// cs_engineer does not share, the same way PermAdmin isn't.
	csEngineerPerms := []Permission{PermView, PermViewOperations, PermTimeCardsAndUpdates, PermEscalate, PermDownloadAttachment, PermWrite, PermViewSecurityCenter, PermUsePlg}
	all := append(append([]Permission{}, csEngineerPerms...), PermAdmin, PermApproveTimeCard, PermManagePlaybooks, PermCreateAnnouncement)
	tests := []struct {
		name  string
		roles []string
		allow []Permission
	}{
		{"viewer reads only", []string{"test-viewer"}, []Permission{PermView}},
		{"escalator can view and escalate", []string{"test-escalator"}, []Permission{PermView, PermEscalate}},
		{"downloader can view and download", []string{"test-attachment-downloader"}, []Permission{PermView, PermDownloadAttachment}},
		{"CS engineer can do every route permission except admin-only and approve-time-card ones", []string{"test-cs-engineer"}, csEngineerPerms},
		{"admin can do every route permission, including admin-only ones", []string{"test-admin"}, all},
		{"announcement creator alone holds only PermCreateAnnouncement -- not even view", []string{"test-announcement-creator"}, []Permission{PermCreateAnnouncement}},
		{"CS engineer with announcement creator adds PermCreateAnnouncement and nothing else", []string{"test-cs-engineer", "test-announcement-creator"}, append(append([]Permission{}, csEngineerPerms...), PermCreateAnnouncement)},
		{"usage metrics viewer can view only", []string{"test-usage-metrics-viewer"}, []Permission{PermView}},
		{"timecard approver can view, use time cards and updates, and approve", []string{"test-timecard-approver"}, []Permission{PermView, PermTimeCardsAndUpdates, PermApproveTimeCard}},
		{"dashboard designer can view only", []string{"test-dashboard-designer"}, []Permission{PermView}},
		{"sales solutions role alone grants none of these -- it holds PermViewSharedEntity/PermViewerAccess instead, tested separately", []string{"test-sales-solutions"}, nil},
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

// TestAccessGuard_PermViewSharedEntityScope guards the fix for the finding
// that granting sales_solutions PermView directly widened it to every
// PermView-gated route in the backend (users, deployments, tasks, SLAs,
// dashboards, ...), not just the accounts/projects/cases/team-members routes
// SPL's screens actually call. PermViewSharedEntity is the narrower
// permission those specific routes are registered with instead.
func TestAccessGuard_PermViewSharedEntityScope(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())

	t.Run("every PermView holder also holds PermViewSharedEntity", func(t *testing.T) {
		for _, roles := range [][]string{
			{"test-viewer"}, {"test-escalator"}, {"test-attachment-downloader"}, {"test-cs-engineer"},
			{"test-admin"}, {"test-usage-metrics-viewer"}, {"test-timecard-approver"}, {"test-dashboard-designer"},
		} {
			if status, _ := serveWithRoles(g, PermViewSharedEntity, roles); status != http.StatusNoContent {
				t.Errorf("roles %v: status = %d, want 204 (PermView implies PermViewSharedEntity)", roles, status)
			}
		}
	})

	t.Run("sales_solutions holds PermViewSharedEntity but not plain PermView or PermViewerAccess", func(t *testing.T) {
		roles := []string{"test-sales-solutions"}
		if status, _ := serveWithRoles(g, PermViewSharedEntity, roles); status != http.StatusNoContent {
			t.Errorf("PermViewSharedEntity: status = %d, want 204", status)
		}
		if status, _ := serveWithRoles(g, PermView, roles); status != http.StatusForbidden {
			t.Errorf("PermView: status = %d, want 403 -- sales_solutions must not gain every PermView route", status)
		}
		// PermViewerAccess is Viewer-gated, not sales_solutions -- see
		// PermViewerAccess's own doc comment.
		if status, _ := serveWithRoles(g, PermViewerAccess, roles); status != http.StatusForbidden {
			t.Errorf("PermViewerAccess: status = %d, want 403 (Viewer-gated, not sales_solutions)", status)
		}
	})

	// Guards PermViewerAccess's grant (Viewer, unconditionally) -- see
	// PermViewerAccess's own doc comment for why.
	t.Run("plain viewer holds PermViewerAccess", func(t *testing.T) {
		if status, _ := serveWithRoles(g, PermViewerAccess, []string{"test-viewer"}); status != http.StatusNoContent {
			t.Errorf("PermViewerAccess: status = %d, want 204", status)
		}
	})

	// PermViewerAccess is the audience check, not the nav-default choice --
	// a caller holding both viewer and cs_engineer still passes it, even
	// though usePortalView.ts's cs_engineer-first precedence means they'd
	// default to the CS/ABT nav in the webapp. See PermViewerAccess's own doc
	// comment for why the backend deliberately doesn't exclude cs_engineer
	// here.
	t.Run("viewer alongside cs_engineer still holds PermViewerAccess", func(t *testing.T) {
		if status, _ := serveWithRoles(g, PermViewerAccess, []string{"test-viewer", "test-cs-engineer"}); status != http.StatusNoContent {
			t.Errorf("PermViewerAccess: status = %d, want 204", status)
		}
	})
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
			"test-dashboard-designer", "test-admin", "test-sales-solutions", "test-worknote-creator", "test-announcement-creator",
		}, []string{"viewer", "escalator", "attachment_downloader", "cs_engineer", "usage_metrics_viewer", "timecard_approver", "dashboard_designer", "admin", "sales_solutions", "worknote_creator", "announcement_creator"}},
		{"announcement creator is reported like any other portal role", []string{"test-cs-engineer", "test-announcement-creator"}, []string{"cs_engineer", "announcement_creator"}},
		{"unrelated roles are ignored", []string{"wso2-everyone", "agent"}, []string{}},
		{"a duplicated held role is reported once", []string{"test-viewer", "test-viewer"}, []string{"viewer"}},
		{"sales solutions is reported like any other portal role, alongside a real capability", []string{"test-viewer", "test-sales-solutions"}, []string{"viewer", "sales_solutions"}},
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
	for _, perm := range []Permission{PermView, PermViewOperations, PermTimeCardsAndUpdates, PermEscalate, PermDownloadAttachment, PermWrite, PermAdmin, PermViewSecurityCenter, PermApproveTimeCard, PermUsePlg, PermManagePlaybooks, PermCreateWorkNote, PermCreateAnnouncement} {
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

func TestAccessGuard_ApproveTimeCardIsForApproversAndAdminsOnly(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{"test-admin", "test-timecard-approver"} {
		if status, _ := serveWithRoles(g, PermApproveTimeCard, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", role, status)
		}
	}
	// CS engineer holds the broader PermTimeCardsAndUpdates but must NOT hold
	// this narrower one -- approving is a dedicated responsibility.
	for _, role := range []string{
		"test-cs-engineer", "test-viewer", "test-escalator",
		"test-attachment-downloader", "test-usage-metrics-viewer", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermApproveTimeCard, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", role, status)
		}
	}
}

// TestAccessGuard_PlgIsForCsEngineersAndAdmins pins PermUsePlg as narrower than
// PermView: every portal role holds PermView, but PLG is a worklist staff act
// on, and a view-only role that could open it would meet a 403 on every control.
func TestAccessGuard_PlgIsForCsEngineersAndAdmins(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{"test-cs-engineer", "test-admin"} {
		if status, _ := serveWithRoles(g, PermUsePlg, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", role, status)
		}
	}
	for _, role := range []string{
		"test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-timecard-approver", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermUsePlg, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s holds PermView but must not hold PermUsePlg: status = %d, want 403", role, status)
		}
	}
}

// TestAccessGuard_ManagePlaybooksIsAdminOnly pins the one split inside PLG: a CS
// engineer works the queue and runs playbooks, but authoring a template is
// admin's. It is separate from PermAdmin on purpose — see the constant's own
// doc comment — so this asserts the CS engineer is denied rather than asserting
// the two permissions are interchangeable.
func TestAccessGuard_ManagePlaybooksIsAdminOnly(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	if status, _ := serveWithRoles(g, PermManagePlaybooks, []string{"test-admin"}); status != http.StatusNoContent {
		t.Errorf("admin: status = %d, want 204", status)
	}
	for _, role := range []string{
		"test-cs-engineer", "test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-timecard-approver", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermManagePlaybooks, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s must not manage playbooks: status = %d, want 403", role, status)
		}
	}
	// The CS engineer keeps everything else in PLG, including running a playbook.
	if status, _ := serveWithRoles(g, PermUsePlg, []string{"test-cs-engineer"}); status != http.StatusNoContent {
		t.Errorf("cs engineer lost PermUsePlg: status = %d, want 204", status)
	}
}

// TestAccessGuard_CreateWorkNoteIsForWorknoteCreatorsCsEngineersAndAdmins pins
// PermCreateWorkNote's deliberately wider holder set than PermWrite's (see
// the constant's own doc comment) -- it's the route-level floor for POST
// /cases/{id}/comments, with CaseHandler itself narrowing back to full
// PermWrite for anything that isn't a work_note. Viewer is read-only and does
// NOT hold it.
func TestAccessGuard_CreateWorkNoteIsForWorknoteCreatorsCsEngineersAndAdmins(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	for _, role := range []string{"test-worknote-creator", "test-cs-engineer", "test-admin"} {
		if status, _ := serveWithRoles(g, PermCreateWorkNote, []string{role}); status != http.StatusNoContent {
			t.Errorf("%s: status = %d, want 204", role, status)
		}
	}
	for _, role := range []string{
		"test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-timecard-approver", "test-dashboard-designer",
	} {
		if status, _ := serveWithRoles(g, PermCreateWorkNote, []string{role}); status != http.StatusForbidden {
			t.Errorf("%s must not hold PermCreateWorkNote: status = %d, want 403", role, status)
		}
	}
	// worknote_creator holds ONLY this -- not the broader PermWrite a
	// customer-visible reply (or any other write) needs.
	if status, _ := serveWithRoles(g, PermWrite, []string{"test-worknote-creator"}); status != http.StatusForbidden {
		t.Errorf("worknote_creator must not hold PermWrite: status = %d, want 403", status)
	}
}

// TestAccessGuard_ViewerWithWorknoteCreatorRoleSet pins the role set a viewer
// holds in practice (read-only viewer plus a few specialised read/act roles,
// with worknote_creator the only one that adds a comment): the read-only
// roles alone cannot add a work note, adding worknote_creator can, and even
// then nothing beyond a work note is writable.
func TestAccessGuard_ViewerWithWorknoteCreatorRoleSet(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	readOnlyish := []string{
		"test-viewer", "test-escalator", "test-attachment-downloader",
		"test-usage-metrics-viewer", "test-timecard-approver",
	}
	if status, _ := serveWithRoles(g, PermCreateWorkNote, readOnlyish); status != http.StatusForbidden {
		t.Errorf("without worknote_creator: PermCreateWorkNote status = %d, want 403", status)
	}
	withNotes := append(append([]string{}, readOnlyish...), "test-worknote-creator")
	if status, _ := serveWithRoles(g, PermCreateWorkNote, withNotes); status != http.StatusNoContent {
		t.Errorf("with worknote_creator: PermCreateWorkNote status = %d, want 204", status)
	}
	if status, _ := serveWithRoles(g, PermWrite, withNotes); status != http.StatusForbidden {
		t.Errorf("with worknote_creator: PermWrite status = %d, want 403 (a work note is not a write)", status)
	}
}

// serveAllWithRoles is serveWithRoles for a route registered through RequireAll.
func serveAllWithRoles(g *AccessGuard, roles []string, perms ...Permission) (status int, reached bool) {
	h := g.RequireAll(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}, perms...)
	user := &middleware.UserInfo{Email: "staff@example.com", UserID: "user-1", Roles: roles}
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	w := httptest.NewRecorder()
	h(w, req.WithContext(middleware.WithUserInfo(req.Context(), user)))
	return w.Code, reached
}

// TestAccessGuard_CreateAnnouncementNarrowsWrite pins what announcement_creator
// is for: sending customers an announcement needs it on top of PermWrite, so a
// plain CS engineer loses that one ability and keeps every other write, while a
// holder of the role who cannot write at all still cannot send anything.
func TestAccessGuard_CreateAnnouncementNarrowsWrite(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	both := []Permission{PermWrite, PermCreateAnnouncement}

	tests := []struct {
		name  string
		roles []string
		want  int
	}{
		{"cs_engineer without the role is refused", []string{"test-cs-engineer"}, http.StatusForbidden},
		{"cs_engineer with the role is allowed", []string{"test-cs-engineer", "test-announcement-creator"}, http.StatusNoContent},
		{"admin is allowed without the role, like every other permission", []string{"test-admin"}, http.StatusNoContent},
		{"the role alone is refused: it makes nobody a writer", []string{"test-announcement-creator"}, http.StatusForbidden},
		{"a viewer with the role is refused", []string{"test-viewer", "test-announcement-creator"}, http.StatusForbidden},
		{"worknote_creator and escalator do not help", []string{"test-worknote-creator", "test-escalator"}, http.StatusForbidden},
		{"no roles", nil, http.StatusForbidden},
		{"role names are case sensitive", []string{"test-cs-engineer", "TEST-ANNOUNCEMENT-CREATOR"}, http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, reached := serveAllWithRoles(g, tc.roles, both...)
			if status != tc.want || reached != (tc.want == http.StatusNoContent) {
				t.Errorf("status = %d reached = %v, want status %d", status, reached, tc.want)
			}
		})
	}

	t.Run("cs_engineer without the role keeps every other write", func(t *testing.T) {
		if status, _ := serveWithRoles(g, PermWrite, []string{"test-cs-engineer"}); status != http.StatusNoContent {
			t.Errorf("PermWrite status = %d, want 204", status)
		}
	})
}

func TestAccessGuard_RequireAll(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())

	t.Run("no permissions listed denies everyone", func(t *testing.T) {
		if status, reached := serveAllWithRoles(g, []string{"test-admin"}); status != http.StatusForbidden || reached {
			t.Errorf("status = %d reached = %v, want 403 and not reached", status, reached)
		}
	})
	t.Run("every listed permission must be held, not any one", func(t *testing.T) {
		if status, _ := serveAllWithRoles(g, []string{"test-escalator"}, PermView, PermEscalate); status != http.StatusNoContent {
			t.Errorf("holds both: status = %d, want 204", status)
		}
		if status, _ := serveAllWithRoles(g, []string{"test-viewer"}, PermView, PermEscalate); status != http.StatusForbidden {
			t.Errorf("holds only one: status = %d, want 403", status)
		}
	})
	t.Run("a missing token is 401, not 403", func(t *testing.T) {
		h := g.RequireAll(func(http.ResponseWriter, *http.Request) {}, PermWrite)
		w := httptest.NewRecorder()
		h(w, httptest.NewRequest(http.MethodPost, "/x", nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	})
}
