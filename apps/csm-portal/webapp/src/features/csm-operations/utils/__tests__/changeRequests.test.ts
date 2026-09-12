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
import {
  buildChangeRequestSearchFilters,
  buildCloneChangeRequestNavState,
  changeRequestBlockingReason,
  countActiveCRFilters,
  DEFAULT_CR_FILTERS,
} from "@features/csm-operations/utils/changeRequests";
import type { BeChangeRequestApproval, BeChangeRequestDetail } from "@api/backend/types";

const FULL_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  description: "<p>Upgrade to the latest patch level.</p>",
  project: { id: "proj-1", name: "Project A" },
  case: { id: "case-1", name: "CASE0001234" },
  deployment: { id: "dep-1", name: "prod" },
  deployedProduct: { id: "dp-1", name: "API Manager" },
  product: { id: "product-1", name: "API Manager" },
  assignedEngineer: { id: "user-1", name: "Jane Doe" },
  assignedTeam: { id: "team-1", name: "Platform" },
  plannedStartOn: "2026-01-01T00:00:00Z",
  plannedEndOn: "2026-01-02T00:00:00Z",
  duration: "1 day",
  impact: "medium",
  state: "closed",
  type: "normal",
  createdOn: "2025-12-01T00:00:00Z",
  updatedOn: "2025-12-02T00:00:00Z",
  createdBy: "someone@example.com",
  justification: "<p>Needed for the security patch.</p>",
  impactDescription: "<p>Brief outage expected.</p>",
  serviceOutage: "<p>5 minutes.</p>",
  communicationPlan: "<p>Notify via status page.</p>",
  rollbackPlan: "<p>Revert to the previous image.</p>",
  testPlan: "<p>Run the smoke suite.</p>",
  hasCustomerApproved: true,
  hasCustomerReviewed: true,
  approvedBy: { id: "approver-1", name: "Approver Name" },
  approvedOn: "2025-12-05T00:00:00Z",
  legalNextStates: [],
};

describe("buildCloneChangeRequestNavState", () => {
  it("carries over the fields that are genuinely the same on read and create", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    expect(state.subject).toBe("Upgrade the gateway cluster");
    expect(state.description).toContain("Upgrade to the latest patch level.");
    expect(state.justification).toContain("Needed for the security patch.");
    expect(state.testPlan).toContain("Run the smoke suite.");
    expect(state.type).toBe("normal");
    expect(state.impact).toBe("medium");
    expect(state.assignedEngineerId).toBe("user-1");
    expect(state.assignedEngineerLabel).toBe("Jane Doe");
    expect(state.sourceNumber).toBe("CHG0009988");
  });

  it("never surfaces a field that create-time payload has no slot for", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    // impactDescription/serviceOutage/communicationPlan/rollbackPlan are
    // read-only on the backend today — BeCreateChangeRequestPayload has no
    // field for any of them, so they must never appear in the clone state.
    expect(keys).not.toContain("impactDescription");
    expect(keys).not.toContain("serviceOutage");
    expect(keys).not.toContain("communicationPlan");
    expect(keys).not.toContain("rollbackPlan");
    // category/priority/risk/implementationPlan/riskImpactAnalysis are
    // write-only — never returned by GET — so there is no source value ever.
    expect(keys).not.toContain("category");
    expect(keys).not.toContain("priority");
    expect(keys).not.toContain("risk");
    expect(keys).not.toContain("implementationPlan");
    expect(keys).not.toContain("riskImpactAnalysis");
  });

  it("never carries the environment, project, or linked-case references", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("deployment");
    expect(keys).not.toContain("deployedProduct");
    expect(keys).not.toContain("project");
    expect(keys).not.toContain("case");
    expect(keys).not.toContain("product");
    expect(keys).not.toContain("assignedTeam");
  });

  it("never carries state, schedule, or approval fields", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("state");
    expect(keys).not.toContain("plannedStartOn");
    expect(keys).not.toContain("plannedEndOn");
    expect(keys).not.toContain("hasCustomerApproved");
    expect(keys).not.toContain("hasCustomerReviewed");
    expect(keys).not.toContain("approvedBy");
    expect(keys).not.toContain("approvedOn");
  });

  it("never carries auto-numbered, created-by, or timestamp fields", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("id");
    expect(keys).not.toContain("createdOn");
    expect(keys).not.toContain("updatedOn");
    expect(keys).not.toContain("createdBy");
    expect(keys).not.toContain("duration");
    expect(keys).not.toContain("legalNextStates");
  });

  it("omits a blank rich-text field instead of copying an empty-looking paragraph", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      description: "<p><br></p>",
      justification: null,
      testPlan: undefined,
    });
    expect(state.description).toBeUndefined();
    expect(state.justification).toBeUndefined();
    expect(state.testPlan).toBeUndefined();
  });

  it("omits the assigned engineer entirely when the source record has none", () => {
    const state = buildCloneChangeRequestNavState({ ...FULL_CR, assignedEngineer: null });
    expect(state.assignedEngineerId).toBeUndefined();
    expect(state.assignedEngineerLabel).toBeUndefined();
  });

  it("sanitizes rich-text content before it reaches the clone form's editor", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      description: '<p>Safe</p><script>alert("xss")</script>',
    });
    expect(state.description).not.toContain("<script>");
    expect(state.description).toContain("Safe");
  });
});

