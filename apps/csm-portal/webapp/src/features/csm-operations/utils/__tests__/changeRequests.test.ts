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
  acceptProposedTimeBlockedReason,
  answerSnapshotMoved,
  approvalStageLabel,
  backendErrorCode,
  ChangeRequestErrorCode,
  isStaleAnswerError,
  buildChangeRequestSearchFilters,
  buildCloneChangeRequestNavState,
  CHANGE_REQUEST_CREATE_TYPE_OPTIONS,
  changeRequestBlockingReason,
  CHANGE_REQUEST_CATEGORY_OPTIONS,
  changeRequestCategoryLabel,
  changeRequestCategoryValue,
  changeRequestScopeLockedReason,
  changeRequestTransitionLabel,
  changeRequestTransitionRequiresReason,
  countActiveCRFilters,
  CUSTOMER_PROPOSAL_WAITING_REASON,
  customerApprovedDisplay,
  CUSTOMER_STEP_NOT_APPLICABLE,
  customerGateWithheldTargets,
  customerProposalProposer,
  customerProposalProposerLabel,
  customerProposalWording,
  formatWindowLength,
  pendingCustomerProposal,
  plannedWindowMs,
  PROPOSER_NOT_RECORDED_ACCEPT_REASON,
  proposedWindowMs,
  STORED_TIME_ADVICE,
  storedTimeSentence,
  isDestructiveChangeRequestTransition,
  pendingCustomerReview,
  rollbackPendingReviewReason,
  customerApprovalLockedReason,
  customerReviewLockedReason,
  DEFAULT_CHANGE_REQUEST_CATEGORY,
  DEFAULT_CR_FILTERS,
  isChangeRequestCategory,
  isChangeRequestCreator,
  isCreatableChangeRequestType,
  isCustomerStepNotApplicable,
  isEmergencyChangeRequestType,
  EMERGENCY_CUSTOMER_STEPS_HELPER,
  anyApproverBeingAsked,
  NO_CUSTOMER_CONTACTS_HELPER,
  NOBODY_ASKED_HELPER,
  NOBODY_ASKED_WAY_OUT,
  NOBODY_ASKED_WAY_OUT_RESCHEDULE_MAY_HELP,
  noCustomerAskedHelper,
  CUSTOMER_PROJECT_FROZEN_REASON,
  CUSTOMER_REQUIREMENT_ADD_ONLY_REASON,
  CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON,
  CUSTOMER_REQUIREMENT_ONCE_SAVED_HELPER,
  REQUEST_APPROVAL_NEEDS_CONTACT_REASON,
  REQUEST_APPROVAL_NEEDS_PROJECT_REASON,
  customerProjectLockedReason,
  customerRequirementOnceSavedHelper,
  isChangeRequestCreationPhase,
  requestApprovalNeedsContactReason,
  requestApprovalNeedsProjectReason,
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
    // priority/risk/riskImpactAnalysis have no clone source. `implementationPlan`
    // is readable now too, but isn't wired into clone yet (separate feature
    // decision), so it also must not appear here.
    expect(keys).not.toContain("priority");
    expect(keys).not.toContain("risk");
    expect(keys).not.toContain("implementationPlan");
    expect(keys).not.toContain("riskImpactAnalysis");
  });

  it("carries the customer project and category, but never the deployments / deployment products / customer group", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      customerContacts: [{ id: "c-1", name: "Alice Aaron" }],
      category: { id: "devops", name: "DevOps" },
      deployments: [{ id: "dep-1", name: "prod" }],
      deploymentProducts: [{ id: "dp-1", name: "API Manager 4.3.0" }],
    });
    expect(state.projectId).toBe("proj-1");
    expect(state.projectLabel).toBe("Project A");
    expect(state.category).toBe("devops");
    // A clone exists to promote the change to a different deployment, so what
    // names the *target* is left for the user to choose; the Customer Group is
    // derived from the project and read-only, so it is never carried either.
    const keys = Object.keys(state);
    expect(keys).not.toContain("deployments");
    expect(keys).not.toContain("deploymentIds");
    expect(keys).not.toContain("customerContacts");
    expect(keys).not.toContain("customerGroupId");
    expect(keys).not.toContain("customerGroupLabel");
    expect(keys).not.toContain("environments");
    expect(keys).not.toContain("environmentIds");
    expect(keys).not.toContain("deploymentProducts");
    expect(keys).not.toContain("deploymentProductIds");
  });

  it("leaves project / category out when the source has none (or an unknown category)", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      project: undefined,
      customerContacts: [],
      category: { id: "something_new", label: "Something new" },
    });
    expect(state.projectId).toBeUndefined();
    expect(state.category).toBeUndefined();
  });

  it("never carries the single-deployment, linked-case or team references", () => {
    const state = buildCloneChangeRequestNavState(FULL_CR);
    const keys = Object.keys(state);
    expect(keys).not.toContain("deployment");
    expect(keys).not.toContain("deployedProduct");
    expect(keys).not.toContain("project");
    expect(keys).not.toContain("case");
    expect(keys).not.toContain("product");
    expect(keys).not.toContain("assignedTeam");
  });

  it("carries the customer approval / review checkbox settings, but not the customer's confirmation", () => {
    const state = buildCloneChangeRequestNavState({
      ...FULL_CR,
      customerApprovalRequired: true,
      customerReviewRequired: false,
    });
    expect(state.customerApprovalRequired).toBe(true);
    expect(state.customerReviewRequired).toBe(false);
    expect(Object.keys(state)).not.toContain("hasCustomerApproved");
    expect(Object.keys(state)).not.toContain("hasCustomerReviewed");
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

  // After a Re-schedule the superseded Customer Approval stage keeps its place in
  // the list, all its approvers cancelled, and the backend reports it PENDING
  // (nothing was approved or rejected on it). It is not what the change waits on.
  it("skips a superseded stage whose approvers were all cancelled, naming the stage that is really waiting", () => {
    const approver = (status: string) => ({ id: `u-${status}`, name: "Someone", status });
    expect(
      changeRequestBlockingReason(
        [
          approval({ stage: "Peer Approval", status: "APPROVED", approvers: [approver("APPROVED")] }),
          approval({ stage: "CAB Approval", status: "APPROVED", approvers: [approver("APPROVED")] }),
          approval({ stage: "Customer Approval", status: "PENDING", approvers: [approver("CANCELLED"), approver("CANCELED")] }),
          approval({ stage: "CAB Approval", status: "PENDING", approvers: [approver("REQUESTED"), approver("CANCELLED")] }),
        ],
        "authorize",
      ),
    ).toBe("Awaiting CAB Approval");
  });

  it("returns null when the only PENDING stage has nobody left to ask", () => {
    expect(
      changeRequestBlockingReason(
        [approval({ stage: "CAB Approval", status: "PENDING", approvers: [{ id: "u", name: "A", status: "NOT_REQUIRED" }] })],
        "authorize",
      ),
    ).toBeNull();
  });

  it("labels a legacy Authorize stage as CAB Approval", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Authorize", status: "REQUESTED" })]),
    ).toBe("Awaiting CAB Approval");
  });

  it("labels a legacy Assess stage as Peer Approval, and treats PENDING the same as REQUESTED", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Assess", status: "PENDING" })]),
    ).toBe("Awaiting Peer Approval");
  });

  it.each([
    ["Peer Approval", "Awaiting Peer Approval"],
    ["CAB Approval", "Awaiting CAB Approval"],
    ["CAB", "Awaiting CAB Approval"],
    // An Emergency change raised before ECAB was retired: its stage still reads as it was named.
    ["ECAB Approval", "Awaiting ECAB Approval"],
    ["ECAB", "Awaiting ECAB Approval"],
    ["Emergency CAB", "Awaiting ECAB Approval"],
    ["emergency_cab_approval", "Awaiting ECAB Approval"],
  ])("names stage %s as '%s' with no doubled 'approval'", (stage, expected) => {
    const reason = changeRequestBlockingReason([approval({ stage, status: "REQUESTED" })]);
    expect(reason).toBe(expected);
    expect(reason?.match(/approval/gi)).toHaveLength(1);
  });

  it("names the post-implementation Review stage 'Awaiting Review'", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Review", status: "REQUESTED" })]),
    ).toBe("Awaiting Review");
  });

  it("prefers the stage label over the approver group name for a recognised stage", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Authorize", status: "REQUESTED", approverName: "Devops Approval" }),
      ]),
    ).toBe("Awaiting CAB Approval");
  });

  it("names the approver group for a stage it has no label for", () => {
    expect(
      changeRequestBlockingReason([
        approval({ stage: "Vendor Sign-off", status: "REQUESTED", approverName: "Acme Contact" }),
      ]),
    ).toBe("Awaiting Acme Contact approval");
  });

  it("does not double the word 'approval' when the approver name already carries it", () => {
    const reason = changeRequestBlockingReason([
      approval({ stage: "Vendor Sign-off", status: "REQUESTED", approverName: "Security Approval Board" }),
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
    ).toBe("Awaiting CAB Approval");
  });

  it("is case-insensitive on the status value", () => {
    expect(
      changeRequestBlockingReason([approval({ stage: "Assess", status: "requested" })]),
    ).toBe("Awaiting Peer Approval");
  });
});

describe("approvalStageLabel", () => {
  it.each([
    ["Assess", "Peer Approval"],
    ["Peer Approval", "Peer Approval"],
    ["Authorize", "CAB Approval"],
    ["CAB Approval", "CAB Approval"],
    // Historic only: a stage an earlier version gave an Emergency change keeps its label.
    ["Emergency CAB", "ECAB Approval"],
    ["ECAB Approval", "ECAB Approval"],
    ["Review", "Review"],
    ["Customer Approval", "Customer Approval"],
    ["customer_approval", "Customer Approval"],
    ["CUSTOMER-APPROVAL", "Customer Approval"],
    ["Customer Review", "Customer Review"],
    ["customer review", "Customer Review"],
    ["Something New", "Something New"],
  ])("maps %s to %s", (stage, expected) => {
    expect(approvalStageLabel(stage)).toBe(expected);
  });

  it("falls back to a generic label for a blank stage", () => {
    expect(approvalStageLabel("")).toBe("Approval");
    expect(approvalStageLabel(undefined)).toBe("Approval");
  });
});

