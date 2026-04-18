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

import {
  Activity,
  CheckCircle,
  CircleAlert,
  CircleCheck,
  CircleQuestionMark,
  CircleX,
  Clock,
  MessageCircle,
  PlayCircle,
  RotateCcw,
  TriangleAlert,
  XCircle,
} from "@wso2/oxygen-ui-icons-react";
import {
  ChatAction,
  ChatStatus,
  ConversationStatus,
  CaseStatus,
  CallRequestStatus,
  CaseType,
  CaseSeverityLevel,
  type CaseTypeInput,
} from "@features/support/constants/supportConstants";
import { CaseReportType } from "@features/security/types/security";
import { SEVERITY_LABEL_TO_DISPLAY } from "@/features/dashboard/constants/dashboard";
import { isS0SeverityLabel } from "@features/dashboard/utils/dashboard";
import type { CaseComment } from "@features/support/types/cases";
import type { InlineAttachment } from "@features/support/types/supportInlineAttachment";
import type { MetadataItem } from "@/types/common";
import { alpha, colors, type Theme } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";
import { createElement, type ComponentType, type ReactNode } from "react";
import { CASE_STATUS } from "@features/project-details/constants/projectDetailsConstants";

/**
 * Extracts Incident and Query case type IDs from caseTypes metadata.
 * Used for default caseTypeIds filter in Support overview and All Cases.
 *
 * @param caseTypes - Case types from useGetProjectFilters response.
 * @returns {string[]} IDs for Incident and Query types.
 */
export function getIncidentAndQueryCaseTypeIds(
  caseTypes?: MetadataItem[],
): string[] {
  if (!caseTypes?.length) return [];
  const normalized = (label: string) => label.trim().toLowerCase();
  return caseTypes
    .filter((ct) => /^incident$|^icident$|^query$/i.test(normalized(ct.label)))
    .map((ct) => ct.id);
}

/**
 * Extracts Incident and Query case type IDs separately for stats API.
 * API expects caseTypes=queryId&caseTypes=incidentId.
 *
 * @param caseTypes - Case types from useGetProjectFilters response.
 * @returns {{ incidentId?: string; queryId?: string }} Incident and Query IDs.
 */
export function getIncidentAndQueryIds(caseTypes?: MetadataItem[]): {
  incidentId?: string;
  queryId?: string;
} {
  if (!caseTypes?.length) return {};
  const normalized = (label: string) => label.trim().toLowerCase();
  let incidentId: string | undefined;
  let queryId: string | undefined;
  for (const ct of caseTypes) {
    const n = normalized(ct.label);
    if (/^incident$|^icident$/i.test(n) && incidentId === undefined)
      incidentId = ct.id;
    else if (/^query$/i.test(n) && queryId === undefined) queryId = ct.id;
  }
  return { incidentId, queryId };
}

/**
 * Detects whether a case type represents Security Report Analysis.
 * Prefers stable id matching and falls back to normalized label matching.
 */
export function isSecurityReportAnalysisType(type: CaseTypeInput): boolean {
  if (!type) return false;

  const target = CaseType.SECURITY_REPORT_ANALYSIS.toLowerCase().replace(
    /_/g,
    "",
  );

  // Helper to normalize any string (id, label, or raw string)
  const normalize = (val: string) =>
    val
      .toLowerCase()
      .replace(/[\s_-]+/g, "")
      .trim();

  switch (typeof type) {
    case "string":
      return normalize(type) === target;

    case "object":
      switch (true) {
        case type.id === CaseType.SECURITY_REPORT_ANALYSIS:
          return true;
        case !!type.label:
          return normalize(type.label!) === target;
        default:
          return false;
      }

    default:
      return false;
  }
}

/**
 * Comprehensive check to determine if a created case is a security report.
 * Consolidates multiple possible indicators into a single check.
 *
 * @param {object} caseData - The case data object returned from the API
 * @param {boolean} isInitialSecurityReport - Whether the case was initially created as a security report
 * @returns {boolean} True if the case is a security report
 */
export function isCreatedCaseSecurityReport(
  caseData: {
    isSecurityReport?: boolean;
    reportType?: string;
    type?: CaseTypeInput | { id?: string | null; label?: string | null } | null;
  },
  isInitialSecurityReport: boolean,
): boolean {
  // Check multiple indicators using a switch-based approach
  switch (true) {
    case isInitialSecurityReport:
      return true;
    case caseData.isSecurityReport === true:
      return true;
    case caseData.reportType === CaseReportType.SECURITY:
      return true;
    case typeof caseData.type === "string":
      return caseData.type === CaseType.SECURITY_REPORT_ANALYSIS;
    case caseData.type !== null && caseData.type !== undefined:
      // Type is object - use the existing type checker
      if (typeof caseData.type === "object") {
        const typeObj = caseData.type as {
          id?: string | null;
          label?: string | null;
        };
        if (typeObj.id != null || typeObj.label != null) {
          return isSecurityReportAnalysisType({
            id: typeObj.id ?? "",
            label: typeObj.label ?? "",
          });
        }
      }
      return false;
    default:
      return false;
  }
}

/**
 * Normalizes UTC date string from API (YYYY-MM-DD HH:mm:ss or MM/DD/YYYY HH:mm:ss) to ISO for parsing.
 * Treats input as UTC; output is suitable for display in browser local time.
 *
 * @param {string} dateStr - Raw UTC date string.
 * @returns {string} Normalized ISO string for Date constructor.
 */
export function normalizeUtcDateString(dateStr: string): string {
  const trimmed = dateStr.trim();
  if (/T\d{2}:\d{2}:\d{2}/.test(trimmed) || /Z$/i.test(trimmed)) return trimmed;

  // Match YYYY-MM-DD HH:mm:ss format
  const yyyymmdd =
    /^(\d{4})-(\d{1,2})-(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})$/.exec(trimmed);
  if (yyyymmdd) {
    const [, yyyy, mm, dd, hh, mi, ss] = yyyymmdd;
    return `${yyyy}-${mm!.padStart(2, "0")}-${dd!.padStart(2, "0")}T${hh!.padStart(2, "0")}:${mi}:${ss}Z`;
  }

  // Match MM/DD/YYYY HH:mm:ss format
  const mmddyyyy =
    /^(\d{1,2})\/(\d{1,2})\/(\d{4})\s+(\d{2}):(\d{2}):(\d{2})$/.exec(trimmed);
  if (mmddyyyy) {
    const [, mm, dd, yyyy, hh, mi, ss] = mmddyyyy;
    return `${yyyy}-${mm!.padStart(2, "0")}-${dd!.padStart(2, "0")}T${hh}:${mi}:${ss}Z`;
  }
  return trimmed;
}

/**
 * Converts a UTC date string to datetime-local input value (YYYY-MM-DDTHH:mm).
 *
 * @param {string} utcStr - UTC date string (ISO or YYYY-MM-DD HH:mm:ss).
 * @returns {string} Local datetime-local value for input.
 */
export function utcToDatetimeLocal(utcStr: string | null | undefined): string {
  if (!utcStr) return "";
  const normalized = normalizeUtcDateString(utcStr.trim());
  const d = new Date(normalized);
  if (Number.isNaN(d.getTime())) return "";
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  const h = String(d.getHours()).padStart(2, "0");
  const min = String(d.getMinutes()).padStart(2, "0");
  return `${y}-${m}-${day}T${h}:${min}`;
}

/**
 * Builds a datetime-local input value from API strings: unzoned `YYYY-MM-DD HH:mm:ss` or
 * `YYYY-MM-DDTHH:mm:ss` as local wall time; zoned/ISO uses {@link utcToDatetimeLocal}.
 *
 * @param dateStr - Raw timestamp from API.
 * @returns {string} `YYYY-MM-DDTHH:mm` or empty if invalid.
 */
