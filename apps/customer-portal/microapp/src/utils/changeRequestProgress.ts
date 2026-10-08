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

/**
 * What the change request pages read of the customer's part of the process, kept free of React and of any
 * component so the rules can be tested on their own (`changeRequestProgress.test.ts`).
 */

/**
 * The API state ids a change request is in once it has moved on from Customer Approval (id 5): Scheduled (-2),
 * Implement (-1), Review (0), Customer Review (1), Rollback (2), Closed (3) and Canceled (4).
 */
export const CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL: readonly string[] = [
  "-2",
  "-1",
  "0",
  "1",
  "2",
  "3",
  "4",
];

/**
 * Whether WSO2 accepted a time the customer proposed AND the change request has moved on because of it
 * (Scheduled or later). Accepting schedules the change, so that is where it stands; the customer's approval flag
 * stays false (no staff action records a customer's approval: the proposal was their own consent).
 *
 * An `agreed` answer is only the answer WSO2 once gave. Nothing clears it when the customers are asked again, so a
 * change that is (back) in Customer Approval, or in a state not known here, with `agreed` standing was NOT
 * scheduled by that acceptance and is waiting for the customer's own answer: reading it as accepted would tell
 * the customer there is nothing left for them to do.
 */
export function isProposedTimeAccepted(answer: string | null | undefined, stateId: string | null | undefined): boolean {
  return answer === "agreed" && !!stateId && CHANGE_REQUEST_STATUS_IDS_PAST_CUSTOMER_APPROVAL.includes(stateId);
}

/** How a step of the progress timeline is drawn (the entry's own `status`). */
export type TimelineStepStatus = "completed" | "active" | "pending";

/**
 * The colour of the Customer Approval step's marker in the progress timeline, or `undefined` for the timeline's
 * ordinary one.
 *
 *  - Green once the customer's approval is on record or WSO2 accepted the time they proposed.
 *  - Nothing for a step the change has not reached yet (`pending`: a grey circle, never a colour that says
 *    "not approved").
 *  - Red only for a step the change request has passed (`completed`) because it was canceled without the
 *    customer's approval ever being given: the one ending in which "not approved" is a result, not just a step
 *    that has yet to happen. The timeline has no history
 *    of where a change was canceled, so it says no more than that.
 *  - Nothing for the rest: a change waiting for the customer is the current step like any other, and one that
 *    moved on without a recorded approval (none was required) is a completed step like any other.
 */
export function customerApprovalStepFill(input: {
  status: TimelineStepStatus;
  customerApproved: boolean;
  canceled: boolean;
}): "green" | "red" | undefined {
  if (input.status === "pending") return undefined;
  if (input.customerApproved) return "green";
  return input.canceled && input.status === "completed" ? "red" : undefined;
}

/** One step of the progress timeline as the view draws it (the entry's `status`, `fill`, `end` and `last`). */
export interface TimelineStepState {
  title: string;
  status: TimelineStepStatus;
  /** Only the Customer Approval step is coloured: see {@link customerApprovalStepFill}. */
  fill: "green" | "red" | undefined;
  /** The step is one of the three closing ones (Rollback, Closed, Canceled): its connector is not highlighted. */
  end: boolean;
  last: boolean;
}

/**
 * How every step of the progress timeline is drawn, given the steps' titles in order, the index of the one the change
 * request is in (`-1` when its state is not known) and whether the customer's approval counts as given (recorded, or WSO2
 * accepted the time they proposed). The three closing steps (Rollback, Closed, Canceled) are only ever the current one or
 * pending: a change request is in one of them, it does not pass through them.
 */
export function timelineStepStates(
  titles: readonly string[],
  activeIndex: number,
  customerApproved: boolean,
): TimelineStepState[] {
  const canceled = titles[activeIndex] === "Canceled";
  return titles.map((title, index) => {
    const status: TimelineStepStatus =
      index >= titles.length - 3
        ? index === activeIndex
          ? "active"
          : "pending"
        : index === activeIndex
          ? "active"
          : index < activeIndex
            ? "completed"
            : "pending";
    return {
      title,
      status,
      fill:
        title === "Customer Approval" ? customerApprovalStepFill({ status, customerApproved, canceled }) : undefined,
      end: index > titles.length - 4,
      last: index === titles.length - 1,
    };
  });
}