describe("changeRequestTransitionLabel", () => {
  it("labels the New -> Assess transition 'Request Approval', never 'Move to Assess'", () => {
    expect(changeRequestTransitionLabel("assess")).toBe("Request Approval");
    expect(changeRequestTransitionLabel("assess")).not.toMatch(/move to assess/i);
  });

  it("has no curated 'Schedule' action label for the scheduled state", () => {
    expect(changeRequestTransitionLabel("scheduled")).not.toMatch(/^schedule$/i);
    expect(changeRequestTransitionLabel("scheduled", "authorize")).not.toMatch(/^schedule$/i);
  });

  it("labels closed 'Close' out of Review (or with no known state); the customer's own review is not a staff action", () => {
    expect(changeRequestTransitionLabel("closed", "review")).toBe("Close");
    expect(changeRequestTransitionLabel("closed")).toBe("Close");
  });

  it("never words a transition as answering for the customer ('Bypass customer ...', the retired 'Record customer approval')", () => {
    const states = [
      undefined, null, "new", "assess", "authorize", "customer_approval", "scheduled", "implement",
      "review", "customer_review", "rollback", "closed", "canceled",
    ];
    const targets = [
      "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review",
      "rollback", "closed", "canceled", "awaiting_vendor",
    ];
    for (const from of states) {
      for (const target of targets) {
        expect(changeRequestTransitionLabel(target, from), `${String(from)} -> ${target}`).not.toMatch(
          /bypass|record customer|on behalf/i,
        );
      }
    }
  });

  it("labels authorize 'Propose a different time' when the customer's proposal is waiting for WSO2, and only then", () => {
    expect(changeRequestTransitionLabel("authorize", "customer_approval", true)).toBe("Propose a different time");
    expect(changeRequestTransitionLabel("authorize", "customer_approval", false)).toBe("Re-schedule");
    // The flag means nothing anywhere but out of Customer Approval.
    expect(changeRequestTransitionLabel("authorize", "assess", true)).not.toBe("Propose a different time");
    expect(changeRequestTransitionLabel("canceled", "customer_approval", true)).toBe("Cancel change");
  });

  it("labels authorize 'Re-schedule' only when leaving customer_approval", () => {
    expect(changeRequestTransitionLabel("authorize", "customer_approval")).toBe("Re-schedule");
    expect(changeRequestTransitionLabel("authorize", "assess")).not.toBe("Re-schedule");
    expect(changeRequestTransitionLabel("authorize")).not.toBe("Re-schedule");
    expect(isDestructiveChangeRequestTransition("authorize")).toBe(false);
  });

  it("labels the customer review and close transitions", () => {
    expect(changeRequestTransitionLabel("customer_review", "review")).toBe("Send for customer review");
    expect(changeRequestTransitionLabel("closed", "review")).toBe("Close");
  });

  it("labels the failed-review off-ramp 'Roll back': destructive and needing a reason, like Cancel change", () => {
    for (const from of ["review", "customer_review"]) {
      expect(changeRequestTransitionLabel("rollback", from)).toBe("Roll back");
    }
    expect(isDestructiveChangeRequestTransition("rollback")).toBe(true);
    expect(changeRequestTransitionRequiresReason("rollback")).toBe(true);
    expect(changeRequestTransitionRequiresReason("canceled")).toBe(true);
    expect(changeRequestTransitionRequiresReason("closed")).toBe(false);
  });
});

describe("staff never answer for the customer", () => {
  it("needs a stated reason for exactly the destructive off-ramps, from any state", () => {
    for (const from of [undefined, "review", "customer_approval", "customer_review", "implement"]) {
      expect(changeRequestTransitionRequiresReason("rollback"), String(from)).toBe(true);
      expect(changeRequestTransitionRequiresReason("canceled"), String(from)).toBe(true);
    }
    // Every other target is an ordinary move: no dialog. (Scheduled and closed out of a customer
    // gate are not moves staff can make at all: the action bar never offers them.)
    for (const target of ["assess", "authorize", "scheduled", "implement", "review", "customer_review", "closed"]) {
      expect(changeRequestTransitionRequiresReason(target), target).toBe(false);
    }
  });

  it("is destructive for rollback and canceled only", () => {
    expect(isDestructiveChangeRequestTransition("scheduled")).toBe(false);
    expect(isDestructiveChangeRequestTransition("closed")).toBe(false);
    expect(isDestructiveChangeRequestTransition("rollback")).toBe(true);
    expect(isDestructiveChangeRequestTransition("canceled")).toBe(true);
  });
});

describe("pendingCustomerReview", () => {
  const stage = (
    name: string,
    approvers: Array<[string, string]>,
    status = "PENDING",
  ): BeChangeRequestApproval => ({
    stage: name,
    approverType: "STATIC_GROUP",
    approverName: "Customer Group",
    status,
    approvers: approvers.map(([n, st], i) => ({ id: `u-${i}`, name: n, status: st })),
  });

  it("names the contacts still being asked at Customer Review", () => {
    expect(
      pendingCustomerReview(
        [stage("Customer Review", [["Mira Santos", "REQUESTED"], ["Noel Prasad", "REQUESTED"]])],
        "customer_review",
      ),
    ).toEqual({ contactNames: ["Mira Santos", "Noel Prasad"], askedCount: 2 });
  });

  it("lists only the approvers still REQUESTED, case-insensitively and without duplicates", () => {
    expect(
      pendingCustomerReview(
        [
          stage("Customer Review", [
            ["Mira Santos", "requested"],
            ["Noel Prasad", "CANCELLED"],
            ["Mira Santos", "REQUESTED"],
          ]),
        ],
        "customer_review",
      ),
    ).toEqual({ contactNames: ["Mira Santos"], askedCount: 2 });
  });

  it("is pending even when no approver has a name", () => {
    expect(
      pendingCustomerReview(
        [{ ...stage("Customer Review", []), approvers: [{ id: "u-1", status: "REQUESTED" }] }],
        "customer_review",
      ),
    ).toEqual({ contactNames: [], askedCount: 1 });
  });

  it("counts everyone still asked, the nameless and the ones sharing a name included", () => {
    const asked = pendingCustomerReview(
      [
        {
          ...stage("Customer Review", [["Dana Lee", "REQUESTED"]]),
          approvers: [
            { id: "u-1", name: "Dana Lee", status: "REQUESTED" },
            { id: "u-2", name: "", status: "REQUESTED" },
            { id: "u-3", status: "REQUESTED" },
            { id: "u-4", name: "Dana Lee", status: "REQUESTED" },
            { id: "u-5", name: "Sam Roe", status: "CANCELLED" },
          ],
        },
      ],
      "customer_review",
    );
    expect(asked).toEqual({ contactNames: ["Dana Lee"], askedCount: 4 });
  });

  it("is null once nobody is being asked, whatever the stage's own status says", () => {
    // A superseded request: the backend still reports the stage PENDING.
    expect(
      pendingCustomerReview(
        [stage("Customer Review", [["Mira Santos", "CANCELLED"]], "PENDING")],
        "customer_review",
      ),
    ).toBeNull();
    expect(
      pendingCustomerReview(
        [stage("Customer Review", [["Mira Santos", "APPROVED"], ["Noel Prasad", "CANCELLED"]], "APPROVED")],
        "customer_review",
      ),
    ).toBeNull();
    expect(pendingCustomerReview([stage("Customer Review", [])], "customer_review")).toBeNull();
  });

  it("reads only the Customer Review stage: a Customer Approval stage, or an internal one, is not it", () => {
    const approvals = [
      stage("Customer Approval", [["Mira Santos", "REQUESTED"]]),
      stage("CAB Approval", [["Cam Cab", "REQUESTED"]]),
      stage("Customer Review", [["Noel Prasad", "REQUESTED"]]),
    ];
    expect(pendingCustomerReview(approvals, "customer_review")?.contactNames).toEqual(["Noel Prasad"]);
    expect(
      pendingCustomerReview([stage("Customer Approval", [["Mira Santos", "REQUESTED"]])], "customer_review"),
    ).toBeNull();
    expect(pendingCustomerReview([stage("CAB Approval", [["Cam Cab", "REQUESTED"]])], "customer_review")).toBeNull();
  });

  it("is null outside Customer Review (Customer Approval included), and while the approvals have not loaded", () => {
    const approvals = [
      stage("Customer Approval", [["Mira Santos", "REQUESTED"]]),
      stage("Customer Review", [["Noel Prasad", "REQUESTED"]]),
    ];
    for (const state of [
      "new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "rollback", "closed", "canceled", undefined, null,
    ]) {
      expect(pendingCustomerReview(approvals, state), String(state)).toBeNull();
    }
    expect(pendingCustomerReview(undefined, "customer_review")).toBeNull();
    expect(pendingCustomerReview(null, "customer_review")).toBeNull();
  });
});

describe("rollbackPendingReviewReason", () => {
  const WHY = "A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here.";

  it("says who the review is waiting on and that a failed review is theirs to give in the Customer Portal", () => {
    expect(rollbackPendingReviewReason({ contactNames: ["Mira Santos", "Noel Prasad"], askedCount: 2 })).toBe(
      `Customer review is pending from Mira Santos, Noel Prasad. ${WHY}`,
    );
    expect(rollbackPendingReviewReason({ contactNames: ["Mira Santos"] })).toBe(
      `Customer review is pending from Mira Santos. ${WHY}`,
    );
  });

  it("summarises a long contact list", () => {
    expect(rollbackPendingReviewReason({ contactNames: ["A", "B", "C", "D", "E"] })).toBe(
      `Customer review is pending from A, B, C and 2 more. ${WHY}`,
    );
  });

  it("counts the people asked who have no name (or the same one) in the 'and N more'", () => {
    // Dana Lee plus two approvers the backend sends no name for: three are asked, one is named.
    expect(rollbackPendingReviewReason({ contactNames: ["Dana Lee"], askedCount: 3 })).toBe(
      `Customer review is pending from Dana Lee and 2 more. ${WHY}`,
    );
    // Three contacts that share a name read as that name and two more, not as one person.
    expect(rollbackPendingReviewReason({ contactNames: ["Sam Lee"], askedCount: 3 })).toBe(
      `Customer review is pending from Sam Lee and 2 more. ${WHY}`,
    );
    // With five distinct names it still says "A, B, C and 2 more".
    expect(rollbackPendingReviewReason({ contactNames: ["A", "B", "C", "D", "E"], askedCount: 5 })).toContain(
      "A, B, C and 2 more.",
    );
    // An absent askedCount reads as the number of names.
    expect(rollbackPendingReviewReason({ contactNames: ["A", "B"] })).toContain("from A, B.");
  });

  it("falls back to a generic sentence without contact names", () => {
    expect(rollbackPendingReviewReason({ contactNames: [] })).toBe(
      "Customer review is pending. A failed review is the customer's to give in the Customer Portal, so the change can't be rolled back from here.",
    );
  });

  it("is null when nothing is pending, and never speaks of a bypass", () => {
    expect(rollbackPendingReviewReason(null)).toBeNull();
    expect(rollbackPendingReviewReason(undefined)).toBeNull();
    expect(rollbackPendingReviewReason({ contactNames: ["Mira Santos"] })).not.toMatch(/bypass/i);
  });
});

