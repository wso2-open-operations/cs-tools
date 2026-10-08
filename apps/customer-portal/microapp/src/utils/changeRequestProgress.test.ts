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
  CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL,
  customerApprovalStepFill,
  isProposedTimeAccepted,
  timelineStepStates,
  type TimelineStepStatus,
} from "./changeRequestProgress";

// The API state ids of a change request (the customer portal's own): New -5, Assess -4, Authorize -3, Customer Approval 5,
// Scheduled -2, Implement -1, Review 0, Customer Review 1, Rollback 2, Closed 3, Canceled 4.
const BEFORE_OR_AT_CUSTOMER_APPROVAL = ["-5", "-4", "-3", "5"];
const PAST_CUSTOMER_APPROVAL = ["-2", "-1", "0", "1", "2", "3", "4"];

describe("isProposedTimeAccepted", () => {
  it("is true when WSO2 agreed and the change request has moved on from Customer Approval, in each state after it", () => {
    expect([...CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL]).toEqual(PAST_CUSTOMER_APPROVAL);
    for (const stateId of PAST_CUSTOMER_APPROVAL) expect(isProposedTimeAccepted("agreed", stateId), stateId).toBe(true);
  });

  // An `agreed` answer is not cleared when the customers are asked again: back in Customer Approval, Approve and Reject are live.
  it("is false while the change request is in Customer Approval, or before it, whatever answer stands", () => {
    for (const stateId of BEFORE_OR_AT_CUSTOMER_APPROVAL)
      expect(isProposedTimeAccepted("agreed", stateId), stateId).toBe(false);
  });

  it("is false for a state that is not known, or none at all", () => {
    for (const stateId of ["", "99", "abc", null, undefined])
      expect(isProposedTimeAccepted("agreed", stateId), String(stateId)).toBe(false);
  });

  it("is false for every other answer, or no proposal, in any state", () => {
    for (const answer of ["pending", "disagreed", "unanswered", "", "AGREED", null, undefined]) {
      for (const stateId of [...BEFORE_OR_AT_CUSTOMER_APPROVAL, ...PAST_CUSTOMER_APPROVAL]) {
        expect(isProposedTimeAccepted(answer, stateId), `${String(answer)} in ${stateId}`).toBe(false);
      }
    }
  });
});

describe("customerApprovalStepFill (the Customer Approval step of the progress timeline)", () => {
  const STATUSES: TimelineStepStatus[] = ["completed", "active", "pending"];

  it("is green once the customer's approval is on record, or WSO2 accepted the time they proposed, whether the step is current or done", () => {
    for (const status of ["completed", "active"] as const) {
      expect(customerApprovalStepFill({ status, customerApproved: true, canceled: false }), status).toBe("green");
      // Approved and then canceled: the approval was given, so it stays green.
      expect(customerApprovalStepFill({ status, customerApproved: true, canceled: true }), `${status}, canceled`).toBe(
        "green",
      );
    }
  });

  it("has no colour for a step the change request has not reached yet: a grey circle, never red", () => {
    for (const customerApproved of [false, true]) {
      for (const canceled of [false, true]) {
        expect(customerApprovalStepFill({ status: "pending", customerApproved, canceled })).toBeUndefined();
      }
    }
  });

  it("is red only for a change request canceled without the customer's approval ever being given", () => {
    expect(customerApprovalStepFill({ status: "completed", customerApproved: false, canceled: true })).toBe("red");
  });

  it("is the ordinary colour, not red, while the change request waits for the customer or moved on with no approval recorded", () => {
    // Waiting for the customer: the current step like any other.
    expect(customerApprovalStepFill({ status: "active", customerApproved: false, canceled: false })).toBeUndefined();
    // Passed without a recorded approval (none was required): a completed step like any other.
    expect(customerApprovalStepFill({ status: "completed", customerApproved: false, canceled: false })).toBeUndefined();
  });

  it("covers every combination: green, red or nothing, and red nowhere but the one case", () => {
    const reds: string[] = [];
    for (const status of STATUSES) {
      for (const customerApproved of [false, true]) {
        for (const canceled of [false, true]) {
          const fill = customerApprovalStepFill({ status, customerApproved, canceled });
          expect([undefined, "green", "red"]).toContain(fill);
          if (fill === "red") reds.push(`${status}/${customerApproved}/${canceled}`);
        }
      }
    }
    expect(reds).toEqual(["completed/false/true"]);
  });
});

