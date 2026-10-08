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
import type { ChangeRequestDetailDto, ChangeRequestState } from "./changeRequest.dto";
import { isProposedTimeAccepted, toChangeRequestDetail } from "./changeRequest.model";

const AFTER_CUSTOMER_APPROVAL: ChangeRequestState[] = [
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "rollback",
  "closed",
  "canceled",
];
const BEFORE_OR_AT_CUSTOMER_APPROVAL: ChangeRequestState[] = ["new", "assess", "authorize", "customer_approval"];

describe("isProposedTimeAccepted", () => {
  it("is true when WSO2 agreed and the change request has moved on from Customer Approval, in every state after it", () => {
    for (const state of AFTER_CUSTOMER_APPROVAL) {
      expect(isProposedTimeAccepted({ state, customerProposal: { answer: "agreed" } }), state).toBe(true);
      // The raw column says the same where the derived read model is absent.
      expect(isProposedTimeAccepted({ state, confirmCustomerUpdatedDate: " Agree " }), state).toBe(true);
    }
  });

  // The Agree stays on the row when the customers are asked again (a later Re-schedule): in Customer Approval nothing was
  // scheduled by it and the customer's own answer is still to come.
  it("is false while the change request is in Customer Approval or before it, however the answer is spelled", () => {
    for (const state of BEFORE_OR_AT_CUSTOMER_APPROVAL) {
      expect(isProposedTimeAccepted({ state, customerProposal: { answer: "agreed" } }), state).toBe(false);
      expect(isProposedTimeAccepted({ state, confirmCustomerUpdatedDate: "agree" }), state).toBe(false);
    }
  });

  it("is false for no state at all, whatever answer stands", () => {
    expect(isProposedTimeAccepted({ state: null, customerProposal: { answer: "agreed" } })).toBe(false);
    expect(isProposedTimeAccepted({ state: null, confirmCustomerUpdatedDate: "agree" })).toBe(false);
  });

  it("is false for every other answer, or none, in any state", () => {
    for (const state of [...BEFORE_OR_AT_CUSTOMER_APPROVAL, ...AFTER_CUSTOMER_APPROVAL]) {
      for (const answer of ["pending", "disagreed", "unanswered", "", null, undefined]) {
        expect(isProposedTimeAccepted({ state, customerProposal: { answer } }), `${state} ${String(answer)}`).toBe(
          false,
        );
      }
      expect(isProposedTimeAccepted({ state, confirmCustomerUpdatedDate: "disagree" }), state).toBe(false);
      expect(isProposedTimeAccepted({ state, confirmCustomerUpdatedDate: null }), state).toBe(false);
      expect(isProposedTimeAccepted({ state }), state).toBe(false);
      expect(isProposedTimeAccepted({ state, customerProposal: null }), state).toBe(false);
    }
  });
});

describe("toChangeRequestDetail: proposedTimeAccepted", () => {
  // Synthetic: shapes only.
  const dto = (overrides: Partial<ChangeRequestDetailDto>): ChangeRequestDetailDto =>
    ({
      id: "chg-1",
      number: "CHG0001234",
      subject: "Upgrade the gateway cluster",
      project: { id: "p-1", name: "Example Corp Production" },
      case: null,
      deployment: null,
      product: null,
      assignedEngineer: null,
      assignedTeam: null,
      plannedStartOn: null,
      plannedEndOn: null,
      duration: null,
      impact: null,
      state: "scheduled",
      createdOn: "2026-01-01 00:00:00",
      updatedOn: "2026-01-01 00:00:00",
      createdBy: "Example User",
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
      approvedBy: null,
      approvedOn: null,
      ...overrides,
    }) as ChangeRequestDetailDto;

  it("is true for a change WSO2 scheduled by accepting the customer's proposal, and false once it is (back) in Customer Approval", () => {
    const agreed = { customerProposal: { answer: "agreed" } };
    expect(toChangeRequestDetail(dto({ ...agreed, state: "scheduled" })).proposedTimeAccepted).toBe(true);
    expect(toChangeRequestDetail(dto({ ...agreed, state: "closed" })).proposedTimeAccepted).toBe(true);
    expect(toChangeRequestDetail(dto({ ...agreed, state: "customer_approval" })).proposedTimeAccepted).toBe(false);
    expect(
      toChangeRequestDetail(dto({ confirmCustomerUpdatedDate: "agree", state: "customer_approval" }))
        .proposedTimeAccepted,
    ).toBe(false);
  });

  it("does not touch the customer's own approval flag", () => {
    expect(toChangeRequestDetail(dto({ hasCustomerApproved: true })).hasCustomerApproved).toBe(true);
    expect(toChangeRequestDetail(dto({ customerProposal: { answer: "agreed" } })).hasCustomerApproved).toBe(false);
  });
});
