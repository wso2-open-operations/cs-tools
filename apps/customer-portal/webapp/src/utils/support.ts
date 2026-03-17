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
} from "@constants/supportConstants";
import { CaseReportType } from "@constants/securityConstants";
import {
  SEVERITY_LABEL_TO_DISPLAY,
  isS0SeverityLabel,
} from "@constants/dashboardConstants";
import type { CaseComment, MetadataItem } from "@models/responses";
import { alpha, colors, type Theme } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";
import { createElement, type ComponentType, type ReactNode } from "react";

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
 * Strips "[Customer]" or "[CUSTOMER]" prefix from call request reason for display in edit form.
 *
 * @param {string} reason - Raw reason from API.
 * @returns {string} Reason without the prefix.
 */
export function stripCustomerPrefixFromReason(reason: string): string {
  return reason.replace(/^\[Customer\]\s*/i, "").trim();
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
 * Formats a UTC date string for display in the user's local timezone.
 *
 * @param {string} dateStr - UTC date string (YYYY-MM-DD HH:mm:ss or MM/DD/YYYY HH:mm:ss).
 * @param {"short" | "long"} [formatStr="long"] - The format style.
 * @param {boolean} [includeTimeZoneName=true] - Include timezone suffix (e.g. +GMT).
 * @returns {string} Formatted date/time in local time.
 */
export function formatUtcToLocal(
  dateStr: string | null | undefined,
  formatStr: "short" | "long" = "long",
  includeTimeZoneName = true,
): string {
  if (!dateStr) return "--";
  const normalized = normalizeUtcDateString(dateStr);
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return "--";
  const tzOption = includeTimeZoneName
    ? { timeZoneName: "short" as const }
    : {};
  if (formatStr === "short") {
    return new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "numeric",
      hour12: true,
      ...tzOption,
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

/**
 * Converts duration from minutes to formatted string (e.g., "4h 30m").
 * Used for change request duration display.
 *
 * @param {number | string | null | undefined} minutes - Duration in minutes (API may return string).
 * @returns {string} Formatted duration string (e.g., "4h 30m") or "Not Available".
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

export type ChatActionState =
  | "primary"
  | "info"
  | "success"
  | "warning"
  | "error";

/** Assigned engineer from API: string or { id, label?, name? } object. */
export type AssignedEngineerValue =
  | string
  | { id: string; label?: string; name?: string }
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
    | { id: string; label?: string; name?: string }
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
 * Returns true if the case has S0 (Catastrophic) severity.
 * Used to filter out S0 cases when project type is not Managed Cloud Subscription.
 *
 * @param caseItem - Case with optional severity.
 * @returns {boolean}
 */
export function isS0Case(
  caseItem: { severity?: { label?: string } | null },
): boolean {
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
  return content.replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "<code>$1</code>");
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
  return content.replace(/\[code\]([\s\S]*?)\[\/code\]/gi, "$1");
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

/** Inline attachment item for image src replacement (supports API id/downloadUrl or legacy sys_id/url). */
export interface InlineAttachment {
  id?: string;
  downloadUrl?: string;
  sys_id?: string;
  url?: string;
}

/**
 * Replaces inline image sources in HTML (e.g. /sys_id.iix or /id.iix) with URLs from attachments.
 * Matches by id or sys_id; uses downloadUrl or url for the replacement.
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
    return DOMPurify.sanitize(normalizedHtml);
  }

  const replaced = normalizedHtml.replace(
    /<img([^>]*?)\s+src\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))([^>]*)>/gi,
    (_match, before, doubleSrc, singleSrc, bareSrc, after) => {
      const src = (doubleSrc ?? singleSrc ?? bareSrc ?? "") as string;
      const refId = src
        .replace(/^\//, "")
        .replace(/\.iix$/i, "")
        .trim();
      const attachment = inlineAttachments.find(
        (a) =>
          a.id === refId ||
          a.sys_id === refId ||
          (a?.id && src.includes(a.id)) ||
          (a?.sys_id && src.includes(a.sys_id)),
      );
      const newSrc = attachment?.downloadUrl ?? attachment?.url ?? src;
      const quote =
        doubleSrc !== undefined ? '"' : singleSrc !== undefined ? "'" : '"';
      return `<img${before} src=${quote}${newSrc}${quote}${after}>`;
    },
  );
  return DOMPurify.sanitize(replaced);
}

/**
 * Normalizes ServiceNow-style timestamp "YYYY-MM-DD HH:MM:SS" to ISO UTC for parsing.
 *
 * @param dateStr - Raw date string (ISO, ServiceNow, or parseable).
 * @returns {string} Normalized string for Date constructor.
 */
function normalizeCommentDateString(dateStr: string): string {
  const trimmed = dateStr.trim();
  if (/^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}$/.test(trimmed)) {
    return trimmed.replace(" ", "T") + "Z";
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
 * Strips custom tags like [code]...[/code], [p]...[/p], [b]...[/b], etc. from content.
 * Used for change request comments that may contain these custom markup tags.
 *
 * @param content - Content string with custom tags.
 * @returns {string} Plain text without custom tags.
 */
export function stripCustomTags(content: string | null | undefined): string {
  if (!content || typeof content !== "string") return "";
  // Remove custom tags like [code], [/code], [p], [/p], [b], [/b], [u], [/u], [br], etc.
  return content.replace(/\[\/?\w+\]/g, "").trim();
}

/**
 * Strips both HTML tags and custom tags from content.
 * Specifically for change request comments that may contain both <tag> and [tag] formats.
 *
 * @param content - Content string with mixed HTML and custom tags.
 * @returns {string} Plain text without any tags.
 */
export function stripAllTags(content: string | null | undefined): string {
  if (!content || typeof content !== "string") return "";
  // First remove custom tags like [code], [/code]
  let cleaned = content.replace(/\[\/?\w+\]/g, "");
  // Then remove HTML tags like <br>, <p>
  cleaned = cleaned.replace(/<[^>]+>/g, "");
  return cleaned.trim();
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
