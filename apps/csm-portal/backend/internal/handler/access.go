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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// Permission is what a route requires of its caller.
type Permission int

const (
	// PermAuthenticated needs a valid token and nothing else. Only the caller's
	// own identity routes use it, so a user holding no portal role can still load
	// their profile and be shown a "no access" screen.
	PermAuthenticated Permission = iota
	// PermView is every read: get, list, search, aggregate.
	PermView
	// PermViewOperations is reading the Operations area: incidents, change
	// requests, problems, incident tasks, outages and their alerts. Narrower than
	// PermView on purpose: a view-only role sees cases and customers but not
	// operations, which support-portal-lite never exposed to them.
	PermViewOperations
	// PermTimeCardsAndUpdates is the Time Cards and Updates areas: every time-card
	// route (search, create, update, delete) and the update-level lookups. Held by
	// the CS engineer and admin, and by the time-card approver so approving does
	// not require being a CS engineer. Narrower than PermView on purpose: a
	// view-only role sees neither area.
	PermTimeCardsAndUpdates
	// PermEscalate is escalating or de-escalating a case.
	PermEscalate
	// PermDownloadAttachment is downloading attachment content, or minting a
	// link that does.
	PermDownloadAttachment
	// PermWrite is every other state-changing route.
	PermWrite
	// PermViewAllDashboards is seeing a dashboard marked dashboard.Dashboard.Restricted
	// (the team/advanced dashboards) rather than only the unrestricted ones every
	// portal role can see. Checked inside DashboardHandler itself, per dashboard —
	// unlike every other permission here, no route is registered with it directly,
	// since GET /dashboards must still run for every viewer and just filter its
	// result rather than reject the whole request.
	PermViewAllDashboards
	// PermAdmin is held by the admin role only — unlike PermWrite, which
	// cs_engineer also holds. Reserved for actions no non-admin staff
	// role should ever reach, such as creating a new platform user.
	PermAdmin
	// PermViewSecurityCenter is the Security Center area: security-report
	// cases (POST /cases/search and GET /cases/{id}, type-checked inside
	// CaseHandler itself — see its own doc comment for why a route-level
	// permission alone can't express this) and both /products/vulnerabilities
	// routes. Admin and cs_engineer only — every other role, including plain
	// viewer/escalator/attachment_downloader, is denied even though they hold
	// PermView, since this is deliberately narrower than the general case/
	// product-data access PermView otherwise grants.
	PermViewSecurityCenter
)

// AccessConfig names, per portal role, the role names on the token that grant
// it. Each field is a list because one portal role can be granted by several
// token roles; holding any one of them is enough. There are deliberately no
// defaults: the names are organisation vocabulary supplied by configuration,
// and a role with no names configured is held by nobody.
type AccessConfig struct {
	Viewer               []string
	Escalator            []string
	AttachmentDownloader []string
	UsageMetricsViewer   []string
	// CsEngineer is read from AUTH_SUPPORT_ENGINEER_ROLES -- the portal role
	// was renamed from support_engineer to cs_engineer, but the env var name
	// was deliberately left as-is to avoid a coordinated deployment config
	// change alongside this rename.
	CsEngineer        []string
	Admin             []string
	TimecardApprover  []string
	DashboardDesigner []string
}

// AccessGuard authorises a request from the roles on the caller's validated
// token. It makes no upstream call: Auth has already decoded the token, so a
// check is a set lookup.
type AccessGuard struct {
	// allowed maps each role-gated permission to every token role that satisfies it.
	allowed map[Permission]map[string]struct{}
	// portalRoles is each portal role and the token roles that grant it, in the
	// fixed order GET /users/me reports them.
	portalRoles []portalRole
}

// portalRole is one portal role: its stable key (what the frontend sees) and
// the configured token role names that grant it.
type portalRole struct {
	key   string
	names map[string]struct{}
}