export function toDatetimeLocalInputFromApiString(
  dateStr: string | null | undefined,
): string {
  if (!dateStr?.trim()) return "";
  const t = dateStr.trim();
  const ms = parseApiLocalDateTimeMs(t);
  if (!Number.isNaN(ms)) {
    const d = new Date(ms);
    const pad = (x: number) => String(x).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
  return utcToDatetimeLocal(t);
}

/**
 * Parses `createdOn` for chat/comments: local wall `YYYY-MM-DD HH:mm:ss`, else UTC-normalized parse.
 *
 * @param createdOn - API timestamp.
 * @returns {Date} Parsed date or current date if unparseable.
 */
export function dateFromApiCreatedOn(
  createdOn: string | null | undefined,
): Date {
  if (!createdOn?.trim()) return new Date();
  const localMs = parseApiLocalDateTimeMs(createdOn);
  if (!Number.isNaN(localMs)) return new Date(localMs);
  const normalized = normalizeUtcDateString(createdOn.trim());
  const d = new Date(normalized);
  return Number.isNaN(d.getTime()) ? new Date() : d;
}

/**
 * Strips "[Customer]" or "[CUSTOMER]" prefix from call request reason for display in edit form.
 *
 * @param {string} reason - Raw reason from API.
 * @returns {string} Reason without the prefix.
 */
export function stripCustomerPrefixFromReason(reason: string): string {
  return reason.replace(/^\[Customer\]\s*/i, "").trim();
}

const CALL_REQ_SHORT_MONTHS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
] as const;

function padClockMinute(n: number): string {
  return String(n).padStart(2, "0");
}

/**
 * Preferred times from call-request API: wall clock with `Z` suffix (not a real UTC offset).
 */
const CALL_REQUEST_API_LITERAL_Z_RE =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?(?:\.\d+)?Z$/i;

function formatWallClockShort(
  month1to12: number,
  day: number,
  hour24: number,
  minute: number,
): string {
  if (
    month1to12 < 1 ||
    month1to12 > 12 ||
    day < 1 ||
    day > 31 ||
    hour24 < 0 ||
    hour24 > 23 ||
    minute < 0 ||
    minute > 59
  ) {
    return "--";
  }
  const mon = CALL_REQ_SHORT_MONTHS[month1to12 - 1];
  const h12 = hour24 % 12 === 0 ? 12 : hour24 % 12;
  const ampm = hour24 < 12 ? "AM" : "PM";
  return `${mon} ${day}, ${h12}:${padClockMinute(minute)} ${ampm}`;
}

/**
 * Formats call-request timestamps exactly as returned by the API (wall-clock), with no timezone conversion.
 * Supports literal `YYYY-MM-DDTHH:mm:ss.sssZ`, `YYYY-MM-DD HH:mm:ss`, and `MM/DD/YYYY HH:mm:ss`.
 *
 * @param dateStr - Raw string from the backend.
 * @returns Short English date/time or "--" if unparseable.
 */
export function formatCallRequestBackendDateTimeShort(
  dateStr: string | null | undefined,
): string {
  if (!dateStr?.trim()) return "--";
  const t = dateStr.trim();

  const literalZ = CALL_REQUEST_API_LITERAL_Z_RE.exec(t);
  if (literalZ) {
    const mo = Number(literalZ[2]);
    const d = Number(literalZ[3]);
    const h = Number(literalZ[4]);
    const mi = Number(literalZ[5]);
    return formatWallClockShort(mo, d, h, mi);
  }

  const ymd =
    /^(\d{4})-(\d{1,2})-(\d{1,2})\s+(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?$/.exec(
      t,
    );
  if (ymd) {
    const mo = Number(ymd[2]);
    const d = Number(ymd[3]);
    const h = Number(ymd[4]);
    const mi = Number(ymd[5]);
    return formatWallClockShort(mo, d, h, mi);
  }

  const mdy =
    /^(\d{1,2})\/(\d{1,2})\/(\d{4})\s+(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?$/.exec(
      t,
    );
  if (mdy) {
    const mo = Number(mdy[1]);
    const d = Number(mdy[2]);
    const h = Number(mdy[4]);
    const mi = Number(mdy[5]);
    return formatWallClockShort(mo, d, h, mi);
  }

  return "--";
}

/**
 * Time string for cancel/reject copy: matches the card (first preferred time), not `scheduleTime`
 * when the API sends a different value there.
 *
 * @param preferredTimes - From call request.
 * @param scheduleTime - Fallback when no preferred times.
 * @returns Short wall-clock string or "--" if nothing parseable.
 */
export function formatCallRequestPromptScheduledTime(
  preferredTimes: string[] | undefined,
  scheduleTime: string | null | undefined,
): string {
  const firstPreferred = preferredTimes?.find((t) => t?.trim());
  const raw = (firstPreferred ?? scheduleTime ?? "").trim();
  if (!raw) return "--";
  return formatCallRequestBackendDateTimeShort(raw);
}

/**
 * Returns whether the "Open Related Case" button should be shown (within 60 days of closedOn).
 *
 * @param {string | null | undefined} closedOn - Closed date from API (UTC string).
 * @returns {boolean} True if closedOn is missing/invalid (show for backward compat) or within 60 days.
 */
export function isWithinOpenRelatedCaseWindow(
  closedOn: string | null | undefined,
): boolean {
  if (!closedOn) return true;
  const normalized = normalizeUtcDateString(closedOn.trim());
  const closed = new Date(normalized);
  if (Number.isNaN(closed.getTime())) return true;
  const sixtyDaysLater = new Date(closed);
  sixtyDaysLater.setDate(sixtyDaysLater.getDate() + 60);
  return new Date() <= sixtyDaysLater;
}

/**
 * Formats a UTC date string for display in the specified (or browser-local) timezone.
 *
 * @param {string} dateStr - UTC date string (YYYY-MM-DD HH:mm:ss or MM/DD/YYYY HH:mm:ss).
 * @param {"short" | "long"} [formatStr="long"] - The format style.
 * @param {boolean} [includeTimeZoneName=true] - Include timezone suffix (e.g. +GMT).
 * @param {string} [userTimeZone] - IANA timezone string (e.g. "America/New_York"). Falls back to browser local timezone when omitted.
 * @returns {string} Formatted date/time in the specified timezone.
 */
export function formatUtcToLocal(
  dateStr: string | null | undefined,
  formatStr: "short" | "long" = "long",
  includeTimeZoneName = true,
  userTimeZone?: string,
): string {
  if (!dateStr) return "--";
  const normalized = normalizeUtcDateString(dateStr);
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return "--";
  const tzOption = includeTimeZoneName
    ? { timeZoneName: "short" as const }
    : {};
  const tzZone = userTimeZone ? { timeZone: userTimeZone } : {};
  if (formatStr === "short") {
    return new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "numeric",
      hour12: true,
      ...tzOption,
      ...tzZone,
    }).format(date);
  }
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "numeric",
    hour12: true,
    ...tzOption,
    ...tzZone,
  }).format(date);
}

/**
 * Formats a UTC date string for display (no timezone suffix like +GMT).
 *
 * @param {string} dateStr - UTC date string (YYYY-MM-DD HH:mm:ss or MM/DD/YYYY HH:mm:ss).
 * @param {"short" | "long"} [formatStr="long"] - The format style.
 * @returns {string} Formatted date/time in local time.
 */
export function formatUtcToLocalNoTimezone(
  dateStr: string | null | undefined,
  formatStr: "short" | "long" = "long",
): string {
  return formatUtcToLocal(dateStr, formatStr, false);
}

/**
 * Formats a date string into a user-friendly date and time format.
 *
 * @param {string} dateStr - The date string to format.
 * @param {"short" | "long"} [formatStr="long"] - The format style.
 * @returns {string} The formatted date and time.
 */
export function formatDateTime(
  dateStr: string | null | undefined,
  formatStr: "short" | "long" = "long",
): string {
  if (!dateStr) {
    return "Not Available";
  }

  // Handle API date format "YYYY-MM-DD HH:mm:ss" by converting space to 'T' for ISO format
  const normalizedDateStr = dateStr.trim().replace(" ", "T");
  const date = new Date(normalizedDateStr);

  if (isNaN(date.getTime())) {
    return "Not Available";
  }

  if (formatStr === "short") {
    return new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "numeric",
      hour12: true,
    }).format(date);
  }
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "numeric",
    hour12: true,
  }).format(date);
}

