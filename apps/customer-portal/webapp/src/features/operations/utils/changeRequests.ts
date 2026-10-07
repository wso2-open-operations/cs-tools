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

import { jsPDF } from "jspdf";
import autoTable from "jspdf-autotable";
import {
  CHANGE_REQUEST_STATE_ORDER,
  ChangeRequestStates,
  type ChangeRequestState,
} from "@features/operations/constants/operationsConstants";
import { resolveChangeRequestCanonicalState } from "@features/operations/utils/changeRequestUi";
import type { ChangeRequestDetails, ChangeRequestStats, ChangeRequestStatsResponse, ProposeNewTimeAvailability } from "@features/operations/types/changeRequests";
import type { CaseComment } from "@features/support/types/cases";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";
import type { ChangeRequestWorkflowStage } from "@features/operations/types/changeRequests";
import {
  formatBackendTimestampForDisplay,
  parseBackendTimestamp,
  resolveDisplayTimeZone,
} from "@utils/dateTime";
import { ApiError } from "@utils/ApiError";

// --- Change request stats (API → card counts) --------------------------------

export const AWAITING_YOUR_ACTION_STATE_IDS = new Set(["5", "1"]);
export const ONGOING_STATE_IDS = new Set(["-5", "-4", "-3", "-2", "-1", "0"]);
export const COMPLETED_STATE_IDS = new Set(["3", "4", "2"]);

export const AWAITING_LABELS = new Set([
  "Customer Approval",
  "Customer Review",
]);
export const ONGOING_LABELS = new Set([
  "New",
  "Assess",
  "Authorize",
  "Scheduled",
  "Implement",
  "Review",
]);
export const COMPLETED_LABELS = new Set([
  "Closed",
  "Canceled",
  "Cancelled",
  "Rollback",
]);

export function sumChangeRequestStateCount(
  stateCount: ChangeRequestStatsResponse["stateCount"],
  idSet: Set<string>,
  labelSet: Set<string>,
): number {
  return stateCount.reduce((sum, s) => {
    const id = s.id != null && String(s.id).length > 0 ? String(s.id) : "";
    if (id && idSet.has(id)) {
      return sum + s.count;
    }
    if (!id && labelSet.has(s.label)) {
      return sum + s.count;
    }
    return sum;
  }, 0);
}

/**
 * Maps the API stats payload to dashboard stat card values.
 *
 * @param response - Raw change request stats from the API.
 * @returns {ChangeRequestStats} Normalized counts.
 */
export function mapChangeRequestStats(
  response: ChangeRequestStatsResponse,
): ChangeRequestStats {
  const { totalCount, stateCount } = response;

  return {
    totalRequests: totalCount,
    awaitingYourAction: sumChangeRequestStateCount(
      stateCount,
      AWAITING_YOUR_ACTION_STATE_IDS,
      AWAITING_LABELS,
    ),
    ongoing: sumChangeRequestStateCount(
      stateCount,
      ONGOING_STATE_IDS,
      ONGOING_LABELS,
    ),
    completed: sumChangeRequestStateCount(
      stateCount,
      COMPLETED_STATE_IDS,
      COMPLETED_LABELS,
    ),
  };
}

// --- Schedule / datetime helpers ----------------------------------------------

/** Returns true if date string is empty or invalid. */
export function isChangeRequestDateAvailable(
  dateStr: string | null | undefined,
): boolean {
  if (!dateStr?.trim()) return false;
  return parseBackendTimestamp(dateStr) !== null;
}

/**
 * Format API date string for display (weekday, long date, time).
 *
 * @param dateStr - API date e.g. "2026-02-28 15:30:50".
 * @returns Formatted string or "Not available".
 */