describe("customerGateWithheldTargets", () => {
  it("withholds only Roll back while the customer's review is live", () => {
    expect(customerGateWithheldTargets("customer_review")).toEqual(["rollback"]);
  });

  it("withholds nothing at Customer Approval (Re-schedule and Cancel change are always on offer) or anywhere else", () => {
    for (const state of [
      "new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "closed", "rollback", "canceled", null, undefined,
    ]) {
      expect(customerGateWithheldTargets(state), String(state)).toEqual([]);
    }
  });
});

describe("changeRequestBlockingReason — customer states", () => {
  it("names the customer approval gate from the state, with or without approvals data", () => {
    expect(changeRequestBlockingReason(undefined, "customer_approval")).toBe(
      "Awaiting Customer Approval",
    );
    expect(
      changeRequestBlockingReason(
        [{ stage: "Authorize", approverType: "STATIC_GROUP", approverName: null, status: "APPROVED", approvers: [] }],
        "customer_approval",
      ),
    ).toBe("Awaiting Customer Approval");
  });

  it("names the customer review gate from the state", () => {
    expect(changeRequestBlockingReason(undefined, "customer_review")).toBe(
      "Awaiting Customer Review",
    );
  });

  it("still derives the reason from approval stages for any other state", () => {
    expect(
      changeRequestBlockingReason(
        [{ stage: "Authorize", approverType: "STATIC_GROUP", approverName: null, status: "REQUESTED", approvers: [] }],
        "authorize",
      ),
    ).toBe("Awaiting CAB Approval");
  });
});

// The rule for the customer's part of a change request is a pure function of
// (state, the stored tick box, whether the change request has a Customer Project),
// and the CSM Edit dialog computes it up front, mirroring the backend. This is the
// table, written out one row per state, with the SAME ROW IDS and outcome codes as
// the backend's Go truth table (entity-service
// `internal/repository/change_request_customer_lock_test.go`): a row id is
// "<STATE>/<column>" with STATE the upper-case state ("NULL" for a change request
// with none recorded). When the rule changes, both tables change; neither module
// imports the other's file on purpose (the container tests copy only entity-service).
//
// Outcome codes:
//
//   ok               the control may be used (in the dialog: the box may be changed)
//   frozen           the Customer Project can no longer be changed
//   cannot-turn-off  a ticked box can no longer be unticked
//   gate-passed      the gate the box controls has been passed
//   needs-project    the box cannot be ticked: there is no Customer Project to ask
//
// Columns the dialog has a control for:
//
//   project-change   the Customer Project of a change request that has one
//   project-set      the Customer Project of one that has none (NULL -> X)
//   approval-on      Customer Approval, unticked, project stored
//   approval-on-bare the same with no Customer Project
//   approval-off     Customer Approval, ticked (project stored)
//   review-on / review-on-bare / review-off    the same for Customer Review
//
// Not columns here, because the dialog has no such control: `project-resend` (the
// dialog resends the stored project beside changed deployments, which the backend
// accepts as a no-op), `to-new` and `rolled-back` (the dialog never sends a state).
type LockCode = "ok" | "frozen" | "cannot-turn-off" | "gate-passed" | "needs-project";
const LOCK_COLUMNS = [
  "project-change", "project-set",
  "approval-on", "approval-on-bare", "approval-off",
  "review-on", "review-on-bare", "review-off",
] as const;
type LockColumn = (typeof LOCK_COLUMNS)[number];