/**
 * Formats a UTC date string to display only the date (no time) in the user's local timezone.
 *
 * @param {string} dateStr - UTC date string (YYYY-MM-DD HH:mm:ss format from API or ISO format).
 * @returns {string} Formatted date without time (e.g., "Feb 25, 2026").
 */
export function formatDateOnly(dateStr: string | null | undefined): string {
  if (!dateStr) return "Not Available";

  const trimmed = dateStr.trim();

  // Handle YYYY-MM-DD HH:mm:ss format (standard API format for chat history and case details)
  const standardFormat =
    /^(\d{4})-(\d{2})-(\d{2})\s+(\d{2}):(\d{2}):(\d{2})$/.exec(trimmed);
  if (standardFormat) {
    const [, year, month, day, hour, minute, second] = standardFormat;
    const isoDate = `${year}-${month}-${day}T${hour}:${minute}:${second}Z`;
    const date = new Date(isoDate);
    if (!Number.isNaN(date.getTime())) {
      return new Intl.DateTimeFormat("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      }).format(date);
    }
  }

  // Handle ISO format (YYYY-MM-DDTHH:mm:ssZ or similar)
  if (/T\d{2}:\d{2}/.test(trimmed)) {
    const date = new Date(trimmed);
    if (!Number.isNaN(date.getTime())) {
      return new Intl.DateTimeFormat("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      }).format(date);
    }
  }

  return "Not Available";
}

export type ChatActionState =
  | "primary"
  | "info"
  | "success"
  | "warning"
  | "error";

/** Assigned engineer from API: string or { id, label?, name? } object. */
export type AssignedEngineerValue =
  | string
  | { id: string; label?: string; name?: string | null }
  | null
  | undefined;

/** Extracts display string from assigned engineer object (label or name). */
function getAssignedEngineerDisplayValue(obj: {
  id: string;
  label?: string;
  name?: string | null;
}): string {
  return obj.label ?? obj.name ?? "";
}

/**
 * Extracts display label from assigned engineer (string or { id, label?, name? } object).
 *
 * @param value - Assigned engineer from API.
 * @returns {string} Display label or empty string if null/undefined.
 */
export function getAssignedEngineerLabel(value: AssignedEngineerValue): string {
  if (value == null || value === "") return "";
  if (typeof value === "object") return getAssignedEngineerDisplayValue(value);
  return typeof value === "string" ? value : "";
}

/**
 * Formats a value for display in case details; null/undefined/empty become "--".
 * Handles assigned engineer object: { id, label? } or { id, name? } -> display value.
 *
 * @param value - Raw value from API or state.
 * @returns {string} Display string.
 */
export function formatValue(
  value:
    | string
    | number
    | { id: string; label?: string; name?: string | null }
    | null
    | undefined,
): string {
  if (value == null || value === "") return "--";
  if (typeof value === "object")
    return getAssignedEngineerDisplayValue(value) || "--";
  return String(value);
}

/**
 * Derives initials from a name string or assigned engineer object (e.g. "John Doe" -> "JD").
 *
 * @param name - Full name string or { id, label?, name? } object from API.
 * @returns {string} Up to 2 uppercase initials, or "--" if empty/invalid.
 */
export function getInitials(
  name:
    | string
    | { id: string; label?: string; name?: string | null }
    | null
    | undefined,
): string {
  const label =
    typeof name === "object" && name
      ? getAssignedEngineerDisplayValue(name)
      : name;
  if (!label || typeof label !== "string") return "--";
  const initials = label
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
  return initials || "--";
}

/**
 * Formats SLA response time from milliseconds (string or number) to human-readable (e.g. "4 hours", "2 days").
 *
 * @param ms - Milliseconds as string or number from API.
 * @returns {string} Formatted string or "--" if invalid.
 */
export function formatSlaResponseTime(
  ms: string | number | null | undefined,
): string {
  const n = typeof ms === "string" ? parseInt(ms, 10) : ms;
  if (n == null || Number.isNaN(n) || n < 0) return "--";
  if (n < 60_000) {
    const seconds = Math.floor(n / 1000);
    return `${seconds} second${seconds === 1 ? "" : "s"}`;
  }
  if (n < 3600_000) {
    const minutes = Math.floor(n / 60_000);
    return `${minutes} minute${minutes === 1 ? "" : "s"}`;
  }
  if (n < 86400_000) {
    const hours = Math.floor(n / 3600_000);
    return `${hours} hour${hours === 1 ? "" : "s"}`;
  }
  const days = Math.floor(n / 86400_000);
  return `${days} day${days === 1 ? "" : "s"}`;
}

/**
 * Formats byte count for display (e.g. 1024 -> "1 KB", 245760 -> "240 KB").
 *
 * @param bytes - Size in bytes (number or string from API).
 * @returns {string} Formatted string like "1.2 MB" or "18 KB".
 */
export function formatFileSize(
  bytes: number | string | null | undefined,
): string {
  const n = typeof bytes === "string" ? parseInt(bytes, 10) : bytes;
  if (n == null || Number.isNaN(n)) return "--";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1).replace(/\.0$/, "")} KB`;
  return `${(n / (1024 * 1024)).toFixed(1).replace(/\.0$/, "")} MB`;
}

/**
 * Returns whether to show View or Resume for a chat status.
 *
 * @param status - Chat status string (e.g. Resolved, Still Open, Abandoned).
 * @returns {ChatAction} "view" or "resume".
 */
export function getChatStatusAction(status: string): ChatAction {
  const normalized = status?.toLowerCase() || "";

  switch (true) {
    case normalized.includes("resolved"):
      return ChatAction.VIEW;
    default:
      return ChatAction.RESUME;
  }
}

/**
 * Returns the color for a chat action button.
 *
 * @param action - The action type ("view" or "resume").
 * @returns {ChatActionState} Palette color path.
 */
export function getChatActionColor(action: ChatAction): ChatActionState {
  switch (action) {
    case ChatAction.RESUME:
      return "warning";
    case ChatAction.VIEW:
      return "info";
    default:
      return "primary";
  }
}

/**
 * Returns the color path for a chat status.
 *
 * @param status - Chat status string.
 * @returns {string} Palette color path.
 */
export function getChatStatusColor(status: string): string {
  const normalized = status?.toLowerCase() || "";

  switch (true) {
    case normalized.includes(ChatStatus.RESOLVED.toLowerCase()):
      return "success.main";
    case normalized.includes(ChatStatus.OPEN.toLowerCase()):
      return "info.main";
    case normalized.includes(ChatStatus.ABANDONED.toLowerCase()):
      return "error.main";
    default:
      return "secondary.main";
  }
}

/**
 * Returns the color path for a conversation status.
 *
 * @param status - Conversation status string.
 * @returns {string} Palette color path.
 */
export function getConversationStatusColor(status: string): string {
  const normalized = status?.toLowerCase() || "";

  switch (true) {
    case normalized.includes(ConversationStatus.ABANDONED.toLowerCase()):
      return "error.main";
    case normalized.includes(ConversationStatus.ACTIVE.toLowerCase()):
      return "success.main";
    case normalized.includes(ConversationStatus.CONVERTED.toLowerCase()):
      return "info.main";
    case normalized.includes(ConversationStatus.OPEN.toLowerCase()):
      return "warning.main";
    case normalized.includes(ConversationStatus.RESOLVED.toLowerCase()):
      return "success.main";
    default:
      return "secondary.main";
  }
}

/**
 * Returns the icon component for a conversation status.
 *
 * @param status - Conversation status string.
 * @returns {ComponentType} Icon component.
 */
export function getConversationStatusIcon(status?: string): ComponentType<{
  size?: number;
  color?: string;
}> {
  const normalized = status?.toLowerCase() || "";

  switch (true) {
    case normalized.includes(ConversationStatus.ABANDONED.toLowerCase()):
      return XCircle;
    case normalized.includes(ConversationStatus.ACTIVE.toLowerCase()):
      return Activity;
    case normalized.includes(ConversationStatus.CONVERTED.toLowerCase()):
      return CheckCircle;
    case normalized.includes(ConversationStatus.OPEN.toLowerCase()):
      return PlayCircle;
    case normalized.includes(ConversationStatus.RESOLVED.toLowerCase()):
      return CircleCheck;
    default:
      return CircleAlert;
  }
}

/**
 * Returns sx object for SupportOverviewCard-style colored chips.
 *
 * @param {string} colorPath - Theme palette path (e.g. "success.main", "info.main").
 * @param {Theme} theme - Oxygen UI theme.
 * @returns {object} MUI sx object for Chip.
 */
export function getSupportOverviewChipSx(
  colorPath: string,
  theme: Theme,
): Record<string, unknown> {
  const resolvedColor = resolveColorFromTheme(colorPath, theme);
  return {
    bgcolor: alpha(resolvedColor, 0.1),
    color: resolvedColor,
    height: 20,
    fontSize: "0.75rem",
  };
}

/**
 * Returns sx object for plain (non-colored) chips.
 *
 * @returns {object} MUI sx object for Chip.
 */
export function getPlainChipSx(): Record<string, unknown> {
  return {
    height: 20,
    fontSize: "0.75rem",
  };
}

/**
 * Resolves a color from the theme palette for the alpha() utility.
 *
 * @param path - Color path.
 * @param theme - Oxygen UI theme.
 * @returns {string} The resolved color string.
 */
export function resolveColorFromTheme(path: string, theme: Theme): string {
  return (
    (path
      .split(".")
      .reduce<unknown>(
        (acc, part) =>
          (acc as Record<string, unknown> | null | undefined)?.[part],
        theme.palette as unknown,
      ) as string) || path
  );
}

/**
 * Formats a date string or Date object into a relative time (e.g., "2 days ago", "1 hour ago").
 *
 * @param date - Date string or Date object.
 * @returns {string} Human readable relative time.
 */
export function formatRelativeTime(date: string | Date | undefined): string {
  if (!date) return "--";

  const d = typeof date === "string" ? new Date(date) : date;
  if (isNaN(d.getTime())) return "--";

  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - d.getTime()) / 1000);

  if (diffInSeconds < 60) return "just now";

  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) {
    return `${diffInMinutes} minute${diffInMinutes === 1 ? "" : "s"} ago`;
  }

  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) {
    return `${diffInHours} hour${diffInHours === 1 ? "" : "s"} ago`;
  }

  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 30) {
    return `${diffInDays} day${diffInDays === 1 ? "" : "s"} ago`;
  }

  const diffInMonths = Math.floor(diffInDays / 30);
  if (diffInMonths < 12) {
    return `${diffInMonths} month${diffInMonths === 1 ? "" : "s"} ago`;
  }

  const diffInYears = Math.floor(diffInMonths / 12);
  return `${diffInYears} year${diffInYears === 1 ? "" : "s"} ago`;
}

/**
 * Derives the label and pluralized "all" label from a filter ID.
 *
 * @param id - The filter ID (e.g., "status").
 * @returns { label: string; allLabel: string } The derived labels.
 */
export function deriveFilterLabels(id: string): {
  label: string;
  allLabel: string;
} {
  if (id === "caseType") {
    return { label: "Case Type", allLabel: "All Case Types" };
  }
  const label = id.charAt(0).toUpperCase() + id.slice(1);
  const allLabel = `All ${
    label.endsWith("s")
      ? `${label}es`
      : label.endsWith("y")
        ? `${label.slice(0, -1)}ies`
        : `${label}s`
  }`;

  return { allLabel, label };
}

/** Attachment file category for icon selection. TODO: Replace with enum in a later PR. */
export type AttachmentFileCategory =
  | "image"
  | "pdf"
  | "archive"
  | "text"
  | "file";

/**
 * Returns the file category for attachment icon/display (image, pdf, archive, text, file).
 *
 * @param fileName - File name.
 * @param type - MIME type.
 * @returns {AttachmentFileCategory} The category.
 */
export function getAttachmentFileCategory(
  fileName: string,
  type: string,
): AttachmentFileCategory {
  const n = fileName.toLowerCase();
  const t = type.toLowerCase();
  if (
    /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i.test(n) ||
    t.startsWith("image/")
  ) {
    return "image";
  }
  if (/\.pdf$/i.test(n) || t.includes("pdf")) return "pdf";
  if (
    /\.(zip|rar|7z|tar|gz)$/i.test(n) ||
    t.includes("zip") ||
    t.includes("archive")
  ) {
    return "archive";
  }
  if (/\.(log|txt)$/i.test(n) || t.startsWith("text/")) return "text";
  return "file";
}

/**
 * Returns the icon component for a given case status label.
 *
 * @param statusLabel - The case status label.
 * @returns {ComponentType<{ size?: number }>} The icon component.
 */
export function getStatusIcon(
  statusLabel?: string,
): ComponentType<{ size?: number }> {
  const normalized = statusLabel?.toLowerCase() || "";

  switch (true) {
    case normalized === CaseStatus.OPEN.toLowerCase():
      return CircleAlert;
    case normalized === CaseStatus.WORK_IN_PROGRESS.toLowerCase():
      return Clock;
    case normalized === CaseStatus.AWAITING_INFO.toLowerCase():
      return MessageCircle;
    case normalized === CaseStatus.WAITING_ON_WSO2.toLowerCase():
      return CircleQuestionMark;
    case normalized === CaseStatus.SOLUTION_PROPOSED.toLowerCase():
      return CircleCheck;
    case normalized === CaseStatus.CLOSED.toLowerCase():
      return CircleX;
    case normalized === CaseStatus.REOPENED.toLowerCase():
      return RotateCcw;
    default:
      return CircleAlert;
  }
}

/**
 * Returns the Oxygen UI color path for a given call request status label.
 *
 * @param status - The call request status (e.g., "SCHEDULED", "PENDING").
 * @returns {string} The Oxygen UI color path.
 */
export function getCallRequestStatusColor(status?: string): string {
  const normalized = status?.toLowerCase() || "";

  switch (true) {
    case normalized.includes(CallRequestStatus.SCHEDULED.toLowerCase()):
      return "info.main";
    case normalized.includes(CallRequestStatus.PENDING.toLowerCase()):
      return "warning.main";
    case normalized.includes(CallRequestStatus.COMPLETED.toLowerCase()):
      return "success.main";
    case normalized.includes(CallRequestStatus.CANCELED.toLowerCase()):
    case normalized.includes(CallRequestStatus.REJECTED.toLowerCase()):
      return "error.main";
    default:
      return "text.secondary";
  }
}

/**
 * Maps severity API label (e.g. "Critical (P1)") to display name (S0-S4).
 *
 * @param {string} label - The severity label from API.
 * @returns {string} Mapped display name (S0, S1, S2, S3, S4) or original if no match.
 */
export function mapSeverityToDisplay(label?: string): string {
  if (!label) return "—";
  const trimmed = label.trim();
  const direct = SEVERITY_LABEL_TO_DISPLAY[trimmed];
  if (direct) return direct;
  const lower = trimmed.toLowerCase();
  const entry = Object.entries(SEVERITY_LABEL_TO_DISPLAY).find(
    ([k]) => k.toLowerCase() === lower,
  );
  return entry?.[1] ?? label;
}

/**
 * Whether a severity chip should be shown (hide when API has no severity to avoid "—" chips).
 *
 * @param label - Raw severity label from API.
 * @returns {boolean} True when severity is present and maps to a display value.
 */
export function hasSeverityLabelForChip(label?: string | null): boolean {
  if (label == null || String(label).trim() === "") {
    return false;
  }
  const display = mapSeverityToDisplay(label).trim();
  return display !== "—" && display !== "";
}

/**
 * Returns true if the case has S0 (Catastrophic) severity.
 * Used to filter out S0 cases when project type is not Managed Cloud Subscription.
 *
 * @param caseItem - Case with optional severity.
 * @returns {boolean}
 */
export function isS0Case(caseItem: {
  severity?: { label?: string } | null;
}): boolean {
  return isS0SeverityLabel(caseItem?.severity?.label);
}

/**
 * Returns the icon component for a severity label (announcement cards).
 * S0/S1: TriangleAlert, S2: CircleAlert, S3: Clock, S4: CircleCheck.
 *
 * @param label - API severity label (e.g. "1 - Critical", "Critical (P1)").
 * @returns {ComponentType} Icon component.
 */
export function getSeverityIcon(label?: string): ComponentType<{
  size?: number;
  color?: string;
}> {
  const display = mapSeverityToDisplay(label);
  const upper = display.toUpperCase();
  switch (upper) {
    case CaseSeverityLevel.S0:
    case CaseSeverityLevel.S1:
      return TriangleAlert;
    case CaseSeverityLevel.S2:
      return CircleAlert;
    case CaseSeverityLevel.S3:
      return Clock;
    case CaseSeverityLevel.S4:
      return CircleCheck;
    default:
      return CircleAlert;
  }
}

/**
 * Returns the Oxygen UI color path for a given severity label.
 *
 * @param {string} label - The severity label.
 * @returns {string} The Oxygen UI color path.
 */
export function getSeverityColor(label?: string): string {
  const normalized = mapSeverityToDisplay(label);
  const upper = normalized.toUpperCase();
  switch (upper) {
    case "S0":
      return "error.main";
    case CaseSeverityLevel.S1:
      return "warning.main";
    case CaseSeverityLevel.S2:
      return "info.main";
    case CaseSeverityLevel.S3:
      return "secondary.main";
    case CaseSeverityLevel.S4:
      return "success.main";
    default:
      return "text.primary";
  }
}

/**
 * Returns the color (hex) for a given case status label using colors.yellow[500] style.
 *
 * @param {string} label - The case status label.
 * @returns {string} Hex color string (e.g. colors.blue[500]).
 */
export function getStatusColor(label?: string): string {
  switch (label) {
    case CaseStatus.OPEN:
      return colors.blue[500];
    case CaseStatus.WORK_IN_PROGRESS:
      return colors.orange[500];
    case CaseStatus.AWAITING_INFO:
      return colors.grey[500];
    case CaseStatus.WAITING_ON_WSO2:
      return colors.green[500];
    case CaseStatus.SOLUTION_PROPOSED:
      return colors.yellow[900];
    case CaseStatus.CLOSED:
      return colors.red[500];
    case CaseStatus.REOPENED:
      return colors.purple[500];
    default:
      return colors.grey[500];
  }
}

/**
 * Returns the icon element for a given case status label (avoids creating a component during render).
 *
 * @param statusLabel - The case status label.
 * @param size - Icon size in pixels.
 * @returns {ReactNode} The icon element.
 */
export function getStatusIconElement(
  statusLabel: string | null | undefined,
  size = 12,
): ReactNode {
  const Icon = getStatusIcon(statusLabel ?? undefined);
  return createElement(Icon, { size });
}

/**
 * Converts [code]...[/code] tags to HTML <code> elements for proper display.
 * Handles inline usage like "Case Task [code]CSTASK0010746[/code] has been created".
 *
 * @param content - Raw content string.
 * @returns {string} Content with [code] tags replaced by <code> elements.
 */
export function convertCodeTagsToHtml(content: string): string {
  if (!content || typeof content !== "string") return "";
  const normalized = content
    .replace(/\[\\\/code\]/gi, "[/code]")
    .replace(/\[\\\/CODE\]/g, "[/code]")
    .replace(/\[\\code\]/gi, "[code]")
    .replace(/\[\\CODE\]/g, "[code]")
    .replace(
    /\[\/code\]\s*\[code\]/gi,
    "[/code]\n[code]",
  );
  return normalized
    .replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "<code>$1</code>")
    .replace(/\[\/?code\]/gi, "\n");
}

/**
 * Strips all [code]...[/code] blocks and returns concatenated inner HTML.
 * Used for multi-block content to avoid grey <code> background on structured sections.
 *
 * @param content - Raw content with one or more [code]...[/code] blocks.
 * @returns {string} Inner HTML without code wrappers.
 */
export function stripAllCodeBlocks(content: string): string {
  if (!content || typeof content !== "string") return "";
  const normalized = content
    .replace(/\[\\\/code\]/gi, "[/code]")
    .replace(/\[\\\/CODE\]/g, "[/code]")
    .replace(/\[\\code\]/gi, "[code]")
    .replace(/\[\\CODE\]/g, "[code]")
    .replace(/\[\/code\]\s*\[code\]/gi, "[/code]\n[code]");
  return normalized
    .replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "$1\n")
    .replace(/\[\/?code\]/gi, "\n");
}

/**
 * Removes leading <br>, <br/>, <br /> and whitespace from HTML.
 * Fixes extra blank first line from content like "[code]<br><b>...</b>[/code]".
 *
 * @param html - HTML string.
 * @returns {string} HTML with leading br/whitespace removed.
 */
export function trimLeadingBr(html: string): string {
  if (!html || typeof html !== "string") return "";
  return html.replace(/^(\s*<br\s*\/?>\s*)+/i, "").trimStart();
}

/**
 * Returns true if content has exactly one top-level [code]...[/code] wrapper
 * (no multiple [code] blocks). Used to decide between stripCodeWrapper and convertCodeTagsToHtml.
 *
 * @param content - Raw content string.
 * @returns {boolean} True when single wrapper.
 */
export function hasSingleCodeWrapper(content: string): boolean {
  if (!content || typeof content !== "string") return false;
  const trimmed = content.trim();
  if (!trimmed.startsWith("[code]") || !trimmed.endsWith("[/code]")) {
    return false;
  }
  const codeOpen = trimmed.match(/\[code\]/gi);
  const codeClose = trimmed.match(/\[\/code\]/gi);
  return (codeOpen?.length ?? 0) === 1 && (codeClose?.length ?? 0) === 1;
}

/**
 * Strips the [code]...[/code] wrapper from comment content.
 * Only strips when there is exactly one wrapper (use hasSingleCodeWrapper first).
 *
 * @param content - Raw content string.
 * @returns {string} Content without the code wrapper.
 */
export function stripCodeWrapper(content: string): string {
  if (!content || typeof content !== "string") return "";
  const trimmed = content.trim();
  if (!hasSingleCodeWrapper(content)) return content;
  return trimmed.slice(6, -7).trim();
}

/**
 * Strips "Customer comment added" label from comment content.
 * The backend may append this; we hide it from the activity timeline.
 *
 * @param html - HTML content string.
 * @returns {string} Content without the label.
 */
export function stripCustomerCommentAddedLabel(html: string): string {
  if (!html || typeof html !== "string") return "";
  return html
    .replace(/<p>\s*Customer comment added\s*<\/p>/gi, "")
    .replace(/Customer comment added/gi, "")
    .trim();
}

/**
 * Returns true if the comment has content worth displaying (after stripping code wrapper and
 * "Customer comment added" label). Used to hide backend entries that render as empty bubbles.
 *
 * @param comment - Case comment from API.
 * @returns {boolean} True when comment has non-empty displayable content.
 */
export function hasDisplayableContent(comment: CaseComment): boolean {
  const raw = comment.content ?? "";
  const stripped = stripCodeWrapper(raw);
  const withoutLabel = stripCustomerCommentAddedLabel(stripped);
  const textOnly = withoutLabel.replace(/<[^>]+>/g, "").trim();
  return textOnly.length > 0;
}

export type { InlineAttachment };

/** DOMPurify config for inline comment HTML. */
export const INLINE_COMMENT_HTML_PURIFY: Record<string, never> = {};

/**
 * Extracts ServiceNow-style attachment id from img src (relative /id.iix or absolute https://host/id.iix).
 *
 * @param src - Raw src attribute.
 * @returns {string} Suspected attachment id/sys_id or empty string.
 */
export function extractInlineImageRefId(src: string): string {
  const s = src.trim();
  const fromPath = s.match(/\/([a-f0-9]{32})\.iix(?:\?|#|$)/i);
  if (fromPath) {
    return fromPath[1];
  }
  const tail =
    s
      .replace(/\.iix$/i, "")
      .split("/")
      .pop()
      ?.trim() ?? "";
  if (/^[a-f0-9]{32}$/i.test(tail)) {
    return tail;
  }
  return s
    .replace(/^\//, "")
    .replace(/\.iix$/i, "")
    .trim();
}

function resolveInlineImageDisplaySrc(
  attachment: InlineAttachment,
  originalSrc: string,
): string {
  const id = attachment.id ?? attachment.sys_id;
  const originMatch = originalSrc.match(/^(https?:\/\/[^/]+)/i);
  if (originMatch && id) {
    return `${originMatch[1]}/${id}.iix`;
  }
  return attachment.previewUrl ?? attachment.downloadUrl ?? attachment.url ?? originalSrc;
}

/**
 * Replaces inline image sources in HTML (e.g. /sys_id.iix or /id.iix) with URLs from attachments.
 * Prefers same-origin `https://host/<id>.iix` when the HTML already used that host (matches ServiceNow inline images).
 * Sanitizes the result with DOMPurify to prevent XSS.
 *
 * @param html - HTML string with img tags.
 * @param inlineAttachments - Optional list of attachments (id/downloadUrl or sys_id/url).
 * @returns {string} Sanitized HTML with img src replaced where matching.
 */
export function replaceInlineImageSources(
  html: string,
  inlineAttachments?: InlineAttachment[] | null,
): string {
  if (!html || typeof html !== "string") return "";

  const normalizedHtml = html.replace(/\\\//g, "/");

  if (!inlineAttachments?.length) {
    return String(
      DOMPurify.sanitize(normalizedHtml, INLINE_COMMENT_HTML_PURIFY),
    );
  }

  const replaced = normalizedHtml.replace(
    /<img([^>]*?)\s+src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))([^>]*)>/gi,
    (_match, before, doubleSrc, singleSrc, bareSrc, after) => {
      const src = (doubleSrc ?? singleSrc ?? bareSrc ?? "") as string;
      const refId = extractInlineImageRefId(src);
      const attachment = inlineAttachments.find(
        (a) =>
          (a.id && (a.id === refId || src.includes(a.id))) ||
          (a.sys_id && (a.sys_id === refId || src.includes(a.sys_id))),
      );
      const quote =
        doubleSrc !== undefined ? '"' : singleSrc !== undefined ? "'" : '"';

      if (!attachment) {
        return `<img${before} src=${quote}${src}${quote}${after}>`;
      }

      const newSrc = resolveInlineImageDisplaySrc(attachment, src);

      return `<img${before} src=${quote}${newSrc}${quote}${after}>`;
    },
  );
  return String(DOMPurify.sanitize(replaced, INLINE_COMMENT_HTML_PURIFY));
}