// NewAccessGuard builds a guard from cfg. Admin satisfies every permission.
// CS engineer (renamed from support_engineer -- see AccessConfig.CsEngineer's
// own doc comment), the role for people who work cases, satisfies every one
// too, EXCEPT PermAdmin — that one is admin-only, held by no other role,
// unlike PermWrite which both share. The escalator and attachment-downloader
// roles exist separately so other staff can be granted just that one ability.
// The time-card approver also holds PermTimeCardsAndUpdates, so it can
// approve without being a CS engineer. The usage-metrics and
// dashboard-designer roles gate nothing here (this backend has no route for
// those features) and grant only View. Every role implies View, so a user
// granted only one specialised role can still open the pages it acts on.
// PermViewSecurityCenter is the one exception to "every role implies View
// covers it": plain viewer/escalator/attachment_downloader/usage_metrics_viewer/
// timecard_approver/dashboard_designer all hold PermView but not this.
func NewAccessGuard(cfg AccessConfig) *AccessGuard {
	build := func(lists ...[]string) map[string]struct{} {
		set := make(map[string]struct{})
		for _, list := range lists {
			for _, role := range list {
				set[role] = struct{}{}
			}
		}
		return set
	}
	return &AccessGuard{
		portalRoles: []portalRole{
			{"viewer", build(cfg.Viewer)},
			{"escalator", build(cfg.Escalator)},
			{"attachment_downloader", build(cfg.AttachmentDownloader)},
			{"cs_engineer", build(cfg.CsEngineer)},
			{"usage_metrics_viewer", build(cfg.UsageMetricsViewer)},
			{"timecard_approver", build(cfg.TimecardApprover)},
			{"dashboard_designer", build(cfg.DashboardDesigner)},
			{"admin", build(cfg.Admin)},
		},
		allowed: map[Permission]map[string]struct{}{
			PermView: build(cfg.Viewer, cfg.Escalator, cfg.AttachmentDownloader,
				cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover, cfg.DashboardDesigner),
			PermViewOperations:      build(cfg.CsEngineer, cfg.Admin),
			PermTimeCardsAndUpdates: build(cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover),
			PermEscalate:            build(cfg.Escalator, cfg.CsEngineer, cfg.Admin),
			PermDownloadAttachment:  build(cfg.AttachmentDownloader, cfg.CsEngineer, cfg.Admin),
			PermWrite:               build(cfg.CsEngineer, cfg.Admin),
			PermViewAllDashboards:   build(cfg.CsEngineer, cfg.Admin),
			PermAdmin:               build(cfg.Admin),
			PermViewSecurityCenter:  build(cfg.CsEngineer, cfg.Admin),
		},
	}
}

// RolesFor returns the key of every portal role the given token roles hold, in
// a fixed order. A caller can hold several. It is never nil, so it serialises as
// [] rather than null for a caller holding no portal role.
func (g *AccessGuard) RolesFor(tokenRoles []string) []string {
	keys := make([]string, 0, len(g.portalRoles))
	for _, r := range g.portalRoles {
		for _, held := range tokenRoles {
			if _, ok := r.names[held]; ok {
				keys = append(keys, r.key)
				break
			}
		}
	}
	return keys
}

// Require wraps next so it only runs for a caller whose token roles satisfy
// perm. Every route must be registered through it: there is deliberately no
// default permission, so a new route cannot go live without someone choosing
// one.
func (g *AccessGuard) Require(perm Permission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserInfoFromContext(r.Context())
		if user == nil {
			writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
			return
		}
		if perm == PermAuthenticated {
			next(w, r)
			return
		}
		if !g.Permits(perm, user.Roles) {
			slog.WarnContext(r.Context(), "access denied: token carries no role granting this permission", "userID", user.UserID, "method", r.Method, "path", r.URL.Path)
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
		next(w, r)
	}
}

// Permits reports whether any of roles satisfies perm. An unknown permission
// has no allowed set and so is denied. Exported so a handler that must gate a
// specific resource against a caller's roles inside its own logic — rather
// than a whole route via Require — can reuse the same policy (see
// DashboardHandler and PermViewAllDashboards).
func (g *AccessGuard) Permits(perm Permission, roles []string) bool {
	allowed := g.allowed[perm]
	for _, role := range roles {
		if _, ok := allowed[role]; ok {
			return true
		}
	}
	return false
}
