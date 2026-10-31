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

import { describe, expect, it, vi } from "vitest";
import {
  APPROVED_ASSIGNEE_SKIPPED,
  APPROVED_FIELDS_SKIPPED,
  assignChangeRequestToMePlan,
  bulkChangeRequestEditProblem,
  bulkChangeRequestPlan,
  bulkEditTouchesContent,
  NEW_ONLY_FIELDS_SKIPPED,
  ON_HOLD_STATE_SKIPPED,
  PLANNED_WINDOW_SKIPPED,
  PROJECT_CHANGED_SKIPPED,
  REQUEST_APPROVAL_NEEDS_PROJECT_SKIPPED,
  REQUEST_APPROVAL_NEEDS_TEAM_SKIPPED,
  type BulkChangeRequestEdit,
} from "@features/csm-operations/utils/bulkChangeRequestEdit";
import {
  CUSTOMER_REQUIREMENT_ADD_ONLY_REASON,
  CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON,
  EMERGENCY_CUSTOMER_STEPS_HELPER,
} from "@features/csm-operations/utils/changeRequests";

const OPEN_STATES = ["new", "assess", "authorize"];
const APPROVED_STATES = ["customer_approval", "scheduled", "implement", "review", "customer_review"];
const FINAL_STATES = ["closed", "canceled", "rollback"];