// One row per state, the columns in the order above. Written out, not derived from
// the helpers: it is the statement of the rule they are held to.
const LOCK_TABLE: Record<string, LockCode[]> = {
  NULL:              ["ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok"],
  NEW:               ["ok", "ok", "ok", "ok", "ok", "ok", "ok", "ok"],
  ASSESS:            ["frozen", "frozen", "ok", "needs-project", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off"],
  AUTHORIZE:         ["frozen", "frozen", "ok", "needs-project", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off"],
  CUSTOMER_APPROVAL: ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off"],
  SCHEDULED:         ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off"],
  IMPLEMENT:         ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off"],
  REVIEW:            ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "ok", "needs-project", "cannot-turn-off"],
  CUSTOMER_REVIEW:   ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off"],
  ROLLBACK:          ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off"],
  CLOSED:            ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off"],
  CANCELED:          ["frozen", "frozen", "gate-passed", "gate-passed", "cannot-turn-off", "gate-passed", "gate-passed", "cannot-turn-off"],
};

/** What the client says for one cell of the table, as an outcome code. */
function lockOutcome(state: string | undefined, column: LockColumn): LockCode {
  const reasonToCode = (reason: string | null, kind: "approval" | "review"): LockCode => {
    if (reason === null) return "ok";
    if (reason === CUSTOMER_REQUIREMENT_ADD_ONLY_REASON) return "cannot-turn-off";
    if (reason === CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON) return "needs-project";
    if (reason === `Locked: the change request has already reached the customer ${kind} step or later.`) return "gate-passed";
    throw new Error(`unrecognised reason ${reason}`);
  };
  switch (column) {
    case "project-change":
    case "project-set":
      // Whether the change request has a project makes no difference to whether it can be changed.
      return customerProjectLockedReason(state) === CUSTOMER_PROJECT_FROZEN_REASON ? "frozen" : "ok";
    case "approval-on":
      return reasonToCode(customerApprovalLockedReason(state, { stored: false, hasProject: true }), "approval");
    case "approval-on-bare":
      return reasonToCode(customerApprovalLockedReason(state, { stored: false, hasProject: false }), "approval");
    case "approval-off":
      return reasonToCode(customerApprovalLockedReason(state, { stored: true, hasProject: true }), "approval");
    case "review-on":
      return reasonToCode(customerReviewLockedReason(state, { stored: false, hasProject: true }), "review");
    case "review-on-bare":
      return reasonToCode(customerReviewLockedReason(state, { stored: false, hasProject: false }), "review");
    case "review-off":
      return reasonToCode(customerReviewLockedReason(state, { stored: true, hasProject: true }), "review");
  }
}

const LOCK_ROWS: Array<[id: string, state: string | undefined, column: LockColumn, want: LockCode]> = Object.entries(
  LOCK_TABLE,
).flatMap(([STATE, codes]) =>
  LOCK_COLUMNS.map((column, i): [string, string | undefined, LockColumn, LockCode] => [
    `${STATE}/${column}`,
    STATE === "NULL" ? undefined : STATE.toLowerCase(),
    column,
    codes[i]!,
  ]),
);

describe("customer approval / review edit rule (the table the backend carries too)", () => {
  it("has a row for every state of a change request, and one for none recorded", () => {
    expect(Object.keys(LOCK_TABLE).sort()).toEqual(
      ["ASSESS", "AUTHORIZE", "CANCELED", "CLOSED", "CUSTOMER_APPROVAL", "CUSTOMER_REVIEW", "IMPLEMENT", "NEW", "NULL", "REVIEW", "ROLLBACK", "SCHEDULED"],
    );
    for (const codes of Object.values(LOCK_TABLE)) expect(codes).toHaveLength(LOCK_COLUMNS.length);
    expect(LOCK_ROWS).toHaveLength(12 * 8);
    expect(new Set(LOCK_ROWS.map((r) => r[0])).size).toBe(LOCK_ROWS.length);
  });

  it.each(LOCK_ROWS)("%s", (_id, state, column, want) => {
    expect(lockOutcome(state, column)).toBe(want);
  });

  it("gives the reason the dialog shows for each refusal, word for word", () => {
    expect(CUSTOMER_PROJECT_FROZEN_REASON).toBe("Fixed when approval was requested. Cancel and clone to change it.");
    expect(CUSTOMER_REQUIREMENT_ADD_ONLY_REASON).toBe(
      "Once approval has been requested a customer requirement can be added but never removed.",
    );
    expect(CUSTOMER_REQUIREMENT_NEEDS_PROJECT_REASON).toBe(
      "Needs a Customer Project, which can no longer be set. Cancel and clone.",
    );
    expect(customerApprovalLockedReason("scheduled", { stored: false, hasProject: true })).toBe(
      "Locked: the change request has already reached the customer approval step or later.",
    );
    expect(customerReviewLockedReason("closed", { stored: false, hasProject: true })).toBe(
      "Locked: the change request has already reached the customer review step or later.",
    );
  });

  it("a ticked box can never be unticked, so a Re-schedule (which never leaves Customer Approval) asks the same contacts again", () => {
    // A change request reaches Customer Approval with the box ticked (Authorize is the
    // state an older change that was re-scheduled before the CAB loop was removed may
    // still sit in), and every state after New refuses to untick it.
    for (const state of ["authorize", "customer_approval", "scheduled"]) {
      expect(customerApprovalLockedReason(state, { stored: true, hasProject: true })).toBe(
        CUSTOMER_REQUIREMENT_ADD_ONLY_REASON,
      );
    }
    for (const state of ["customer_review", "closed", "rollback"]) {
      expect(customerReviewLockedReason(state, { stored: true, hasProject: true })).toBe(
        CUSTOMER_REQUIREMENT_ADD_ONLY_REASON,
      );
    }
  });
});

describe("customerRequirementOnceSavedHelper", () => {
  it("warns that an addable requirement cannot be removed, only after New and only while unticked", () => {
    expect(customerRequirementOnceSavedHelper("assess", false)).toBe(CUSTOMER_REQUIREMENT_ONCE_SAVED_HELPER);
    expect(customerRequirementOnceSavedHelper("authorize", false)).toBe("Once saved this can't be removed.");
    expect(customerRequirementOnceSavedHelper("new", false)).toBeNull();
    expect(customerRequirementOnceSavedHelper(undefined, false)).toBeNull();
    expect(customerRequirementOnceSavedHelper("assess", true)).toBeNull();
  });
});

describe("the Customer Project is fixed once approval was requested", () => {
  it("is editable in New (and when no state is recorded yet)", () => {
    expect(isChangeRequestCreationPhase("new")).toBe(true);
    expect(isChangeRequestCreationPhase(undefined)).toBe(true);
    expect(isChangeRequestCreationPhase(null)).toBe(true);
    expect(customerProjectLockedReason("new")).toBeNull();
    expect(customerProjectLockedReason(undefined)).toBeNull();
  });

  it.each(["assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"])(
    "is read-only in %s, with the reason",
    (state) => {
      expect(isChangeRequestCreationPhase(state)).toBe(false);
      expect(customerProjectLockedReason(state)).toBe("Fixed when approval was requested. Cancel and clone to change it.");
    },
  );
});

describe("requestApprovalNeedsProjectReason", () => {
  const withProject = { id: "p1", name: "Acme" };
  it("blocks Request Approval when a customer box is ticked and there is no Customer Project", () => {
    expect(requestApprovalNeedsProjectReason({ state: "new", customerApprovalRequired: true })).toBe(
      "Select a Customer Project before requesting approval",
    );
    expect(requestApprovalNeedsProjectReason({ state: "new", customerReviewRequired: true })).toBe(
      REQUEST_APPROVAL_NEEDS_PROJECT_REASON,
    );
    expect(
      requestApprovalNeedsProjectReason({ state: "new", customerApprovalRequired: true, customerReviewRequired: true }),
    ).toBe(REQUEST_APPROVAL_NEEDS_PROJECT_REASON);
  });

  it("does not block when there is a project, or when no customer part is required", () => {
    expect(requestApprovalNeedsProjectReason({ state: "new", customerApprovalRequired: true, project: withProject })).toBeNull();
    expect(requestApprovalNeedsProjectReason({ state: "new", customerReviewRequired: true, project: withProject })).toBeNull();
    expect(requestApprovalNeedsProjectReason({ state: "new" })).toBeNull();
    expect(requestApprovalNeedsProjectReason({ state: "new", customerApprovalRequired: false, customerReviewRequired: false })).toBeNull();
  });

  it("is about the move out of New only", () => {
    expect(requestApprovalNeedsProjectReason({ state: "assess", customerApprovalRequired: true })).toBeNull();
  });

  describe("an Emergency change (the backend ignores its two customer boxes: nobody is asked, so no project is needed)", () => {
    it("never blocks, whichever box is still ticked on the stored row and whether or not it has a project", () => {
      for (const flags of [
        { customerApprovalRequired: true },
        { customerReviewRequired: true },
        { customerApprovalRequired: true, customerReviewRequired: true },
      ]) {
        expect(requestApprovalNeedsProjectReason({ state: "new", type: "emergency", ...flags }), JSON.stringify(flags)).toBeNull();
        expect(requestApprovalNeedsProjectReason({ state: "new", type: "emergency", project: withProject, ...flags }), JSON.stringify(flags)).toBeNull();
      }
    });

    it("reads the type the way the rest of the page does: case and padding ignored", () => {
      for (const type of ["Emergency", "EMERGENCY", " emergency "]) {
        expect(requestApprovalNeedsProjectReason({ state: "new", type, customerApprovalRequired: true }), type).toBeNull();
      }
    });

    it("still blocks every other change with a ticked box and no project: Normal, Standard, no type recorded or an unknown one", () => {
      for (const type of ["normal", "Normal", "standard", "emergency-ish", "", null, undefined]) {
        expect(requestApprovalNeedsProjectReason({ state: "new", type, customerApprovalRequired: true }), String(type)).toBe(
          REQUEST_APPROVAL_NEEDS_PROJECT_REASON,
        );
        expect(requestApprovalNeedsProjectReason({ state: "new", type, customerReviewRequired: true }), String(type)).toBe(
          REQUEST_APPROVAL_NEEDS_PROJECT_REASON,
        );
      }
    });
  });
});

describe("requestApprovalNeedsContactReason", () => {
  const project = { id: "p1", name: "Example Corp Platform" };
  const contact = { id: "k1", name: "Mia Member", email: "mia.member@example.com" };

  it("blocks Request Approval when a customer box is ticked and the project has no registered contact (the page knows the list is empty)", () => {
    expect(requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: true, project, customerContacts: [] })).toBe(
      "Register a contact for the Customer Project before requesting approval",
    );
    expect(requestApprovalNeedsContactReason({ state: "new", customerReviewRequired: true, project, customerContacts: [] })).toBe(
      REQUEST_APPROVAL_NEEDS_CONTACT_REASON,
    );
    expect(
      requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: true, customerReviewRequired: true, project, customerContacts: [] }),
    ).toBe(REQUEST_APPROVAL_NEEDS_CONTACT_REASON);
    // A change with no state recorded yet is in the creation phase too.
    expect(requestApprovalNeedsContactReason({ customerApprovalRequired: true, project, customerContacts: [] })).toBe(REQUEST_APPROVAL_NEEDS_CONTACT_REASON);
  });

  it("does not block while the project has a registered contact: the requester-only case is the backend's refusal, which the page cannot tell apart", () => {
    expect(requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: true, project, customerContacts: [contact] })).toBeNull();
    expect(requestApprovalNeedsContactReason({ state: "new", customerReviewRequired: true, project, customerContacts: [contact] })).toBeNull();
  });

  it("does not block when no customer part is required, however empty the group is", () => {
    expect(requestApprovalNeedsContactReason({ state: "new", project, customerContacts: [] })).toBeNull();
    expect(
      requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: false, customerReviewRequired: false, project, customerContacts: [] }),
    ).toBeNull();
  });

  it("claims nothing while the contacts are not known (not in the payload: another data source)", () => {
    expect(requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: true, project })).toBeNull();
    expect(requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: true, project, customerContacts: undefined })).toBeNull();
  });

  it("leaves a change with no Customer Project to its own reason, not this one", () => {
    expect(requestApprovalNeedsContactReason({ state: "new", customerApprovalRequired: true, customerContacts: [] })).toBeNull();
    expect(requestApprovalNeedsProjectReason({ state: "new", customerApprovalRequired: true })).toBe(REQUEST_APPROVAL_NEEDS_PROJECT_REASON);
  });

  it("is about the move out of New only", () => {
    for (const state of ["assess", "authorize", "customer_approval", "scheduled", "review", "customer_review", "closed", "canceled", "rollback"]) {
      expect(requestApprovalNeedsContactReason({ state, customerApprovalRequired: true, project, customerContacts: [] }), state).toBeNull();
    }
  });

  describe("an Emergency change (the backend ignores its two customer boxes: nobody is asked, so nobody is missing)", () => {
    it("never blocks for an empty contact list, whichever box is still ticked on the stored row", () => {
      for (const flags of [
        { customerApprovalRequired: true },
        { customerReviewRequired: true },
        { customerApprovalRequired: true, customerReviewRequired: true },
      ]) {
        expect(requestApprovalNeedsContactReason({ state: "new", type: "emergency", project, customerContacts: [], ...flags }), JSON.stringify(flags)).toBeNull();
      }
      // No state recorded yet is the creation phase too.
      expect(requestApprovalNeedsContactReason({ type: "emergency", customerApprovalRequired: true, project, customerContacts: [] })).toBeNull();
    });

    it("reads the type the way the rest of the page does: case and padding ignored", () => {
      for (const type of ["Emergency", "EMERGENCY", " emergency "]) {
        expect(requestApprovalNeedsContactReason({ state: "new", type, customerApprovalRequired: true, project, customerContacts: [] }), type).toBeNull();
      }
    });

    it("still blocks every other change with a ticked box and a project with no registered contact: Normal, Standard, no type recorded or an unknown one", () => {
      for (const type of ["normal", "Normal", "standard", "emergency-ish", "", null, undefined]) {
        expect(
          requestApprovalNeedsContactReason({ state: "new", type, customerApprovalRequired: true, project, customerContacts: [] }),
          String(type),
        ).toBe(REQUEST_APPROVAL_NEEDS_CONTACT_REASON);
        expect(
          requestApprovalNeedsContactReason({ state: "new", type, customerReviewRequired: true, project, customerContacts: [] }),
          String(type),
        ).toBe(REQUEST_APPROVAL_NEEDS_CONTACT_REASON);
      }
    });
  });
});

describe("CHANGE_REQUEST_CREATE_TYPE_OPTIONS", () => {
  it("offers exactly Normal, Standard, Emergency, in that order, with the backend enum values", () => {
    expect(CHANGE_REQUEST_CREATE_TYPE_OPTIONS.map((o) => [o.value, o.label])).toEqual([
      ["normal", "Normal"],
      ["standard", "Standard"],
      ["emergency", "Emergency"],
    ]);
    CHANGE_REQUEST_CREATE_TYPE_OPTIONS.forEach((o) => expect(o.description.length).toBeGreaterThan(10));
  });

  it("only treats those three values as creatable", () => {
    expect(isCreatableChangeRequestType("normal")).toBe(true);
    expect(isCreatableChangeRequestType("standard")).toBe(true);
    expect(isCreatableChangeRequestType("emergency")).toBe(true);
    expect(isCreatableChangeRequestType("azure")).toBe(false);
    expect(isCreatableChangeRequestType("model")).toBe(false);
    expect(isCreatableChangeRequestType("")).toBe(false);
    expect(isCreatableChangeRequestType(undefined)).toBe(false);
  });
});

