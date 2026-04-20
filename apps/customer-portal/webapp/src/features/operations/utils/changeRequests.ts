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
import type { ChangeRequestDetails, ChangeRequestStats, ChangeRequestStatsResponse } from "@features/operations/types/changeRequests";
import type { CaseComment } from "@features/support/types/cases";
import { ChangeRequestDecisionMode } from "@features/operations/types/changeRequests";
import type { ChangeRequestWorkflowStage } from "@features/operations/types/changeRequests";
import {
  formatBackendTimestampForDisplay,
  parseBackendTimestamp,
  resolveDisplayTimeZone,
} from "@utils/dateTime";

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
  return dateStr;
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
