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
  changeRequestToApiDatetime,
  changeRequestToDatetimeLocal,
  formatChangeRequestDuration,
  formatDuration,
  getAnsweredWindow,
  getChangeRequestDecisionMode,
  describeChangeRequestActionError,
  isAwaitingInternalReview,
  getCustomerDecisionLabels,
  getCustomerDecisionMessages,
  getCustomerProposal,
  getCustomerRejectConfirmCopy,
  getProposalNote,
  isProposalAccepted,
  isProposalNotAccepted,
  isProposalPending,
  buildChangeRequestWorkflowStages,
  mapChangeRequestStats,
  resolveCustomerDecisionMode,
  stripChangeRequestCustomTags,
  sumChangeRequestStateCount,
  AWAITING_YOUR_ACTION_STATE_IDS,
  AWAITING_LABELS,
  CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NO_WINDOW_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
  CHANGE_REQUEST_PROPOSAL_NOT_NOW_MESSAGE,
  CHANGE_REQUEST_ON_HOLD_MESSAGE,
  CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE,
  ChangeRequestErrorCode,
} from "@features/operations/utils/changeRequests";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";
import { ApiError } from "@utils/ApiError";

describe("changeRequests utils", () => {
  it("sumChangeRequestStateCount sums by id and label", () => {
    const total = sumChangeRequestStateCount(
      [
        { id: "5", label: "Customer Approval", count: 2 },
        { id: "", label: "Customer Review", count: 1 },
        { id: "9", label: "Other", count: 99 },
      ],
      AWAITING_YOUR_ACTION_STATE_IDS,
      AWAITING_LABELS,
    );
    expect(total).toBe(3);
  });

  it("mapChangeRequestStats maps API payload to card stats", () => {
    const stats = mapChangeRequestStats({
      stateCount: [{ id: "5", label: "Customer Approval", count: 4 }],
      totalCount: 10,
      resolvedCount: { total: 0, currentMonth: 0, pastThirtyDays: 0 },
    });
    expect(stats.awaitingYourAction).toBe(4);
    expect(stats.totalRequests).toBe(10);
  });

  it("mapChangeRequestStats files a change request waiting in Authorize under Ongoing, never under Awaiting Your Action", () => {
    // The shape backend-v2 sends: Authorize is {id: -3, label: Authorize}; New and
    // Assess are not sent at all.
    const stats = mapChangeRequestStats({
      stateCount: [
        { id: "-3", label: "Authorize", count: 2 },
        { id: "5", label: "Customer Approval", count: 1 },
        { id: "-2", label: "Scheduled", count: 3 },
        { id: "3", label: "Closed", count: 4 },
      ],
      totalCount: 10,
      resolvedCount: { total: 4, currentMonth: 0, pastThirtyDays: 0 },
    });
    expect(stats.ongoing).toBe(5);
    expect(stats.awaitingYourAction).toBe(1);
    expect(stats.completed).toBe(4);
    expect(stats.totalRequests).toBe(10);
    expect(stats.ongoing + stats.awaitingYourAction + stats.completed).toBe(stats.totalRequests);
  });

  it("mapChangeRequestStats counts Authorize by label too when the API carries no id", () => {
    const stats = mapChangeRequestStats({
      stateCount: [{ id: "", label: "Authorize", count: 2 }],
      totalCount: 2,
      resolvedCount: { total: 0, currentMonth: 0, pastThirtyDays: 0 },
    });
    expect(stats.ongoing).toBe(2);
  });

  it("formatChangeRequestDuration formats minutes", () => {
    expect(formatChangeRequestDuration(90)).toBe("1 hour 30 minutes");
    expect(formatChangeRequestDuration(45)).toBe("45 minutes");
  });

  it("formatDuration handles hour and minute segments", () => {
    expect(formatDuration(90)).toBe("1h 30m");
  });

  it("changeRequest datetime helpers round-trip wall time", () => {
    const api = changeRequestToApiDatetime("2026-06-01T10:30");
    expect(api).toMatch(/2026-06-01 10:30:00/);
    expect(changeRequestToDatetimeLocal(api)).toBe("2026-06-01T10:30");
  });

  it("stripChangeRequestCustomTags removes custom tags", () => {
    expect(stripChangeRequestCustomTags("[code]hello[/code]")).toBe("hello");
  });

  it("getChangeRequestDecisionMode detects customer approval state", () => {
    expect(
      getChangeRequestDecisionMode({
        state: { id: "5", label: "Customer Approval" },
      } as never),
    ).toBe(ChangeRequestDecisionMode.CUSTOMER_APPROVAL);
  });
});