describe("the Emergency change type", () => {
  it("is recognised by the backend's enum value, however it is cased or padded", () => {
    expect(isEmergencyChangeRequestType("emergency")).toBe(true);
    expect(isEmergencyChangeRequestType(" Emergency ")).toBe(true);
    expect(isEmergencyChangeRequestType("normal")).toBe(false);
    expect(isEmergencyChangeRequestType("standard")).toBe(false);
    expect(isEmergencyChangeRequestType("")).toBe(false);
    expect(isEmergencyChangeRequestType(null)).toBe(false);
    expect(isEmergencyChangeRequestType(undefined)).toBe(false);
  });

  it("says in one line why the two customer boxes are off", () => {
    expect(EMERGENCY_CUSTOMER_STEPS_HELPER).toBe("Emergency changes proceed without customer approval or review.");
  });

  describe("a customer step is not applicable to an Emergency change (the flow never asks the customer), unless the record shows it went there", () => {
    const emergency = { type: "emergency", state: "authorize" };

    it("reads Not applicable on an Emergency change, whatever its stored boxes hold", () => {
      expect(CUSTOMER_STEP_NOT_APPLICABLE).toBe("Not applicable");
      for (const state of ["new", "authorize", "scheduled", "implement", "review", "closed", "canceled", "rollback"]) {
        expect(isCustomerStepNotApplicable({ type: "emergency", state }, "approval"), state).toBe(true);
        expect(isCustomerStepNotApplicable({ type: "emergency", state }, "review"), state).toBe(true);
      }
    });

    it("shows a step as it is when the change sits in that customer state", () => {
      expect(isCustomerStepNotApplicable({ type: "emergency", state: "customer_approval" }, "approval")).toBe(false);
      expect(isCustomerStepNotApplicable({ type: "emergency", state: "customer_approval" }, "review")).toBe(true);
      expect(isCustomerStepNotApplicable({ type: "emergency", state: "customer_review" }, "review")).toBe(false);
      expect(isCustomerStepNotApplicable({ type: "emergency", state: "customer_review" }, "approval")).toBe(true);
    });

    it("shows a step as it is when the customer's outcome is on record", () => {
      expect(isCustomerStepNotApplicable({ ...emergency, hasCustomerApproved: true }, "approval")).toBe(false);
      expect(isCustomerStepNotApplicable({ ...emergency, hasCustomerApproved: true }, "review")).toBe(true);
      expect(isCustomerStepNotApplicable({ ...emergency, hasCustomerReviewed: true }, "review")).toBe(false);
    });

    it("shows a step as it is when a stage row of that gate exists, however the backend spells it", () => {
      expect(isCustomerStepNotApplicable(emergency, "approval", [{ stage: "Customer Approval" }])).toBe(false);
      expect(isCustomerStepNotApplicable(emergency, "approval", [{ stage: "customer_approval" }])).toBe(false);
      expect(isCustomerStepNotApplicable(emergency, "review", [{ stage: "Customer Review" }])).toBe(false);
      expect(isCustomerStepNotApplicable(emergency, "review", [{ stage: "Customer Approval" }])).toBe(true);
      expect(isCustomerStepNotApplicable(emergency, "approval", [{ stage: "CAB Approval" }, { stage: "ECAB Approval" }])).toBe(true);
      expect(isCustomerStepNotApplicable(emergency, "approval", [])).toBe(true);
    });

    it("never applies to any other type: Normal, Standard and the rest read Yes / No", () => {
      for (const type of ["normal", "standard", "model", undefined, null]) {
        expect(isCustomerStepNotApplicable({ type, state: "authorize" }, "approval"), String(type)).toBe(false);
        expect(isCustomerStepNotApplicable({ type, state: "authorize" }, "review"), String(type)).toBe(false);
      }
    });
  });
});

