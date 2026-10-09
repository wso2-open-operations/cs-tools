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
  canUseSecurityCenter: false,
  canUsePlg: false,
  canManagePlaybooks: false,
  canCreateAnnouncement: false,
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
    expect(getPortalAccess(["viewer"])).toEqual({ ...NONE, hasAnyRole: true });
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
    for (const role of ["usage_metrics_viewer", "dashboard_designer"]) {
      expect(getPortalAccess([role])).toEqual({ ...NONE, hasAnyRole: true });
    }
  });

  it("the time-card approver also gets Time cards and Updates, and nothing else", () => {
    expect(getPortalAccess(["timecard_approver"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canUseTimeCardsAndUpdates: true,
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
    };
    // Both hold canEscalate (any internal engineer may escalate, as in
    // ServiceNow). canManagePlaybooks is the further exception beyond
    // canCreateUser: both roles hold canUsePlg, only admin may author a
    // playbook template. canCreateAnnouncement is the one write a plain CS
    // engineer no longer holds: sending an announcement needs the
    // announcement_creator role (admin always has it).
    expect(getPortalAccess(["cs_engineer"])).toEqual({
      ...all,
      canCreateUser: false,
      canEscalate: true,
      canManagePlaybooks: false,
      canCreateAnnouncement: false,
    });
    expect(getPortalAccess(["admin"])).toEqual({
      ...all,
      canCreateUser: true,
      canEscalate: true,
      canManagePlaybooks: true,
      canCreateAnnouncement: true,
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

  it("only admin and CS engineer can use Security Center", () => {
    expect(getPortalAccess(["admin"]).canUseSecurityCenter).toBe(true);
    expect(getPortalAccess(["cs_engineer"]).canUseSecurityCenter).toBe(true);
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

  it("only full-access roles get the Operations section", () => {
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
  });

  it("a user holding several roles gets the union", () => {
    expect(getPortalAccess(["viewer", "escalator", "attachment_downloader"])).toEqual({
      ...NONE,
      hasAnyRole: true,
      canEscalate: true,
      canDownloadAttachment: true,
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
});