describe("resolveCustomerDecisionMode", () => {
  const { CUSTOMER_APPROVAL, CUSTOMER_REVIEW, NONE } = ChangeRequestDecisionMode;
  const approval = { id: "5", label: "Customer Approval" };
  const review = { id: "1", label: "Customer Review" };

  // state x customerCanAnswer x hasCustomerApproved -> what the customer is offered
  const table: Array<{
    name: string;
    state: { id: string; label: string };
    customerCanAnswer: boolean | undefined;
    hasCustomerApproved: boolean | undefined;
    expected: ChangeRequestDecisionMode;
  }> = [
    { name: "approval, can answer, stamp unset", state: approval, customerCanAnswer: true, hasCustomerApproved: false, expected: CUSTOMER_APPROVAL },
    { name: "approval, can answer, stamp set", state: approval, customerCanAnswer: true, hasCustomerApproved: true, expected: CUSTOMER_APPROVAL },
    { name: "approval, cannot answer, stamp set", state: approval, customerCanAnswer: false, hasCustomerApproved: true, expected: NONE },
    { name: "approval, cannot answer, stamp unset", state: approval, customerCanAnswer: false, hasCustomerApproved: false, expected: NONE },
    { name: "approval, unknown, stamp set (legacy gate)", state: approval, customerCanAnswer: undefined, hasCustomerApproved: true, expected: CUSTOMER_APPROVAL },
    { name: "approval, unknown, stamp unset (legacy gate)", state: approval, customerCanAnswer: undefined, hasCustomerApproved: false, expected: NONE },
    { name: "approval, unknown, stamp missing", state: approval, customerCanAnswer: undefined, hasCustomerApproved: undefined, expected: NONE },
    { name: "review, can answer", state: review, customerCanAnswer: true, hasCustomerApproved: false, expected: CUSTOMER_REVIEW },
    { name: "review, unknown (no gate)", state: review, customerCanAnswer: undefined, hasCustomerApproved: false, expected: CUSTOMER_REVIEW },
    { name: "review, cannot answer", state: review, customerCanAnswer: false, hasCustomerApproved: true, expected: NONE },
    { name: "scheduled, can answer", state: { id: "-2", label: "Scheduled" }, customerCanAnswer: true, hasCustomerApproved: true, expected: NONE },
    { name: "authorize, can answer", state: { id: "-3", label: "Authorize" }, customerCanAnswer: true, hasCustomerApproved: true, expected: NONE },
    { name: "closed, unknown", state: { id: "3", label: "Closed" }, customerCanAnswer: undefined, hasCustomerApproved: true, expected: NONE },
  ];

  it.each(table)("$name -> $expected", ({ state, customerCanAnswer, hasCustomerApproved, expected }) => {
    expect(
      resolveCustomerDecisionMode({
        state,
        customerCanAnswer,
        hasCustomerApproved,
      } as never),
    ).toBe(expected);
  });

  it("recognises the state by label when the id is missing", () => {
    expect(
      resolveCustomerDecisionMode({
        state: { label: "Customer Approval" },
        customerCanAnswer: true,
      } as never),
    ).toBe(CUSTOMER_APPROVAL);
  });

  it("treats a null customerCanAnswer (not a boolean) as unknown", () => {
    expect(
      resolveCustomerDecisionMode({
        state: approval,
        customerCanAnswer: null,
        hasCustomerApproved: true,
      } as never),
    ).toBe(CUSTOMER_APPROVAL);
  });

  it("offers nothing without a change request or a state", () => {
    expect(resolveCustomerDecisionMode(null)).toBe(NONE);
    expect(resolveCustomerDecisionMode(undefined)).toBe(NONE);
    expect(resolveCustomerDecisionMode({ customerCanAnswer: true } as never)).toBe(NONE);
  });
});