describe("isChangeRequestCreator", () => {
  it("matches the requester id against the user id", () => {
    expect(isChangeRequestCreator({ requestedBy: { id: "u-1" } }, { id: "u-1" })).toBe(true);
  });

  it("matches createdBy against the user id or email, case-insensitively", () => {
    expect(isChangeRequestCreator({ createdBy: "U-1" }, { id: "u-1" })).toBe(true);
    expect(isChangeRequestCreator({ createdBy: "Jane@Example.com" }, { email: "jane@example.com" })).toBe(true);
  });

  it("is false for someone else, an unloaded user, or a CR with no creator data", () => {
    expect(isChangeRequestCreator({ requestedBy: { id: "u-1" }, createdBy: "x" }, { id: "u-2", email: "b@x.com" })).toBe(false);
    expect(isChangeRequestCreator({ requestedBy: { id: "u-1" } }, undefined)).toBe(false);
    expect(isChangeRequestCreator({}, { id: "u-1" })).toBe(false);
    expect(isChangeRequestCreator({ requestedBy: null, createdBy: "" }, { id: "" })).toBe(false);
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

  it("is 1 when a project filter is set", () => {
    expect(
      countActiveCRFilters({ ...DEFAULT_CR_FILTERS, projectIds: ["proj-1"] }),
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

  it("sends selected projects as the named projectIds field", () => {
    expect(
      buildChangeRequestSearchFilters(
        { ...DEFAULT_CR_FILTERS, projectIds: ["proj-1", "proj-2"] },
        "",
      ),
    ).toEqual({ projectIds: ["proj-1", "proj-2"] });
  });

  it("omits projectIds entirely when no project is selected", () => {
    expect(buildChangeRequestSearchFilters(DEFAULT_CR_FILTERS, "")).not.toHaveProperty(
      "projectIds",
    );
  });
});

describe("change request category helpers", () => {
  it("offers the 13 ServiceNow categories, with 'other' as the default", () => {
    expect(CHANGE_REQUEST_CATEGORY_OPTIONS).toHaveLength(13);
    expect(DEFAULT_CHANGE_REQUEST_CATEGORY).toBe("other");
    expect(CHANGE_REQUEST_CATEGORY_OPTIONS.find((o) => o.value === "other")?.label).toBe("Other");
  });

  it("recognises only backend enum values", () => {
    expect(isChangeRequestCategory("devops")).toBe(true);
    expect(isChangeRequestCategory("Other")).toBe(false);
    expect(isChangeRequestCategory("")).toBe(false);
    expect(isChangeRequestCategory(undefined)).toBe(false);
  });

  it("reads the enum value off a detail category, whether it carries a name or a label", () => {
    expect(changeRequestCategoryValue({ id: "network", name: "Network" })).toBe("network");
    expect(changeRequestCategoryValue({ id: "network", label: "Network" })).toBe("network");
    expect(changeRequestCategoryValue("devops")).toBe("devops");
    expect(changeRequestCategoryValue({ id: "unknown" })).toBe("");
    expect(changeRequestCategoryValue("unknown")).toBe("");
    expect(changeRequestCategoryValue(null)).toBe("");
  });

  it("labels a category from the known list first, then from the backend, with a dash when absent", () => {
    expect(changeRequestCategoryLabel({ id: "regular_release_cloud", name: "x" })).toBe("Regular Release - Cloud");
    expect(changeRequestCategoryLabel({ id: "mystery", name: "Mystery" })).toBe("Mystery");
    expect(changeRequestCategoryLabel({ id: "mystery", label: "Old label" })).toBe("Old label");
    expect(changeRequestCategoryLabel("hotfix_release_cloud")).toBe("Hotfix Release - Cloud");
    expect(changeRequestCategoryLabel("Free text")).toBe("Free text");
    expect(changeRequestCategoryLabel(undefined)).toBe("—");
  });
});

describe("changeRequestScopeLockedReason (the deployments; the Customer Project is frozen earlier)", () => {
  it("is editable before implementation and locked from implement onwards", () => {
    for (const state of ["new", "assess", "authorize", "customer_approval", "scheduled"]) {
      expect(changeRequestScopeLockedReason(state)).toBeNull();
    }
    for (const state of ["implement", "review", "customer_review", "closed", "rollback", "canceled"]) {
      expect(changeRequestScopeLockedReason(state)).toMatch(/deployments can't be changed/);
    }
    expect(changeRequestScopeLockedReason(undefined)).toBeNull();
  });
});

describe("changeRequestBlockingReason — customer group stages", () => {
  const customerStage = (stage: string, status = "REQUESTED"): BeChangeRequestApproval => ({
    stage,
    approverType: "STATIC_GROUP",
    approverName: "Acme Reviewers",
    status,
    approvers: [],
  });

  it.each([
    ["Customer Approval", "Awaiting Customer Approval"],
    ["Customer Review", "Awaiting Customer Review"],
    ["customer_review", "Awaiting Customer Review"],
  ])("names a waiting %s stage '%s', never 'approval approval'", (stage, expected) => {
    const reason = changeRequestBlockingReason([customerStage(stage)], "implement");
    expect(reason).toBe(expected);
    expect(reason).not.toMatch(/approval approval/i);
  });

  it("uses the same wording from the state whether or not a customer stage exists", () => {
    expect(changeRequestBlockingReason([customerStage("Customer Approval")], "customer_approval")).toBe(
      "Awaiting Customer Approval",
    );
    expect(changeRequestBlockingReason([], "customer_approval")).toBe("Awaiting Customer Approval");
    expect(changeRequestBlockingReason([customerStage("Customer Review")], "customer_review")).toBe(
      "Awaiting Customer Review",
    );
    expect(changeRequestBlockingReason([], "customer_review")).toBe("Awaiting Customer Review");
  });
});

describe("noCustomerAskedHelper (the Approval tab's note: nobody is being asked at a customer gate)", () => {
  const GATES = ["customer_approval", "customer_review"] as const;
  // Approver rows in the shapes the backend sends them. Names are synthetic.
  const approver = (status: string, name = "Contact One") => ({ id: `u-${name}`, name, status });
  const stageOf = (stage: string, approvers: ReturnType<typeof approver>[], status = "REQUESTED"): BeChangeRequestApproval => ({
    stage,
    approverType: "STATIC_GROUP",
    approverName: stage,
    status,
    approvers,
  });
  const CONTACTS = [{ id: "c1", name: "Contact One" }];

  describe("a project with no registered contacts", () => {
    it.each(GATES)("says so at %s, before the approvals load, with what staff are left with", (state) => {
      expect(noCustomerAskedHelper(state, [])).toBe(`${NO_CUSTOMER_CONTACTS_HELPER} ${NOBODY_ASKED_WAY_OUT[state]}`);
      expect(noCustomerAskedHelper(state, null)).toBe(`${NO_CUSTOMER_CONTACTS_HELPER} ${NOBODY_ASKED_WAY_OUT[state]}`);
    });

    it.each(GATES)("says so at %s with the approvals loaded and nobody waiting", (state) => {
      expect(noCustomerAskedHelper(state, [], [])).toBe(`${NO_CUSTOMER_CONTACTS_HELPER} ${NOBODY_ASKED_WAY_OUT[state]}`);
      expect(noCustomerAskedHelper(state, [], [stageOf("Peer Approval", [approver("APPROVED")], "APPROVED")])).toContain(
        "No registered customer contacts are assigned",
      );
    });

    it.each(GATES)("is silent at %s when an old request still waits on somebody (only those asked may answer it)", (state) => {
      expect(noCustomerAskedHelper(state, [], [stageOf("Customer Approval", [approver("REQUESTED")])])).toBeNull();
    });
  });

  describe("a project with registered contacts, none of them waiting", () => {
    // With registered contacts a Re-schedule asks them afresh, so Customer Approval does not claim Cancel is the only exit.
    const withContacts = (state: (typeof GATES)[number]): string =>
      `${NOBODY_ASKED_HELPER} ${state === "customer_approval" ? NOBODY_ASKED_WAY_OUT_RESCHEDULE_MAY_HELP : NOBODY_ASKED_WAY_OUT.customer_review}`;

    it.each(GATES)("says nobody is asked at %s when the approvals hold no stage at all (a legacy change with no stage)", (state) => {
      expect(noCustomerAskedHelper(state, CONTACTS, [])).toBe(withContacts(state));
    });

    it.each(GATES)("says it at %s when every row is settled or cancelled (creator-only, deactivated contacts, a superseded request)", (state) => {
      const rows = [
        stageOf("Peer Approval", [approver("APPROVED", "Peer")], "APPROVED"),
        stageOf("CAB Approval", [approver("approved", "Cab")], "APPROVED"),
        stageOf("Customer Approval", [approver("CANCELLED"), approver("NOT_REQUIRED", "Contact Two")], "PENDING"),
      ];
      expect(noCustomerAskedHelper(state, CONTACTS, rows)).toBe(withContacts(state));
    });

    it.each(GATES)("is silent at %s once somebody is asked, whatever the stage is labelled", (state) => {
      expect(noCustomerAskedHelper(state, CONTACTS, [stageOf("Customer Approval", [approver("REQUESTED")])])).toBeNull();
      expect(noCustomerAskedHelper(state, CONTACTS, [stageOf("Customer Review", [approver(" requested ")])])).toBeNull();
      // A synced customer stage is labelled by its position (here as an internal stage's name): still somebody asked.
      expect(noCustomerAskedHelper(state, CONTACTS, [stageOf("Authorize", [approver("REQUESTED")])])).toBeNull();
    });

    it("is silent while the approvals are unknown (not loaded, or being reloaded): it never guesses", () => {
      for (const state of GATES) {
        expect(noCustomerAskedHelper(state, CONTACTS)).toBeNull();
        expect(noCustomerAskedHelper(state, CONTACTS, undefined)).toBeNull();
        expect(noCustomerAskedHelper(state, CONTACTS, null)).toBeNull();
      }
    });
  });

  it("is silent when the payload carries no customerContacts field at all (another data source: nothing is claimed)", () => {
    for (const state of GATES) {
      expect(noCustomerAskedHelper(state, undefined)).toBeNull();
      expect(noCustomerAskedHelper(state, undefined, [])).toBeNull();
    }
  });

  it("is silent outside the customer gates, whoever is registered and whoever is waiting", () => {
    for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review", "closed", "canceled", "rollback", undefined, null]) {
      expect(noCustomerAskedHelper(state, [])).toBeNull();
      expect(noCustomerAskedHelper(state, CONTACTS, [])).toBeNull();
    }
  });

  it("says what is going on, and does not promise that changing the project fixes it", () => {
    expect(NO_CUSTOMER_CONTACTS_HELPER).toMatch(
      /^No registered customer contacts are assigned to this change request's project, so no customer approvers were assigned\./,
    );
    expect(NO_CUSTOMER_CONTACTS_HELPER).not.toMatch(/changing the Customer Project and saving/i);
    expect(NO_CUSTOMER_CONTACTS_HELPER).toMatch(/fixed once approval is requested/i);
  });

  it("names the causes the web cannot tell apart (the requester alone, contacts no longer active, no request at all) without pinning one on the change", () => {
    expect(NOBODY_ASKED_HELPER).toMatch(/^Nobody is being asked to answer at this step\./);
    expect(NOBODY_ASKED_HELPER).toMatch(/leaving out whoever raised the change and anyone no longer active/);
    expect(NOBODY_ASKED_HELPER).toMatch(/no request at all/);
  });

  it("says plainly that Cancel change is the only way out of Customer Approval, and Roll back or Cancel change out of Customer Review", () => {
    expect(NOBODY_ASKED_WAY_OUT.customer_approval).toMatch(/Cancel change is the only way out/);
    // Re-schedule asks the contacts again at once (no trip back through approval), and is refused while nobody can be asked.
    expect(NOBODY_ASKED_WAY_OUT.customer_approval).toMatch(/Re-schedule would ask the same group again, and it is refused while nobody can be asked/);
    expect(NOBODY_ASKED_WAY_OUT.customer_approval).not.toMatch(/through approval/);
    expect(NOBODY_ASKED_WAY_OUT.customer_review).toMatch(/Roll back or Cancel change are the only ways out/);
    // Close is not an exit: neither text offers one.
    expect(NOBODY_ASKED_WAY_OUT.customer_review).not.toMatch(/\bclose\b/i);
  });

  it("does not claim Cancel is the only exit from Customer Approval where a Re-schedule might find somebody to ask (registered contacts, no request)", () => {
    // With no registered contacts a Re-schedule asks the same empty group: Cancel is the only way out, unconditionally.
    expect(noCustomerAskedHelper("customer_approval", [])).toMatch(/Cancel change is the only way out\. Re-schedule would ask/);
    // With contacts it says what Re-schedule does and when Cancel is the only way out.
    const text = noCustomerAskedHelper("customer_approval", CONTACTS, []) ?? "";
    expect(text).toMatch(/Re-schedule asks the project's registered contacts again at once/);
    expect(text).not.toMatch(/back through approval/);
    expect(text).toMatch(/helps only if someone can be asked this time \(it is refused when nobody can be\); if nobody can, Cancel change is the only way out/);
    expect(text).not.toMatch(/Cancel change is the only way out\. /);
  });

  it("never offers a bypass or a manual record of the customer's answer", () => {
    for (const text of [NO_CUSTOMER_CONTACTS_HELPER, NOBODY_ASKED_HELPER, NOBODY_ASKED_WAY_OUT_RESCHEDULE_MAY_HELP, ...Object.values(NOBODY_ASKED_WAY_OUT)]) {
      expect(text).not.toMatch(/bypass|recorded manually|answer for/i);
    }
    expect(NOBODY_ASKED_WAY_OUT.customer_approval).toMatch(/staff never record a customer's approval/i);
    expect(NOBODY_ASKED_WAY_OUT.customer_review).toMatch(/staff never record a customer's review/i);
  });
});

describe("anyApproverBeingAsked", () => {
  const row = (status: string) => ({ id: "u", name: "Contact", status });
  const stage = (...statuses: string[]): BeChangeRequestApproval => ({
    stage: "Customer Approval",
    approverType: "STATIC_GROUP",
    status: "REQUESTED",
    approvers: statuses.map(row),
  });

  it("is null while the approvals are not known", () => {
    expect(anyApproverBeingAsked(undefined)).toBeNull();
    expect(anyApproverBeingAsked(null)).toBeNull();
  });

  it("is true when any approver of any stage is REQUESTED, in any case and padding", () => {
    expect(anyApproverBeingAsked([stage("APPROVED"), stage("CANCELLED", "requested")])).toBe(true);
    expect(anyApproverBeingAsked([stage(" Requested ")])).toBe(true);
  });

  it("is false with no stage, no approver, or only settled / cancelled / not-required rows (a stage's own PENDING status does not count)", () => {
    expect(anyApproverBeingAsked([])).toBe(false);
    expect(anyApproverBeingAsked([stage()])).toBe(false);
    expect(anyApproverBeingAsked([stage("APPROVED", "REJECTED", "CANCELLED", "NOT_REQUIRED", "NOT_REQUESTED", "NOT_ENTITLED")])).toBe(false);
  });
});

describe("a time the customer proposed (the previous system's customer_updated_on / confirmation pair)", () => {
  const PROPOSAL = { startOn: "2030-03-08T09:00:00Z", endOn: "2030-03-08T11:00:00Z", answer: "pending" };
  const PLANNED = { plannedStartOn: "2030-03-01 09:00:00", plannedEndOn: "2030-03-01 11:00:00" };

  describe("pendingCustomerProposal", () => {
    it("is the backend's own verdict, and only in Customer Approval", () => {
      expect(pendingCustomerProposal({ state: "customer_approval", customerProposal: PROPOSAL })).toBe(PROPOSAL);
      for (const state of ["new", "assess", "authorize", "scheduled", "implement", "review", "customer_review", "closed", "canceled", "rollback", null, undefined]) {
        expect(pendingCustomerProposal({ state, customerProposal: PROPOSAL }), String(state)).toBeNull();
      }
    });

    it("is null for every answer but pending, and for a change with no proposal at all", () => {
      for (const answer of ["agreed", "disagreed", "unanswered", "something-new", ""]) {
        expect(pendingCustomerProposal({ state: "customer_approval", customerProposal: { ...PROPOSAL, answer } }), answer).toBeNull();
      }
      expect(pendingCustomerProposal({ state: "customer_approval" })).toBeNull();
      expect(pendingCustomerProposal({ state: "customer_approval", customerProposal: null })).toBeNull();
    });

    it("never infers a proposal from the raw columns: customerUpdatedOn alone is not one", () => {
      const cr = { state: "customer_approval", customerUpdatedOn: "2030-03-08T09:00:00Z", confirmCustomerUpdatedDate: null } as Pick<BeChangeRequestDetail, "state" | "customerProposal">;
      expect(pendingCustomerProposal(cr)).toBeNull();
    });
  });

  describe("customerProposalProposer", () => {
    it("names the proposer only when the backend named one", () => {
      expect(customerProposalProposer({ proposedByName: "Mia Member", proposedByEmail: "mia@example.com", proposedOn: "2030-02-01T10:00:00Z" })).toEqual({
        name: "Mia Member",
        email: "mia@example.com",
        on: "2030-02-01T10:00:00Z",
      });
      expect(customerProposalProposer({ proposedByEmail: "mia@example.com" })).toEqual({ name: undefined, email: "mia@example.com", on: undefined });
    });

    it("follows the backend's own verdict: proposerRecorded false names nobody, whatever else came with it; true with a name or an email names them", () => {
      expect(
        customerProposalProposer({ proposerRecorded: false, proposedByName: "Mia Member", proposedByEmail: "mia@example.com", proposedOn: "2030-02-01T10:00:00Z" }),
      ).toBeNull();
      expect(customerProposalProposer({ proposerRecorded: true, proposedByEmail: "mia@example.com" })).toEqual({
        name: undefined,
        email: "mia@example.com",
        on: undefined,
      });
      // `true` with nothing to show is still nobody to name: the page never prints an empty guess.
      expect(customerProposalProposer({ proposerRecorded: true })).toBeNull();
      expect(customerProposalProposer({ proposerRecorded: null, proposedByName: "Mia Member" })).toEqual({ name: "Mia Member", email: undefined, on: undefined });
    });

    it("is null when it is not recorded: no name, no email (blank counts as none), whatever 'proposedOn' says", () => {
      expect(customerProposalProposer({})).toBeNull();
      expect(customerProposalProposer({ proposedByName: "  ", proposedByEmail: "", proposedOn: "2030-02-01T10:00:00Z" })).toBeNull();
      expect(customerProposalProposer({ proposedByName: null, proposedByEmail: null })).toBeNull();
    });

    it("labels the proposer by name and email, or whichever there is", () => {
      expect(customerProposalProposerLabel({ name: "Mia Member", email: "mia@example.com" })).toBe("Mia Member (mia@example.com)");
      expect(customerProposalProposerLabel({ name: "Mia Member" })).toBe("Mia Member");
      expect(customerProposalProposerLabel({ email: "mia@example.com" })).toBe("mia@example.com");
    });

    it("says what it does not know, without guessing who or why, and never offers a way to accept it", () => {
      expect(PROPOSER_NOT_RECORDED_ACCEPT_REASON).toMatch(/^Nobody is recorded as having proposed this time/);
      expect(PROPOSER_NOT_RECORDED_ACCEPT_REASON).toMatch(/written by someone at WSO2 or left over from an earlier cycle/);
      expect(PROPOSER_NOT_RECORDED_ACCEPT_REASON).toMatch(/so it cannot be accepted/);
      expect(PROPOSER_NOT_RECORDED_ACCEPT_REASON).toMatch(/Propose a different time/);
      expect(STORED_TIME_ADVICE).toMatch(/no proposal to accept/);
      // The last writer is all that is on record, so a customer's own time followed by a staff edit reads the same way: the page says so.
      expect(STORED_TIME_ADVICE).toMatch(/edited after \(only the last edit is on record\)/);
      expect(STORED_TIME_ADVICE).toMatch(/still being asked to approve the planned time/);
      // Nothing there invites the engineer to check and accept it anyway.
      for (const text of [PROPOSER_NOT_RECORDED_ACCEPT_REASON, STORED_TIME_ADVICE]) expect(text).not.toMatch(/check that|before you accept|confirm/i);
      expect(storedTimeSentence("Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM")).toBe(
        "A time is stored (Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM) but nobody is recorded as having proposed it.",
      );
    });
  });

  describe("windows", () => {
    it("reads the planned window, and none when it has no length", () => {
      expect(plannedWindowMs(PLANNED)).toEqual({ startMs: Date.UTC(2030, 2, 1, 9), endMs: Date.UTC(2030, 2, 1, 11) });
      expect(plannedWindowMs({ plannedStartOn: "2030-03-01 09:00:00", plannedEndOn: "2030-03-01 09:00:00" })).toBeNull();
      expect(plannedWindowMs({ plannedStartOn: "2030-03-01 11:00:00", plannedEndOn: "2030-03-01 09:00:00" })).toBeNull();
      expect(plannedWindowMs({ plannedStartOn: "2030-03-01 09:00:00" })).toBeNull();
      expect(plannedWindowMs({})).toBeNull();
    });

    it("takes the proposed end from the backend, else keeps the planned length from the proposed start", () => {
      expect(proposedWindowMs(PLANNED, PROPOSAL)).toEqual({ startMs: Date.UTC(2030, 2, 8, 9), endMs: Date.UTC(2030, 2, 8, 11) });
      // No end sent: start + the planned length (2 hours).
      expect(proposedWindowMs(PLANNED, { startOn: "2030-03-08T10:30:00Z" })).toEqual({ startMs: Date.UTC(2030, 2, 8, 10, 30), endMs: Date.UTC(2030, 2, 8, 12, 30) });
      // No planned length to keep either: the start alone, no invented end.
      expect(proposedWindowMs({}, { startOn: "2030-03-08T10:30:00Z" })).toEqual({ startMs: Date.UTC(2030, 2, 8, 10, 30), endMs: null });
      expect(proposedWindowMs(PLANNED, { startOn: "not a date" })).toBeNull();
    });

    it("words a length", () => {
      expect(formatWindowLength(2 * 3_600_000)).toBe("2 hours");
      expect(formatWindowLength(3_600_000)).toBe("1 hour");
      expect(formatWindowLength(90 * 60_000)).toBe("1 hour 30 minutes");
      expect(formatWindowLength(45 * 60_000)).toBe("45 minutes");
      expect(formatWindowLength(26 * 3_600_000)).toBe("1 day 2 hours");
      expect(formatWindowLength(0)).toBe("");
      expect(formatWindowLength(-5)).toBe("");
      expect(formatWindowLength(Number.NaN)).toBe("");
    });
  });

  describe("acceptProposedTimeBlockedReason", () => {
    const now = Date.UTC(2030, 1, 1);
    // A customer is recorded as the proposer: the only kind of stored time that can be accepted.
    const WHO = { proposerRecorded: true, proposedByName: "Mia Member", proposedByEmail: "mia@example.com" };
    const RECORDED = { ...PROPOSAL, ...WHO };
    it("is null when the proposal is in the future, the change is not on hold and it has a planned window", () => {
      expect(acceptProposedTimeBlockedReason({ ...PLANNED, onHold: false }, RECORDED, now)).toBeNull();
      expect(acceptProposedTimeBlockedReason(PLANNED, RECORDED, now)).toBeNull();
    });

    it("is held back, before anything else, when nobody is recorded as having proposed the time: no staff action stands in for the customer's answer", () => {
      // The backend's own words when it sends them (with canAccept false), else the page's own, in the same words.
      const said = 'nobody is recorded as having proposed this time (it may have been written by someone at WSO2 or left over from an earlier cycle), so it cannot be accepted: use "Propose a different time" to ask the customer to approve a time';
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...PROPOSAL, canAccept: false, acceptBlockedReason: said }, now)).toBe(`${said.charAt(0).toUpperCase()}${said.slice(1)}.`);
      expect(acceptProposedTimeBlockedReason(PLANNED, PROPOSAL, now)).toBe(PROPOSER_NOT_RECORDED_ACCEPT_REASON);
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...PROPOSAL, proposerRecorded: false, proposedByName: "Mia Member" }, now)).toBe(PROPOSER_NOT_RECORDED_ACCEPT_REASON);
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...PROPOSAL, proposedByName: "  ", proposedByEmail: null }, now)).toBe(PROPOSER_NOT_RECORDED_ACCEPT_REASON);
      // Before every other reason, and whatever the other flags say (canAccept true included: the page's own reading is not overruled).
      expect(acceptProposedTimeBlockedReason({ ...PLANNED, onHold: true }, { startOn: "2020-01-01T00:00:00Z", canAccept: true }, now)).toBe(PROPOSER_NOT_RECORDED_ACCEPT_REASON);
      expect(acceptProposedTimeBlockedReason({}, PROPOSAL, now)).toBe(PROPOSER_NOT_RECORDED_ACCEPT_REASON);
    });

    it("says the change is on hold, before the other readings of the time", () => {
      expect(acceptProposedTimeBlockedReason({ ...PLANNED, onHold: true }, { ...WHO, startOn: "2020-01-01T00:00:00Z" }, now)).toBe(
        "This change request is on hold. Take it off hold first.",
      );
    });

    it("says the proposed time has passed, pointing at the other action", () => {
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...WHO, startOn: "2030-01-31T23:59:00Z" }, now)).toBe(
        "The proposed time has passed. Propose a different time.",
      );
      // Right now counts as passed: the backend refuses a start that is not after NOW().
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...WHO, startOn: new Date(now).toISOString() }, now)).toMatch(/has passed/);
    });

    it("holds Accept back on the backend's own verdict too, in its words, for what the page cannot tell", () => {
      const refused = "a proposal that the data says cannot be accepted right now";
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...RECORDED, canAccept: false, acceptBlockedReason: refused }, now)).toBe(
        "A proposal that the data says cannot be accepted right now.",
      );
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...RECORDED, canAccept: false, acceptBlockedReason: "Already a sentence." }, now)).toBe("Already a sentence.");
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...RECORDED, canAccept: false }, now)).toMatch(/would refuse this right now/);
      // canAccept true (or absent: an older backend) never holds it back by itself.
      expect(acceptProposedTimeBlockedReason(PLANNED, { ...RECORDED, canAccept: true }, now)).toBeNull();
      // The page's own reading comes first, in its shorter words.
      expect(acceptProposedTimeBlockedReason({ ...PLANNED, onHold: true }, { ...RECORDED, canAccept: false, acceptBlockedReason: refused }, now)).toBe(
        "This change request is on hold. Take it off hold first.",
      );
    });

    it("says there is no planned window to keep the length of", () => {
      expect(acceptProposedTimeBlockedReason({}, RECORDED, now)).toMatch(/no planned window whose length the proposed time could keep/);
      expect(acceptProposedTimeBlockedReason({ plannedStartOn: "2030-03-01 09:00:00", plannedEndOn: "2030-03-01 09:00:00" }, RECORDED, now)).toMatch(/no planned window/);
    });
  });

  describe("changeRequestBlockingReason with a proposal waiting", () => {
    const RECORDED = { ...PROPOSAL, proposerRecorded: true, proposedByName: "Mia Member", proposedByEmail: "mia@example.com" };
    const NOT_RECORDED = { ...PROPOSAL, proposerRecorded: false };

    it("reads that the change is waiting for WSO2, not for the customer, at Customer Approval only", () => {
      expect(CUSTOMER_PROPOSAL_WAITING_REASON).toBe("Waiting for WSO2 to respond to the customer's proposed time");
      expect(changeRequestBlockingReason([], "customer_approval", RECORDED)).toBe(CUSTOMER_PROPOSAL_WAITING_REASON);
      expect(changeRequestBlockingReason([], "customer_approval", null)).toBe("Awaiting Customer Approval");
      expect(changeRequestBlockingReason([], "customer_approval")).toBe("Awaiting Customer Approval");
      // The proposal is the page's `pendingCustomerProposal`, which is only ever set in Customer Approval; the other
      // customer gate and every approval stage keep their own words.
      expect(changeRequestBlockingReason([], "customer_review", RECORDED)).toBe("Awaiting Customer Review");
      expect(changeRequestBlockingReason([{ stage: "CAB Approval", approverType: "STATIC_GROUP", approverName: null, status: "REQUESTED", approvers: [] }], "authorize", RECORDED)).toBe("Awaiting CAB Approval");
    });

    it("a stored time nobody is recorded as having proposed is no proposal waiting for WSO2: the customer is still being asked, and the header says so", () => {
      // Nothing recorded at all, the backend's own "not recorded" (whatever name came with it), or a name that is blank.
      for (const stored of [
        PROPOSAL,
        NOT_RECORDED,
        { ...NOT_RECORDED, proposedByName: "Mia Member", proposedByEmail: "mia@example.com" },
        { ...PROPOSAL, proposedByName: "  ", proposedByEmail: null },
      ]) {
        expect(changeRequestBlockingReason([], "customer_approval", stored)).toBe("Awaiting Customer Approval");
      }
      // The other states are untouched.
      expect(changeRequestBlockingReason([], "customer_review", NOT_RECORDED)).toBe("Awaiting Customer Review");
    });

    it("names the proposer when a name or an email is recorded, without proposerRecorded", () => {
      expect(changeRequestBlockingReason([], "customer_approval", { ...PROPOSAL, proposedByName: "Mia Member" })).toBe(CUSTOMER_PROPOSAL_WAITING_REASON);
      expect(changeRequestBlockingReason([], "customer_approval", { ...PROPOSAL, proposedByEmail: "mia@example.com" })).toBe(CUSTOMER_PROPOSAL_WAITING_REASON);
    });
  });

  describe("customerProposalWording", () => {
    it("says 'the customer' only when the proposer is on record", () => {
      const named = customerProposalWording({ name: "Mia Member" });
      expect(named.bannerTitle).toBe("The customer proposed a new time");
      expect(named.windowLabel).toBe("Proposed by the customer");
      expect(named.counterLead("Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM")).toBe("The customer proposed Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM.");
      expect(named.isTheProposedTimeNote).toBe("That is the time the customer proposed. Close this and use Accept proposed time instead.");
    });

    it("says a time is stored, and that nobody proposed it, when nobody is on record: nothing waits for an answer", () => {
      const stored = customerProposalWording(null);
      expect(stored.bannerTitle).toBe("A time is stored on this change request");
      expect(stored.windowLabel).toBe("Stored time");
      expect(stored.counterLead("Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM")).toBe(
        "A time is stored (Mar 8, 2030, 9:00 AM to Mar 8, 2030, 11:00 AM) but nobody is recorded as having proposed it. There is no proposal to decline.",
      );
      // There is no "that is the customer's time, use Accept" note to give: nothing there can be accepted.
      expect(stored.isTheProposedTimeNote).toBe("");
      for (const text of [stored.bannerTitle, stored.windowLabel]) expect(text).not.toMatch(/customer|waiting|your answer/i);
    });

    it("follows customerProposalProposer: recorded -> customer wording, not recorded -> neutral", () => {
      const recorded = customerProposalProposer({ proposerRecorded: true, proposedByName: "Mia Member" });
      expect(customerProposalWording(recorded).bannerTitle).toBe("The customer proposed a new time");
      const notRecorded = customerProposalProposer({ proposerRecorded: false, proposedByName: "Mia Member" });
      expect(customerProposalWording(notRecorded).bannerTitle).toBe("A time is stored on this change request");
    });
  });

  describe("the refusals that mean the change moved on", () => {
    // A BackendApiError, as far as the helpers read it: a status and the payload the backend sent.
    const refusal = (status: number, errorCode?: string): { status: number; payload?: { errorCode?: string } } => ({
      status,
      payload: errorCode === undefined ? undefined : { errorCode },
    });

    it("reads the errorCode off the error's payload, and nothing else", () => {
      expect(backendErrorCode(refusal(409, "change_request_schedule_changed"))).toBe("change_request_schedule_changed");
      expect(backendErrorCode(refusal(409))).toBeUndefined();
      expect(backendErrorCode({ status: 409, payload: { errorCode: "" } })).toBeUndefined();
      expect(backendErrorCode({ status: 409, payload: { errorCode: 7 } })).toBeUndefined();
      expect(backendErrorCode({ status: 409, errorCode: "change_request_schedule_changed" })).toBeUndefined(); // not on the error itself
      expect(backendErrorCode(new Error("boom"))).toBeUndefined();
      for (const bad of [null, undefined, "change_request_schedule_changed", 409]) expect(backendErrorCode(bad)).toBeUndefined();
    });

    it("is stale for a 409 that names one of the codes, each of them", () => {
      expect(Object.values(ChangeRequestErrorCode).sort()).toEqual([
        "change_request_no_planned_window",
        "change_request_not_proposable",
        "change_request_on_hold",
        "change_request_proposal_not_now",
        "change_request_proposer_not_recorded",
        "change_request_schedule_changed",
      ]);
      for (const code of Object.values(ChangeRequestErrorCode)) expect(isStaleAnswerError(refusal(409, code)), code).toBe(true);
    });

    it("is not stale for an unknown or missing code, for another status, or for something that is not an error from the backend", () => {
      expect(isStaleAnswerError(refusal(409))).toBe(false);
      expect(isStaleAnswerError(refusal(409, "change_request_from_the_future"))).toBe(false);
      // The approvals' own and the 403 codes are not about a moved change.
      for (const code of ["change_request_approval_not_pending", "change_request_not_asked", "change_request_forbidden"]) {
        expect(isStaleAnswerError(refusal(409, code)), code).toBe(false);
      }
      // A code on a status other than 409 is not this refusal.
      for (const status of [400, 403, 404, 422, 500]) expect(isStaleAnswerError(refusal(status, ChangeRequestErrorCode.SCHEDULE_CHANGED)), String(status)).toBe(false);
      for (const bad of [null, undefined, "change_request_schedule_changed", new Error("boom")]) expect(isStaleAnswerError(bad)).toBe(false);
    });
  });

  describe("answerSnapshotMoved", () => {
    const cr = { state: "customer_approval", ...PLANNED };
    const proposal = { startOn: "2030-03-08T09:00:00Z", proposerRecorded: true, proposedByName: "Mia Member" };
    const shown = { cr, proposal };

    it("is false for what the dialog was opened on, however its times are spelled", () => {
      expect(answerSnapshotMoved(shown, shown)).toBe(false);
      expect(answerSnapshotMoved(shown, { cr: { ...cr, plannedStartOn: "2030-03-01T09:00:00Z", plannedEndOn: "2030-03-01T11:00:00Z" }, proposal })).toBe(false);
      expect(answerSnapshotMoved(shown, { cr, proposal: { ...proposal, startOn: "2030-03-08 09:00:00" } })).toBe(false);
      expect(answerSnapshotMoved({ cr, proposal: null }, { cr, proposal: null })).toBe(false);
    });

    it("is true when the state, the planned window, the proposal or who proposed it moved", () => {
      expect(answerSnapshotMoved(shown, { cr: { ...cr, state: "scheduled" }, proposal })).toBe(true);
      expect(answerSnapshotMoved(shown, { cr: { ...cr, plannedStartOn: "2030-03-02 09:00:00" }, proposal })).toBe(true);
      expect(answerSnapshotMoved(shown, { cr: { ...cr, plannedEndOn: "2030-03-01 12:00:00" }, proposal })).toBe(true);
      expect(answerSnapshotMoved(shown, { cr, proposal: { ...proposal, startOn: "2030-03-20T10:00:00Z" } })).toBe(true);
      expect(answerSnapshotMoved(shown, { cr, proposal: null })).toBe(true); // answered or gone
      expect(answerSnapshotMoved({ cr, proposal: null }, shown)).toBe(true); // proposed behind a plain Re-schedule
      expect(answerSnapshotMoved(shown, { cr, proposal: { startOn: proposal.startOn, proposerRecorded: false } })).toBe(true); // the proposer is no longer on record
    });
  });

  describe("customerApprovedDisplay (the Overview cell, the PDF row, the microapp row)", () => {
    const AFTER = ["scheduled", "implement", "review", "customer_review", "rollback", "closed", "canceled"];
    const BEFORE_OR_AT = ["new", "assess", "authorize", "customer_approval"];

    it("reads Yes when the customer's own approval was recorded, whatever else is on the record", () => {
      expect(customerApprovedDisplay({ hasCustomerApproved: true })).toBe("Yes");
      expect(customerApprovedDisplay({ hasCustomerApproved: true, state: "scheduled", customerProposal: { startOn: "x", answer: "agreed" } })).toBe("Yes");
      expect(customerApprovedDisplay({ hasCustomerApproved: true, state: "customer_approval", customerProposal: { startOn: "x", answer: "agreed" } })).toBe("Yes");
    });

    it("reads 'Proposed time accepted' when WSO2 accepted the customer's proposal, the change has moved on, and nothing was stamped (no staff action records the customer's approval)", () => {
      for (const state of AFTER) {
        expect(customerApprovedDisplay({ hasCustomerApproved: false, state, customerProposal: { startOn: "x", answer: "agreed" } }), state).toBe("Proposed time accepted");
      }
      expect(customerApprovedDisplay({ state: "scheduled", customerProposal: { startOn: "x", answer: "agreed" } })).toBe("Proposed time accepted");
      // The raw column says the same where the derived read model is absent (another data source).
      expect(customerApprovedDisplay({ hasCustomerApproved: false, state: "scheduled", confirmCustomerUpdatedDate: "agree" })).toBe("Proposed time accepted");
      expect(customerApprovedDisplay({ hasCustomerApproved: false, state: "closed", confirmCustomerUpdatedDate: " AGREE " })).toBe("Proposed time accepted");
    });

    // The answer stays on the row when the customers are asked again (a later Re-schedule): Approve and Reject are live in Customer Approval.
    it("reads No while the change is in Customer Approval, or before it, however the Agree is spelled: the customers are being asked", () => {
      for (const state of BEFORE_OR_AT) {
        expect(customerApprovedDisplay({ hasCustomerApproved: false, state, customerProposal: { startOn: "x", answer: "agreed" } }), state).toBe("No");
        expect(customerApprovedDisplay({ hasCustomerApproved: false, state, confirmCustomerUpdatedDate: "agree" }), state).toBe("No");
      }
      // A state this page does not know, or none, is not "moved on" either.
      expect(customerApprovedDisplay({ hasCustomerApproved: false, state: null, customerProposal: { startOn: "x", answer: "agreed" } })).toBe("No");
      expect(customerApprovedDisplay({ hasCustomerApproved: false, customerProposal: { startOn: "x", answer: "agreed" } })).toBe("No");
      expect(customerApprovedDisplay({ hasCustomerApproved: false, state: "somewhere_else" as never, confirmCustomerUpdatedDate: "agree" })).toBe("No");
    });

    it("reads No for everything else: no proposal, a pending one, a declined one, a history nobody answered", () => {
      expect(customerApprovedDisplay({})).toBe("No");
      expect(customerApprovedDisplay({ hasCustomerApproved: false })).toBe("No");
      for (const state of ["customer_approval", "scheduled"] as const) {
        for (const answer of ["pending", "disagreed", "unanswered"]) {
          expect(customerApprovedDisplay({ hasCustomerApproved: false, state, customerProposal: { startOn: "x", answer } }), `${state} ${answer}`).toBe("No");
        }
        expect(customerApprovedDisplay({ hasCustomerApproved: false, state, confirmCustomerUpdatedDate: "disagree" })).toBe("No");
      }
    });
  });
});