describe("bulkChangeRequestPlan", () => {
  const edit = { state: "canceled", impact: "high" as const, category: "devops" as const, workNote: "  why  " };

  it.each(OPEN_STATES)("sends every field to a change request in %s", (state) => {
    expect(bulkChangeRequestPlan({ state }, edit)).toEqual({
      patch: { state: "canceled", impact: "high", category: "devops", workNote: "why" },
    });
  });

  it.each(APPROVED_STATES)("sends only the state and the work note to an approved change request (%s)", (state) => {
    expect(bulkChangeRequestPlan({ state }, edit)).toEqual({ patch: { state: "canceled", workNote: "why" } });
  });

  it.each(APPROVED_STATES)("skips an approved change request (%s) asked only to change its fields", (state) => {
    expect(bulkChangeRequestPlan({ state }, { priority: "critical" })).toEqual({ skip: APPROVED_FIELDS_SKIPPED });
  });

  it.each(FINAL_STATES)("never edits a %s change request", (state) => {
    expect(bulkChangeRequestPlan({ state }, edit)).toEqual({
      skip: "Closed, canceled or rolled back: it can't be edited.",
    });
  });

  it("sends the assignment group and the assignee as ids", () => {
    expect(bulkChangeRequestPlan({ state: "new" }, { assignedTeamId: "g-1", assignedEngineerId: "u-1" })).toEqual({
      patch: { assignedTeamId: "g-1", assignedEngineerId: "u-1" },
    });
  });

  it("adds a work note to an approved change request on its own", () => {
    expect(bulkChangeRequestPlan({ state: "implement" }, { workNote: "deployed to staging" })).toEqual({
      patch: { workNote: "deployed to staging" },
    });
  });

  it("skips when there is nothing to send", () => {
    expect(bulkChangeRequestPlan({ state: "new" }, { workNote: "   " })).toEqual({ skip: "Nothing to change." });
  });

  // Every field the dialog can set, as it sends them.
  const FULL_EDIT: BulkChangeRequestEdit = {
    title: "Rotate the gateway certificates",
    description: "<p>What</p>",
    type: "normal",
    category: "software",
    impact: "medium",
    priority: "high",
    serviceId: "svc-1",
    serviceOfferingId: "so-1",
    assignedTeamId: "g-1",
    assignedEngineerId: "u-1",
    requestedById: "u-2",
    caseId: "case-1",
    plannedStartOn: "2026-11-01T10:00:00Z",
    plannedEndOn: "2026-11-01T12:00:00Z",
    rollbackDurationText: "30 mins",
    justification: "<p>Why</p>",
    implementationPlan: "<p>How</p>",
    impactDescription: "<p>Risk</p>",
    rollbackPlan: "<p>Back out</p>",
    testPlan: "<p>Test</p>",
    serviceOutage: "None",
    communicationPlan: "Email the customer",
    affectedServicesText: "<p>Gateway</p>",
    affectedComponentsText: "<p>Certs</p>",
    projectId: "proj-1",
    deploymentIds: [],
    deploymentProductIds: [],
    customerApprovalRequired: true,
    customerReviewRequired: true,
    isPlanningVisibleToCustomers: false,
    onHold: true,
    onHoldReason: "Waiting on the vendor",
    workNote: "note",
    comment: "comment",
  };

  it("sends every field the API takes to a change request in New, false and empty lists included", () => {
    expect(bulkChangeRequestPlan({ state: "new", type: "normal" }, FULL_EDIT)).toEqual({ patch: FULL_EDIT });
  });

  it.each(APPROVED_STATES)("sends an approved change request (%s) only the state, the hold and the notes", (state) => {
    expect(bulkChangeRequestPlan({ state }, { ...FULL_EDIT, state: "canceled" })).toEqual({
      patch: { state: "canceled", onHold: true, onHoldReason: "Waiting on the vendor", workNote: "note", comment: "comment" },
    });
  });

  it("never sends a blank text, so a bulk edit clears nothing", () => {
    expect(bulkChangeRequestPlan({ state: "new" }, { title: "   ", rollbackDurationText: "", comment: " x " })).toEqual({
      patch: { comment: "x" },
    });
  });

  it.each(["assess", "authorize"])(
    "skips a change request in %s asked for another type or Customer Project: fixed once approval was requested",
    (state) => {
      const project = { id: "proj-1", name: "P" };
      expect(bulkChangeRequestPlan({ state, type: "normal" }, { type: "standard", impact: "low" })).toEqual({
        skip: NEW_ONLY_FIELDS_SKIPPED,
      });
      expect(bulkChangeRequestPlan({ state, project }, { projectId: "proj-2", deploymentIds: [] })).toEqual({
        skip: NEW_ONLY_FIELDS_SKIPPED,
      });
      // Its own type and project are a no-op resend.
      expect(bulkChangeRequestPlan({ state, type: "Normal" }, { type: "normal", impact: "low" })).toEqual({
        patch: { impact: "low" },
      });
    },
  );

  // The dialog sends zoneless UTC ("YYYY-MM-DD HH:mm:ss", zonedInputToBackendUtc); the detail reads RFC 3339 with "Z".
  // Read naively, the zoneless side would be the browser's local time: pinned here (vi.stubEnv), as a user's browser would be.
  it.each(["Asia/Colombo", "America/New_York", "UTC"])(
    "compares the dialog's zoneless UTC date with the stored one on the same clock (browser in %s)",
    (timeZone) => {
      // Node reads TZ again whenever it is set, so Date follows the stub.
      vi.stubEnv("TZ", timeZone);
      try {
        const stored = { state: "new", plannedStartOn: "2026-11-01T10:00:00Z", plannedEndOn: "2026-11-01T12:00:00Z" };
        // 13:30 UTC is after the stored 12:00 end: skipped in every timezone.
        expect(bulkChangeRequestPlan(stored, { plannedStartOn: "2026-11-01 13:30:00" })).toEqual({ skip: PLANNED_WINDOW_SKIPPED });
        // 11:00 UTC is inside the stored window: sent in every timezone.
        expect(bulkChangeRequestPlan(stored, { plannedEndOn: "2026-11-01 11:00:00" })).toEqual({
          patch: { plannedEndOn: "2026-11-01 11:00:00" },
        });
        expect(bulkChangeRequestEditProblem({ plannedStartOn: "2026-11-01 13:30:00", plannedEndOn: "2026-11-01 13:00:00" })).toMatch(
          /Planned end must be after/,
        );
      } finally {
        vi.unstubAllEnvs();
      }
    },
  );

  it("never moves a change request to the project shown fixed: one whose project changed meanwhile is skipped", () => {
    const edit = { projectFixed: true, projectId: "proj-1", deploymentIds: ["d-1"], deploymentProductIds: ["dp-1"] };
    // A New one moved to another project since it was picked.
    expect(bulkChangeRequestPlan({ state: "new", project: { id: "proj-2", name: "Q" } }, edit)).toEqual({
      skip: PROJECT_CHANGED_SKIPPED,
    });
    // Still on it: the deployments change, and the marker itself is never sent.
    expect(bulkChangeRequestPlan({ state: "assess", project: { id: "proj-1", name: "P" } }, edit)).toEqual({
      patch: { projectId: "proj-1", deploymentIds: ["d-1"], deploymentProductIds: ["dp-1"] },
    });
    expect(bulkEditTouchesContent({ projectFixed: true })).toBe(false);
  });

  it.each(["assess", "authorize"])("changes the deployments of a change request in %s, within its own project", (state) => {
    const project = { id: "proj-1", name: "P" };
    const edit = { projectId: "proj-1", deploymentIds: ["d-1"], deploymentProductIds: ["dp-1"] };
    expect(bulkChangeRequestPlan({ state, project }, edit)).toEqual({ patch: edit });
  });

  it("judges the planned window as it will stand: one date set alone meets the change request's own other date", () => {
    const window = { plannedStartOn: "2026-11-01T10:00:00Z", plannedEndOn: "2026-11-01T12:00:00Z" };
    expect(bulkChangeRequestPlan({ state: "new", ...window }, { plannedStartOn: "2026-11-03T09:00:00Z" })).toEqual({
      skip: PLANNED_WINDOW_SKIPPED,
    });
    expect(bulkChangeRequestPlan({ state: "new", ...window }, { plannedEndOn: "2026-11-01T09:00:00Z" })).toEqual({
      skip: PLANNED_WINDOW_SKIPPED,
    });
    expect(bulkChangeRequestPlan({ state: "new", ...window }, { plannedStartOn: "2026-11-01T11:00:00Z" })).toEqual({
      patch: { plannedStartOn: "2026-11-01T11:00:00Z" },
    });
    // No stored other date: nothing to judge against.
    expect(bulkChangeRequestPlan({ state: "new" }, { plannedStartOn: "2026-11-03T09:00:00Z" })).toEqual({
      patch: { plannedStartOn: "2026-11-03T09:00:00Z" },
    });
  });

  it("leaves the New-only fields out for an approved change request, as every other field", () => {
    expect(bulkChangeRequestPlan({ state: "scheduled" }, { type: "standard", workNote: "x" })).toEqual({
      patch: { workNote: "x" },
    });
  });

  describe("customer requirements", () => {
    it("turns both off when a change in New is re-typed to Emergency, unless they are off already", () => {
      expect(
        bulkChangeRequestPlan(
          { state: "new", type: "normal", customerApprovalRequired: true, customerReviewRequired: false },
          { type: "emergency" },
        ),
      ).toEqual({ patch: { type: "emergency", customerApprovalRequired: false } });
    });

    it("never ticks one on an Emergency change", () => {
      expect(bulkChangeRequestPlan({ state: "new", type: "emergency" }, { customerReviewRequired: true })).toEqual({
        skip: EMERGENCY_CUSTOMER_STEPS_HELPER,
      });
    });

    it("leaves alone an Emergency change already ticked from before the rule, and edits the rest of it", () => {
      expect(
        bulkChangeRequestPlan(
          { state: "assess", type: "emergency", customerApprovalRequired: true },
          { customerApprovalRequired: true, assignedTeamId: "g-1" },
        ),
      ).toEqual({ patch: { assignedTeamId: "g-1" } });
    });

    it("does not send one that already reads as asked", () => {
      expect(
        bulkChangeRequestPlan({ state: "assess", customerApprovalRequired: true }, { customerApprovalRequired: true }),
      ).toEqual({ skip: "Nothing to change." });
    });

    it("adds one after New only with a Customer Project, and never removes one then", () => {
      expect(bulkChangeRequestPlan({ state: "assess" }, { customerApprovalRequired: true })).toEqual({
        skip: CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON,
      });
      expect(
        bulkChangeRequestPlan({ state: "authorize", project: { id: "p", name: "P" } }, { customerApprovalRequired: true }),
      ).toEqual({ patch: { customerApprovalRequired: true } });
      expect(
        bulkChangeRequestPlan({ state: "assess", customerReviewRequired: true }, { customerReviewRequired: false }),
      ).toEqual({ skip: CUSTOMER_REQUIREMENT_ADD_ONLY_REASON });
    });

    it("takes the project being set in the same edit as the change request's project", () => {
      expect(
        bulkChangeRequestPlan({ state: "new" }, { projectId: "proj-1", deploymentIds: [], customerApprovalRequired: true }),
      ).toEqual({ patch: { projectId: "proj-1", deploymentIds: [], customerApprovalRequired: true } });
    });
  });

  describe("on hold", () => {
    it("only updates the reason of one already on hold (a second hold would clear it)", () => {
      expect(bulkChangeRequestPlan({ state: "assess", onHold: true }, { onHold: true, onHoldReason: "Vendor" })).toEqual({
        patch: { onHoldReason: "Vendor" },
      });
      expect(bulkChangeRequestPlan({ state: "assess", onHold: true }, { onHold: true })).toEqual({ skip: "Already on hold." });
    });

    it("does not take off hold one that isn't on hold", () => {
      expect(bulkChangeRequestPlan({ state: "assess", onHold: false }, { onHold: false })).toEqual({ skip: "Not on hold." });
    });

    it("moves one on hold only when the same edit takes it off hold", () => {
      expect(bulkChangeRequestPlan({ state: "implement", onHold: true }, { state: "review" })).toEqual({
        skip: ON_HOLD_STATE_SKIPPED,
      });
      expect(bulkChangeRequestPlan({ state: "implement", onHold: true }, { state: "review", onHold: false })).toEqual({
        patch: { state: "review", onHold: false },
      });
    });
  });
});