describe("customer decision copy", () => {
  it("labels the buttons for each stage", () => {
    expect(getCustomerDecisionLabels(ChangeRequestDecisionMode.CUSTOMER_APPROVAL)).toEqual({
      approve: "Approve",
      reject: "Reject",
    });
    expect(getCustomerDecisionLabels(ChangeRequestDecisionMode.CUSTOMER_REVIEW)).toEqual({
      approve: "Successful",
      reject: "Unsuccessful",
    });
  });

  it("states the consequence of rejecting or failing a review", () => {
    expect(getCustomerRejectConfirmCopy(ChangeRequestDecisionMode.CUSTOMER_APPROVAL).message).toBe(
      "Rejecting cancels this change request.",
    );
    expect(getCustomerRejectConfirmCopy(ChangeRequestDecisionMode.CUSTOMER_REVIEW).message).toBe(
      "Marking it unsuccessful sends the change into rollback.",
    );
  });

  describe("the hint about a different time follows what Propose New Time can do", () => {
    const approval = ChangeRequestDecisionMode.CUSTOMER_APPROVAL;

    it("points at Propose New Time only while it is on", () => {
      expect(getCustomerRejectConfirmCopy(approval, "available").hint).toBe(
        "If you only need a different time, go back and use Propose New Time instead.",
      );
    });

    it("says what is true instead while WSO2 has the change on hold, and does not point at the switched-off action", () => {
      const { hint } = getCustomerRejectConfirmCopy(approval, "on_hold");
      expect(hint).toBe("A new time cannot be proposed right now because WSO2 has this change request on hold.");
      expect(hint).not.toMatch(/use Propose New Time/);
    });

    it("says nothing about it where it is not offered, and by default", () => {
      expect(getCustomerRejectConfirmCopy(approval, "unavailable").hint).toBeUndefined();
      expect(getCustomerRejectConfirmCopy(approval).hint).toBeUndefined();
    });

    it("keeps the consequence the same whatever the hint says", () => {
      for (const availability of ["available", "on_hold", "unavailable"] as const) {
        expect(getCustomerRejectConfirmCopy(approval, availability).message).toBe(
          "Rejecting cancels this change request.",
        );
      }
    });

    it("has no hint for a review, whatever Propose New Time can do", () => {
      for (const availability of ["available", "on_hold", "unavailable"] as const) {
        expect(
          getCustomerRejectConfirmCopy(ChangeRequestDecisionMode.CUSTOMER_REVIEW, availability).hint,
        ).toBeUndefined();
      }
    });
  });

  it("says what happened to the change request after an answer", () => {
    const approval = ChangeRequestDecisionMode.CUSTOMER_APPROVAL;
    const review = ChangeRequestDecisionMode.CUSTOMER_REVIEW;
    expect(getCustomerDecisionMessages(approval, true).success).toBe(
      "Change request approved. It is now scheduled.",
    );
    expect(getCustomerDecisionMessages(approval, false).success).toBe(
      "Change request rejected. It has been canceled.",
    );
    expect(getCustomerDecisionMessages(review, true).success).toBe(
      "Change request marked as successful. It is now closed.",
    );
    expect(getCustomerDecisionMessages(review, false).success).toBe(
      "Change request marked as unsuccessful. It is now in rollback.",
    );
  });
});

