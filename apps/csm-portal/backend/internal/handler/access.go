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
	// operations, which support-portal-lite never exposed to them. Held by
	// cs_engineer, admin, and security_team (read-only -- see
	// AccessConfig.SecurityTeam's own doc comment); every other role, Viewer
	// included, is denied.
	PermViewOperations
	// PermTimeCardsAndUpdates is the Time Cards and Updates areas: every time-card
	// route (search, create, update, delete) and the update-level lookups. Held by
	// the CS engineer and admin, and by the time-card approver so viewing/managing
	// time cards does not require being a CS engineer. This is deliberately
	// broader than approving one — see PermApproveTimeCard below, which is what's
	// actually narrowed to the approver role.
	PermTimeCardsAndUpdates
	// PermEscalate is escalating a case. Held by the CS engineer, the
	// escalator role and admin. It is NOT a route permission: POST
	// /cases/{id}/escalations only requires PermView, and
	// CaseHandler.CreateCaseEscalation enforces the rest because the rule
	// depends on the body's action and on the case. ESCALATE needs this
	// permission; DEESCALATE needs being one of the case's team leads
	// instead (any portal role, no escalation role), and holding this
	// permission does not substitute for that.
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
	// PermViewSecurityCenter is the Security Center area: both
	// /products/vulnerabilities routes. Admin, cs_engineer, comment_updater
	// and security_team only — every other role, including plain
	// viewer/escalator/attachment_downloader, is denied even though they
	// hold PermView, since this is deliberately narrower than the general
	// case/product-data access PermView otherwise grants. comment_updater
	// holds it because that role is also used for staff who triage
	// security reports but don't otherwise hold cs_engineer. security_team
	// is the dedicated read-only role for the security team itself (see
	// AccessConfig.SecurityTeam's own doc comment) — it holds this and
	// PermViewOperations but never PermWrite, so Security Center and
	// Operations render for it, read-only.
	//
	// Security-report-analysis CASE search (POST /cases/search,
	// type-checked inside CaseHandler itself — see its own doc comment for
	// why a route-level permission alone can't express this) is gated by
	// PermViewSecurityReportCases below, NOT this one — the two were split
	// apart deliberately so Viewer could be granted read access to
	// security-report (and service-request) cases without also opening the
	// Vulnerabilities tab/routes, which stay restricted to this permission
	// alone.
	PermViewSecurityCenter
	// PermViewSecurityReportCases is viewing security-report-analysis cases
	// through POST /cases/search (CaseHandler.SearchCases' own in-handler
	// caseSearchTargetsSecurityReports check — see WithAccessGuard's doc
	// comment). Deliberately its own permission, not PermViewSecurityCenter:
	// Viewer needed read access to these cases (an API-only change, no
	// webapp/nav change alongside it) without also gaining
	// /products/vulnerabilities access, which PermViewSecurityCenter still
	// gates alone. Also held by security_team, which could already reach
	// this case search via PermViewSecurityCenter before the two
	// permissions were split apart — this keeps that role's existing
	// access unchanged. service_request cases have no equivalent gate at
	// all — SearchCases/GetCase never restrict that type by role, so every
	// PermView holder (Viewer included) can already search/fetch them.
	PermViewSecurityReportCases
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
	//
	// Customer Health is not on this permission: see PermViewCustomerHealth.
	// The project time and CS reports are not on it either: they use PermView.
	PermViewerAccess
	// PermViewCustomerHealth is the Customer Health dashboard: its summary,
	// account detail, risk and action-item routes (/customer-health/*) and the
	// two filter lists it reads (GET /products, GET /abt-teams).
	//
	// Held by Viewer, the audience the dashboard was built for, and also by
	// cs_engineer and admin, who work the same accounts and need the same
	// view -- so unlike PermViewerAccess it does not require the Viewer role:
	// a cs_engineer or admin reaches it without separately holding Viewer.
	// Every other route in the viewer-audience block (the account
	// escalation list, Drive files) stays on PermViewerAccess. The project
	// time and CS reports use PermView instead.
	// Keep the webapp's canViewCustomerHealth (portalAccess.ts) in sync with
	// this set.
	PermViewCustomerHealth
	// PermUsageMetricsViewer is the Usage Metrics domain (/usage-metrics/*).
	// Unlike the ex-"Support Portal Lite" domains still gated by
	// PermViewerAccess (Viewer only: account escalations, Drive files),
	// and unlike Customer Health (PermViewCustomerHealth, which cs_engineer
	// and admin hold too), Usage Metrics is registered directly
	// on THIS permission at the route level, not layered on top of
	// PermViewerAccess: it was reported live as needing a broader audience
	// than "holds the Viewer role" once Support Portal Lite's separate
	// app/nav was folded into the main portal — a cs_engineer, admin or
	// dedicated usage_metrics_viewer holder must reach it even without
	// separately holding Viewer too. Viewer itself does NOT hold this
	// (confirmed live, correcting an earlier pass that added it): a
	// Viewer-only caller must not see Usage Metrics at all, unlike its ex-SPL
	// siblings.
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
	// PermCreateAnnouncement is creating and sending a customer announcement:
	// every write on the announcement-request workflow (create, edit, dry run,
	// submit, schedule, publish, add an update, record deliveries), listing the
	// requests (the Requests tab: drafts and requests awaiting approval, which is
	// the creators' workspace) and creating a
	// work item of type announcement through POST /cases, which is how the
	// dry-run case and the per-project cases of a publish are made. Held by the
	// announcement_creator role and admin.
	//
	// It is checked IN ADDITION to PermWrite, never instead of it: this role
	// narrows who among the people who can already write may send customers an
	// announcement, it does not make anyone a writer. So a cs_engineer without
	// announcement_creator keeps every other write and loses this one, and an
	// announcement_creator who is not a cs_engineer or admin cannot send
	// anything. The routes are therefore registered through AccessGuard.RequireAll
	// with both permissions, and CaseHandler.CreateCase applies it only when the
	// request body asks for type announcement (a route permission cannot see
	// the body -- same shape as PermViewSecurityCenter on POST /cases/search).
	//
	// Approving a request ("Mark as approved") is deliberately NOT gated by it:
	// that step only records a decision taken over email, outside this portal,
	// and may be recorded by someone other than the creator.
	PermCreateAnnouncement
	// PermUpdateDeleteComment is the route-level floor for PATCH/DELETE
	// /comments/{id} -- CommentUpdater ∪ CsEngineer ∪ Admin, the same
	// "specialised role, or a CS Portal role that already dominates it"
	// shape as PermCreateWorkNote, not PermCreateAnnouncement's narrowing
	// shape: a comment_updater-only caller must be able to reach the
	// handler at all (today only cs_engineer/admin can), and cs_engineer
	// must keep its existing ability to edit/delete its own comments.
	//
	// This permission only decides who may REACH the handler, not which
	// specific comment they may act on -- entity-service performs NO
	// authorization check of its own for this path any more (it has no way
	// to see this backend's Asgardeo role vocabulary, and its own separate
	// Postgres "admin" role was dropped as redundant and confusing). The
	// finer-grained decision -- author, or PermUpdateDeleteAnyComment below
	// -- is made entirely in CommentHandler itself (see its
	// authorizeCommentActor), which calls entity-service's GetComment first
	// purely to learn the comment's author. A plain cs_engineer holds this
	// permission (reaches the handler) but not PermUpdateDeleteAnyComment,
	// so they can still only successfully edit/delete a comment THEY
	// authored.
	PermUpdateDeleteComment
	// PermUpdateDeleteAnyComment is held by Admin and CommentUpdater ONLY --
	// deliberately NOT CsEngineer, unlike PermUpdateDeleteComment above. It
	// does not gate a route directly: CommentHandler.authorizeCommentActor
	// checks it in-handler (mirroring PermCreateAnnouncement/
	// PermViewSecurityCenter's own in-handler-check shape), alongside an
	// explicit author check against the comment it fetches via GetComment,
	// to decide whether THIS caller may act on THIS comment -- a decision
	// made entirely here, not forwarded to or re-checked by entity-service
	// in any way. A cs_engineer must NOT hold this permission:
	// PermUpdateDeleteComment above already lets them reach the handler for
	// their OWN comment, and authorizeCommentActor's own author check is
	// what keeps that scoped to authorship -- granting this one too would
	// let them act on EVERY comment, erasing that distinction.
	PermUpdateDeleteAnyComment
	// PermManageOnboarding is setting a project's onboarding status and
	// onboarding owner -- the onboardingStatus and onboardingOwner fields of
	// PATCH /projects/{id}. Held ONLY by the onboarding_lead role and admin --
	// NOT the CS engineer, same narrowing as PermApproveTimeCard. It is checked
	// in-handler (ProjectHandler.UpdateProject), since a route permission cannot
	// see which fields the body targets, and it does not grant any read access:
	// a holder needs a viewing role separately to see the project at all.
	PermManageOnboarding
	// PermUpdateProject is the route-level floor for PATCH /projects/{id} --
	// CsEngineer ∪ Admin ∪ OnboardingLead, the same "specialised role, or a
	// role that already dominates it" shape as PermCreateWorkNote, so an
	// onboarding_lead-only caller can reach the handler at all. It decides who
	// may REACH the handler, nothing more: ProjectHandler.UpdateProject then
	// requires PermManageOnboarding for the onboarding fields and PermWrite for
	// every other field, so an onboarding_lead-only caller can still never edit
	// anything but the two onboarding fields.
	PermUpdateProject
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
	// AnnouncementCreator grants PermCreateAnnouncement (see that permission's
	// own doc comment): creating and sending a customer announcement, on top
	// of a caller's own PermWrite. Unlike most roles here an unset variable is
	// not an "everyone" or a "nobody-can-use-the-portal" state: it means only
	// admin can create announcements, so deploy it with the variable set.
	AnnouncementCreator []string
	// CommentUpdater grants PermUpdateDeleteComment (see that permission's
	// own doc comment) -- reaching PATCH/DELETE /comments/{id} at all, on
	// top of cs_engineer/admin's own existing access. Unlike most roles
	// here an unset variable is not a lockout: cs_engineer/admin already
	// hold this permission regardless (same "unconfigured is a normal
	// state" reasoning as WorknoteCreator's own doc comment).
	CommentUpdater []string
	// OnboardingLead grants PermManageOnboarding (see that permission's own
	// doc comment) -- setting a project's onboarding status and owner -- and
	// reaching PATCH /projects/{id} at all, but nothing else: no read access
	// and no other write. Like comment_updater, an unset variable is not a
	// lockout: admin already holds PermManageOnboarding regardless.
	OnboardingLead []string
	// SecurityTeam grants PermView, PermViewOperations,
	// PermViewSecurityCenter and PermViewSecurityReportCases -- the
	// dedicated role for security-team staff who need to see the Operations
	// area and Security Center (security reports + vulnerabilities) but
	// hold no write access of any kind: it never grants PermWrite,
	// PermEscalate, PermApproveTimeCard or any other state-changing
	// permission, so both sections render read-only for a holder with no
	// other role. Deliberately separate from comment_updater (which also
	// holds PermViewSecurityCenter and PermViewSecurityReportCases, for
	// staff who triage security reports while also handling comments) and
	// from cs_engineer/admin (full access) -- this role's whole purpose is
	// "read-only, Operations and Security Center only."
	SecurityTeam []string
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

