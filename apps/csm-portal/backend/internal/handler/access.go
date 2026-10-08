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
	// the CS engineer and admin, and by the time-card approver so viewing/managing
	// time cards does not require being a CS engineer. This is deliberately
	// broader than approving one — see PermApproveTimeCard below, which is what's
	// actually narrowed to the approver role.
	PermTimeCardsAndUpdates
	// PermEscalate is escalating or de-escalating a case. Held ONLY by the
	// escalator role and admin — NOT the CS engineer, unlike most other
	// permissions here. Escalation is a dedicated responsibility, not something
	// being a CS engineer alone should grant.
	PermEscalate
	// PermApproveTimeCard is approving or rejecting a time card — a state
	// transition on the same PATCH /time-cards/{id} route ordinary field edits
	// use (see UpdateTimeCardRequest.State in entity-service's own domain
	// types), so a route-level permission alone can't express this; TimeCardHandler
	// inspects the request body itself (mirroring CaseHandler's identical
	// approach for PermViewSecurityCenter — see that permission's own doc
	// comment) and additionally requires this permission only when `state` is
	// present. Held ONLY by the time-card approver role and admin — NOT the CS
	// engineer, same narrowing as PermEscalate above.
	PermApproveTimeCard
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
	// cases (POST /cases/search and POST /cases/aggregate, type-checked inside
	// CaseHandler itself — see its own doc comment for why a route-level
	// permission alone can't express this) and both /products/vulnerabilities
	// routes. Admin and cs_engineer only — every other role, including plain
	// viewer/escalator/attachment_downloader, is denied even though they hold
	// PermView, since this is deliberately narrower than the general case/
	// product-data access PermView otherwise grants.
	PermViewSecurityCenter
	// PermViewerAccess is the blanket audience gate for every SupportPortalLite
	// (Sales/Solutions-Architecture) route — replacing the old
	// SPL_ALLOWED_GROUPS raw-Asgardeo-groups check (internal/splauth,
	// removed).
	//
	// Granted to plain Viewer, unconditionally -- including callers who
	// also hold CsEngineer. That's deliberate: this permission answers
	// "can this caller reach SPL's API at all," which is a broader
	// question than "which portal's nav should a caller land in by
	// default." The latter is a webapp-only routing choice
	// (usePortalView.ts), where CsEngineer takes precedence over Viewer so
	// CS/ABT staff default to the CSM Portal nav even once they also carry
	// Viewer (the baseline read role most staff role sets compose in).
	// PermViewerAccess itself stays a plain Viewer-implies-access check with
	// no CsEngineer exclusion, so a CS engineer who navigates to an SPL
	// URL directly isn't hard-blocked by the backend -- only steered away
	// from it by default in the webapp's own nav. See usePortalView.ts and
	// useAccess.ts for the matching frontend halves of this split;
	// keep all three in sync on which role each one checks.
	PermViewerAccess
	// PermUsageMetricsViewer is the SPL Usage Metrics domain
	// (/usage-metrics/*), layered on top of PermViewerAccess the same way
	// PermEscalate/PermDownloadAttachment layer on top of PermView —
	// replacing the old SPL_USAGE_METRICS_GROUPS sub-group check. Unlike
	// AccessConfig.UsageMetricsViewer's original CS-Portal-side grant (View
	// only, since this backend had no usage-metrics route of its own before
	// SPL), this is the real permission those SPL routes now check.
	PermUsageMetricsViewer
	// PermViewSharedEntity is read access to exactly the routes SupportPortalLite's
	// merged accounts/projects/cases/team-members screens call: GET /accounts/{id},
	// POST /accounts/search, GET /projects/{id}, POST /projects/search,
	// POST /projects/{id}/contacts/search, POST /cases/search, GET /cases/{id},
	// POST /cases/{id}/comments/search, and GET /teams/{id}/members — see
	// main.go's own route registrations for the exact list. Deliberately its
	// own permission rather than PermView itself: PermView is every read
	// across the whole backend (users, deployments, tasks, SLAs, dashboards,
	// schedules, announcements, ...), and sales_solutions must not gain all of
	// that just because SPL's screens need this one narrow slice of it. Every
	// existing PermView holder also holds this (nothing they could already
	// read stops being readable); it exists only to grant sales_solutions
	// this slice without the rest.
	PermViewSharedEntity
	// PermUsePlg is the PLG Customer Success Portal: every one of its routes
	// except playbook management. CS engineer and admin only, which is
	// deliberately narrower than PermView — PLG is a worklist staff act on, not
	// a record the portal's view-only roles have any use for, and a viewer who
	// could open it would see a section where every control returns 403.
	//
	// There is no view/write split within it on purpose. PLG's queue is a shared
	// worklist: an engineer who can see a pairing is expected to act on it, and a
	// read-only PLG user would be someone who watches work pile up and cannot
	// touch it. Playbook management is the one exception — see below.
	//
	// This does NOT replace PLG's identity middleware, which resolves the caller
	// to the "user".id every PLG write records and refuses anyone who is not
	// ACTIVE INTERNAL staff. The two answer different questions: this one asks
	// what the token claims, that one asks whether the person is still an
	// employee. See internal/plg/plg.go.
	PermUsePlg
	// PermCreateWorkNote is posting a work_note-type comment on a case --
	// POST /cases/{id}/comments, the route this gates. Deliberately broader
	// than PermWrite at the ROUTE level (WorknoteCreator ∪ CsEngineer ∪
	// Admin, a superset of PermWrite's CsEngineer ∪ Admin) so a
	// WorknoteCreator-only caller can reach the handler at all; CaseHandler
	// then requires the caller ALSO hold full PermWrite for any comment
	// whose type is NOT work_note (a customer-visible reply, or any future
	// type) -- same "broader route floor, narrower in-handler check for the
	// more sensitive sub-action" shape as PermApproveTimeCard/
	// PermViewSecurityCenter, just inverted: here the floor is the new
	// permission and the narrower gate is the pre-existing one. A
	// WorknoteCreator-only caller can therefore only ever post internal
	// work notes, never a customer-visible comment.
	//
	// Viewer deliberately does NOT hold this: it is the read-only role, and
	// work notes have their own dedicated role (worknote_creator).
	PermCreateWorkNote
	// PermManagePlaybooks is authoring a PLG playbook template: creating one,
	// editing it, replacing its tasks, deleting it. Admin only.
	//
	// Separate from PermAdmin, which it currently matches exactly, because the
	// two mean different things: PermAdmin is "actions no non-admin staff role
	// should reach", and granting playbook authoring to some future PLG-admin
	// role must not also hand out platform-user creation.
	//
	// READING playbooks is PermUsePlg, not this. A CS engineer browses templates
	// and assigns them to a pairing; they just cannot change one. Assignment is
	// POST /organizations/{id}/products/{product}/playbook-runs, a different path
	// from the four this guards.
	PermManagePlaybooks
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
	// SalesSolutions grants PermViewSharedEntity (see that permission's own
	// doc comment for exactly which routes -- deliberately NOT all of
	// PermView). It's also, independently, a marker role: GET /users/me
	// reports "sales_solutions" in its roles list. It does NOT grant
	// PermViewerAccess or drive the webapp's SPL-vs-CS-Portal nav choice --
	// that's Viewer's and CsEngineer's job respectively (see
	// PermViewerAccess's own doc comment). A holder still needs one of the
	// roles above to write, escalate, download an attachment, or
	// administer anything — PermEscalate/PermDownloadAttachment/
	// PermUsageMetricsViewer/PermWrite/PermAdmin etc. are unaffected by
	// this role.
	SalesSolutions []string
	// WorknoteCreator grants PermCreateWorkNote (see that permission's own
	// doc comment) -- creating a work_note-type comment on a case, and
	// nothing else. A holder still needs CsEngineer/Admin's own PermWrite
	// to post a customer-visible reply, escalate, download an attachment,
	// or any other write action; this role grants none of those.
	WorknoteCreator []string
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
// own doc comment), the role for people who work cases, satisfies every other
// permission EXCEPT THREE: PermAdmin (admin-only, held by no other role,
// unlike PermWrite which both share), PermEscalate, and PermApproveTimeCard —
// escalating a case and approving a time card are each a dedicated
// responsibility, held only by their own role (escalator / time-card
// approver) plus admin, not by being a CS engineer alone. CS engineer DOES
// still hold the broader PermTimeCardsAndUpdates (viewing/managing time
// cards short of approving them). The attachment-downloader role exists
// separately so other staff can be granted just that one ability. The
// usage-metrics and dashboard-designer roles gate nothing here (this backend
// has no route for those features) and grant only View. Every role implies
// View, so a user granted only one specialised role can still open the pages
// it acts on. Every View-implying role also holds PermViewSharedEntity, the
// narrower slice of View that sales_solutions gets instead (see that
// permission's own doc comment) -- nothing already readable stops being
// readable. PermViewSecurityCenter is the one further exception to "every
// role implies View covers it": plain viewer/escalator/attachment_downloader/
// usage_metrics_viewer/timecard_approver/dashboard_designer all hold PermView
// but not this. PermUsePlg is narrower the same way — CS engineer and admin
// only — and PermManagePlaybooks narrower again, admin alone.
// sales_solutions is a separate exception again: it implies
// PermViewSharedEntity (only) rather than being implied BY it — see
// AccessConfig.SalesSolutions's own doc comment. PermViewerAccess is implied by
// plain Viewer, not sales_solutions or cs_engineer specifically -- see
// PermViewerAccess's own doc comment for why that's a deliberately broader
// audience check than the webapp's CsEngineer-first portal-nav choice.
// worknote_creator is narrower still: it implies nothing but
// PermCreateWorkNote, and even that is capped to work_note-type comments
// only -- see that permission's own doc comment.
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
			{"sales_solutions", build(cfg.SalesSolutions)},
			{"worknote_creator", build(cfg.WorknoteCreator)},
		},
		allowed: map[Permission]map[string]struct{}{
			PermView: build(cfg.Viewer, cfg.Escalator, cfg.AttachmentDownloader,
				cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover, cfg.DashboardDesigner),
			PermViewOperations:      build(cfg.CsEngineer, cfg.Admin),
			PermTimeCardsAndUpdates: build(cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover),
			PermEscalate:            build(cfg.Escalator, cfg.Admin),
			PermDownloadAttachment:  build(cfg.AttachmentDownloader, cfg.CsEngineer, cfg.Admin),
			PermWrite:               build(cfg.CsEngineer, cfg.Admin),
			PermViewAllDashboards:   build(cfg.CsEngineer, cfg.Admin),
			PermAdmin:               build(cfg.Admin),
			PermViewSecurityCenter:  build(cfg.CsEngineer, cfg.Admin),
			PermApproveTimeCard:     build(cfg.TimecardApprover, cfg.Admin),
			// Viewer, unconditionally (no cs_engineer exclusion) -- see
			// PermViewerAccess's own doc comment for why.
			PermViewerAccess: build(cfg.Viewer),
			// Every existing PermView holder, so nothing they could already
			// read stops being readable, plus SalesSolutions for exactly the
			// routes this permission is registered on -- see
			// PermViewSharedEntity's own doc comment for why this is not
			// just PermView with SalesSolutions folded in.
			PermViewSharedEntity: build(cfg.Viewer, cfg.Escalator, cfg.AttachmentDownloader,
				cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover, cfg.DashboardDesigner,
				cfg.SalesSolutions),
			// Same population as PermEscalate/PermDownloadAttachment's own
			// "the specialised role, or a CS Portal role that already
			// dominates it" shape -- see PermUsageMetricsViewer's own doc
			// comment.
			PermUsageMetricsViewer: build(cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin),
			PermUsePlg:             build(cfg.CsEngineer, cfg.Admin),
			PermManagePlaybooks:    build(cfg.Admin),
			// The route-level floor for POST /cases/{id}/comments -- see
			// PermCreateWorkNote's own doc comment for the in-handler
			// narrowing that keeps a WorknoteCreator-only caller from
			// posting anything but a work_note.
			PermCreateWorkNote: build(cfg.WorknoteCreator, cfg.CsEngineer, cfg.Admin),
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
