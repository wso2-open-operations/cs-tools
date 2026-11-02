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
import { describe, expect, it } from "vitest";
import { getPortalAccess } from "@context/current-user/portalAccess";

const NONE = {
  hasAnyRole: false,
  canEscalate: false,
  canDownloadAttachment: false,
  canUseOperations: false,
  canUseTimeCardsAndUpdates: false,
  canWrite: false,
  canAddWorkNotes: false,
  canCreateUser: false,
  canUpdateProjectClosureState: false,
  canEditProjectOnboarding: false,
  canUseSecurityCenter: false,
  canUsePlg: false,
  canManagePlaybooks: false,
  canCreateAnnouncement: false,
  canUpdateDeleteAnyComment: false,
  isSplAudience: false,
  canViewCustomerHealth: false,
  showCustomerHealthUnderCustomers: false,
  canViewUsageMetrics: false,
  // NONE represents "no roles at all" everywhere it's used directly, and a
  // caller with no roles can't reach the portal to begin with -- but every
  // single-role test below spreads NONE then overrides just its own role's
  // flags, so this must default true (role set is NOT exactly {viewer}) for
  // every one of those single-other-role cases to stay correct unchanged.
  canViewStaffSections: true,
  // Unlike canViewStaffSections, these three are explicit allow-lists (false
  // by default, true only for a qualifying role) -- see each flag's own doc
  // comment.
  canViewTeamSchedule: false,
  canViewSettings: false,
  canViewWorkItemsStaffView: false,
  isAdmin: false,
  canViewSlaTab: false,
};