export function formatChangeRequestDisplayDate(
  dateStr: string | null | undefined,
): string {
  if (!isChangeRequestDateAvailable(dateStr)) return "Not available";
  const formatted = formatBackendTimestampForDisplay(dateStr, {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
  return formatted ?? "Not available";
}

/**
 * Format minutes as "X hours Y minutes".
 *
 * @param minutes - Total minutes.
 * @returns Human-readable duration.
 */
export function formatChangeRequestDuration(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours} hour${hours === 1 ? "" : "s"}`);
  parts.push(`${mins} minute${mins === 1 ? "" : "s"}`);
  return parts.join(" ");
}

/**
 * Converts duration from minutes to a compact string (e.g. "4h 30m") for list rows.
 *
 * @param minutes - Duration in minutes (API may return string).
 * @returns Formatted duration or "Not Available".
 */
export function formatDuration(
  minutes: number | string | null | undefined,
): string {
  if (minutes == null) return "Not Available";
  const n = typeof minutes === "number" ? minutes : parseInt(String(minutes), 10);
  if (Number.isNaN(n) || n < 0) return "Not Available";

  const hours = Math.floor(n / 60);
  const mins = n % 60;

  if (hours === 0 && mins === 0) return "0m";
  if (hours === 0) return `${mins}m`;
  if (mins === 0) return `${hours}h`;

  return `${hours}h ${mins}m`;
}

/**
 * Strips custom tags like [code]...[/code] from change request comment content.
 *
 * @param content - Raw string from API.
 * @returns Plain text without bracket tags.
 */
export function stripChangeRequestCustomTags(
  content: string | null | undefined,
): string {
  if (!content || typeof content !== "string") return "";
  return content.replace(/\[\/?\w+\]/g, "").trim();
}

/**
 * Strips HTML and custom bracket tags (for CR comments with mixed markup).
 *
 * @param content - Raw string from API.
 * @returns Plain text.
 */
export function stripChangeRequestAllTags(
  content: string | null | undefined,
): string {
  if (!content || typeof content !== "string") return "";
  let cleaned = content.replace(/\[\/?\w+\]/g, "");
  cleaned = cleaned.replace(/<[^>]+>/g, "");
  return cleaned.trim();
}

/**
 * Convert datetime-local input value to API format "YYYY-MM-DD HH:mm:ss".
 *
 * @param datetimeLocal - From input type="datetime-local".
 * @returns API format string.
 */
export function changeRequestToApiDatetime(datetimeLocal: string): string {
  if (!datetimeLocal) return "";
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(datetimeLocal);
  if (!match) return "";
  const [, y, m, day, h, min, s = "00"] = match;
  return `${y}-${m}-${day} ${h}:${min}:${s}`;
}

/**
 * Convert API datetime to datetime-local input value.
 *
 * @param apiDatetime - "YYYY-MM-DD HH:mm:ss".
 * @returns Value for input type="datetime-local".
 */
export function changeRequestToDatetimeLocal(apiDatetime: string): string {
  if (!apiDatetime) return "";
  const match = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2})$/.exec(apiDatetime);
  if (!match) return "";
  const [, y, m, day, h, min] = match;
  return `${y}-${m}-${day}T${h}:${min}`;
}

// --- Workflow / decision mode -------------------------------------------------

export function getChangeRequestDecisionMode(
  changeRequest?: ChangeRequestDetails | null,
): ChangeRequestDecisionMode {
  const canonical = resolveChangeRequestCanonicalState(changeRequest?.state);
  if (canonical === ChangeRequestStates.CUSTOMER_APPROVAL) {
    return ChangeRequestDecisionMode.CUSTOMER_APPROVAL;
  }
  if (canonical === ChangeRequestStates.CUSTOMER_REVIEW) {
    return ChangeRequestDecisionMode.CUSTOMER_REVIEW;
  }
  return ChangeRequestDecisionMode.NONE;
}

/**
 * Which customer answer the signed-in customer can give on this change request
 * right now, and so which action buttons the details page offers.
 *
 * The state decides what could be asked of the customer (Customer Approval:
 * approve / reject / propose a new time; Customer Review: successful /
 * unsuccessful). `customerCanAnswer` (viewer-specific, from the backend) then
 * decides whether THIS customer has such an answer pending:
 *
 * | state             | customerCanAnswer | hasCustomerApproved | result            |
 * |-------------------|-------------------|---------------------|-------------------|
 * | Customer Approval | true              | any                 | CUSTOMER_APPROVAL |
 * | Customer Approval | false             | any                 | NONE              |
 * | Customer Approval | absent            | true                | CUSTOMER_APPROVAL |
 * | Customer Approval | absent            | false / absent      | NONE              |
 * | Customer Review   | true or absent    | any                 | CUSTOMER_REVIEW   |
 * | Customer Review   | false             | any                 | NONE              |
 * | any other state   | any               | any                 | NONE              |
 *
 * `customerCanAnswer` is absent when the data source cannot say (ServiceNow);
 * the old gate (`hasCustomerApproved` at Customer Approval) then still applies.
 *
 * @param changeRequest - The change request as returned by the details API.
 * @returns {ChangeRequestDecisionMode} The answer to offer, or NONE.
 */
export function resolveCustomerDecisionMode(
  changeRequest?: ChangeRequestDetails | null,
): ChangeRequestDecisionMode {
  const stateMode = getChangeRequestDecisionMode(changeRequest);
  const canAnswer =
    typeof changeRequest?.customerCanAnswer === "boolean"
      ? changeRequest.customerCanAnswer
      : undefined;

  switch (stateMode) {
    case ChangeRequestDecisionMode.CUSTOMER_APPROVAL:
      return (canAnswer ?? changeRequest?.hasCustomerApproved === true)
        ? ChangeRequestDecisionMode.CUSTOMER_APPROVAL
        : ChangeRequestDecisionMode.NONE;
    case ChangeRequestDecisionMode.CUSTOMER_REVIEW:
      return canAnswer === false
        ? ChangeRequestDecisionMode.NONE
        : ChangeRequestDecisionMode.CUSTOMER_REVIEW;
    default:
      return ChangeRequestDecisionMode.NONE;
  }
}

/**
 * The planned window the customer is looking at, as the answer's precondition:
 * the backend records the answer only while it is still the change request's
 * window, so a page opened before the change was re-scheduled cannot approve a
 * time its reader never saw. A bound the change request does not have is left out
 * (nothing to compare it with).
 */
export function getAnsweredWindow(
  changeRequest: Pick<ChangeRequestDetails, "startDate" | "endDate">,
): { expectedPlannedStartOn?: string; expectedPlannedEndOn?: string } {
  const start = changeRequest.startDate?.trim();
  const end = changeRequest.endDate?.trim();
  return {
    ...(start ? { expectedPlannedStartOn: start } : {}),
    ...(end ? { expectedPlannedEndOn: end } : {}),
  };
}

/**
 * True while the change request waits on WSO2 alone: it is in Authorize, which
 * a customer sees only after proposing a new time (the proposal sends the change
 * back through WSO2's internal approval before the customer is asked again).
 */
export function isAwaitingInternalReview(
  changeRequest?: Pick<ChangeRequestDetails, "state"> | null,
): boolean {
  return (
    resolveChangeRequestCanonicalState(changeRequest?.state) ===
    ChangeRequestStates.AUTHORIZE
  );
}

/** Labels of the two answer buttons for a decision mode. */
export function getCustomerDecisionLabels(mode: ChangeRequestDecisionMode): {
  approve: string;
  reject: string;
} {
  return mode === ChangeRequestDecisionMode.CUSTOMER_REVIEW
    ? { approve: "Successful", reject: "Unsuccessful" }
    : { approve: "Approve", reject: "Reject" };
}

/** What the reject confirmation says about proposing a different time, by what the page allows. */
const REJECT_HINTS: Record<ProposeNewTimeAvailability, string | undefined> = {
  available:
    "If you only need a different time, go back and use Propose New Time instead.",
  on_hold:
    "A new time cannot be proposed right now because WSO2 has this change request on hold.",
  unavailable: undefined,
};

/**
 * Copy for the confirmation shown before the answer that cannot be taken back:
 * a rejected change is canceled, an unsuccessful one goes into rollback.
 *
 * When rejecting, the hint is about the alternative, a different time, and says
 * only what is true of Propose New Time right now (`proposeNewTime`, as the page
 * decides it): that it can be used, or that it cannot because WSO2 has the
 * change on hold, and nothing at all where it is not offered. It never points at
 * an action that is switched off.
 */
export function getCustomerRejectConfirmCopy(
  mode: ChangeRequestDecisionMode,
  proposeNewTime: ProposeNewTimeAvailability = "unavailable",
): {
  title: string;
  message: string;
  hint?: string;
  confirmLabel: string;
} {
  if (mode === ChangeRequestDecisionMode.CUSTOMER_REVIEW) {
    return {
      title: "Mark this change as unsuccessful?",
      message: "Marking it unsuccessful sends the change into rollback.",
      confirmLabel: "Mark unsuccessful",
    };
  }
  return {
    title: "Reject this change request?",
    message: "Rejecting cancels this change request.",
    hint: REJECT_HINTS[proposeNewTime],
    confirmLabel: "Reject change request",
  };
}

/** Toast / banner text for a customer's answer: what happened, in plain words. */
export function getCustomerDecisionMessages(
  mode: ChangeRequestDecisionMode,
  approved: boolean,
): { success: string; failure: string } {
  if (mode === ChangeRequestDecisionMode.CUSTOMER_REVIEW) {
    return approved
      ? {
          success: "Change request marked as successful. It is now closed.",
          failure: "Could not mark the change request as successful. Please try again.",
        }
      : {
          success: "Change request marked as unsuccessful. It is now in rollback.",
          failure: "Could not mark the change request as unsuccessful. Please try again.",
        };
  }
  return approved
    ? {
        success: "Change request approved. It is now scheduled.",
        failure: "Could not approve the change request. Please try again.",
      }
    : {
        success: "Change request rejected. It has been canceled.",
        failure: "Could not reject the change request. Please try again.",
      };
}

/**
 * The 404 of a change request's page. A change request that is not shared with
 * this customer is a 404 exactly like one that does not exist, so the page says
 * neither more nor less than that.
 */
export const CHANGE_REQUEST_NOT_FOUND_MESSAGE =
  "This change request was not found. It may not have been shared with you.";

/**
 * The machine-readable names of the refusals of a customer's answer or proposed
 * time (the error body's `errorCode`, see `ApiError.code`). The backend names them
 * and the webapp classifies by them, never by the wording of the message. The list
 * only ever grows: a code this webapp does not know is a newer backend's, and is
 * handled as the safe default (`CHANGE_REQUEST_ACTION_FAILED_MESSAGE`).
 */
export const ChangeRequestErrorCode = {
  /** 409: a proposed time while WSO2 has the change on hold. Answering still works. */
  ON_HOLD: "change_request_on_hold",
  /** 409: an answer for a planned window that has since changed. Nothing recorded. */
  SCHEDULE_CHANGED: "change_request_schedule_changed",
  /** 409: already answered (by this contact or a sibling), or nothing waits for it. */
  APPROVAL_NOT_PENDING: "change_request_approval_not_pending",
  /** 409: a proposed time where none can be proposed (not in Customer Approval, nobody asked). */
  NOT_PROPOSABLE: "change_request_not_proposable",
  /** 403: a contact of the project whom the customer's request was never sent to. */
  NOT_ASKED: "change_request_not_asked",
  /** 403: not a registered contact of the project, its creator, or a field a customer may not set. */
  FORBIDDEN: "change_request_forbidden",
} as const;

/** A conflict (409): the answer was already given, or is no longer asked for. */
export const CHANGE_REQUEST_ANSWER_STALE_MESSAGE =
  "This request was already answered or is no longer waiting for your answer.";

/** A refusal (403): the caller is not a contact who may answer this change. */
export const CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE =
  "You are not one of the contacts who can answer this change request.";

/** A conflict (409) on a proposal because WSO2 has the change on hold. */
export const CHANGE_REQUEST_ON_HOLD_MESSAGE =
  "This change request is on hold, so a new time cannot be proposed right now.";

/**
 * A conflict (409) on an answer because the schedule moved after the page was
 * opened: the answer was given for a time its reader never saw, so it was not
 * recorded. The page re-reads the change request, so the new time is on screen.
 */
export const CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE =
  "The schedule of this change request changed after you opened it. Review the updated schedule, then answer again.";

/**
 * A conflict (409) that names nothing this webapp knows: the backend sent no
 * `errorCode` (an older one) or a code from a newer one. It could be a hold, a
 * moved schedule or an answer that was already given, so it claims none of them:
 * it says something went wrong and sends the customer to the refreshed page,
 * which the failed request has already asked for.
 */
export const CHANGE_REQUEST_ACTION_FAILED_MESSAGE =
  "Something went wrong with this change request. Refresh the page to see where it stands, then try again.";

/** Backend 400 messages the customer can act on, in the customer's words. */
const BAD_REQUEST_MESSAGES: ReadonlyArray<readonly [needle: string, message: string]> = [
  [
    "must not be after the planned end",
    "The proposed end must be after the proposed start.",
  ],
  [
    "requires a changed planned start or end",
    "This is the same as the current schedule. Change the start or the end to propose a different time.",
  ],
  [
    "must not be the same as the planned end",
    "The proposed end must be after the proposed start.",
  ],
  [
    "is in the past",
    "The proposed time must be in the future.",
  ],
  [
    "must be a valid date-time",
    "Enter a valid start and end date and time.",
  ],
  [
    "is locked once set to true",
    "This answer has already been given and cannot be changed.",
  ],
];

/**
 * Turns the error of a customer's PATCH (answer or proposed time) into text a
 * customer can act on.
 *
 * A refusal is classified by its machine-readable `ApiError.code`, never by the
 * wording of the backend's message (which is for people and can change). The
 * message of a 400 is shown (in the customer's words where
 * `BAD_REQUEST_MESSAGES` knows it) because it is the display text of a mistake in
 * what was typed, and decides nothing.
 *
 *   - a 409 with a known code says what it is: a hold, a moved schedule, an
 *     answer already given (or a proposal where none can be made);
 *   - a 409 with no code (an older backend) or one this webapp does not know
 *     (a newer backend) is `CHANGE_REQUEST_ACTION_FAILED_MESSAGE`: never claimed
 *     to be "already answered", which could be wrong;
 *   - a 403 is "not one of the contacts who can answer" whatever its code
 *     (a code only tells apart why).
 *
 * `terminal` is true when retrying or editing cannot help because the change
 * request no longer waits on this customer: the caller should close the form
 * and move on to the refreshed page rather than leave the customer in a form
 * that cannot succeed. A hold is not terminal (the proposal can be tried again
 * once WSO2 takes it off hold, and the customer can still answer), and neither is
 * a failure that is no refusal of the change request's state (a 400, a 500).
 *
 * @param error - What the mutation rejected with.
 * @param fallback - Text for failures with nothing better to say.
 * @returns The message and whether the action is no longer possible.
 */
export function describeChangeRequestActionError(
  error: unknown,
  fallback: string,
): { message: string; terminal: boolean } {
  if (error instanceof ApiError) {
    if (error.status === 409) {
      switch (error.code) {
        case ChangeRequestErrorCode.ON_HOLD:
          // A hold is the one conflict that is about neither the answer nor the
          // state: the customer is still being asked, a proposal just cannot go in.
          return { message: CHANGE_REQUEST_ON_HOLD_MESSAGE, terminal: false };
        case ChangeRequestErrorCode.SCHEDULE_CHANGED:
          // The schedule moved: the answer was given for a time its reader never saw.
          return { message: CHANGE_REQUEST_SCHEDULE_CHANGED_MESSAGE, terminal: true };
        case ChangeRequestErrorCode.APPROVAL_NOT_PENDING:
        case ChangeRequestErrorCode.NOT_PROPOSABLE:
          return { message: CHANGE_REQUEST_ANSWER_STALE_MESSAGE, terminal: true };
        default:
          // Terminal all the same: a 409 is a conflict with the state, and the
          // request has already asked for the page to be re-read (the patch hook
          // refreshes on every 409), so the form is closed for the refreshed page
          // to take over rather than left open on a state it no longer matches.
          return { message: CHANGE_REQUEST_ACTION_FAILED_MESSAGE, terminal: true };
      }
    }
    if (error.status === 403) {
      // Who may act is not the customer's to fix: however it is refused (never
      // asked, not a contact of the project, the creator, a field that may not be
      // set: ChangeRequestErrorCode.NOT_ASKED / FORBIDDEN say why) the answer is
      // the same and final, so a 403 with another code, or none, reads the same.
      return { message: CHANGE_REQUEST_NOT_A_CONTACT_MESSAGE, terminal: true };
    }
    const backendMessage = error.message?.trim();
    if (error.status === 400 && backendMessage) {
      const known = BAD_REQUEST_MESSAGES.find(([needle]) =>
        backendMessage.includes(needle),
      );
      return { message: known ? known[1] : backendMessage, terminal: false };
    }
    // No message in the body: the hook then carries the bare status text
    // ("Internal Server Error", "HTTP 500"), which tells a customer nothing.
    const isBareStatus =
      backendMessage === `${error.status} ${error.statusText}` ||
      backendMessage === error.statusText?.trim() ||
      backendMessage === `HTTP ${error.status}`;
    return {
      message: backendMessage && !isBareStatus ? backendMessage : fallback,
      terminal: false,
    };
  }
  return { message: fallback, terminal: false };
}

export function buildChangeRequestWorkflowStages(
  changeRequest?: ChangeRequestDetails | null,
): { workflowStages: ChangeRequestWorkflowStage[]; currentStateIndex: number } {
  if (!changeRequest) {
    return { workflowStages: [], currentStateIndex: -1 };
  }

  const currentState: ChangeRequestState =
    resolveChangeRequestCanonicalState(changeRequest.state) ??
    ChangeRequestStates.NEW;
  const { hasCustomerApproved, hasCustomerReviewed } = changeRequest;
  const currentIndex = CHANGE_REQUEST_STATE_ORDER.indexOf(currentState);
  const isCanceled = currentState === ChangeRequestStates.CANCELED;
  const allowIndexProgress = !isCanceled && currentIndex >= 0;

  return {
    currentStateIndex: currentIndex,
    workflowStages: [
      {
        name: ChangeRequestStates.NEW,
        description: "Change request created",
        completed: allowIndexProgress && currentIndex > 0,
        current: currentState === ChangeRequestStates.NEW,
        disabled: false,
      },
      {
        name: ChangeRequestStates.ASSESS,
        description: "Technical assessment completed",
        completed: allowIndexProgress && currentIndex > 1,
        current: currentState === ChangeRequestStates.ASSESS,
        disabled: false,
      },
      {
        name: ChangeRequestStates.AUTHORIZE,
        description: "Internal authorization obtained",
        completed: allowIndexProgress && currentIndex > 2,
        current: currentState === ChangeRequestStates.AUTHORIZE,
        disabled: false,
      },
      {
        name: ChangeRequestStates.CUSTOMER_APPROVAL,
        description: "Customer approval received",
        completed:
          allowIndexProgress && currentIndex > 3 && hasCustomerApproved,
        current: currentState === ChangeRequestStates.CUSTOMER_APPROVAL,
        disabled:
          (currentState === ChangeRequestStates.IMPLEMENT ||
            currentState === ChangeRequestStates.REVIEW) &&
          !hasCustomerApproved,
      },
      {
        name: ChangeRequestStates.SCHEDULED,
        description: "Maintenance window scheduled",
        completed: allowIndexProgress && currentIndex > 4,
        current: currentState === ChangeRequestStates.SCHEDULED,
        disabled: false,
      },
      {
        name: ChangeRequestStates.IMPLEMENT,
        description: "Change implementation",
        completed: allowIndexProgress && currentIndex > 5,
        current: currentState === ChangeRequestStates.IMPLEMENT,
        disabled: false,
      },
      {
        name: ChangeRequestStates.REVIEW,
        description: "Internal review",
        completed: allowIndexProgress && currentIndex > 6,
        current: currentState === ChangeRequestStates.REVIEW,
        disabled: false,
      },
      {
        name: ChangeRequestStates.CUSTOMER_REVIEW,
        description: "Customer validation",
        completed:
          allowIndexProgress && currentIndex > 7 && hasCustomerReviewed,
        current: currentState === ChangeRequestStates.CUSTOMER_REVIEW,
        disabled:
          (currentState === ChangeRequestStates.ROLLBACK ||
            currentState === ChangeRequestStates.CLOSED ||
            currentState === ChangeRequestStates.CANCELED) &&
          !hasCustomerReviewed,
      },
      {
        name: ChangeRequestStates.ROLLBACK,
        description: "Change rollback if needed",
        completed: false,
        current: currentState === ChangeRequestStates.ROLLBACK,
        disabled:
          currentState === ChangeRequestStates.CLOSED ||
          currentState === ChangeRequestStates.CANCELED,
      },
      {
        name: ChangeRequestStates.CLOSED,
        description: "Change request completed",
        completed: false,
        current: currentState === ChangeRequestStates.CLOSED,
        disabled:
          currentState === ChangeRequestStates.CANCELED ||
          currentState === ChangeRequestStates.ROLLBACK,
      },
      {
        name: ChangeRequestStates.CANCELED,
        description: "Change request canceled",
        completed: false,
        current: currentState === ChangeRequestStates.CANCELED,
        disabled:
          currentState === ChangeRequestStates.CLOSED ||
          currentState === ChangeRequestStates.ROLLBACK,
      },
    ],
  };
}

// --- Change request details PDF -----------------------------------------------

function stripHtmlOrNA(html: string | null | undefined): string {
  if (!html) return "N/A";
  return html.replace(/<[^>]*>/g, "").trim() || "N/A";
}

function getDateOrNA(dateStr: string | null | undefined): string {
  if (!dateStr) return "N/A";
  const formatted = formatBackendTimestampForDisplay(dateStr, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });
  return formatted ?? "N/A";
}

/**
 * Generates and downloads a Change Request Details PDF.
 */
export function generateChangeRequestDetailsPdf(
  changeRequest: ChangeRequestDetails,
  comments?: CaseComment[],
): void {
  const doc = new jsPDF() as jsPDF & { lastAutoTable?: { finalY: number } };
  const pageWidth = doc.internal.pageSize.getWidth();

  doc.setFontSize(18);
  doc.setFont("helvetica", "bold");
  doc.text("Change Request Details", 14, 20);

  doc.setDrawColor(200, 200, 200);
  doc.setLineWidth(0.5);
  doc.line(14, 24, pageWidth - 14, 24);

  const tableData: string[][] = [
    ["Number", changeRequest.number || "N/A"],
    ["Description", stripHtmlOrNA(changeRequest.description)],
    ["Customer Project", changeRequest.project?.label || "N/A"],
    ["Environment", changeRequest.deployment?.label || "N/A"],
    ["State", changeRequest.state?.label || "N/A"],
    ["Service Request", changeRequest.case?.number || "N/A"],
    ["Created By", changeRequest.createdBy || "N/A"],
    ["Created Date", getDateOrNA(changeRequest.createdOn)],
    ["Start Date", getDateOrNA(changeRequest.startDate)],
    ["End Date", getDateOrNA(changeRequest.endDate)],
    ["Type", changeRequest.type?.label || "N/A"],
    ["Assigned Engineer", changeRequest.assignedEngineer?.label || "N/A"],
    ["Assigned Team", changeRequest.assignedTeam?.label || "N/A"],
    ["Impact", changeRequest.impact?.label || "N/A"],
    ["Service Outage Details", stripHtmlOrNA(changeRequest.serviceOutage)],
    ["Communication Plan", stripHtmlOrNA(changeRequest.communicationPlan)],
    ["Test Plan", stripHtmlOrNA(changeRequest.testPlan)],
    ["Rollback Plan", stripHtmlOrNA(changeRequest.rollbackPlan)],
  ];

  if (changeRequest.approvedBy) {
    tableData.push(["Approved By", changeRequest.approvedBy.label]);
    tableData.push(["Approved On", getDateOrNA(changeRequest.approvedOn)]);
  }

  if (changeRequest.product) {
    tableData.push(["Product", changeRequest.product.label]);
  }

  if (changeRequest.deployedProduct) {
    tableData.push(["Deployed Product", changeRequest.deployedProduct.label]);
  }

  if (changeRequest.justification) {
    tableData.push(["Justification", stripHtmlOrNA(changeRequest.justification)]);
  }

  if (changeRequest.impactDescription) {
    tableData.push([
      "Impact Description",
      stripHtmlOrNA(changeRequest.impactDescription),
    ]);
  }

  if (comments && comments.length > 0) {
    const commentsText = comments
      .map(
        (comment) =>
          `${comment.createdBy} - ${getDateOrNA(comment.createdOn)}:\n${stripHtmlOrNA(comment.content)}`,
      )
      .join("\n\n");
    tableData.push(["Notes & Comments", commentsText]);
  }

  autoTable(doc, {
    startY: 28,
    body: tableData,
    theme: "grid",
    styles: {
      fontSize: 10,
      cellPadding: 4,
      lineColor: [200, 200, 200],
      lineWidth: 0.1,
    },
    columnStyles: {
      0: {
        fontStyle: "bold",
        cellWidth: 65,
        valign: "top",
      },
      1: {
        fontStyle: "normal",
        cellWidth: 115,
        valign: "top",
      },
    },
    margin: { bottom: 25 },
  });

  const createdDateTime = new Date().toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
    timeZone: resolveDisplayTimeZone(),
  });
  const totalPages = doc.getNumberOfPages();

  for (let i = 1; i <= totalPages; i++) {
    doc.setPage(i);
    doc.setFontSize(8);
    doc.setFont("helvetica", "normal");
    const pageHeight = doc.internal.pageSize.getHeight();
    const footerY = pageHeight - 10;

    doc.text("WSO2 Support 24x7", 14, footerY);

    const pageText = `Page ${i} of ${totalPages}`;
    const textWidth = doc.getTextWidth(pageText);
    doc.text(pageText, (pageWidth - textWidth) / 2, footerY);

    const dateWidth = doc.getTextWidth(createdDateTime);
    doc.text(createdDateTime, pageWidth - 14 - dateWidth, footerY);
  }

  const fileName = `Change-Request-${changeRequest.number || "Details"}-${new Date().toISOString().split("T")[0]}.pdf`;
  doc.save(fileName);
}
