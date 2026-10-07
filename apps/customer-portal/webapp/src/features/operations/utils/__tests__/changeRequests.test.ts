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
  getCustomerRejectConfirmCopy,
  buildChangeRequestWorkflowStages,
  mapChangeRequestStats,
  resolveCustomerDecisionMode,
  stripChangeRequestCustomTags,
  sumChangeRequestStateCount,
  AWAITING_YOUR_ACTION_STATE_IDS,
  AWAITING_LABELS,
  CHANGE_REQUEST_ACTION_FAILED_MESSAGE,
  CHANGE_REQUEST_ANSWER_STALE_MESSAGE,
  CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE,
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
    ).toMatch(/same as the current schedule/);
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
  it("is true in Authorize, where a proposed time waits for WSO2, and nowhere else", () => {
    expect(isAwaitingInternalReview({ state: { id: "-3", label: "Authorize" } })).toBe(true);
    for (const label of ["New", "Assess", "Customer Approval", "Scheduled", "Customer Review", "Closed", "Canceled"]) {
      expect(isAwaitingInternalReview({ state: { id: "x", label } }), label).toBe(false);
    }
    expect(isAwaitingInternalReview(null)).toBe(false);
    expect(isAwaitingInternalReview({ state: null })).toBe(false);
  });
});
