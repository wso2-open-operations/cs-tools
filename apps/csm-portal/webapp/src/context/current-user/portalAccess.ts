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
import { devBypassAccessCheck } from "@config/devFlags";

/**
 * Portal roles as returned in `GET /users/me`'s `roles` (stable keys, not the
 * IdP role names). A user can hold several.
 */
export const PORTAL_ROLE = {
  viewer: "viewer",
  escalator: "escalator",
  attachmentDownloader: "attachment_downloader",
  // Renamed from support_engineer -- the wire value must stay byte-for-byte
  // in sync with the backend's own AccessGuard portalRoles key
  // (apps/csm-portal/backend/internal/handler/access.go), which returns this
  // exact string in GET /users/me's roles array.
  csEngineer: "cs_engineer",
  usageMetricsViewer: "usage_metrics_viewer",
  timecardApprover: "timecard_approver",
  dashboardDesigner: "dashboard_designer",
  admin: "admin",
  // Grants PermCreateWorkNote on the backend (internal/handler/access.go) --
  // posting a work_note-type comment on a case, nothing else. See
  // PermissionProvider.tsx's canAddWorkNotes for the one capability this
  // backs.
  worknoteCreator: "worknote_creator",
  // Grants PermCreateAnnouncement on the backend (internal/handler/access.go)
  // -- creating and sending a customer announcement, ON TOP OF write access.
  // See canCreateAnnouncement below.
  announcementCreator: "announcement_creator",
  // Grants PermUpdateDeleteComment on the backend (internal/handler/access.go)
  // -- reaching PATCH/DELETE /comments/{id} for a comment the holder did NOT
  // author. See canUpdateDeleteAnyComment below.
  commentUpdater: "comment_updater",
  // Grants PermViewOperations and PermViewSecurityCenter on the backend
  // (internal/handler/access.go) -- read-only access to the Operations area
  // and Security Center (security reports + vulnerabilities), and nothing
  // else: never PermWrite, so both sections render read-only for a holder
  // with no other role. See canUseOperations/canUseSecurityCenter below.
  securityTeam: "security_team",
  // Grants PermManageOnboarding on the backend (internal/handler/access.go) --
  // setting a project's onboarding status and owner via PATCH /projects/{id},
  // and nothing else. It grants no read access. See canEditProjectOnboarding.
  onboardingLead: "onboarding_lead",
} as const;

// Roles that, held alone, let someone use the portal at all. announcement_creator
// is left out on purpose: it only narrows who among the people who can already
// write may send an announcement (see canCreateAnnouncement), so on its own it
// grants nothing, and counting it would let a caller holding nothing else in
// past the "no access" screen into a portal where every call is a 403.
// onboarding_lead is excluded too: the backend gives it no read permission,
// only the one onboarding PATCH.
// comment_updater is excluded for the identical reason: on its own it grants
// only editing/deleting someone else's comment (canUpdateDeleteAnyComment),
// and reaching a comment at all first requires being able to view the case
// it's on -- a comment_updater-only caller has no view access of any kind.
const ROLES_THAT_GRANT_ACCESS: readonly string[] = Object.values(PORTAL_ROLE).filter(
  (role) => role !== PORTAL_ROLE.announcementCreator &&
    role !== PORTAL_ROLE.commentUpdater &&
    role !== PORTAL_ROLE.onboardingLead,
);