// portalRole is one portal role: its stable key (what the frontend sees),
// the configured token role names that grant it, -- once WithRoleIDs has
// run -- the real Asgardeo role IDs that resolve to those same names, and --
// once WithGroupNames has run -- the real Asgardeo group display names
// configured directly against this role's own key.
type portalRole struct {
	key   string
	names map[string]struct{}
	// ids is nil until WithRoleIDs runs; RolesForIDs then treats a nil map
	// the same as an empty one (no match), so calling it before WithRoleIDs
	// is a safe, silent no-op rather than a nil-map panic.
	ids map[string]struct{}
	// groupNames is nil until WithGroupNames runs; RolesForGroupNames and
	// RolesForIDsAndGroupNames treat a nil map the same way ids is treated
	// above -- a safe, silent no-op, never a nil-map panic. Matched
	// case-sensitively, the same exact-match convention AUTH_<ROLE>_ROLES
	// itself uses for role names.
	groupNames map[string]struct{}
}

// NewAccessGuard builds a guard from cfg. Admin satisfies every permission.
// CS engineer (renamed from support_engineer -- see AccessConfig.CsEngineer's
// own doc comment), the role for people who work cases, satisfies every other
// permission EXCEPT TWO: PermAdmin (admin-only, held by no other role,
// unlike PermWrite which both share) and PermApproveTimeCard — approving a
// time card is a dedicated responsibility, held only by its own role
// (time-card approver) plus admin, not by being a CS engineer alone. CS
// engineer holds PermEscalate (with the escalator role and admin); see its
// own doc comment for how it is enforced. CS engineer also
// holds the broader PermTimeCardsAndUpdates (viewing/managing time
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
// but not this — Viewer's read access to security-report case search comes
// from the separate, narrower PermViewSecurityReportCases instead (see that
// permission's own doc comment), not from holding PermViewSecurityCenter.
// PermUsePlg is narrower the same way — CS engineer and admin
// only — and PermManagePlaybooks narrower again, admin alone.
// sales_solutions is a separate exception again: it implies
// PermViewSharedEntity (only) rather than being implied BY it — see
// AccessConfig.SalesSolutions's own doc comment. PermViewerAccess is implied by
// plain Viewer, not sales_solutions or cs_engineer specifically -- see
// PermViewerAccess's own doc comment for why that's a deliberately broader
// audience check than the webapp's CsEngineer-first portal-nav choice.
// worknote_creator is narrower still: it implies nothing but
// PermCreateWorkNote, and even that is capped to work_note-type comments
// only -- see that permission's own doc comment. announcement_creator is
// narrower in a different way: it implies nothing, not even View, and
// PermCreateAnnouncement is only ever checked alongside PermWrite, so it
// removes an ability from cs_engineer rather than adding one. comment_updater
// is the same added-floor shape as worknote_creator for PermUpdateDeleteComment
// (cs_engineer/admin keep holding that permission too), but ALSO implies
// PermUpdateDeleteAnyComment alongside admin only -- cs_engineer deliberately
// does not hold that second one -- and PermViewSecurityCenter, which
// cs_engineer and admin both already hold anyway. See
// PermUpdateDeleteComment's and PermUpdateDeleteAnyComment's own doc comments
// for what each actually controls. security_team is narrower still, and in a
// different direction from every role above: it implies View, PermViewOperations,
// PermViewSecurityCenter and PermViewSecurityReportCases and nothing else -- not
// PermWrite, not PermEscalate, not any other state-changing permission -- so both
// areas it can see render read-only for a holder with no other role. See
// AccessConfig.SecurityTeam's own doc comment. onboarding_lead implies only
// PermManageOnboarding and the PermUpdateProject route floor -- no View, no
// PermWrite -- and admin alone also holds PermManageOnboarding; cs_engineer
// deliberately does not.
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
			{key: "viewer", names: build(cfg.Viewer)},
			{key: "escalator", names: build(cfg.Escalator)},
			{key: "attachment_downloader", names: build(cfg.AttachmentDownloader)},
			{key: "cs_engineer", names: build(cfg.CsEngineer)},
			{key: "usage_metrics_viewer", names: build(cfg.UsageMetricsViewer)},
			{key: "timecard_approver", names: build(cfg.TimecardApprover)},
			{key: "dashboard_designer", names: build(cfg.DashboardDesigner)},
			{key: "admin", names: build(cfg.Admin)},
			{key: "sales_solutions", names: build(cfg.SalesSolutions)},
			{key: "worknote_creator", names: build(cfg.WorknoteCreator)},
			{key: "announcement_creator", names: build(cfg.AnnouncementCreator)},
			{key: "comment_updater", names: build(cfg.CommentUpdater)},
			{key: "onboarding_lead", names: build(cfg.OnboardingLead)},
			{key: "security_team", names: build(cfg.SecurityTeam)},
		},
		allowed: map[Permission]map[string]struct{}{
			PermView: build(cfg.Viewer, cfg.Escalator, cfg.AttachmentDownloader,
				cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover, cfg.DashboardDesigner,
				cfg.SecurityTeam),
			PermViewOperations:      build(cfg.CsEngineer, cfg.Admin, cfg.SecurityTeam),
			PermTimeCardsAndUpdates: build(cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover),
			PermEscalate:            build(cfg.Escalator, cfg.CsEngineer, cfg.Admin),
			PermDownloadAttachment:  build(cfg.AttachmentDownloader, cfg.CsEngineer, cfg.Admin),
			PermWrite:               build(cfg.CsEngineer, cfg.Admin),
			PermViewAllDashboards:   build(cfg.CsEngineer, cfg.Admin),
			PermAdmin:               build(cfg.Admin),
			PermViewSecurityCenter:  build(cfg.CsEngineer, cfg.Admin, cfg.CommentUpdater, cfg.SecurityTeam),
			// Everyone PermViewSecurityCenter already covers, plus Viewer —
			// see that permission's own doc comment for why this is a
			// separate permission rather than adding cfg.Viewer to
			// PermViewSecurityCenter directly (that would also open
			// /products/vulnerabilities to Viewer, which is out of scope).
			PermViewSecurityReportCases: build(cfg.Viewer, cfg.CsEngineer, cfg.Admin, cfg.CommentUpdater, cfg.SecurityTeam),
			PermApproveTimeCard:         build(cfg.TimecardApprover, cfg.Admin),
			// Viewer, unconditionally (no cs_engineer exclusion) -- see
			// PermViewerAccess's own doc comment for why.
			PermViewerAccess: build(cfg.Viewer),
			// Viewer plus the staff roles that hold full access -- see
			// PermViewCustomerHealth's own doc comment.
			PermViewCustomerHealth: build(cfg.Viewer, cfg.CsEngineer, cfg.Admin),
			// Every existing PermView holder, so nothing they could already
			// read stops being readable, plus SalesSolutions for exactly the
			// routes this permission is registered on -- see
			// PermViewSharedEntity's own doc comment for why this is not
			// just PermView with SalesSolutions folded in.
			PermViewSharedEntity: build(cfg.Viewer, cfg.Escalator, cfg.AttachmentDownloader,
				cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin, cfg.TimecardApprover, cfg.DashboardDesigner,
				cfg.SalesSolutions, cfg.SecurityTeam),
			// CsEngineer, Admin, and the dedicated role grant this directly
			// (no separate PermViewerAccess layer, and Viewer itself is
			// deliberately NOT included) -- see PermUsageMetricsViewer's own
			// doc comment.
			PermUsageMetricsViewer: build(cfg.UsageMetricsViewer, cfg.CsEngineer, cfg.Admin),
			PermUsePlg:             build(cfg.CsEngineer, cfg.Admin),
			PermManagePlaybooks:    build(cfg.Admin),
			// The route-level floor for POST /cases/{id}/comments -- see
			// PermCreateWorkNote's own doc comment for the in-handler
			// narrowing that keeps a WorknoteCreator-only caller from
			// posting anything but a work_note.
			PermCreateWorkNote: build(cfg.WorknoteCreator, cfg.CsEngineer, cfg.Admin),
			// announcement_creator and admin only -- cs_engineer deliberately
			// does NOT hold it, which is the whole point of the role. Always
			// checked together with PermWrite (see its doc comment).
			PermCreateAnnouncement: build(cfg.AnnouncementCreator, cfg.Admin),
			// The route-level floor for PATCH/DELETE /comments/{id} -- see
			// PermUpdateDeleteComment's own doc comment for why cs_engineer
			// stays in this set (unlike PermCreateAnnouncement's) and for
			// what entity-service still separately enforces per comment.
			PermUpdateDeleteComment: build(cfg.CommentUpdater, cfg.CsEngineer, cfg.Admin),
			// Deliberately excludes cs_engineer -- see
			// PermUpdateDeleteAnyComment's own doc comment for why granting
			// it to them would erase entity-service's authorship scoping
			// rather than just widen who can reach the handler.
			PermUpdateDeleteAnyComment: build(cfg.CommentUpdater, cfg.Admin),
			// onboarding_lead and admin only -- cs_engineer deliberately does
			// NOT hold it. See PermManageOnboarding's own doc comment.
			PermManageOnboarding: build(cfg.OnboardingLead, cfg.Admin),
			// The route-level floor for PATCH /projects/{id} -- see
			// PermUpdateProject's own doc comment for the in-handler checks
			// that narrow what each holder may actually set.
			PermUpdateProject: build(cfg.CsEngineer, cfg.Admin, cfg.OnboardingLead),
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

// WithRoleIDs cross-references each portal role's configured real role
// name(s) against idsByRoleName (ASGARDEO_ROLE_IDS, see
// directory.ParseRoleIDs) to resolve the real Asgardeo role ID(s) that grant
// each portal role, enabling RolesForIDs below. A name with no configured ID
// simply contributes nothing -- the same "no entry means not wired up here"
// posture ASGARDEO_ROLE_IDS itself documents; RolesForIDs answers empty for
// every portal role until this has been called, rather than panicking on a
// nil map. Returns g for chaining, the same post-construction wiring shape
// as handler.UsersHandler.WithAccessGuard/WithGrantableRoles.
func (g *AccessGuard) WithRoleIDs(idsByRoleName map[string]string) *AccessGuard {
	for i, r := range g.portalRoles {
		ids := make(map[string]struct{}, len(r.names))
		for name := range r.names {
			if id, ok := idsByRoleName[name]; ok {
				ids[id] = struct{}{}
			}
		}
		g.portalRoles[i].ids = ids
	}
	return g
}

// RolesForIDs is RolesFor's counterpart for a caller that knows which real
// roles a user holds by their own Asgardeo role ID (e.g. a SCIM GET-by-ID
// response's "value" field for each role in its "roles" array), not by
// name -- needed because two distinct Asgardeo roles can share the same
// display ("name"), which RolesFor's name-based matching cannot tell apart,
// while a role's ID is always unique. Same fixed GET /users/me-reporting
// order as RolesFor, and the same "always a non-nil, possibly empty slice"
// contract. Answers empty for every role until WithRoleIDs has been called.
func (g *AccessGuard) RolesForIDs(roleIDs []string) []string {
	keys := make([]string, 0, len(g.portalRoles))
	for _, r := range g.portalRoles {
		for _, id := range roleIDs {
			if _, ok := r.ids[id]; ok {
				keys = append(keys, r.key)
				break
			}
		}
	}
	return keys
}

// WithGroupNames wires ASGARDEO_ROLE_GROUP_NAMES -- a portal-role-key ->
// Asgardeo-group-display-name(s) mapping (see directory.ParseRoleGroupNames)
// -- enabling RolesForGroupNames/RolesForIDsAndGroupNames below. Unlike
// WithRoleIDs, which cross-references a portal role's configured real role
// NAMES against ASGARDEO_ROLE_IDS, this matches directly on the portal
// role's own stable KEY: a group has no equivalent of a role's "real name"
// on the token to cross-reference against AUTH_<ROLE>_ROLES, so the
// deployment names the portal role being granted directly (e.g.
// "cs_engineer") and lists the group display name(s) that should grant it
// -- matched against a group's SCIM "display" value (the same name visible
// in the Asgardeo console), not its opaque id. A portal role key with no
// entry here simply never matches by group membership -- the same "no entry
// means not wired up here" posture WithRoleIDs already has for role names.
//
// Unlike a real Asgardeo role name (an open-ended, deployment-specific
// string ASGARDEO_ROLE_IDS has no fixed vocabulary to check against), a
// portal role key IS a closed, known set -- exactly g.portalRoles' own keys,
// fixed by NewAccessGuard above. A key in groupNamesByRoleKey that matches
// none of them is therefore always a configuration mistake (a typo like
// "cs_enginer"), not a legitimate "not relevant to this deployment yet"
// case, and would otherwise grant nothing with no signal anywhere that it
// silently failed to wire up. Logged as a startup warning naming the
// offending key(s) -- not a hard failure, matching this guard's existing
// posture elsewhere (an unset/misconfigured AUTH_<ROLE>_ROLES also only
// warns, never exits) rather than taking down the whole backend over one
// bad entry in an otherwise-optional, purely additive feature.
// Returns g for chaining.
func (g *AccessGuard) WithGroupNames(groupNamesByRoleKey map[string][]string) *AccessGuard {
	known := make(map[string]struct{}, len(g.portalRoles))
	for _, r := range g.portalRoles {
		known[r.key] = struct{}{}
	}
	for key := range groupNamesByRoleKey {
		if _, ok := known[key]; !ok {
			slog.Warn("ASGARDEO_ROLE_GROUP_NAMES names an unknown portal role; its configured group(s) grant nothing", "key", key)
		}
	}

	for i, r := range g.portalRoles {
		names := make(map[string]struct{}, len(groupNamesByRoleKey[r.key]))
		for _, name := range groupNamesByRoleKey[r.key] {
			names[name] = struct{}{}
		}
		g.portalRoles[i].groupNames = names
	}
	return g
}

// RolesForGroupNames is RolesForIDs' counterpart for matching by Asgardeo
// group membership instead of role assignment: some portal roles are
// granted by placing a user in a specific Asgardeo group rather than
// assigning them a role, configured via WithGroupNames. Matches on each
// group's display name, case-sensitively -- the same exact-match convention
// AUTH_<ROLE>_ROLES itself uses for role names. Same fixed order and
// non-nil-slice contract as RolesFor/RolesForIDs. Answers empty for every
// role until WithGroupNames has been called.
func (g *AccessGuard) RolesForGroupNames(groupNames []string) []string {
	keys := make([]string, 0, len(g.portalRoles))
	for _, r := range g.portalRoles {
		for _, name := range groupNames {
			if _, ok := r.groupNames[name]; ok {
				keys = append(keys, r.key)
				break
			}
		}
	}
	return keys
}

// RolesForIDsAndGroupNames merges RolesForIDs and RolesForGroupNames into one
// fixed-order, deduplicated list: a portal role granted by EITHER a held
// role ID or group membership appears exactly once, in GET /users/me's usual
// order, never twice (and never out of order) the way separately computing
// and concatenating the two slices could produce. This is what
// withPortalRoles actually calls -- a user can reach the same portal role
// through a role assignment, a group membership, or both, and the caller
// should not have to merge/dedupe two independently-ordered slices itself.
func (g *AccessGuard) RolesForIDsAndGroupNames(roleIDs, groupNames []string) []string {
	keys := make([]string, 0, len(g.portalRoles))
	for _, r := range g.portalRoles {
		matched := false
		for _, id := range roleIDs {
			if _, ok := r.ids[id]; ok {
				matched = true
				break
			}
		}
		if !matched {
			for _, name := range groupNames {
				if _, ok := r.groupNames[name]; ok {
					matched = true
					break
				}
			}
		}
		if matched {
			keys = append(keys, r.key)
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

// RequireAll is Require for a route that needs EVERY one of perms: the caller
// must satisfy each, not just one of them. It exists for PermCreateAnnouncement,
// which narrows PermWrite rather than replacing it. An empty perms list denies
// everyone, so a registration that forgot to pass one cannot go live open.
func (g *AccessGuard) RequireAll(next http.HandlerFunc, perms ...Permission) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserInfoFromContext(r.Context())
		if user == nil {
			writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
			return
		}
		if len(perms) == 0 {
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
			return
		}
		for _, perm := range perms {
			if perm == PermAuthenticated {
				continue
			}
			if !g.Permits(perm, user.Roles) {
				slog.WarnContext(r.Context(), "access denied: token carries no role granting this permission", "userID", user.UserID, "method", r.Method, "path", r.URL.Path)
				writeError(w, http.StatusForbidden, ErrMsgForbidden)
				return
			}
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
