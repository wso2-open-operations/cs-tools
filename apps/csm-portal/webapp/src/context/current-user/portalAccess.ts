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
} as const;

// Roles that, held alone, let someone use the portal at all. announcement_creator
// is left out on purpose: it only narrows who among the people who can already
// write may send an announcement (see canCreateAnnouncement), so on its own it
// grants nothing, and counting it would let a caller holding nothing else in
// past the "no access" screen into a portal where every call is a 403.
const ROLES_THAT_GRANT_ACCESS: readonly string[] = Object.values(PORTAL_ROLE).filter(
  (role) => role !== PORTAL_ROLE.announcementCreator,
);

export interface PortalAccess {
  /** Holds at least one portal role — the minimum to use the portal at all. */
  hasAnyRole: boolean;
  /**
   * Escalating or de-escalating a case: `admin`, `cs_engineer` and
   * `escalator`, mirroring the backend's `PermEscalate` -- any internal
   * engineer may escalate, as in ServiceNow. De-escalating also requires
   * being one of the case's ABT team leads.
   */
  canEscalate: boolean;
  canDownloadAttachment: boolean;
  /**
   * The Operations section (incidents, change requests, problems, outages,
   * service requests). Support-portal-lite had no such area, so its
   * view-oriented roles never saw one: only full-access roles do here.
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
   * The Security Center section (Security reports + Vulnerabilities tabs)
   * and the API calls behind it. `admin` and `cs_engineer` only — mirrors
   * the backend's `PermViewSecurityCenter`, which (unlike `PermView`) plain
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
 * every other role, `viewer` included, is view-only here.
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
      canUseSecurityCenter: true,
      canUsePlg: true,
      canManagePlaybooks: true,
      canCreateAnnouncement: true,
    };
  }
  const held = new Set((roles ?? []).map((r) => r.toLowerCase()));
  const has = (role: string): boolean => held.has(role);
  const isAdmin = has(PORTAL_ROLE.admin);
  const full = isAdmin || has(PORTAL_ROLE.csEngineer);
  return {
    hasAnyRole: ROLES_THAT_GRANT_ACCESS.some(has),
    // Any internal engineer may escalate, as in ServiceNow; de-escalating is
    // further limited to the case's ABT team leads (CsmCaseDetailPage).
    canEscalate: full || has(PORTAL_ROLE.escalator),
    canDownloadAttachment: full || has(PORTAL_ROLE.attachmentDownloader),
    canUseOperations: full,
    canUseTimeCardsAndUpdates: full || has(PORTAL_ROLE.timecardApprover),
    canWrite: full,
    canAddWorkNotes: full || has(PORTAL_ROLE.worknoteCreator),
    canCreateUser: isAdmin,
    canUseSecurityCenter: full,
    canUsePlg: full,
    canManagePlaybooks: isAdmin,
    canCreateAnnouncement: full && (isAdmin || has(PORTAL_ROLE.announcementCreator)),
  };
}