export interface PortalAccess {
  /** Holds at least one portal role — the minimum to use the portal at all. */
  hasAnyRole: boolean;
  /**
   * Escalating a case: `admin`, `cs_engineer` and `escalator`, mirroring the
   * backend's `PermEscalate`. Does NOT govern de-escalating: that is allowed
   * only to the case's ABT team leads and needs no portal role (see
   * CsmCaseDetailPage).
   */
  canEscalate: boolean;
  canDownloadAttachment: boolean;
  /**
   * The Operations section (incidents, change requests, problems, outages,
   * service requests). Support-portal-lite had no such area, so its
   * view-oriented roles never saw one: only full-access roles do here, plus
   * the dedicated `security_team` role (read-only -- that role never holds
   * {@link canWrite}, so this section renders read-only for a holder with no
   * other role). Mirrors the backend's `PermViewOperations`.
   */
  canUseOperations: boolean;
  /**
   * The Time Cards and Updates sections, and every time-card and update-level
   * call behind them. Full-access roles have it, and so does the time-card
   * approver, so viewing/managing time cards does not require being a CS
   * engineer. This governs the section as a whole, not approval specifically
   * — mirrors the backend's `PermTimeCardsAndUpdates`, which `cs_engineer`
   * still holds. Approving/rejecting a time card is a narrower, separate
   * concern gated by `useTimecardRole()` (`isApprover`/`isAdmin`), not this
   * flag — mirroring the backend's own narrower `PermApproveTimeCard`.
   */
  canUseTimeCardsAndUpdates: boolean;
  /** Every other state-changing action (create/update cases, tasks, ...). */
  canWrite: boolean;
  /**
   * Posting an internal work note on a case. `cs_engineer`/`admin` (full
   * write) can, and so can `worknote_creator` -- which is ALL that role can
   * do: no customer-visible reply, no attachment upload, no other write. A
   * plain `viewer` is read-only and cannot. Mirrors the backend's
   * `PermCreateWorkNote`, whose handler narrows a non-write caller to
   * `type=work_note` only. Callers that offer a composer to someone with this
   * but not {@link canWrite} must lock it to internal notes.
   */
  canAddWorkNotes: boolean;
  /**
   * Creating a new platform user. Unlike every other flag here, this is
   * `admin` only — `cs_engineer` does not hold it, mirroring the
   * backend's `PermAdmin` (the one permission `cs_engineer` does not
   * share with `admin`).
   */
  canCreateUser: boolean;
  /**
   * Directly overriding a project's overall closure state (OPEN/READ_ONLY/
   * CLOSED/RESTRICTED/SUSPENDED) via `PATCH /projects/{id}`'s `closureState`
   * field. `admin` only, same reasoning as {@link canCreateUser} — mirrors
   * the backend's in-handler `PermAdmin` narrowing on that one field (see
   * `apps/csm-portal/backend`'s own CLAUDE.md,
   * "PATCH /projects/{id}: closureState requires PermAdmin").
   */
  canUpdateProjectClosureState: boolean;
  /**
   * Setting a project's onboarding status and onboarding owner via
   * `PATCH /projects/{id}`'s `onboardingStatus`/`onboardingOwner` fields.
   * `onboarding_lead` or `admin` -- `cs_engineer` alone does not hold it.
   * Mirrors the backend's in-handler `PermManageOnboarding` check on those
   * two fields (see `apps/csm-portal/backend`'s own CLAUDE.md,
   * "PermManageOnboarding is checked in-handler on PATCH /projects/{id}").
   * The backend's 403 is the real gate; this only decides whether the
   * controls are offered.
   */
  canEditProjectOnboarding: boolean;
  /**
   * The Security Center section (Security reports + Vulnerabilities tabs)
   * and the API calls behind it. `admin`, `cs_engineer`, and the dedicated
   * `security_team` role (read-only) — mirrors the backend's
   * `PermViewSecurityCenter`, which (unlike `PermView`) plain
   * viewer/escalator/attachment_downloader/usage_metrics_viewer/
   * timecard_approver/dashboard_designer do not hold.
   */
  canUseSecurityCenter: boolean;
  /**
   * The PLG Customer Success Portal section. `admin` and `cs_engineer` only —
   * mirrors the backend's `PermUsePlg`, which (unlike `PermView`) the view-only
   * roles do not hold. PLG is a worklist staff act on, so a role that could open
   * it but not use it would meet a 403 on every control.
   */
  canUsePlg: boolean;
  /**
   * Authoring a PLG playbook template: creating one, editing it, changing its
   * tasks, deleting it. `admin` only, mirroring the backend's
   * `PermManagePlaybooks`.
   *
   * A `cs_engineer` holds `canUsePlg` without this: they browse templates and
   * assign them to a pairing, but the Manage Playbooks page renders read-only
   * for them.
   */
  canManagePlaybooks: boolean;
  /**
   * Creating and sending a customer announcement: the New announcement page and
   * button, and every action that edits, submits, schedules, publishes or
   * updates a request. `admin`, or a caller who has write access
   * ({@link canWrite}) AND the `announcement_creator` role -- mirroring the
   * backend's `PermCreateAnnouncement`, which is checked on top of `PermWrite`
   * rather than instead of it. A `cs_engineer` without the role does not get
   * it (that is the point of the role), and the role on its own does not make
   * anyone a writer. Marking a request as approved is not gated by this: it
   * stays under {@link canWrite}.
   */
  canCreateAnnouncement: boolean;
  /**
   * Editing or deleting a comment the caller did NOT author. `admin`, or the
   * `comment_updater` role — mirrors the backend's `PermUpdateDeleteAnyComment`
   * (not `PermUpdateDeleteComment`, the broader route-level floor that also
   * includes `cs_engineer`). Deliberately NOT `full`/{@link canWrite}: a plain
   * `cs_engineer` keeps editing/deleting only their OWN comments (the existing
   * author check in `CsmCaseCommentBubble`, unaffected by this flag) — this
   * flag is only for touching someone else's. The backend makes the same
   * author-or-this-role decision itself before ever calling entity-service
   * (which performs no author/role check of its own for this path at all), so
   * a holder of this role can act on any comment end-to-end, not just reach
   * the route — see the backend's own CLAUDE.md.
   */
  canUpdateDeleteAnyComment: boolean;
  /**
   * The small set of sections that used to live in the separate, now-removed
   * "Support Portal Lite" app (the Time project report and the
   * extra Account detail tabs they added): `viewer` only, regardless of what
   * other roles the caller also holds — mirrors the backend's
   * `PermViewerAccess` (`access.go`), which is built only from the viewer
   * role, not folded into the usual "full access" `cs_engineer`/`admin`
   * bundle the way every other flag above is. A `cs_engineer` who does not
   * also separately hold `viewer` does not get these sections; one who holds
   * both does, same as before this app was merged into the main portal.
   * Customer Health used to be in this set and now has its own, wider flag
   * ({@link canViewCustomerHealth}).
   */
  isSplAudience: boolean;
  /**
   * The Customer Health dashboard (list, account detail, risk and action
   * items): `viewer`, the audience it was built for, plus `cs_engineer` and
   * `admin` (full access), who work the same accounts. Unlike
   * {@link isSplAudience} it does not require `viewer` — a `cs_engineer` or
   * `admin` who does not also hold it still sees the dashboard. Mirrors the
   * backend's `PermViewCustomerHealth` (`access.go`).
   */
  canViewCustomerHealth: boolean;
  /**
   * Where the sidebar offers Customer Health, not whether it is offered: true
   * for a `viewer` who holds neither `cs_engineer` nor `admin`. For them the
   * Customers entry opens as a dropdown (Accounts, Projects, Customer Health)
   * and the separate top-level Customer Health entry is dropped; everyone else
   * who can see the dashboard ({@link canViewCustomerHealth}) keeps the
   * top-level entry. A `cs_engineer` or `admin` who also holds `viewer` counts
   * as a CS engineer here. The page, its route and the backend permission are
   * the same either way.
   */
  showCustomerHealthUnderCustomers: boolean;
  /**
   * Usage metrics requires `cs_engineer`/`admin` (full access) or the
   * dedicated `usage_metrics_viewer` role. Unlike {@link isSplAudience},
   * holding plain `viewer` does NOT grant this on its own — reported live:
   * a viewer-only account must not see this section at all, even though it
   * is an ex-Support-Portal-Lite feature.
   */
  canViewUsageMetrics: boolean;
  /**
   * Sections that exist for staff generally, not for the former Support
   * Portal Lite (viewer-only) audience specifically — Knowledge and Settings
   * so far. False only for a caller whose role set is *exactly* `{viewer}`
   * (no other role at all) — everyone else, including a plain `cs_engineer`,
   * keeps seeing these sections exactly as before. A viewer who also holds
   * any other role is unaffected by this flag.
   *
   * Team Schedule and the project Work items tab's staff framing are each
   * gated by their OWN, narrower flag below ({@link canViewTeamSchedule},
   * {@link canViewWorkItemsStaffView}) rather than this one — reported live:
   * a caller holding `viewer` plus one unrelated role (so not caught by this
   * flag's exactly-`{viewer}` check) could still reach both, which is wider
   * than intended for either.
   */
  canViewStaffSections: boolean;
  /**
   * The Team Schedule section. `cs_engineer`/`admin` (full access), or the
   * `comment_updater` role — an explicit allow-list, independent of whether
   * the caller holds `viewer` at all (unlike {@link canViewStaffSections}):
   * reported live, a `viewer` who also held `attachment_downloader` could
   * still see Team Schedule under the old exactly-`{viewer}` check.
   */
  canViewTeamSchedule: boolean;
  /**
   * The Settings section (`/admin`: user management, roles, groups, teams,
   * the dashboard builder). `cs_engineer`/`admin` only — an explicit
   * allow-list, independent of {@link canViewStaffSections} (which also
   * gates Settings' nav entry today but is intentionally broader/shared
   * with Knowledge): Settings holds user/role/group/team administration,
   * which a plain `viewer`/`escalator`/`attachment_downloader`/
   * `usage_metrics_viewer`/`timecard_approver`/`dashboard_designer` must
   * not reach just by also holding one unrelated role.
   */
  canViewSettings: boolean;
  /**
   * Whether a project's Work items tab shows its staff framing: the "Work
   * items" label (vs. plain "Cases"), the Chats sub-tab, and the sub-tab
   * strip itself. `cs_engineer`/`admin` (full access), or the
   * `timecard_approver` role — an explicit allow-list, independent of
   * `viewer`, the same shape as {@link canViewTeamSchedule} and for the same
   * reason: a caller holding `viewer` plus one unrelated role used to still
   * see this under the old exactly-`{viewer}` check.
   */
  canViewWorkItemsStaffView: boolean;
  /**
   * Holds the `admin` role specifically — the one capability gate in this
   * type keyed on the role itself rather than a derived permission, needed by
   * the feature-preview toggle (see `featurePreview.ts`) which is
   * deliberately admin-only regardless of any other capability a caller
   * might hold.
   */
  isAdmin: boolean;
  /**
   * The SLAs tab on a case's detail page. `admin`, `cs_engineer`, or the
   * dedicated `security_team` role — every other role, `viewer` included,
   * does not see this tab at all.
   */
  canViewSlaTab: boolean;
}