/**
 * Normalizes ServiceNow-style timestamp "YYYY-MM-DD HH:MM:SS" to ISO UTC for parsing.
 *
 * @param dateStr - Raw date string (ISO, ServiceNow, or parseable).
 * @returns {string} Normalized string for Date constructor.
 */
function normalizeCommentDateString(dateStr: string): string {
  const trimmed = dateStr.trim();
  // API sends wall-clock local time without offset; do not treat as UTC.
  if (/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}$/.test(trimmed)) {
    return trimmed.replace(" ", "T");
  }
  return trimmed;
}

/**
 * Formats a comment date string for display (e.g. "Feb 13, 2026 3:45 PM").
 *
 * @param date - Date string from API (ISO or ServiceNow "YYYY-MM-DD HH:MM:SS").
 * @returns {string} Formatted date string.
 */
export function formatCommentDate(date: string | null | undefined): string {
  if (!date) return "--";
  const normalized = normalizeCommentDateString(date);
  const d = new Date(normalized);
  if (Number.isNaN(d.getTime())) return "--";
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/**
 * Strips HTML tags from a string.
 *
 * @param html - HTML content string.
 * @returns {string} Plain text without HTML tags.
 */
export function stripHtml(html: string | null | undefined): string {
  if (!html || typeof html !== "string") return "";
  return html.replace(/<[^>]+>/g, "").trim();
}

/**
 * Maps action labels to present tense for display (e.g., "Closed" -> "Close").
 *
 * @param label - Action label (e.g., "Closed", "Reopened").
 * @returns {string} Present tense label.
 */
export function toPresentTenseActionLabel(label: string): string {
  const map: Record<string, string> = {
    Closed: "Close",
    Reopened: "Reopen",
    "Waiting on WSO2": "Wait on WSO2",
    "Waiting On WSO2": "Wait on WSO2",
  };
  return map[label] ?? label;
}

/**
 * Maps action labels to present continuous for loading state (e.g., "Closed" -> "Closing...").
 *
 * @param label - Action label (e.g., "Closed", "Accept Solution").
 * @returns {string} Present continuous label with ellipsis.
 */
export function toPresentContinuousActionLabel(label: string): string {
  const map: Record<string, string> = {
    Closed: "Closing...",
    Reopened: "Reopening...",
    "Waiting on WSO2": "Waiting on WSO2...",
    "Waiting On WSO2": "Waiting on WSO2...",
    "Accept Solution": "Accepting...",
    "Reject Solution": "Rejecting...",
  };
  return map[label] ?? `${label}...`;
}

/** Maps action labels (from getAvailableCaseActions) to caseState labels for lookup. */
export const ACTION_TO_CASE_STATE_LABEL: Record<string, string> = {
  Closed: "Closed",
  "Waiting on WSO2": "Waiting On WSO2",
  "Accept Solution": "Closed",
  "Reject Solution": "Waiting On WSO2",
};

/**
 * Returns the list of available action button labels based on the case status.
 *
 * @param status - Current status of the case.
 * @returns {string[]} Array of action button labels to display.
 */
export function getAvailableCaseActions(
  status: string | null | undefined,
): string[] {
  const normalized = status?.toLowerCase() || "";

  switch (normalized) {
    case CaseStatus.CLOSED.toLowerCase():
      return ["Open Related Case"];

    case CaseStatus.SOLUTION_PROPOSED.toLowerCase():
      return ["Accept Solution", "Reject Solution"];

    case CaseStatus.AWAITING_INFO.toLowerCase():
      return ["Closed", "Waiting on WSO2"];

    default:
      // Covers Open, Work in Progress, Waiting on WSO2, Reopened.
      return ["Closed"];
  }
}

/**
 * Returns the Announcement case type ID from the case types metadata.
 * Used to fetch announcements (cases with type Announcement) from the cases API.
 *
 * @param caseTypes - Array of case types from useGetProjectFilters.
 * @returns {string | undefined} The Announcement type ID or undefined.
 */
export function getAnnouncementCaseTypeId(
  caseTypes?: MetadataItem[] | null,
): string | undefined {
  if (!caseTypes?.length) return undefined;
  return caseTypes.find((c) => c.label.toLowerCase() === "announcement")?.id;
}

/**
 * Normalizes case type options for display in a selector/filter.
 *
 * - Merges "Query" and "Incident" types into a single "Case" option
 *   (with their IDs joined by comma as the value)
 * - Removes "Announcement" types entirely
 * - Keeps all other case types as-is
 */
export const normalizeCaseTypeOptions = (
  caseTypes: { id: string; label: string }[],
) => {
  // Collect IDs of "Query" and "Incident" (including backend typo "icident") to merge into one "Case" option
  const caseIds = caseTypes
    .filter((c) =>
      ["query", "incident", "icident"].includes(c.label.toLowerCase()),
    )
    .map((c) => c.id);

  // Keep all types except Query, Incident, Icident, and Announcement
  const others = caseTypes.filter(
    (c) =>
      !["query", "incident", "icident", "announcement"].includes(
        c.label.toLowerCase(),
      ),
  );

  return [
    ...others.map((c) => ({ label: c.label, value: c.id })),
    ...(caseIds.length ? [{ label: "Case", value: caseIds.join(",") }] : []),
  ];
};

/**
 * Estimates the number of display lines for given HTML content.
 * Counts only non-empty content lines, ignoring empty lines and whitespace.
 *
 * @param {string} html - HTML content to analyze.
 * @returns {number} Estimated line count.
 */
export function estimateLineCount(html: string): number {
  const processed = html
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/(p|div|h[1-6]|li|blockquote|pre|ul|ol)>/gi, "\n");

  // Then strip remaining HTML tags
  const plainText = processed.replace(/<[^>]+>/g, "");

  // Split by newlines and filter out empty lines
  const lines = plainText
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);

  if (lines.length === 0) {
    return 0;
  }

  // Count lines with content, and estimate long lines that wrap
  let totalLines = 0;
  const CHARS_PER_LINE = 80;

  for (const line of lines) {
    // Each content line takes at least 1 line, more if it's long
    totalLines += Math.max(1, Math.ceil(line.length / CHARS_PER_LINE));
  }

  return totalLines;
}