describe("bulkChangeRequestEditProblem", () => {
  it("refuses a planned end at or before the planned start", () => {
    const start = "2026-11-01T10:00:00Z";
    expect(bulkChangeRequestEditProblem({ plannedStartOn: start, plannedEndOn: start })).toMatch(/Planned end must be after/);
    expect(bulkChangeRequestEditProblem({ plannedStartOn: start, plannedEndOn: "2026-11-01T11:00:00Z" })).toBeNull();
  });

  it("refuses a hold reason without putting on hold, and customer steps on an Emergency change", () => {
    expect(bulkChangeRequestEditProblem({ onHoldReason: "x" })).toMatch(/hold reason/);
    expect(bulkChangeRequestEditProblem({ onHold: true, onHoldReason: "x" })).toBeNull();
    expect(bulkChangeRequestEditProblem({ type: "emergency", customerApprovalRequired: true })).toBe(
      EMERGENCY_CUSTOMER_STEPS_HELPER,
    );
  });

  it("asks for a reason to cancel or roll back, and for nothing else", () => {
    expect(bulkChangeRequestEditProblem({ state: "canceled" })).toMatch(/A reason is required/);
    expect(bulkChangeRequestEditProblem({ state: "rollback", workNote: " " })).toMatch(/A reason is required/);
    expect(bulkChangeRequestEditProblem({ state: "rollback", workNote: "failed checks" })).toBeNull();
    expect(bulkChangeRequestEditProblem({ state: "implement" })).toBeNull();
    expect(bulkChangeRequestEditProblem({ impact: "low" })).toBeNull();
  });
});