describe("getPortalAccess", () => {
  it("grants nothing when roles are missing or empty", () => {
    expect(getPortalAccess(undefined)).toEqual(NONE);
    expect(getPortalAccess([])).toEqual(NONE);
  });

  it("ignores roles the portal does not know", () => {
    expect(getPortalAccess(["internal", "customer", "agent"])).toEqual(NONE);
  });

  it("viewer can use the portal and read, but do nothing else (no work notes)", () => {
    expect(getPortalAccess(["viewer"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      isSplAudience: true,
      canViewCustomerHealth: true,
      // A viewer who is not also a CS engineer/admin finds Customer Health
      // under Customers in the sidebar.
      showCustomerHealthUnderCustomers: true,
      // A viewer-only role set (exactly {viewer}) is the one case this flag
      // is false for -- see the dedicated canViewStaffSections describe block.
      canViewStaffSections: false,
    });
  });

  it("worknote_creator adds internal work notes and nothing else", () => {
    expect(getPortalAccess(["worknote_creator"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canAddWorkNotes: true,
    });
  });

  it("work notes are not a write: only full-write roles can do both", () => {
    expect(getPortalAccess(["cs_engineer"])).toMatchObject({ canWrite: true, canAddWorkNotes: true });
    expect(getPortalAccess(["admin"])).toMatchObject({ canWrite: true, canAddWorkNotes: true });
    for (const role of ["viewer", "escalator", "attachment_downloader", "usage_metrics_viewer", "timecard_approver", "dashboard_designer"]) {
      expect(getPortalAccess([role]).canAddWorkNotes).toBe(false);
    }
  });

  // The role set a viewer holds in practice: viewer reads, worknote_creator
  // is the only role that adds a comment (an internal work note), and none of
  // them is a write.
  it("a viewer's role set can add internal work notes only through worknote_creator", () => {
    const readers = ["viewer", "escalator", "attachment_downloader", "usage_metrics_viewer", "timecard_approver"];
    expect(getPortalAccess(readers)).toMatchObject({ canWrite: false, canAddWorkNotes: false });
    expect(getPortalAccess([...readers, "worknote_creator"])).toMatchObject({
      canWrite: false,
      canAddWorkNotes: true,
      canEscalate: true,
      canDownloadAttachment: true,
      canUseOperations: false,
      canUseSecurityCenter: false,
    });
  });

  it("each specialised role adds only its own ability", () => {
    expect(getPortalAccess(["escalator"])).toEqual({ ...NONE, hasAnyRole: true, canEscalate: true });
    expect(getPortalAccess(["attachment_downloader"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canDownloadAttachment: true,
    });
  });

  it("the feature roles are view-only for these controls", () => {
    expect(getPortalAccess(["dashboard_designer"])).toEqual({ ...NONE, hasAnyRole: true });
    // usage_metrics_viewer is view-only for everything EXCEPT the one section
    // it names -- see the dedicated canViewUsageMetrics describe block below.
    expect(getPortalAccess(["usage_metrics_viewer"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canViewUsageMetrics: true,
    });
  });

  it("the time-card approver also gets Time cards and Updates and the Work items staff view, and nothing else", () => {
    expect(getPortalAccess(["timecard_approver"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canUseTimeCardsAndUpdates: true,
      canViewWorkItemsStaffView: true,
    });
  });

  it("CS engineer and admin can do everything except admin can also create users, escalate and manage playbooks", () => {
    const all = {
      hasAnyRole: true,
      canDownloadAttachment: true,
      canUseOperations: true,
      canUseTimeCardsAndUpdates: true,
      canWrite: true,
      canAddWorkNotes: true,
      canUseSecurityCenter: true,
      canUsePlg: true,
      // Full access grants canViewCustomerHealth and canViewUsageMetrics too,
      // but neither role holds the viewer role itself, so isSplAudience stays
      // false for both.
      isSplAudience: false,
      canViewCustomerHealth: true,
      // CS engineers and admins keep the top-level Customer Health entry.
      showCustomerHealthUnderCustomers: false,
      canViewUsageMetrics: true,
      // Role set is {cs_engineer}/{admin}, not exactly {viewer}.
      canViewStaffSections: true,
      // Both are full access -- qualify for all three allow-list flags too.
      canViewTeamSchedule: true,
      canViewSettings: true,
      canViewWorkItemsStaffView: true,
      canViewSlaTab: true,
    };
    // Both hold canEscalate (any internal engineer may escalate, as in
    // ServiceNow). canManagePlaybooks is the further exception beyond
    // canCreateUser: both roles hold canUsePlg, only admin may author a
    // playbook template. canCreateAnnouncement is the one write a plain CS
    // engineer no longer holds: sending an announcement needs the
    // announcement_creator role (admin always has it). isAdmin is the one
    // flag keyed on the role itself, not a derived permission -- cs_engineer
    // never holds it even though it shares every other capability with admin.
    expect(getPortalAccess(["cs_engineer"])).toEqual({
      ...all,
      canCreateUser: false,
      canUpdateProjectClosureState: false,
      canEditProjectOnboarding: false,
      canEscalate: true,
      canManagePlaybooks: false,
      canCreateAnnouncement: false,
      canUpdateDeleteAnyComment: false,
      isAdmin: false,
    });
    expect(getPortalAccess(["admin"])).toEqual({
      ...all,
      canCreateUser: true,
      canUpdateProjectClosureState: true,
      canEditProjectOnboarding: true,
      canEscalate: true,
      canManagePlaybooks: true,
      canCreateAnnouncement: true,
      canUpdateDeleteAnyComment: true,
      isAdmin: true,
    });
  });

  it("PLG is narrower than view -- the view-only roles are shut out entirely", () => {
    for (const role of [
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canUsePlg).toBe(false);
      expect(getPortalAccess([role]).canManagePlaybooks).toBe(false);
    }
    expect(getPortalAccess(["cs_engineer"]).canUsePlg).toBe(true);
    expect(getPortalAccess(["admin"]).canUsePlg).toBe(true);
  });

  it("only admin can manage playbooks -- CS engineer reads them but cannot author one", () => {
    expect(getPortalAccess(["admin"]).canManagePlaybooks).toBe(true);
    expect(getPortalAccess(["cs_engineer"]).canManagePlaybooks).toBe(false);
    // ...and losing that does not cost them PLG itself.
    expect(getPortalAccess(["cs_engineer"]).canUsePlg).toBe(true);
  });

  it("only admin can create a user -- CS engineer does not share this one", () => {
    expect(getPortalAccess(["admin"]).canCreateUser).toBe(true);
    for (const role of [
      "cs_engineer",
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canCreateUser).toBe(false);
    }
  });

  it("only admin can directly override a project's closure state -- CS engineer does not share this one", () => {
    expect(getPortalAccess(["admin"]).canUpdateProjectClosureState).toBe(true);
    for (const role of [
      "cs_engineer",
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canUpdateProjectClosureState).toBe(false);
    }
  });

  it("only onboarding_lead and admin can edit a project's onboarding fields -- CS engineer alone cannot", () => {
    expect(getPortalAccess(["onboarding_lead"]).canEditProjectOnboarding).toBe(true);
    expect(getPortalAccess(["admin"]).canEditProjectOnboarding).toBe(true);
    // The realistic holder: an engineer who also carries the lead role.
    expect(
      getPortalAccess(["cs_engineer", "onboarding_lead"]).canEditProjectOnboarding,
    ).toBe(true);
    for (const role of [
      "cs_engineer",
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canEditProjectOnboarding).toBe(false);
    }
    expect(getPortalAccess(undefined).canEditProjectOnboarding).toBe(false);
  });

  it("onboarding_lead alone grants no portal access, only the onboarding edit", () => {
    expect(getPortalAccess(["onboarding_lead"]).hasAnyRole).toBe(false);
  });

  it("admin, CS engineer and escalator can escalate -- any internal engineer, as in ServiceNow", () => {
    expect(getPortalAccess(["admin"]).canEscalate).toBe(true);
    expect(getPortalAccess(["cs_engineer"]).canEscalate).toBe(true);
    expect(getPortalAccess(["escalator"]).canEscalate).toBe(true);
    for (const role of [
      "viewer",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canEscalate).toBe(false);
    }
  });

  it("only admin, CS engineer and security_team can use Security Center", () => {
    expect(getPortalAccess(["admin"]).canUseSecurityCenter).toBe(true);
    expect(getPortalAccess(["cs_engineer"]).canUseSecurityCenter).toBe(true);
    expect(getPortalAccess(["security_team"]).canUseSecurityCenter).toBe(true);
    for (const role of [
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canUseSecurityCenter).toBe(false);
    }
  });

  it("only CS engineer, admin and the time-card approver get Time cards and Updates", () => {
    for (const role of [
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canUseTimeCardsAndUpdates).toBe(false);
    }
    expect(getPortalAccess(["viewer", "escalator"]).canUseTimeCardsAndUpdates).toBe(false);
    expect(getPortalAccess(["viewer", "timecard_approver"]).canUseTimeCardsAndUpdates).toBe(true);
    expect(getPortalAccess(["cs_engineer"]).canUseTimeCardsAndUpdates).toBe(true);
    expect(getPortalAccess(["admin"]).canUseTimeCardsAndUpdates).toBe(true);
  });

  it("only full-access roles and security_team get the Operations section", () => {
    for (const role of [
      "viewer",
      "escalator",
      "attachment_downloader",
      "usage_metrics_viewer",
      "timecard_approver",
      "dashboard_designer",
    ]) {
      expect(getPortalAccess([role]).canUseOperations).toBe(false);
    }
    expect(getPortalAccess(["viewer", "escalator", "attachment_downloader"]).canUseOperations).toBe(false);
    expect(getPortalAccess(["cs_engineer"]).canUseOperations).toBe(true);
    expect(getPortalAccess(["admin"]).canUseOperations).toBe(true);
    expect(getPortalAccess(["security_team"]).canUseOperations).toBe(true);
  });

  it("a user holding several roles gets the union", () => {
    expect(getPortalAccess(["viewer", "escalator", "attachment_downloader"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canEscalate: true,
      canDownloadAttachment: true,
      isSplAudience: true,
      canViewCustomerHealth: true,
      showCustomerHealthUnderCustomers: true,
    });
  });

  it("matches role keys case-insensitively", () => {
    expect(getPortalAccess(["CS_Engineer"]).canWrite).toBe(true);
  });

  // Mirrors the backend's PermCreateAnnouncement, which is checked on top of
  // PermWrite: the role narrows who may send an announcement, it does not make
  // anyone a writer.
  describe("canCreateAnnouncement", () => {
    it("a CS engineer without the announcement_creator role cannot create an announcement but still writes", () => {
      expect(getPortalAccess(["cs_engineer"])).toMatchObject({ canWrite: true, canCreateAnnouncement: false });
    });

    it("a CS engineer who also holds announcement_creator can", () => {
      expect(getPortalAccess(["cs_engineer", "announcement_creator"])).toMatchObject({
        canWrite: true,
        canCreateAnnouncement: true,
      });
    });

    it("admin can without the role, like every other capability", () => {
      expect(getPortalAccess(["admin"]).canCreateAnnouncement).toBe(true);
    });

    it("the role on its own grants nothing at all, not even entry past the no-access screen", () => {
      expect(getPortalAccess(["announcement_creator"])).toEqual(NONE);
    });

    it("alongside any role that does grant access, hasAnyRole is true as before", () => {
      expect(getPortalAccess(["viewer", "announcement_creator"]).hasAnyRole).toBe(true);
    });

    it("no role other than admin and cs_engineer + announcement_creator can create one", () => {
      for (const role of ["viewer", "escalator", "attachment_downloader", "usage_metrics_viewer", "timecard_approver", "dashboard_designer", "worknote_creator", "sales_solutions"]) {
        expect(getPortalAccess([role, "announcement_creator"]).canCreateAnnouncement).toBe(false);
        expect(getPortalAccess([role]).canCreateAnnouncement).toBe(false);
      }
    });

    it("matches the role key case-insensitively, like every other role", () => {
      expect(getPortalAccess(["CS_Engineer", "Announcement_Creator"]).canCreateAnnouncement).toBe(true);
    });

    it("is false while roles are not loaded, so the controls fail closed", () => {
      expect(getPortalAccess(undefined).canCreateAnnouncement).toBe(false);
    });
  });

  // Mirrors the backend's PermUpdateDeleteComment: unlike announcement_creator,
  // this role grants the ability on its own -- it does not also require
  // cs_engineer/admin's write access, since editing/deleting a comment the
  // caller didn't author is the one thing this role is for.
  describe("canUpdateDeleteAnyComment", () => {
    it("a CS engineer without comment_updater cannot touch someone else's comment, but still writes", () => {
      expect(getPortalAccess(["cs_engineer"])).toMatchObject({ canWrite: true, canUpdateDeleteAnyComment: false });
    });

    it("a CS engineer who also holds comment_updater can", () => {
      expect(getPortalAccess(["cs_engineer", "comment_updater"])).toMatchObject({
        canWrite: true,
        canUpdateDeleteAnyComment: true,
      });
    });

    it("admin can without the role, like every other capability", () => {
      expect(getPortalAccess(["admin"]).canUpdateDeleteAnyComment).toBe(true);
    });

    it("the role on its own grants the ability and Team Schedule, but nothing else, not even entry past the no-access screen", () => {
      expect(getPortalAccess(["comment_updater"])).toEqual({
        ...NONE,
        canUpdateDeleteAnyComment: true,
        // comment_updater is also one of canViewTeamSchedule's allow-listed
        // roles -- see that flag's own doc comment.
        canViewTeamSchedule: true,
      });
    });

    it("alongside any role that does grant access, hasAnyRole is true as before", () => {
      expect(getPortalAccess(["viewer", "comment_updater"]).hasAnyRole).toBe(true);
    });

    it("no role other than admin or comment_updater grants it", () => {
      for (const role of ["viewer", "escalator", "attachment_downloader", "usage_metrics_viewer", "timecard_approver", "dashboard_designer", "worknote_creator", "sales_solutions"]) {
        expect(getPortalAccess([role]).canUpdateDeleteAnyComment).toBe(false);
      }
    });

    it("matches the role key case-insensitively, like every other role", () => {
      expect(getPortalAccess(["Comment_Updater"]).canUpdateDeleteAnyComment).toBe(true);
    });

    it("is false while roles are not loaded, so the controls fail closed", () => {
      expect(getPortalAccess(undefined).canUpdateDeleteAnyComment).toBe(false);
    });
  });

  // Mirrors the backend's PermViewerAccess (access.go), which is built only
  // from the viewer role -- unlike every other flag above, holding
  // cs_engineer/admin does NOT grant this on its own.
  describe("isSplAudience", () => {
    it("viewer holds it alone", () => {
      expect(getPortalAccess(["viewer"]).isSplAudience).toBe(true);
    });

    it("cs_engineer and admin do not hold it unless they also separately hold viewer", () => {
      expect(getPortalAccess(["cs_engineer"]).isSplAudience).toBe(false);
      expect(getPortalAccess(["admin"]).isSplAudience).toBe(false);
      expect(getPortalAccess(["cs_engineer", "viewer"]).isSplAudience).toBe(true);
      expect(getPortalAccess(["admin", "viewer"]).isSplAudience).toBe(true);
    });

    it("no other role grants it on its own", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
      ]) {
        expect(getPortalAccess([role]).isSplAudience).toBe(false);
      }
    });
  });

  // Mirrors the backend's PermViewCustomerHealth (access.go): the viewer role
  // (the audience the dashboard was built for) plus cs_engineer and admin,
  // who do not need to hold viewer as well.
  describe("canViewCustomerHealth", () => {
    it("viewer, cs_engineer and admin each hold it on their own", () => {
      expect(getPortalAccess(["viewer"]).canViewCustomerHealth).toBe(true);
      expect(getPortalAccess(["cs_engineer"]).canViewCustomerHealth).toBe(true);
      expect(getPortalAccess(["admin"]).canViewCustomerHealth).toBe(true);
    });

    it("holding cs_engineer or admin alongside viewer still grants it", () => {
      expect(getPortalAccess(["cs_engineer", "viewer"]).canViewCustomerHealth).toBe(true);
      expect(getPortalAccess(["admin", "viewer"]).canViewCustomerHealth).toBe(true);
    });

    it("is wider than isSplAudience: a cs_engineer sees it without the other viewer-only sections", () => {
      expect(getPortalAccess(["cs_engineer"])).toMatchObject({
        canViewCustomerHealth: true,
        isSplAudience: false,
      });
    });

    it("no other role grants it on its own", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
        "comment_updater",
        "security_team",
      ]) {
        expect(getPortalAccess([role]).canViewCustomerHealth).toBe(false);
      }
    });
  });

  // Placement only: a viewer who is not a CS engineer/admin gets Customer
  // Health under the Customers dropdown; everyone else who sees it keeps the
  // top-level entry. See the flag's own doc comment.
  describe("showCustomerHealthUnderCustomers", () => {
    it("is true for a viewer, alone or with other non-staff roles", () => {
      expect(getPortalAccess(["viewer"]).showCustomerHealthUnderCustomers).toBe(true);
      expect(getPortalAccess(["viewer", "escalator"]).showCustomerHealthUnderCustomers).toBe(true);
      expect(getPortalAccess(["viewer", "usage_metrics_viewer"]).showCustomerHealthUnderCustomers).toBe(true);
      expect(getPortalAccess(["viewer", "worknote_creator"]).showCustomerHealthUnderCustomers).toBe(true);
    });

    it("is false for a CS engineer or admin, even one who also holds viewer", () => {
      for (const roles of [["cs_engineer"], ["admin"], ["cs_engineer", "viewer"], ["admin", "viewer"]]) {
        expect(getPortalAccess(roles).showCustomerHealthUnderCustomers).toBe(false);
      }
    });

    it("is false for every role that cannot see Customer Health at all", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
        "comment_updater",
        "security_team",
      ]) {
        expect(getPortalAccess([role]).showCustomerHealthUnderCustomers).toBe(false);
      }
      expect(getPortalAccess(undefined).showCustomerHealthUnderCustomers).toBe(false);
    });
  });

  // Requires full access or the dedicated usage_metrics_viewer role.
  // Reported live: a plain viewer-only account must NOT see this section,
  // unlike the isSplAudience-gated sections -- holding viewer grants nothing
  // here on its own.
  describe("canViewUsageMetrics", () => {
    it("usage_metrics_viewer, cs_engineer and admin all hold it", () => {
      expect(getPortalAccess(["usage_metrics_viewer"]).canViewUsageMetrics).toBe(true);
      expect(getPortalAccess(["cs_engineer"]).canViewUsageMetrics).toBe(true);
      expect(getPortalAccess(["admin"]).canViewUsageMetrics).toBe(true);
    });

    it("a plain viewer does not hold it, even combined with other non-qualifying roles", () => {
      expect(getPortalAccess(["viewer"]).canViewUsageMetrics).toBe(false);
      expect(getPortalAccess(["viewer", "escalator"]).canViewUsageMetrics).toBe(false);
    });

    it("a viewer who also holds usage_metrics_viewer or full access does hold it", () => {
      expect(getPortalAccess(["viewer", "usage_metrics_viewer"]).canViewUsageMetrics).toBe(true);
      expect(getPortalAccess(["viewer", "cs_engineer"]).canViewUsageMetrics).toBe(true);
    });

    it("no other role grants it", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
      ]) {
        expect(getPortalAccess([role]).canViewUsageMetrics).toBe(false);
      }
    });
  });

  describe("canViewStaffSections", () => {
    it("is false only when the role set is exactly {viewer}", () => {
      expect(getPortalAccess(["viewer"]).canViewStaffSections).toBe(false);
    });

    it("stays true for a viewer who also holds any other role", () => {
      expect(getPortalAccess(["viewer", "cs_engineer"]).canViewStaffSections).toBe(true);
      expect(getPortalAccess(["viewer", "escalator"]).canViewStaffSections).toBe(true);
      expect(getPortalAccess(["viewer", "attachment_downloader"]).canViewStaffSections).toBe(true);
    });

    it("is true for every non-viewer role on its own", () => {
      for (const role of [
        "cs_engineer",
        "admin",
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
      ]) {
        expect(getPortalAccess([role]).canViewStaffSections).toBe(true);
      }
    });

    it("is true when no roles are held at all (nothing to specifically hide it from)", () => {
      expect(getPortalAccess(undefined).canViewStaffSections).toBe(true);
      expect(getPortalAccess([]).canViewStaffSections).toBe(true);
    });
  });

  // An explicit allow-list, independent of viewer -- reported live: a viewer
  // who also held attachment_downloader was not exactly {viewer}, so the old
  // canViewStaffSections check let Team Schedule through for them too.
  describe("canViewTeamSchedule", () => {
    it("cs_engineer, admin and comment_updater each hold it on their own", () => {
      expect(getPortalAccess(["cs_engineer"]).canViewTeamSchedule).toBe(true);
      expect(getPortalAccess(["admin"]).canViewTeamSchedule).toBe(true);
      expect(getPortalAccess(["comment_updater"]).canViewTeamSchedule).toBe(true);
    });

    it("a viewer combined with an unrelated role does not qualify", () => {
      expect(getPortalAccess(["viewer"]).canViewTeamSchedule).toBe(false);
      expect(getPortalAccess(["viewer", "attachment_downloader"]).canViewTeamSchedule).toBe(false);
      expect(getPortalAccess(["viewer", "escalator"]).canViewTeamSchedule).toBe(false);
    });

    it("a viewer who also holds a qualifying role does qualify", () => {
      expect(getPortalAccess(["viewer", "cs_engineer"]).canViewTeamSchedule).toBe(true);
      expect(getPortalAccess(["viewer", "comment_updater"]).canViewTeamSchedule).toBe(true);
    });

    it("no other role grants it", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
      ]) {
        expect(getPortalAccess([role]).canViewTeamSchedule).toBe(false);
      }
    });

    it("is false while roles are not loaded, so the controls fail closed", () => {
      expect(getPortalAccess(undefined).canViewTeamSchedule).toBe(false);
    });
  });

  // The project Work items tab's staff framing (the "Work items" label, the
  // Chats sub-tab) -- same explicit-allow-list shape as canViewTeamSchedule,
  // for the same reported-live reason, with timecard_approver as the one
  // extra qualifying role instead of comment_updater.
  describe("canViewWorkItemsStaffView", () => {
    it("cs_engineer, admin and timecard_approver each hold it on their own", () => {
      expect(getPortalAccess(["cs_engineer"]).canViewWorkItemsStaffView).toBe(true);
      expect(getPortalAccess(["admin"]).canViewWorkItemsStaffView).toBe(true);
      expect(getPortalAccess(["timecard_approver"]).canViewWorkItemsStaffView).toBe(true);
    });

    it("a viewer combined with an unrelated role does not qualify", () => {
      expect(getPortalAccess(["viewer"]).canViewWorkItemsStaffView).toBe(false);
      expect(getPortalAccess(["viewer", "attachment_downloader"]).canViewWorkItemsStaffView).toBe(false);
      expect(getPortalAccess(["viewer", "escalator"]).canViewWorkItemsStaffView).toBe(false);
    });

    it("a viewer who also holds a qualifying role does qualify", () => {
      expect(getPortalAccess(["viewer", "cs_engineer"]).canViewWorkItemsStaffView).toBe(true);
      expect(getPortalAccess(["viewer", "timecard_approver"]).canViewWorkItemsStaffView).toBe(true);
    });

    it("no other role grants it", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
        "comment_updater",
      ]) {
        expect(getPortalAccess([role]).canViewWorkItemsStaffView).toBe(false);
      }
    });

    it("is false while roles are not loaded, so the controls fail closed", () => {
      expect(getPortalAccess(undefined).canViewWorkItemsStaffView).toBe(false);
    });
  });

  // Settings (/admin): an explicit cs_engineer/admin-only allow-list, with no
  // third qualifying role (unlike canViewTeamSchedule/canViewWorkItemsStaffView),
  // independent of viewer -- a viewer combined with an unrelated role must not
  // reach user/role/group/team administration just by also holding it.
  describe("canViewSettings", () => {
    it("cs_engineer and admin each hold it on their own", () => {
      expect(getPortalAccess(["cs_engineer"]).canViewSettings).toBe(true);
      expect(getPortalAccess(["admin"]).canViewSettings).toBe(true);
    });

    it("a viewer combined with an unrelated role does not qualify", () => {
      expect(getPortalAccess(["viewer"]).canViewSettings).toBe(false);
      expect(getPortalAccess(["viewer", "attachment_downloader"]).canViewSettings).toBe(false);
      expect(getPortalAccess(["viewer", "escalator"]).canViewSettings).toBe(false);
    });

    it("a viewer who also holds a qualifying role does qualify", () => {
      expect(getPortalAccess(["viewer", "cs_engineer"]).canViewSettings).toBe(true);
      expect(getPortalAccess(["viewer", "admin"]).canViewSettings).toBe(true);
    });

    it("no other role grants it", () => {
      for (const role of [
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
        "comment_updater",
      ]) {
        expect(getPortalAccess([role]).canViewSettings).toBe(false);
      }
    });

    it("is false while roles are not loaded, so the controls fail closed", () => {
      expect(getPortalAccess(undefined).canViewSettings).toBe(false);
    });
  });

  // The dedicated read-only security role: grants canUseOperations,
  // canUseSecurityCenter and canViewSlaTab and nothing else -- never
  // canWrite, so both sections render read-only for a holder with no other
  // role. Mirrors the backend's AccessConfig.SecurityTeam.
  describe("security_team", () => {
    it("on its own grants exactly Operations, Security Center and the SLA tab, read-only", () => {
      expect(getPortalAccess(["security_team"])).toEqual({
        ...NONE,
        hasAnyRole: true,
        canUseOperations: true,
        canUseSecurityCenter: true,
        canViewSlaTab: true,
      });
    });

    it("does not grant write access of any kind", () => {
      expect(getPortalAccess(["security_team"]).canWrite).toBe(false);
      expect(getPortalAccess(["security_team"]).canEscalate).toBe(false);
      expect(getPortalAccess(["security_team"]).canAddWorkNotes).toBe(false);
      expect(getPortalAccess(["security_team"]).canCreateAnnouncement).toBe(false);
      expect(getPortalAccess(["security_team"]).canUpdateDeleteAnyComment).toBe(false);
    });

    it("matches the role key case-insensitively, like every other role", () => {
      expect(getPortalAccess(["Security_Team"]).canUseOperations).toBe(true);
      expect(getPortalAccess(["Security_Team"]).canUseSecurityCenter).toBe(true);
    });

    it("is false while roles are not loaded, so the controls fail closed", () => {
      expect(getPortalAccess(undefined).canUseOperations).toBe(false);
      expect(getPortalAccess(undefined).canUseSecurityCenter).toBe(false);
    });
  });

  // A case's SLAs tab: admin, cs_engineer or security_team -- same role set
  // as canUseSecurityCenter.
  describe("canViewSlaTab", () => {
    it("admin, CS engineer and security_team all hold it", () => {
      expect(getPortalAccess(["admin"]).canViewSlaTab).toBe(true);
      expect(getPortalAccess(["cs_engineer"]).canViewSlaTab).toBe(true);
      expect(getPortalAccess(["security_team"]).canViewSlaTab).toBe(true);
    });

    it("no other role grants it", () => {
      for (const role of [
        "viewer",
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "worknote_creator",
        "announcement_creator",
        "comment_updater",
      ]) {
        expect(getPortalAccess([role]).canViewSlaTab).toBe(false);
      }
    });

    it("is false while roles are not loaded, so the tab fails closed", () => {
      expect(getPortalAccess(undefined).canViewSlaTab).toBe(false);
    });
  });

  // Unlike every other flag, this one is keyed on holding the `admin` role
  // itself, not a derived permission -- cs_engineer shares every other
  // capability with admin but never holds this one.
  describe("isAdmin", () => {
    it("is true only for admin, not for cs_engineer or any other role", () => {
      expect(getPortalAccess(["admin"]).isAdmin).toBe(true);
      for (const role of [
        "cs_engineer",
        "viewer",
        "escalator",
        "attachment_downloader",
        "usage_metrics_viewer",
        "timecard_approver",
        "dashboard_designer",
        "plg_admin",
      ]) {
        expect(getPortalAccess([role]).isAdmin).toBe(false);
      }
    });

    it("is false while roles are not loaded or empty", () => {
      expect(getPortalAccess(undefined).isAdmin).toBe(false);
      expect(getPortalAccess([]).isAdmin).toBe(false);
    });

    it("matches the role key case-insensitively", () => {
      expect(getPortalAccess(["Admin"]).isAdmin).toBe(true);
    });
  });
});