/**
 * Parses API timestamps for ordering: unzoned `YYYY-MM-DD HH:mm:ss` or `YYYY-MM-DDTHH:mm:ss`
 * as local wall time; strings with `Z` or numeric offset use instant parsing.
 *
 * @param dateStr - Raw timestamp from API.
 * @returns {number} Epoch ms or NaN if invalid.
 */
export function parseApiLocalDateTimeMs(
  dateStr: string | null | undefined,
): number {
  if (!dateStr?.trim()) return Number.NaN;
  const trimmed = dateStr.trim();

  if (/Z$/i.test(trimmed) || /[+-]\d{2}:?\d{2}(?::?\d{2})?$/.test(trimmed)) {
    const t = Date.parse(trimmed);
    return Number.isNaN(t) ? Number.NaN : t;
  }

  const local =
    /^(\d{4})-(\d{2})-(\d{2})[\sT](\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?$/.exec(
      trimmed,
    );
  if (local) {
    const y = Number(local[1]);
    const mo = Number(local[2]);
    const d = Number(local[3]);
    const h = Number(local[4]);
    const mi = Number(local[5]);
    const s = Number(local[6]);
    if (
      mo < 1 ||
      mo > 12 ||
      d < 1 ||
      d > 31 ||
      h < 0 ||
      h > 23 ||
      mi > 59 ||
      s > 59
    ) {
      return Number.NaN;
    }
    const dt = new Date(y, mo - 1, d, h, mi, s);
    return dt.getTime();
  }

  const t = Date.parse(trimmed);
  return Number.isNaN(t) ? Number.NaN : t;
}

/**
 * True for Novera / bot automation rows (conversation + case comments).
 *
 * @param createdBy - Comment author.
 * @param type - Optional message type (e.g. "bot").
 * @returns {boolean} Whether this row is treated as assistant/bot for ordering.
 */
export function isNoveraOrBotSender(
  createdBy?: string | null,
  type?: string | null,
): boolean {
  const by = (createdBy ?? "").trim().toLowerCase();
  const ty = (type ?? "").trim().toLowerCase();
  return ty === "bot" || by === "novera";
}

/** Shape accepted by {@link compareByCreatedOnThenId}. */
export type CreatedOnSortable = {
  createdOn?: string | null;
  id?: string | null;
  createdBy?: string | null;
  type?: string | null;
};

/**
 * Stable sort: time ascending, then human before Novera/bot when timestamps tie, then id.
 */
export function compareByCreatedOnThenId(
  a: CreatedOnSortable,
  b: CreatedOnSortable,
): number {
  const aT = parseApiLocalDateTimeMs(a.createdOn ?? undefined);
  const bT = parseApiLocalDateTimeMs(b.createdOn ?? undefined);
  const aOk = !Number.isNaN(aT);
  const bOk = !Number.isNaN(bT);
  if (aOk && bOk && aT !== bT) return aT - bT;
  if (aOk && !bOk) return -1;
  if (!aOk && bOk) return 1;
  if (aOk && bOk && aT === bT) {
    const aBot = isNoveraOrBotSender(a.createdBy, a.type);
    const bBot = isNoveraOrBotSender(b.createdBy, b.type);
    if (aBot !== bBot) {
      return aBot ? 1 : -1;
    }
  }
  const idA = a.id ?? "";
  const idB = b.id ?? "";
  return idA.localeCompare(idB);
}

/**
 * Earliest datetime-local value for scheduling a call: now + severity allocation minutes,
 * rounded up to the next 5-minute boundary (e.g. 9:23 + 30m → 9:55).
 *
 * @param allocationMinutes - Minutes from filter metadata for this severity id.
 * @returns {string} Value for input type="datetime-local" min attribute.
 */
export function computeMinScheduleDatetimeLocal(
  allocationMinutes?: number | null,
): string {
  const pad = (x: number) => String(x).padStart(2, "0");
  const toLocalStr = (d: Date) =>
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;

  const nowPlusOne = new Date(Date.now() + 60 * 1000);
  const floorStr = toLocalStr(nowPlusOne);

  if (
    allocationMinutes == null ||
    Number.isNaN(allocationMinutes) ||
    allocationMinutes < 0
  ) {
    return floorStr;
  }

  const target = new Date(Date.now() + allocationMinutes * 60 * 1000);
  const rem = target.getMinutes() % 5;
  if (rem !== 0) {
    target.setMinutes(target.getMinutes() + (5 - rem));
  }
  target.setSeconds(0, 0);
  const severityStr = toLocalStr(target);
  return severityStr > floorStr ? severityStr : floorStr;
}

/**
 * Filter API / profile may return IDs that are not valid for `Intl` `timeZone`
 * (e.g. WSO2/Colombo). Map those to canonical IANA zones.
 */
const API_TIMEZONE_TO_INTL_ALIASES: Record<string, string> = {
  "WSO2/Colombo": "Asia/Colombo",
};

/**
 * Returns an ID accepted by `Intl.DateTimeFormat` `{ timeZone }`, or null if unknown/invalid.
 *
 * @param apiTimeZone - Value from profile or filters metadata.
 */
function normalizeApiTimeZoneToIntlTimeZone(
  apiTimeZone: string | null | undefined,
): string | null {
  const raw = apiTimeZone?.trim();
  if (!raw) return null;
  const candidate = API_TIMEZONE_TO_INTL_ALIASES[raw] ?? raw;
  try {
    Intl.DateTimeFormat("en-US", { timeZone: candidate }).format(
      new Date("2020-06-15T12:00:00Z"),
    );
    return candidate;
  } catch {
    return null;
  }
}

/**
 * Zone for call scheduling: normalized profile/filter value, or the browser's time zone.
 *
 * @param profileOrApiTimeZone - User profile `timeZone` (may be API-specific id).
 */
export function resolveCallSchedulingTimeZone(
  profileOrApiTimeZone: string | null | undefined,
): string {
  const normalized = normalizeApiTimeZoneToIntlTimeZone(profileOrApiTimeZone);
  if (normalized) return normalized;
  try {
    const fromBrowser = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (fromBrowser && typeof fromBrowser === "string") return fromBrowser;
  } catch {
    /* ignore */
  }
  return "UTC";
}

const DATETIME_LOCAL_VALUE_RE =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/;

/**
 * Normalizes a datetime-local string to `YYYY-MM-DDTHH:mm` for ordering (lexicographic compare).
 *
 * @param localValue - Value from input type="datetime-local".
 * @returns Normalized string or null if unparseable.
 */
export function normalizeDatetimeLocalForCompare(
  localValue: string | null | undefined,
): string | null {
  const m = DATETIME_LOCAL_VALUE_RE.exec(localValue?.trim() ?? "");
  if (!m) return null;
  return `${m[1]}-${m[2]}-${m[3]}T${m[4]}:${m[5]}`;
}

/**
 * Converts datetime-local wall-clock value to a real UTC ISO string using the given profile/browser timezone.
 *
 * @param localValue - Value from input type="datetime-local" (profile-zone civil time).
 * @returns ISO string e.g. `2026-04-01T16:55:00.000Z`, or "" if invalid.
 */
export function callRequestPreferredTimeFromDatetimeLocal(
  localValue: string,
  profileTimeZone?: string | null,
): string {
  const trimmed = localValue.trim();
  const m = DATETIME_LOCAL_VALUE_RE.exec(trimmed);
  if (!m) return "";
  const utcMs = datetimeLocalWallTimeToUtcMs(trimmed, profileTimeZone);
  if (utcMs == null) return "";
  return new Date(utcMs).toISOString();
}

/**
 * Maps API preferred/schedule UTC strings to `datetime-local` in a given profile/browser timezone.
 *
 * @param apiStr - Raw string from preferredTimes or scheduleTime.
 * @returns `YYYY-MM-DDTHH:mm` or empty if unparseable.
 */
export function callRequestApiPreferredTimeToDatetimeLocal(
  apiStr: string | null | undefined,
  profileTimeZone?: string | null,
): string {
  return toDatetimeLocalInTimeZoneFromApiString(apiStr, profileTimeZone);
}

/**
 * Sort preferred time strings (ISO or API) ascending for stable reschedule/approve display.
 *
 * @param times - Raw API strings.
 * @returns Sorted copy.
 */
export function sortCallRequestPreferredTimeStringsAsc(
  times: string[],
): string[] {
  return [...times].sort((a, b) => {
    const ta = Date.parse(a.trim());
    const tb = Date.parse(b.trim());
    const aOk = !Number.isNaN(ta);
    const bOk = !Number.isNaN(tb);
    if (aOk && bOk && ta !== tb) return ta - tb;
    if (aOk && !bOk) return -1;
    if (!aOk && bOk) return 1;
    return a.trim().localeCompare(b.trim());
  });
}

/**
 * Formats an instant as YYYY-MM-DDTHH:mm in a given IANA time zone (civil time).
 *
 * @param instantMs - UTC epoch milliseconds.
 * @param timeZone - IANA zone (e.g. Asia/Colombo).
 * @returns {string} datetime-local compatible string.
 */
export function instantToDatetimeLocalStringInZone(
  instantMs: number,
  timeZone: string,
): string {
  const fmt = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  });
  const parts = fmt.formatToParts(new Date(instantMs));
  const g = (t: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === t)?.value ?? "";
  return `${g("year")}-${g("month")}-${g("day")}T${g("hour")}:${g("minute")}`;
}