describe("timelineStepStates (what the view draws, step by step)", () => {
  // The steps of the customer portal's change request timeline, in order (`TIMELINE_META`).
  const TITLES = [
    "New",
    "Assess",
    "Authorize",
    "Customer Approval",
    "Scheduled",
    "Implement",
    "Review",
    "Customer Review",
    "Rollback",
    "Closed",
    "Canceled",
  ];
  const stepOf = (activeTitle: string | null, customerApproved: boolean, title = "Customer Approval") => {
    const states = timelineStepStates(TITLES, activeTitle ? TITLES.indexOf(activeTitle) : -1, customerApproved);
    return states.find((s) => s.title === title)!;
  };

  it("draws a change request that has not reached Customer Approval as a grey, uncoloured step: nothing says 'not approved' yet", () => {
    for (const active of ["New", "Assess", "Authorize"]) {
      for (const approved of [false, true]) {
        expect(stepOf(active, approved), `${active} ${approved}`).toMatchObject({ status: "pending", fill: undefined });
      }
    }
  });

  it("draws the step current and uncoloured while the change request waits for the customer, green once the approval counts as given", () => {
    expect(stepOf("Customer Approval", false)).toMatchObject({ status: "active", fill: undefined });
    expect(stepOf("Customer Approval", true)).toMatchObject({ status: "active", fill: "green" });
  });

  it("draws the step done and green for every change request that moved on with the approval given (recorded, or a proposed time WSO2 accepted)", () => {
    for (const active of ["Scheduled", "Implement", "Review", "Customer Review", "Rollback", "Closed", "Canceled"]) {
      expect(stepOf(active, true), active).toMatchObject({ status: "completed", fill: "green" });
    }
  });

  it("draws the step done and uncoloured for one that moved on with no approval recorded (none was required), red only when it was canceled", () => {
    for (const active of ["Scheduled", "Implement", "Review", "Customer Review", "Rollback", "Closed"]) {
      expect(stepOf(active, false), active).toMatchObject({ status: "completed", fill: undefined });
    }
    expect(stepOf("Canceled", false)).toMatchObject({ status: "completed", fill: "red" });
  });

  it("colours no other step, and an unknown state leaves every step pending", () => {
    for (const active of [null, ...TITLES]) {
      for (const approved of [false, true]) {
        const others = timelineStepStates(TITLES, active ? TITLES.indexOf(active) : -1, approved).filter(
          (s) => s.title !== "Customer Approval",
        );
        expect(
          others.every((s) => s.fill === undefined),
          `${active} ${approved}`,
        ).toBe(true);
      }
    }
    expect(timelineStepStates(TITLES, -1, true).every((s) => s.status === "pending")).toBe(true);
  });

  it("keeps the existing statuses: steps before the current one are done, the closing three are current or pending, never done", () => {
    const at = (title: string) => timelineStepStates(TITLES, TITLES.indexOf(title), false).map((s) => s.status);
    expect(at("Review")).toEqual([
      "completed",
      "completed",
      "completed",
      "completed",
      "completed",
      "completed",
      "active",
      "pending",
      "pending",
      "pending",
      "pending",
    ]);
    expect(at("Canceled")).toEqual([
      "completed",
      "completed",
      "completed",
      "completed",
      "completed",
      "completed",
      "completed",
      "completed",
      "pending",
      "pending",
      "active",
    ]);
    expect(at("Closed").slice(8)).toEqual(["pending", "active", "pending"]);
  });

  it("marks the closing three as the ends of the line, and the last step", () => {
    const states = timelineStepStates(TITLES, 0, false);
    expect(states.map((s) => s.end)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      true,
      true,
      true,
    ]);
    expect(states.map((s) => s.last)).toEqual([
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      false,
      true,
    ]);
  });
});