describe("bulkEditTouchesContent", () => {
  it("is true only for the change request's own fields", () => {
    expect(bulkEditTouchesContent({ state: "implement", workNote: "x" })).toBe(false);
    expect(bulkEditTouchesContent({ priority: "low" })).toBe(true);
    expect(bulkEditTouchesContent({ assignedEngineerId: "u" })).toBe(true);
    expect(bulkEditTouchesContent({ onHold: false, onHoldReason: "x", comment: "y" })).toBe(false);
    expect(bulkEditTouchesContent({ isPlanningVisibleToCustomers: false })).toBe(true);
    expect(bulkEditTouchesContent({ deploymentIds: [] })).toBe(true);
  });

});

describe("assignChangeRequestToMePlan", () => {
  it("assigns an open change request to the signed-in user", () => {
    expect(assignChangeRequestToMePlan({ state: "assess" }, "me")).toEqual({ patch: { assignedEngineerId: "me" } });
  });

  it.each(APPROVED_STATES)("skips an approved change request (%s): its assignee is read-only", (state) => {
    expect(assignChangeRequestToMePlan({ state }, "me")).toEqual({ skip: APPROVED_ASSIGNEE_SKIPPED });
  });

  it("skips a change request already assigned to the user, and a final one", () => {
    expect(assignChangeRequestToMePlan({ state: "new", assignedEngineer: { id: "me" } }, "me")).toEqual({
      skip: "Already assigned to you.",
    });
    expect(assignChangeRequestToMePlan({ state: "closed" }, "me")).toEqual({
      skip: "Closed, canceled or rolled back: it can't be edited.",
    });
  });
});