function readZonedCalendarParts(ms: number, timeZone: string) {
  const fmt = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  });
  const parts = fmt.formatToParts(new Date(ms));
  const g = (t: Intl.DateTimeFormatPartTypes) =>
    Number(parts.find((p) => p.type === t)?.value ?? NaN);
  return {
    y: g("year"),
    mo: g("month"),
    d: g("day"),
    ho: g("hour"),
    mi: g("minute"),
  };
}

/**
 * Resolves UTC epoch ms for a civil wall-clock time in an IANA time zone.
 *
 * @param year - Calendar year.
 * @param month - 1–12.
 * @param day - Day of month.
 * @param hour - 0–23.
 * @param minute - 0–59.
 * @param timeZone - IANA zone.
 * @returns UTC ms, or null if no match (invalid local time).
 */
export function wallClockToUtcMilliseconds(
  year: number,
  month: number,
  day: number,
  hour: number,
  minute: number,
  timeZone: string,
): number | null {
  if (
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > 31 ||
    hour < 0 ||
    hour > 23 ||
    minute < 0 ||
    minute > 59
  ) {
    return null;
  }

  const anchor = Date.UTC(year, month - 1, day, 12, 0, 0);
  const windowMs = 72 * 3600 * 1000;
  for (let ms = anchor - windowMs; ms <= anchor + windowMs; ms += 60 * 1000) {
    const p = readZonedCalendarParts(ms, timeZone);
    if (
      p.y === year &&
      p.mo === month &&
      p.d === day &&
      p.ho === hour &&
      p.mi === minute
    ) {
      return ms;
    }
  }
  return null;
}