/**
 * What a user's `GET /users/me` roles let them see and do. Matched
 * case-insensitively. `admin` can do everything; `cs_engineer` can do
 * everything except admin-only actions, escalating included (see
 * `canEscalate`'s own doc comment) — approving a time card is a dedicated
 * responsibility, but it
 * isn't a flag on this type at all, see `canUseTimeCardsAndUpdates`'s own
 * doc comment for why; `attachment_downloader` adds just that one ability;
 * `worknote_creator` also adds internal work notes (see `canAddWorkNotes`);
 * `comment_updater` adds editing/deleting someone else's comment (see
 * `canUpdateDeleteAnyComment`); every other role, `viewer` included, is
 * view-only here.
 *
 * Mirrors the backend's `AccessGuard` policy so controls can be hidden up
 * front — but it is a UX affordance only. The backend's 403 is the real gate,
 * so if the two ever disagree the backend wins. `undefined` roles (profile not
 * loaded, or the request failed) grant nothing, so controls fail closed.
 */
export function getPortalAccess(roles: string[] | undefined): PortalAccess {
  // TEMPORARY / LOCAL DEV ONLY — see authConfig.ts's devBypassAccessCheck.
  // Grants every capability regardless of the real `roles` claim, so a local
  // account the staging backend hasn't provisioned a portal role for yet can
  // still see every nav section/action during development.
  if (devBypassAccessCheck) {
    return {
      hasAnyRole: true,
      canEscalate: true,
      canDownloadAttachment: true,
      canUseOperations: true,
      canUseTimeCardsAndUpdates: true,
      canWrite: true,
      canAddWorkNotes: true,
      canCreateUser: true,
      canUpdateProjectClosureState: true,
      canEditProjectOnboarding: true,
      canUseSecurityCenter: true,
      canUsePlg: true,
      canManagePlaybooks: true,
      canCreateAnnouncement: true,
      canUpdateDeleteAnyComment: true,
      isSplAudience: true,
      canViewCustomerHealth: true,
      // Placement, not a capability: the bypass grants full access, and full
      // access keeps the top-level entry.
      showCustomerHealthUnderCustomers: false,
      canViewUsageMetrics: true,
      canViewStaffSections: true,
      canViewTeamSchedule: true,
      canViewSettings: true,
      canViewWorkItemsStaffView: true,
      isAdmin: true,
      canViewSlaTab: true,
    };
  }
  const held = new Set((roles ?? []).map((r) => r.toLowerCase()));
  const has = (role: string): boolean => held.has(role);
  const isAdmin = has(PORTAL_ROLE.admin);
  const full = isAdmin || has(PORTAL_ROLE.csEngineer);
  return {
    hasAnyRole: ROLES_THAT_GRANT_ACCESS.some(has),
    // Escalating only; de-escalating is gated on the case's team lead
    // (CsmCaseDetailPage), not on this flag.
    canEscalate: full || has(PORTAL_ROLE.escalator),
    canDownloadAttachment: full || has(PORTAL_ROLE.attachmentDownloader),
    canUseOperations: full || has(PORTAL_ROLE.securityTeam),
    canUseTimeCardsAndUpdates: full || has(PORTAL_ROLE.timecardApprover),
    canWrite: full,
    canAddWorkNotes: full || has(PORTAL_ROLE.worknoteCreator),
    canCreateUser: isAdmin,
    canUpdateProjectClosureState: isAdmin,
    canEditProjectOnboarding: isAdmin || has(PORTAL_ROLE.onboardingLead),
    canUseSecurityCenter: full || has(PORTAL_ROLE.securityTeam),
    canUsePlg: full,
    canManagePlaybooks: isAdmin,
    canCreateAnnouncement: full && (isAdmin || has(PORTAL_ROLE.announcementCreator)),
    canUpdateDeleteAnyComment: isAdmin || has(PORTAL_ROLE.commentUpdater),
    isSplAudience: has(PORTAL_ROLE.viewer),
    canViewCustomerHealth: full || has(PORTAL_ROLE.viewer),
    showCustomerHealthUnderCustomers: has(PORTAL_ROLE.viewer) && !full,
    canViewUsageMetrics: full || has(PORTAL_ROLE.usageMetricsViewer),
    canViewStaffSections: !(held.size === 1 && has(PORTAL_ROLE.viewer)),
    canViewTeamSchedule: full || has(PORTAL_ROLE.commentUpdater),
    canViewSettings: full,
    canViewWorkItemsStaffView: full || has(PORTAL_ROLE.timecardApprover),
    isAdmin,
    canViewSlaTab: full || has(PORTAL_ROLE.securityTeam),
  };
}