describe("describeChangeRequestActionError", () => {
  const fallback = "Could not do it. Please try again.";

  // A refusal is classified by its machine-readable code, never by the wording of
  // its message: every case below gives the SAME message under a different wording
  // (and an empty one) and the same answer comes back.
  describe("by the refusal's code", () => {
    const wordings = [
      "stale approval: whatever",
      "some entirely different sentence",
      "this change request is on hold",
      "the planned implementation time of this change request changed after you opened it",
      "HTTP 409",
      "",
    ];

    it.each([
      [ChangeRequestErrorCode.APPROVAL_NOT_PENDING, { message: CHANGE_REQUEST_ANSWER_STALE_MESSAGE, terminal: true }],
      [ChangeRequestErrorCode.NOT_PROPOSABLE, { message: CHANGE_REQUEST_ANSWER_STALE_MESSAGE, terminal: true }],
      [ChangeRequestErrorCode.SCHEDULE_CHANGED, { message: CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE, terminal: true }],
      // A hold keeps the customer where they are: the answer is still possible.
      [ChangeRequestErrorCode.ON_HOLD, { message: CHANGE_REQUEST_ON_HOLD_MESSAGE, terminal: false }],
    ])("a 409 named %s", (code, expected) => {
      for (const wording of wordings) {
        expect(
          describeChangeRequestActionError(new ApiError(409, "Conflict", wording, undefined, code), fallback),
          `wording ${JSON.stringify(wording)}`,
        ).toEqual(expected);
      }
    });

    it.each([ChangeRequestErrorCode.NOT_ASKED, ChangeRequestErrorCode.FORBIDDEN])("a 403 named %s", (code) => {
      for (const wording of wordings) {
        expect(
          describeChangeRequestActionError(new ApiError(403, "Forbidden", wording, undefined, code), fallback),
        ).toEqual({ message: CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE, terminal: true });
      }
    });

    it("never takes a hold, a moved schedule or an answered request from the words alone", () => {
      // The old classification matched the English wording of the message: it must
      // not any more. These are the very sentences it matched, with a code that
      // says something else, and with none.
      const onHoldWords = "this change request is on hold, so a new implementation time cannot be proposed now";
      const movedWords =
        "the planned implementation time of this change request changed after you opened it (it is now 2026-09-15T10:00:00Z to 2026-09-15T12:00:00Z); read it again before giving your answer";
      expect(
        describeChangeRequestActionError(
          new ApiError(409, "Conflict", onHoldWords, undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING),
          fallback,
        ).message,
      ).toBe(CHANGE_REQUEST_ANSWER_STALE_MESSAGE);
      expect(
        describeChangeRequestActionError(
          new ApiError(409, "Conflict", movedWords, undefined, ChangeRequestErrorCode.ON_HOLD),
          fallback,
        ).message,
      ).toBe(CHANGE_REQUEST_ON_HOLD_MESSAGE);
      for (const words of [onHoldWords, movedWords]) {
        expect(describeChangeRequestActionError(new ApiError(409, "Conflict", words), fallback).message).toBe(
          CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
        );
      }
    });

    it("says only that something went wrong, and to refresh, for a code it does not know", () => {
      expect(
        describeChangeRequestActionError(
          new ApiError(409, "Conflict", "this approval is no longer pending", undefined, "change_request_from_the_future"),
          fallback,
        ),
      ).toEqual({ message: CHANGE_REQUEST_ACTION_FAILED_MESSAGE, terminal: true });
      expect(CHANGE_REQUEST_ACTION_FAILED_MESSAGE).toBe(
        "Something went wrong with this change request. Refresh the page to see where it stands, then try again.",
      );
    });

    it("keeps an older backend working: a 409 with no code is not called an answered request", () => {
      const result = describeChangeRequestActionError(
        new ApiError(409, "Conflict", "this approval is no longer pending: the change request is in Scheduled"),
        fallback,
      );
      expect(result).toEqual({ message: CHANGE_REQUEST_ACTION_FAILED_MESSAGE, terminal: true });
      expect(result.message).not.toBe(CHANGE_REQUEST_ANSWER_STALE_MESSAGE);
    });

    it("keeps an older backend working: a 403 with no code, or an unknown one, is still not a contact who can answer", () => {
      for (const code of [undefined, "change_request_from_the_future"]) {
        expect(
          describeChangeRequestActionError(new ApiError(403, "Forbidden", "You do not have permission.", undefined, code), fallback),
        ).toEqual({ message: CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE, terminal: true });
      }
      expect(CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE).toBe(
        "You are not one of the contacts who can answer this change request.",
      );
    });

    it("leaves a failure that is no refusal of the state to its status, whatever code rides on it", () => {
      expect(
        describeChangeRequestActionError(
          new ApiError(500, "Internal Server Error", "Failed to update change request.", undefined, ChangeRequestErrorCode.ON_HOLD),
          fallback,
        ),
      ).toEqual({ message: "Failed to update change request.", terminal: false });
      expect(
        describeChangeRequestActionError(
          new ApiError(400, "Bad Request", "plannedStartOn is in the past", undefined, ChangeRequestErrorCode.APPROVAL_NOT_PENDING),
          fallback,
        ),
      ).toEqual({ message: "The proposed time must be in the future.", terminal: false });
    });

    it("keeps the wire names the backends send", () => {
      expect(ChangeRequestErrorCode).toEqual({
        ON_HOLD: "change_request_on_hold",
        SCHEDULE_CHANGED: "change_request_schedule_changed",
        APPROVAL_NOT_PENDING: "change_request_approval_not_pending",
        NOT_PROPOSABLE: "change_request_not_proposable",
        PROPOSAL_NOT_NOW: "change_request_proposal_not_now",
        NO_PLANNED_WINDOW: "change_request_no_planned_window",
        NOT_ASKED: "change_request_not_asked",
        FORBIDDEN: "change_request_forbidden",
      });
    });

    it("the stale-answer text is unchanged", () => {
      expect(CHANGE_REQUEST_ANSWER_STALE_MESSAGE).toBe(
        "This request was already answered or is no longer waiting for your answer.",
      );
      expect(CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE).toMatch(/schedule .* changed after you opened it/);
    });
  });

  it("keeps the customer where they are when a proposal is refused because there is no window to move", () => {
    expect(
      describeChangeRequestActionError(
        new ApiError(
          409,
          "Conflict",
          "any wording at all",
          undefined,
          ChangeRequestErrorCode.NO_PLANNED_WINDOW,
        ),
        fallback,
      ),
    ).toEqual({ message: CHANGE_REQUEST_NO_WINDOW_MESSAGE, terminal: false });
    expect(CHANGE_REQUEST_NO_WINDOW_MESSAGE).toBe(
      "This change request has no planned time yet, so a new time cannot be proposed for it.",
    );
  });

  it("says only that a time cannot be proposed right now when an approval that is not the customer's is still asked, and names no stage", () => {
    const result = describeChangeRequestActionError(
      new ApiError(
        409,
        "Conflict",
        "any wording at all",
        undefined,
        ChangeRequestErrorCode.PROPOSAL_NOT_NOW,
      ),
      fallback,
    );
    expect(result).toEqual({ message: CHANGE_REQUEST_PROPOSAL_NOT_NOW_MESSAGE, terminal: false });
    expect(result.message).not.toMatch(/approval|CAB|internal/i);
  });

  it("never reads the wording of a proposal refusal: the same words with no code, or an unknown one, are the safe default", () => {
    for (const code of [undefined, "change_request_from_the_future"]) {
      for (const words of [
        "this change request has no planned window to move, so a new time cannot be proposed for it",
        "this change request is also waiting for an approval that is not the customer's, so a new time cannot be proposed for it right now",
      ]) {
        expect(
          describeChangeRequestActionError(new ApiError(409, "Conflict", words, undefined, code), fallback),
        ).toEqual({ message: CHANGE_REQUEST_ACTION_FAILED_MESSAGE, terminal: true });
      }
    }
  });

  it("puts the refusals of a start-only proposal in the customer's words", () => {
    const say = (backend: string) =>
      describeChangeRequestActionError(new ApiError(400, "Bad Request", backend), fallback);
    expect(say("a proposed implementation time needs a new start: send plannedStartOn")).toEqual({
      message: "Enter the proposed start date and time.",
      terminal: false,
    });
    expect(
      say("a proposed time moves the start and keeps the planned length of 2h0m0s: plannedEndOn must be 2030-03-01T11:00:00Z, or be left out").message,
    ).toBe("A proposed time moves the start and keeps the planned length. Choose a different start.");
    expect(
      say("plannedStartOn is the planned start already: propose a different start").message,
    ).toBe("This is the same as the current schedule. Choose a different start.");
    expect(
      say("that time is already proposed and is waiting for WSO2's response").message,
    ).toBe("That time is already proposed and is waiting for WSO2's response. Choose a different start.");
    expect(
      say("WSO2 asked for a different time than that one: propose another start").message,
    ).toBe("WSO2 asked for a different time than that one. Choose another start.");
    // The end the start implies would pass the last year the service accepts.
    expect(
      say("plannedStartOn is too far ahead: with the planned length of 2h0m0s the proposed window would end after the year 2100").message,
    ).toBe("That start is too far ahead. Choose an earlier start.");
  });

  it("puts the newer window refusals in the customer's words", () => {
    const say = (backend: string) =>
      describeChangeRequestActionError(new ApiError(400, "Bad Request", backend), fallback);
    expect(say("the planned start must not be the same as the planned end: the window must have a duration")).toEqual({
      message: "The proposed end must be after the proposed start.",
      terminal: false,
    });
    expect(say("plannedStartOn is in the past: a proposed implementation time must be one still to come").message).toBe(
      "The proposed time must be in the future.",
    );
    expect(
      say("plannedEndOn must be a valid date-time, either RFC 3339 (2030-03-01T09:00:00Z) or YYYY-MM-DD HH:MM:SS in UTC, in the years 2000 to 2100").message,
    ).toBe("Enter a valid start and end date and time.");
  });

  it("shows the backend's message for a bad request, in plainer words for the window ones", () => {
    expect(
      describeChangeRequestActionError(new ApiError(400, "Bad Request", "plannedStartOn must follow the format: YYYY-MM-DD HH:mm:ss"), fallback),
    ).toEqual({
      message: "plannedStartOn must follow the format: YYYY-MM-DD HH:mm:ss",
      terminal: false,
    });
    expect(
      describeChangeRequestActionError(new ApiError(400, "Bad Request", "the planned start must not be after the planned end"), fallback).message,
    ).toBe("The proposed end must be after the proposed start.");
    expect(
      describeChangeRequestActionError(
        new ApiError(400, "Bad Request", "re-scheduling requires a changed planned start or end: send plannedStartOn and/or plannedEndOn with a value different from the stored one"),
        fallback,
      ).message,
    ).toBe("This is the same as the current schedule. Choose a different start.");
  });

  it("explains an answer that was already locked in", () => {
    expect(
      describeChangeRequestActionError(
        new ApiError(400, "Bad Request", "isCustomerApproved is locked once set to true and cannot be reverted to false"),
        fallback,
      ).message,
    ).toBe("This answer has already been given and cannot be changed.");
  });

  it("shows the backend's message for other failures, or the fallback when it has none", () => {
    expect(
      describeChangeRequestActionError(new ApiError(500, "Internal Server Error", "Failed to update change request."), fallback),
    ).toEqual({ message: "Failed to update change request.", terminal: false });
    expect(
      describeChangeRequestActionError(new ApiError(500, "Internal Server Error", "Internal Server Error"), fallback).message,
    ).toBe(fallback);
    expect(
      describeChangeRequestActionError(new ApiError(502, "", "HTTP 502"), fallback).message,
    ).toBe(fallback);
  });

  it("uses the fallback for anything that is not an API error", () => {
    expect(describeChangeRequestActionError(new Error("Failed to fetch"), fallback)).toEqual({
      message: fallback,
      terminal: false,
    });
    expect(describeChangeRequestActionError("boom", fallback).message).toBe(fallback);
  });
});