/**
 * Parses a datetime-local value (YYYY-MM-DDTHH:mm) to UTC ms using the user's profile zone
 * (filter API ids like WSO2/Colombo are normalized). If profile zone is missing or invalid,
 * uses the browser's time zone. For POST/PATCH `utcTimes`, use
 * {@link callRequestPreferredTimeFromDatetimeLocal} instead (backend expects modal wall clock + Z).
 *
 * @param localValue - Value from input type="datetime-local".
 * @param profileTimeZone - Profile/filter time zone id (optional).
 * @returns UTC ms or null if unparseable.
 */
export function datetimeLocalWallTimeToUtcMs(
  localValue: string | null | undefined,
  profileTimeZone?: string | null,
): number | null {
  const trimmed = localValue?.trim() ?? "";
  if (!trimmed) return null;
  const m = DATETIME_LOCAL_VALUE_RE.exec(trimmed);
  if (!m) return null;
  const y = Number(m[1]);
  const mo = Number(m[2]);
  const d = Number(m[3]);
  const h = Number(m[4]);
  const mi = Number(m[5]);
  const tz = resolveCallSchedulingTimeZone(profileTimeZone);
  return wallClockToUtcMilliseconds(y, mo, d, h, mi, tz);
}

/**
 * Same as {@link toDatetimeLocalInputFromApiString} but expresses the instant in a given IANA zone.
 *
 * @param apiStr - API date/time string.
 * @param profileTimeZone - Profile/filter zone; if missing/invalid, uses browser zone.
 * @returns {string} YYYY-MM-DDTHH:mm for datetime-local.
 */