describe("bulkChangeRequestPlan — Request Approval", () => {
  const TEAM = { id: "g-1", name: "Platform" };
  const PROJECT = { id: "proj-1", name: "Acme" };

  it("skips a change request with no assignment group, and sends one with a team stored or set in the edit", () => {
    expect(bulkChangeRequestPlan({ state: "new" }, { state: "assess" })).toEqual({ skip: REQUEST_APPROVAL_NEEDS_TEAM_SKIPPED });
    expect(bulkChangeRequestPlan({ state: "new", assignedTeam: TEAM }, { state: "assess" })).toEqual({
      patch: { state: "assess" },
    });
    expect(bulkChangeRequestPlan({ state: "new" }, { state: "assess", assignedTeamId: "g-2" })).toEqual({
      patch: { state: "assess", assignedTeamId: "g-2" },
    });
  });

  it("asks for no team for another state", () => {
    expect(bulkChangeRequestPlan({ state: "new" }, { state: "canceled", workNote: "why" })).toEqual({
      patch: { state: "canceled", workNote: "why" },
    });
  });

  it("skips one with a customer step ticked and no Customer Project, judged as the edit leaves it", () => {
    const stored = { state: "new", type: "normal", assignedTeam: TEAM };
    expect(bulkChangeRequestPlan({ ...stored, customerReviewRequired: true }, { state: "assess" })).toEqual({
      skip: REQUEST_APPROVAL_NEEDS_PROJECT_SKIPPED,
    });
    // A step ticked in the same edit counts; one turned off in it does too.
    expect(bulkChangeRequestPlan(stored, { state: "assess", customerApprovalRequired: true })).toEqual({
      skip: REQUEST_APPROVAL_NEEDS_PROJECT_SKIPPED,
    });
    expect(
      bulkChangeRequestPlan({ ...stored, customerApprovalRequired: true }, { state: "assess", customerApprovalRequired: false }),
    ).toEqual({ patch: { state: "assess", customerApprovalRequired: false } });
    // A project stored, or set in the same edit, satisfies it.
    expect(bulkChangeRequestPlan({ ...stored, customerApprovalRequired: true, project: PROJECT }, { state: "assess" })).toEqual({
      patch: { state: "assess" },
    });
    expect(
      bulkChangeRequestPlan(
        { ...stored, customerApprovalRequired: true },
        { state: "assess", projectId: "proj-1", deploymentIds: [] },
      ),
    ).toEqual({ patch: { state: "assess", projectId: "proj-1", deploymentIds: [] } });
  });

  it("words that skip so it holds for a mixed selection, whose project the same edit can't set", () => {
    expect(REQUEST_APPROVAL_NEEDS_PROJECT_SKIPPED).toBe(
      "A customer step is ticked and there is no Customer Project, so approval can't be requested until it has one.",
    );
  });

  it("asks no project of an Emergency change, stored or re-typed in the edit", () => {
    expect(
      bulkChangeRequestPlan({ state: "new", type: "emergency", assignedTeam: TEAM, customerApprovalRequired: true }, { state: "assess" }),
    ).toEqual({ patch: { state: "assess" } });
    expect(
      bulkChangeRequestPlan(
        { state: "new", type: "normal", assignedTeam: TEAM, customerApprovalRequired: true },
        { state: "assess", type: "emergency" },
      ),
    ).toEqual({ patch: { state: "assess", type: "emergency", customerApprovalRequired: false } });
  });
});