describe("changeRequestBlockingReason", () => {
  function approval(overrides: Partial<BeChangeRequestApproval>): BeChangeRequestApproval {
    return {
      stage: "Assess",
      approverType: "STATIC_GROUP",
      approverName: null,
      status: "APPROVED",
      approvers: [],
      ...overrides,
    };
  }

  it("returns null when there are no approval stages yet", () => {
    expect(changeRequestBlockingReason(undefined)).toBeNull();
    expect(changeRequestBlockingReason([])).toBeNull();
  });

  it("returns null when every stage is settled (approved/rejected/not required)", () => {
    expect(
      changeRequestBlockingReason([
        approval({ status: "APPROVED" }),
        approval({ stage: "Authorize", status: "NOT_REQUIRED" }),
      ]),
    ).toBeNull();
  });

  it("names the stage when a stage is REQUESTED and has no named approver group", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Authorize", status: "REQUESTED" })]),
    ).toBe("Awaiting Authorize approval");
  });

  it("treats PENDING the same as REQUESTED", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Assess", status: "PENDING" })]),
    ).toBe("Awaiting Assess approval");
  });

  it("names the approver group instead of the stage when one is present", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Authorize", status: "REQUESTED", approverName: "Devops Approval" }),
      ]),
    ).toBe("Awaiting Devops Approval");
  });

  it("does not double the word 'approval' when the approver name already carries it", () => {
    const reason = changeRequestBlockingReason([
      approval({ status: "REQUESTED", approverName: "Security Approval Board" }),
    ]);
    expect(reason).toBe("Awaiting Security Approval Board");
    expect(reason?.match(/approval/gi)).toHaveLength(1);
  });

  it("returns the first waiting stage, in stage order, when several are unsettled", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Assess", status: "APPROVED" }),
        approval({ stage: "Authorize", status: "REQUESTED" }),
        approval({ stage: "Customer Approval", status: "PENDING" }),
      ]),
    ).toBe("Awaiting Authorize approval");
  });

  it("is case-insensitive on the status value", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Assess", status: "requested" })]),
    ).toBe("Awaiting Assess approval");
  });
});

describe("countActiveCRFilters", () => {
  it("is 0 for the default filters", () => {
    expect(countActiveCRFilters(DEFAULT_CR_FILTERS)).toBe(0);
  });

  it("is 1 when an SRE team filter is set", () => {
    expect(
      countActiveCRFilters({ ...DEFAULT_CR_FILTERS, sreTeamIds: ["team-apollo"] }),
    ).toBe(1);
  });
});

describe("buildChangeRequestSearchFilters", () => {
  it("returns an empty object for the defaults with no search text", () => {
    expect(buildChangeRequestSearchFilters(DEFAULT_CR_FILTERS, "")).toEqual({});
  });

  it("includes states/impacts/closed-date bounds when set", () => {
    expect(
      buildChangeRequestSearchFilters(
        {
          ...DEFAULT_CR_FILTERS,
          states: ["implement"],
          impacts: ["high"],
          closedStartDate: "2026-01-01",
          closedEndDate: "2026-01-31",
        },
        "rollback",
      ),
    ).toEqual({
      searchQuery: "rollback",
      states: ["implement"],
      impacts: ["high"],
      closedStartDate: "2026-01-01T00:00:00Z",
      closedEndDate: "2026-01-31T23:59:59Z",
    });
  });

  it("sends selected SRE teams as an assignmentGroupId/in generic filter entry", () => {
    expect(
      buildChangeRequestSearchFilters(
        { ...DEFAULT_CR_FILTERS, sreTeamIds: ["team-apollo", "team-atlas"] },
        "",
      ),
    ).toEqual({
      filters: [{ field: "assignmentGroupId", op: "in", values: ["team-apollo", "team-atlas"] }],
    });
  });

  it("omits the generic filters array entirely when no SRE team is selected", () => {
    expect(buildChangeRequestSearchFilters(DEFAULT_CR_FILTERS, "")).not.toHaveProperty("filters");
  });
});