describe("buildChangeRequestWorkflowStages", () => {
  it("shows Authorize as current for the raw state a customer's proposal leaves the change in", () => {
    const { workflowStages, currentStateIndex } = buildChangeRequestWorkflowStages({
      state: { label: "authorize" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    expect(workflowStages.find((s) => s.current)?.name).toBe("Authorize");
    expect(currentStateIndex).toBe(2);
    expect(workflowStages.filter((s) => s.current)).toHaveLength(1);
    expect(workflowStages[0]).toMatchObject({ name: "New", completed: true, current: false });
  });

  it("still marks Customer Approval current, with the old gate's stamp untouched", () => {
    const { workflowStages } = buildChangeRequestWorkflowStages({
      state: { id: "5", label: "Customer Approval" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    expect(workflowStages.find((s) => s.current)?.name).toBe("Customer Approval");
  });

  it("marks Customer Approval completed once moved past Customer Approval (e.g. Scheduled)", () => {
    const { workflowStages } = buildChangeRequestWorkflowStages({
      state: { id: "-2", label: "Scheduled" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    const customerApproval = workflowStages.find((s) => s.name === "Customer Approval");
    expect(customerApproval).toMatchObject({
      completed: true,
      current: false,
      disabled: false,
      description: "Customer approval received",
    });
  });

  it("marks Customer Approval and Customer Review completed when Closed", () => {
    const { workflowStages } = buildChangeRequestWorkflowStages({
      state: { id: "3", label: "Closed" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    const approval = workflowStages.find((s) => s.name === "Customer Approval");
    const review = workflowStages.find((s) => s.name === "Customer Review");
    expect(approval).toMatchObject({ completed: true, disabled: false });
    expect(review).toMatchObject({ completed: true, disabled: false });
  });

  it("marks Customer Review disabled and not completed when Rollback", () => {
    const { workflowStages } = buildChangeRequestWorkflowStages({
      state: { id: "-7", label: "Rollback" },
      hasCustomerApproved: false,
      hasCustomerReviewed: false,
    } as never);
    const approval = workflowStages.find((s) => s.name === "Customer Approval");
    const review = workflowStages.find((s) => s.name === "Customer Review");
    expect(approval).toMatchObject({ completed: true, disabled: false });
    expect(review).toMatchObject({ completed: false, disabled: true });
  });
});

describe("buildChangeRequestWorkflowStages and a proposed time", () => {
  const stageOf = (changeRequest: object, name: string) =>
    buildChangeRequestWorkflowStages(changeRequest as never).workflowStages.find((s) => s.name === name);
  const approval = { id: "5", label: "Customer Approval" };
  const scheduled = { id: "-2", label: "Scheduled" };
  const base = { hasCustomerApproved: false, hasCustomerReviewed: false };

  it("says a proposed time waits for WSO2 on the Customer Approval step, in the viewer's own words when it is theirs", () => {
    const own = stageOf(
      { ...base, state: approval, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "pending", proposedByViewer: true } },
      "Customer Approval",
    );
    expect(own).toMatchObject({ current: true, completed: false, disabled: false });
    expect(own?.description).toBe("Waiting for WSO2 to respond to your proposed time");

    const colleague = stageOf(
      { ...base, state: approval, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "pending" } },
      "Customer Approval",
    );
    expect(colleague?.description).toBe("A proposed time is waiting for WSO2's response");
  });

  it("keeps the usual caption when nothing is pending, and the step still current", () => {
    for (const customerProposal of [undefined, null, { startDate: "2030-03-01 09:00:00", answer: "disagreed" }]) {
      const step = stageOf({ ...base, state: approval, customerProposal }, "Customer Approval");
      expect(step?.description).toBe("Customer approval received");
      expect(step?.current).toBe(true);
    }
  });

  it("shows Customer Approval done for a change WSO2 accepted a proposed time on, though the approval flag stays false", () => {
    const step = stageOf(
      { ...base, state: scheduled, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "agreed" } },
      "Customer Approval",
    );
    expect(step).toMatchObject({ completed: true, current: false, disabled: false });
    expect(step?.description).toBe("Proposed time accepted by WSO2");
    // Without a proposed time the change still shows Customer Approval done once in Scheduled.
    const without = stageOf({ ...base, state: scheduled }, "Customer Approval");
    expect(without).toMatchObject({ completed: true, current: false, disabled: false });
    expect(without?.description).toBe("Customer approval received");
  });

  // An `agreed` answer is not cleared when the customers are asked again: back in Customer Approval it must not read as done.
  it("an agreed answer on a change that is (back) in Customer Approval is not accepted: the step is current and the usual caption stays", () => {
    const step = stageOf(
      { ...base, state: approval, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "agreed" } },
      "Customer Approval",
    );
    expect(step).toMatchObject({ current: true, completed: false, disabled: false });
    expect(step?.description).toBe("Customer approval received");
    expect(step?.description).not.toMatch(/accepted/i);
  });

  it("does not grey the step out in Implement or Review after an accepted proposal or regular approval", () => {
    for (const state of [{ id: "-1", label: "Implement" }, { id: "0", label: "Review" }]) {
      const accepted = stageOf(
        { ...base, state, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "agreed" } },
        "Customer Approval",
      );
      expect(accepted, state.label).toMatchObject({ completed: true, disabled: false });
      expect(stageOf({ ...base, state }, "Customer Approval"), state.label).toMatchObject({ completed: true, disabled: false });
    }
  });

  it("never shows a step done for a canceled change request, accepted proposal or not", () => {
    const canceled = stageOf(
      { ...base, state: { id: "4", label: "Canceled" }, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "agreed" } },
      "Customer Approval",
    );
    expect(canceled?.completed).toBe(false);
  });

  it("a pending answer left on a change that moved on is not shown as pending", () => {
    const step = stageOf(
      { ...base, state: scheduled, customerProposal: { startDate: "2030-03-01 09:00:00", answer: "pending" } },
      "Customer Approval",
    );
    expect(step?.description).toBe("Customer approval received");
  });
});

describe("a customer's proposed time", () => {
  const approval = { id: "5", label: "Customer Approval" };
  const proposal = (answer: string, extra: object = {}) => ({
    startDate: "2030-03-01 09:00:00",
    answer,
    ...extra,
  });
  const cr = (state: object, customerProposal: unknown, extra: object = {}) =>
    ({
      state,
      startDate: "2030-02-20 09:00:00",
      endDate: "2030-02-20 11:00:00",
      customerProposal,
      ...extra,
    }) as never;

  describe("getCustomerProposal", () => {
    it("returns a proposal that has a start and a known answer", () => {
      expect(getCustomerProposal(cr(approval, proposal("pending")))).toEqual(proposal("pending"));
      for (const answer of ["pending", "agreed", "disagreed", "unanswered"]) {
        expect(getCustomerProposal(cr(approval, proposal(answer)))?.answer, answer).toBe(answer);
      }
    });

    it("returns null for nothing proposed, no start, or an answer it does not know", () => {
      expect(getCustomerProposal(undefined)).toBeNull();
      expect(getCustomerProposal(null)).toBeNull();
      expect(getCustomerProposal(cr(approval, undefined))).toBeNull();
      expect(getCustomerProposal(cr(approval, null))).toBeNull();
      expect(getCustomerProposal(cr(approval, { answer: "pending" }))).toBeNull();
      expect(getCustomerProposal(cr(approval, { startDate: "  ", answer: "pending" }))).toBeNull();
      expect(getCustomerProposal(cr(approval, proposal("maybe")))).toBeNull();
    });
  });

  describe("pending, accepted and not accepted", () => {
    it("is pending only in Customer Approval", () => {
      expect(isProposalPending(cr(approval, proposal("pending")))).toBe(true);
      expect(isProposalPending(cr({ label: "Customer Approval" }, proposal("pending")))).toBe(true);
      for (const label of ["Authorize", "Scheduled", "Customer Review", "Closed", "Canceled"]) {
        expect(isProposalPending(cr({ id: "x", label }, proposal("pending"))), label).toBe(false);
      }
      for (const answer of ["agreed", "disagreed", "unanswered"]) {
        expect(isProposalPending(cr(approval, proposal(answer))), answer).toBe(false);
      }
      expect(isProposalPending(cr(approval, undefined))).toBe(false);
    });

    it("is accepted when WSO2 agreed AND the change has moved on from Customer Approval", () => {
      for (const label of ["Scheduled", "Implement", "Review", "Customer Review", "Rollback", "Closed", "Canceled"]) {
        expect(isProposalAccepted(cr({ id: "x", label }, proposal("agreed"))), label).toBe(true);
      }
      expect(isProposalAccepted(cr({ id: "-2", label: "Scheduled" }, proposal("agreed")))).toBe(true);
      expect(isProposalAccepted(cr({ id: "3", label: "Closed" }, proposal("agreed")))).toBe(true);
    });

    // An `agreed` answer stays on the row when the customers are asked again (a later Re-schedule, or an answer the previous
    // system wrote): while the change is in Customer Approval, Approve and Reject are live and nothing was scheduled by it.
    it("is NOT accepted while the change is in Customer Approval, whatever the answer says: Approve and Reject are live there", () => {
      expect(isProposalAccepted(cr(approval, proposal("agreed")))).toBe(false);
      expect(isProposalAccepted(cr({ label: "Customer Approval" }, proposal("agreed")))).toBe(false);
    });

    it("is not accepted before Customer Approval or in a state the page cannot place either", () => {
      for (const label of ["New", "Assess", "Authorize"]) {
        expect(isProposalAccepted(cr({ id: "x", label }, proposal("agreed"))), label).toBe(false);
      }
      expect(isProposalAccepted(cr({ id: "x", label: "Something Else" }, proposal("agreed")))).toBe(false);
      expect(isProposalAccepted(cr({}, proposal("agreed")))).toBe(false);
      expect(isProposalAccepted({ customerProposal: proposal("agreed") } as never)).toBe(false);
    });

    it("is not accepted for any other answer, or with nothing proposed, in any state", () => {
      for (const label of ["Customer Approval", "Scheduled", "Closed"]) {
        for (const answer of ["pending", "disagreed", "unanswered"]) {
          expect(isProposalAccepted(cr({ id: "x", label }, proposal(answer))), `${label} ${answer}`).toBe(false);
        }
        expect(isProposalAccepted(cr({ id: "x", label }, undefined)), label).toBe(false);
      }
      expect(isProposalAccepted(undefined)).toBe(false);
      expect(isProposalAccepted(null)).toBe(false);
    });

    it("is not accepted only while the change is still in Customer Approval", () => {
      expect(isProposalNotAccepted(cr(approval, proposal("disagreed")))).toBe(true);
      expect(isProposalNotAccepted(cr({ id: "-2", label: "Scheduled" }, proposal("disagreed")))).toBe(false);
      expect(isProposalNotAccepted(cr(approval, proposal("pending")))).toBe(false);
    });
  });

  describe("getProposalNote", () => {
    it("tells the proposer WSO2 has not answered yet, and that Approve approves the current schedule", () => {
      const note = getProposalNote(cr(approval, proposal("pending", { proposedByViewer: true })), true);
      expect(note?.kind).toBe("waiting");
      expect(note?.text).toMatch(/^Waiting for WSO2 to respond to your proposed time \(.*2030.*\)\./);
      expect(note?.text).toMatch(/Approving now approves the current schedule \(.*February 20, 2030.*\), not the proposed time\./);
    });

    it("says a colleague's proposal neutrally (and so a pending one an older backend sends with nothing about who proposed it)", () => {
      for (const extra of [{ proposedByViewer: false }, {}]) {
        const note = getProposalNote(cr(approval, proposal("pending", extra)), true);
        expect(note?.kind).toBe("waiting");
        expect(note?.text).toMatch(/^A new time \(.*2030.*\) was proposed for this change request and is waiting for WSO2's response\./);
        expect(note?.text).not.toMatch(/your proposed time/);
      }
    });

    it("does not tell a customer with nothing to answer that Approve exists", () => {
      const note = getProposalNote(cr(approval, proposal("pending", { proposedByViewer: true })), false);
      expect(note?.text).toMatch(/waiting for WSO2/i);
      expect(note?.text).not.toMatch(/Approving now/);
    });

    it("says WSO2 did not accept it, and points at the CURRENT window, never a new one", () => {
      const note = getProposalNote(cr(approval, proposal("disagreed")), true);
      expect(note?.kind).toBe("not-accepted");
      expect(note?.text).toMatch(/^WSO2 did not accept the proposed time \(.*2030.*\)\./);
      expect(note?.text).toMatch(/The current planned window is shown below: approve it, reject it, or propose another start\./);
      expect(note?.text).not.toMatch(/new planned window/i);
      expect(getProposalNote(cr(approval, proposal("disagreed")), false)?.text).toMatch(
        /The current planned window is shown below\.$/,
      );
    });

    it("has nothing to say when no proposed time is in play", () => {
      expect(getProposalNote(cr(approval, undefined), true)).toBeNull();
      expect(getProposalNote(cr(approval, proposal("unanswered")), true)).toBeNull();
      expect(getProposalNote(cr({ id: "-2", label: "Scheduled" }, proposal("agreed")), true)).toBeNull();
      expect(getProposalNote(cr({ id: "-2", label: "Scheduled" }, proposal("pending")), true)).toBeNull();
      expect(getProposalNote(cr({ id: "-3", label: "Authorize" }, proposal("disagreed")), true)).toBeNull();
      expect(getProposalNote(undefined, true)).toBeNull();
    });
  });
});

describe("getAnsweredWindow", () => {
  it("names the window the details showed, as they showed it", () => {
    expect(getAnsweredWindow({ startDate: "2026-06-10T04:30:00Z", endDate: "2026-06-10T06:30:00Z" })).toEqual({
      expectedPlannedStartOn: "2026-06-10T04:30:00Z",
      expectedPlannedEndOn: "2026-06-10T06:30:00Z",
    });
  });

  it("leaves out a bound the change request does not have", () => {
    expect(getAnsweredWindow({ startDate: "2026-06-10T04:30:00Z", endDate: "" })).toEqual({
      expectedPlannedStartOn: "2026-06-10T04:30:00Z",
    });
    expect(getAnsweredWindow({ startDate: undefined as never, endDate: "  " })).toEqual({});
  });
});

describe("isAwaitingInternalReview", () => {
  it("is true in Authorize, where only a proposal made before proposals waited in Customer Approval leaves a change, and nowhere else", () => {
    expect(isAwaitingInternalReview({ state: { id: "-3", label: "Authorize" } })).toBe(true);
    for (const label of ["New", "Assess", "Customer Approval", "Scheduled", "Customer Review", "Closed", "Canceled"]) {
      expect(isAwaitingInternalReview({ state: { id: "x", label } }), label).toBe(false);
    }
    expect(isAwaitingInternalReview(null)).toBe(false);
    expect(isAwaitingInternalReview({ state: null })).toBe(false);
  });
});