export function toDatetimeLocalInTimeZoneFromApiString(
  apiStr: string | null | undefined,
  profileTimeZone?: string | null,
): string {
  if (!apiStr?.trim()) return "";
  const normalizedApi = normalizeUtcDateString(apiStr.trim());
  const ms = Date.parse(normalizedApi);
  if (Number.isNaN(ms)) return toDatetimeLocalInputFromApiString(apiStr);
  const tz = resolveCallSchedulingTimeZone(profileTimeZone);
  return instantToDatetimeLocalStringInZone(ms, tz);
}

function roundUpToNextFiveMinuteMarkInTimeZone(
  instantMs: number,
  timeZone: string,
): number {
  const fmt = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  });
  let t = instantMs;
  for (let i = 0; i < 200; i++) {
    const parts = fmt.formatToParts(new Date(t));
    const s = Number(parts.find((p) => p.type === "second")?.value ?? 0);
    const mi = Number(parts.find((p) => p.type === "minute")?.value ?? 0);
    if (s === 0 && mi % 5 === 0) return t;
    if (s !== 0) {
      t += (60 - s) * 1000;
    } else {
      t += ((5 - (mi % 5)) % 5 || 5) * 60 * 1000;
    }
  }
  return t;
}

/**
 * Like {@link computeMinScheduleDatetimeLocal} but uses the user's IANA zone for civil date/time strings.
 * Ensures datetime-local values align with profile timezone when converting to UTC for the API.
 *
 * @param allocationMinutes - Minutes after now before first slot; null/undefined uses now+1m floor only.
 * @param profileTimeZone - Profile/filter zone; falls back to browser when omitted/invalid.
 */
export function computeMinScheduleDatetimeLocalForTimeZone(
  allocationMinutes?: number | null,
  profileTimeZone?: string | null,
): string {
  const tz = resolveCallSchedulingTimeZone(profileTimeZone);

  const floorMs = Date.now() + 60 * 1000;
  const floorStr = instantToDatetimeLocalStringInZone(floorMs, tz);

  if (
    allocationMinutes == null ||
    Number.isNaN(allocationMinutes) ||
    allocationMinutes < 0
  ) {
    return floorStr;
  }

  let targetMs = Date.now() + allocationMinutes * 60 * 1000;
  targetMs = roundUpToNextFiveMinuteMarkInTimeZone(targetMs, tz);
  const severityStr = instantToDatetimeLocalStringInZone(targetMs, tz);
  return severityStr > floorStr ? severityStr : floorStr;
}

export { hasListSearchOrFilters, countListSearchAndFilters } from "./listView";

/** Hide terminal states from the Outstanding Cases overview list only. */
export function isClosedLikeCaseStatus(statusLabel?: string | null): boolean {
  const normalized = statusLabel?.trim().toLowerCase() ?? "";
  return normalized === CASE_STATUS.CLOSED;
}
