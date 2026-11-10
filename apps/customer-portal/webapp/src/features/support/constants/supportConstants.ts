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
  Bell,
  BookOpen,
  Bot,
  Calendar,
  CircleAlert,
  CircleCheck,
  CirclePause,
  CircleQuestionMark,
  CircleX,
  Clock,
  FileText,
  GitBranch,
  Info,
  MessageCircle,
  MessageSquare,
  Paperclip,
  Phone,
  TriangleAlert,
  RotateCcw,
  XCircle,
  FileCheck,
  AlertCircle,
  ShieldCheck,
} from "@wso2/oxygen-ui-icons-react";
import type { ProjectCasesStats } from "@features/support/types/cases";
import type {
  AllCasesFilterDefinition,
  AllConversationsFilterDefinition,
  AnnouncementFilterDefinition,
  CaseDetailsTabConfig,
  CaseStatusAction,
  CaseTypeObject,
  SupportStatConfig,
} from "@features/support/types/supportUiConfig";

export type {
  SupportStatConfig,
  CaseDetailsTabConfig,
  CaseStatusPaletteIntent,
  CaseStatusAction,
  AllCasesFilterDefinition,
  AllConversationsFilterDefinition,
  AnnouncementFilterValues,
  AnnouncementFilterDefinition,
  CaseTypeObject,
} from "@features/support/types/supportUiConfig";

export const KB_ARTICLE_VIEW_BASE_URL =
  "https://support.wso2.com/kb?id=kb_article_view&sys_kb_id=";

// Chat actions for the history list.
export const ChatAction = {
  VIEW: "view",
  RESUME: "resume",
  ACTIVE: "active",
  OPEN: "open",
  ABANDONED: "abandoned",
} as const;

export type ChatAction = (typeof ChatAction)[keyof typeof ChatAction];

// Chat status types.
export const ChatStatus = {
  RESOLVED: "Resolved",
  OPEN: "Open",
  ABANDONED: "Abandoned",
} as const;

export type ChatStatus = (typeof ChatStatus)[keyof typeof ChatStatus];

export const ConversationStatus = {
  ABANDONED: "Abandoned",
  ACTIVE: "Active",
  CLOSE: "Close",
  CONVERTED: "Converted",
  OPEN: "Open",
  RESOLVED: "Resolved",
} as const;

export type ConversationStatus =
  (typeof ConversationStatus)[keyof typeof ConversationStatus];

// Case status types matching API labels.
export const CaseStatus = {
  AWAITING_INFO: "Awaiting Info",
  CLOSED: "Closed",
  OPEN: "Open",
  REOPENED: "Reopened",
  SOLUTION_PROPOSED: "Solution Proposed",
  WAITING_ON_WSO2: "Waiting On WSO2",
  WORK_IN_PROGRESS: "Work In Progress",
} as const;

export type CaseStatus = (typeof CaseStatus)[keyof typeof CaseStatus];

export const SUPPORT_STATE_CLOSED = CaseStatus.CLOSED;
export const SUPPORT_STATE_AWAITING_INFO = CaseStatus.AWAITING_INFO;
export const SUPPORT_STATE_WAITING_ON_WSO2 = CaseStatus.WAITING_ON_WSO2;
export const CALL_SCHEDULABLE_CASE_STATUSES: CaseStatus[] = [
  CaseStatus.WORK_IN_PROGRESS,
  CaseStatus.AWAITING_INFO,
  CaseStatus.WAITING_ON_WSO2,
  CaseStatus.SOLUTION_PROPOSED,
  CaseStatus.REOPENED,
];

// Call request status types.
export const CallRequestStatus = {
  CANCELED: "Canceled",
  COMPLETED: "Completed",
  PENDING: "Pending",
  PENDING_ON_CUSTOMER: "Pending on Customer",
  REJECTED: "Rejected",
  SCHEDULED: "Scheduled",
  NOTES_PENDING: "Notes Pending",
} as const;

/** API call request state id for Notes Pending (hide customer reschedule/cancel). */
export const CALL_REQUEST_STATE_NOTES_PENDING_ID = "7";

export type CallRequestStatus =
  (typeof CallRequestStatus)[keyof typeof CallRequestStatus];

/** API stateKey: customer has rejected the proposed time (PATCH). */
export const CALL_REQUEST_STATE_CUSTOMER_REJECTED = 4;

/** API stateKey: rescheduled back to WSO2 with new preferred times (PATCH). */
export const CALL_REQUEST_STATE_PENDING_ON_WSO2 = 2;

/** API stateKey value for cancelling a call request (PATCH). */
export const CALL_REQUEST_STATE_CANCELLED = 6;

/** API stateKey of a call request that has taken place (state "concluded"). */
export const CALL_REQUEST_STATE_CONCLUDED = 8;

/**
 * Every call-request stateKey (PATCH / search filter), by entity-service's own
 * string enum id (GET /projects/{id}/filters' callRequestStates[].id) -- the
 * fixed ServiceNow-era numeric scheme, mirroring backend-v2's own
 * callRequestStateKeyToEnum (internal/dto/call_request_enum_mapping.go) in
 * reverse. The string id is never itself numeric (Number(id) is always NaN),
 * so any code deriving a stateKey from callRequestStates must go through this
 * map (or the single-purpose constants above), never Number(id) directly.
 */
export const CALL_REQUEST_STATE_KEY_BY_ID: Readonly<Record<string, number>> = {
  pending_on_customer: 1,
  pending_on_wso2: CALL_REQUEST_STATE_PENDING_ON_WSO2,
  scheduled: 3,
  customer_rejected: CALL_REQUEST_STATE_CUSTOMER_REJECTED,
  wso2_rejected: 5,
  canceled: CALL_REQUEST_STATE_CANCELLED,
  notes_pending: 7,
  concluded: CALL_REQUEST_STATE_CONCLUDED,
};

/**
 * The Calls tab's status dropdown. Pending is every state that is neither
 * Canceled nor Concluded (the call has not finished), so a state added to
 * CALL_REQUEST_STATE_KEY_BY_ID later is picked up under Pending.
 */
export const CALL_REQUEST_STATUS_FILTER_OPTIONS = [
  { value: "pending", label: "Pending" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
] as const;

/** The search stateKeys behind each CALL_REQUEST_STATUS_FILTER_OPTIONS value. */
export const CALL_REQUEST_STATE_KEYS_BY_STATUS_FILTER: Readonly<
  Record<string, number[]>
> = {
  pending: Object.values(CALL_REQUEST_STATE_KEY_BY_ID).filter(
    (key) =>
      key !== CALL_REQUEST_STATE_CANCELLED &&
      key !== CALL_REQUEST_STATE_CONCLUDED,
  ),
  completed: [CALL_REQUEST_STATE_CONCLUDED],
  cancelled: [CALL_REQUEST_STATE_CANCELLED],
};

// Case severity types matching API labels.
export const CaseSeverity = {
  CATASTROPHIC: "Catastrophic (P0)",
  CRITICAL: "Critical (P1)",
  HIGH: "High (P2)",
  LOW: "Low (P4)",
  MEDIUM: "Medium (P3)",
} as const;

export type CaseSeverity = (typeof CaseSeverity)[keyof typeof CaseSeverity];

// Case severity levels (S0-S4).
export const CaseSeverityLevel = {
  S0: "S0",
  S1: "S1",
  S2: "S2",
  S3: "S3",
  S4: "S4(Query)",
} as const;

export type CaseSeverityLevel =
  (typeof CaseSeverityLevel)[keyof typeof CaseSeverityLevel];

/**
 * ServiceNow's own CallRequestUtils._PRIORITY_TIME_OFFSETS, mirrored here:
 * the minimum lead time (minutes) a call request's preferred time must sit
 * ahead of now, by case priority. Keyed by the same numeric priority id
 * GET /projects/{id}/filters' severity choice list uses (confirmed against
 * entity-service's own severityEnumToChoice: S0=14, S1=10, S2=11, S3=12,
 * S4=13 -- the numbering is P0(14) -> P1(10) -> P2(11) -> P3(12) -> P4(13),
 * not a simple sequence), not a severity label string.
 *
 * entity-service's own severityBasedAllocationTime (the field this was
 * meant to come from) is currently always an empty map -- no per-severity
 * SLA-allocation-time table exists in Postgres yet -- so this is the one
 * place this matrix is defined today. If severityBasedAllocationTime is
 * ever populated, prefer it over this table (see CallsPanel.tsx's own
 * severityAllocationMinutes, which checks the API value first).
 */
export const CALL_REQUEST_LEAD_MINUTES_BY_PRIORITY_ID: Readonly<
  Record<string, number>
> = {
  "14": 15, // Catastrophic (P0)
  "10": 30, // Critical (P1)
  "11": 60, // High (P2)
  "12": 90, // Medium (P3)
  "13": 120, // Low (P4)
};

// Case type values for case creation and stats filters.
export const CaseType = {
  DEFAULT_CASE: "default_case",
  SERVICE_REQUEST: "service_request",
  ENGAGEMENT: "engagement",
  SECURITY_REPORT_ANALYSIS: "security_report_analysis",
  ANNOUNCEMENT: "announcement",
} as const;

export type CaseType = (typeof CaseType)[keyof typeof CaseType];

// Whole-request cap for POST /cases bodies. The margin below the BE's own cap
// keeps the FE check from ever passing a body the BE would reject. Case files
// are not part of this body: they are uploaded after the case is created.
export const MAX_CASE_REQUEST_BODY_BYTES = 15 * 1024 * 1024 - 16 * 1024;

// Deliberately stricter than the BE's own request-body cap for
// POST /cases/{id}/comments (backend-v2's generic `readJSONBody` helper,
// capped at the blanket `maxRequestBodyBytes = 1 << 20`, i.e. 1 MiB — this was
// NOT raised for comments). Images pasted into a comment are uploaded and
// referenced by id, so a comment body stays small; this FE-only ceiling blocks a
// body the BE would reject with a 413, and matches the same 1 MiB guard applied
// to the CSM portal webapp for consistency across both portals.
export const MAX_COMMENT_BODY_BYTES = 1 * 1024 * 1024;
// Reserve headroom for the JSON envelope ({ type, content }) + string escaping
// so the FE blocks before the BE rejects with 413.
export const MAX_COMMENT_CONTENT_BYTES = MAX_COMMENT_BODY_BYTES - 1024;

// Allowed MIME types for inline image uploads in the rich text editor.
export const ALLOWED_INLINE_IMAGE_TYPES = [
  "image/png",
  "image/jpeg",
  "image/webp",
] as const;

// Allowed file extensions for inline image uploads (used in accept attribute and validation).
export const ALLOWED_INLINE_IMAGE_EXTENSIONS = [
  "png",
  "jpeg",
  "jpg",
  "webp",
] as const;

// Initial limit for case attachments list.
export const CASE_ATTACHMENTS_INITIAL_LIMIT = 50;

// Max rows per search page when exporting.
export const CASE_SEARCH_RESULTS_PAGE_SIZE = 50;

// Tooltip when delete is disabled because the case is closed.
export const ATTACHMENT_DELETE_TOOLTIP_CASE_CLOSED =
  "Attachments cannot be deleted on a closed case.";

// Tooltip when delete is disabled because the current user did not upload the file.
export const ATTACHMENT_DELETE_TOOLTIP_NOT_OWNER =
  "Only the person who uploaded this attachment can delete it.";

// Tooltip for deployment documents when delete is disabled for non-owners.
export const DEPLOYMENT_DOCUMENT_DELETE_TOOLTIP_NOT_OWNER =
  "Only the person who uploaded this document can delete it.";

/**
 * Valid keys for all cases statistics.
 */
export type AllCasesStatKey =
  | "openCases"
  | "workInProgress"
  | "waitingOnClient"
  | "waitingOnWso2";

/**
 * Configuration for the all cases statistics cards.
 */
export const ALL_CASES_STAT_CONFIGS: SupportStatConfig<AllCasesStatKey>[] = [
  {
    icon: CircleAlert,
    iconColor: "error",
    key: "openCases",
    label: "Total Active",
  },
  {
    icon: Clock,
    iconColor: "success",
    key: "workInProgress",
    label: "Work in Progress",
  },
  {
    icon: MessageCircle,
    iconColor: "warning",
    key: "waitingOnClient",
    label: "Awaiting Info",
  },
  {
    icon: CircleQuestionMark,
    iconColor: "info",
    key: "waitingOnWso2",
    label: "Waiting on WSO2",
  },
];

/**
 * Flattens the project cases statistics for the stat cards.
 * Total Active = totalCases; Work in Progress, Awaiting Info, Waiting on WSO2 from stateCount.
 *
 * @param {ProjectCasesStats | undefined} stats - The original stats.
 * @returns {Record<AllCasesStatKey, number | undefined>} The flattened stats.
 */
export const getAllCasesFlattenedStats = (
  stats: ProjectCasesStats | undefined,
): Record<AllCasesStatKey, number | undefined> => {
  const stateCount = stats?.stateCount ?? [];
  const workInProgress = stateCount.find(
    (s) => s.label === CaseStatus.WORK_IN_PROGRESS,
  )?.count;
  const waitingOnClient = stateCount.find(
    (s) => s.label === CaseStatus.AWAITING_INFO,
  )?.count;
  const waitingOnWso2 = stateCount.find(
    (s) => s.label === CaseStatus.WAITING_ON_WSO2,
  )?.count;
  const openCases = stats?.totalCases;
  return {
    openCases: openCases != null ? openCases : undefined,
    waitingOnClient,
    waitingOnWso2,
    workInProgress,
  };
};

// Configuration for the support statistics cards.
export const SUPPORT_STAT_CONFIGS: SupportStatConfig[] = [
  {
    icon: FileText,
    iconColor: "error",
    key: "ongoingCases",
    label: "Outstanding Cases",
  },
  {
    icon: Clock,
    iconColor: "warning",
    key: "activeChats",
    label: "Active Chats",
  },
  {
    icon: FileCheck,
    iconColor: "success",
    key: "resolvedPast30DaysCasesCount",
    label: "Resolved Cases (Last 30d)",
  },
  {
    icon: CircleCheck,
    iconColor: "info",
    key: "resolvedChats",
    label: "Resolved via Chat (Last 30d)",
  },
];

/**
 * Case details tab configuration (label + icon for Activity, Details, Attachments, etc.).
 */
export const CASE_DETAILS_TABS: CaseDetailsTabConfig[] = [
  { label: "Activity", Icon: MessageSquare },
  { label: "Details", Icon: Info },
  { label: "Attachments (0)", Icon: Paperclip },
  { label: "Calls (0)", Icon: Phone },
  { label: "Knowledge Base (0)", Icon: BookOpen },
  { label: "Related Change Requests", Icon: GitBranch },
  { label: "Escalation", Icon: TriangleAlert },
  { label: "Verifications", Icon: ShieldCheck },
];

export const ESCALATION_MAX_LEVEL_ID = "5";

// Escalation levels that require the user to have isLead: true (EL3→EL4 and EL4→EL5).
export const ESCALATION_LEAD_REQUIRED_FROM_LEVEL = new Set(["3", "4"]);

// Only at these escalation levels is the user who created the escalation
// (in addition to customer admins and project leads) eligible to de-escalate.
// Any other/unknown level (including EL4/EL5) fails closed to admins/leads only.
export const ESCALATION_DEESCALATE_CREATOR_ELIGIBLE_LEVELS = new Set([
  "1",
  "2",
  "3",
]);

export const ESCALATION_NEXT_LEVEL: Record<
  string,
  { nextId: string; nextLabel: string; notifiedLabel: string }
> = {
  "0": { nextId: "1", nextLabel: "EL1", notifiedLabel: "Team Lead" },
  "1": { nextId: "2", nextLabel: "EL2", notifiedLabel: "Technology Unit Head" },
  "2": { nextId: "3", nextLabel: "EL3", notifiedLabel: "CRE Head" },
  "3": { nextId: "4", nextLabel: "EL4", notifiedLabel: "CCO / CRO" },
  "4": { nextId: "5", nextLabel: "EL5", notifiedLabel: "CEO" },
};

// Case status actions shown in the case details action row. Close button last.
export const CASE_STATUS_ACTIONS: CaseStatusAction[] = [
  { label: "Waiting on WSO2", Icon: CirclePause, paletteIntent: "warning" },
  { label: "Accept Solution", Icon: CircleCheck, paletteIntent: "success" },
  { label: "Reject Solution", Icon: TriangleAlert, paletteIntent: "error" },
  { label: "Open Related Case", Icon: RotateCcw, paletteIntent: "info" },
  { label: "Closed", Icon: CircleX, paletteIntent: "info" },
];

// Number of outstanding cases to show on support overview cards.
export const SUPPORT_OVERVIEW_CASES_LIMIT = 5;

// Number of chat history items to show on support overview cards.
export const SUPPORT_OVERVIEW_CHAT_LIMIT = 7;

// Number of service requests / change requests to show on operations overview cards.
export const OPERATIONS_OVERVIEW_LIST_LIMIT = 5;

// Rich text editor constants
export const RICH_TEXT_HISTORY_LIMIT = 50;
export const RICH_TEXT_UNDO_DEBOUNCE_MS = 600;

export type RichTextBlockVariant =
  | "h1"
  | "h2"
  | "h3"
  | "h4"
  | "h5"
  | "h6"
  | "subtitle1"
  | "subtitle2"
  | "body1"
  | "body2"
  | "caption";

export const RICH_TEXT_BLOCK_TAGS: Array<{
  value: string;
  label: string;
  variant: RichTextBlockVariant;
}> = [
  { value: "h1", label: "Heading 1", variant: "h1" },
  { value: "h2", label: "Heading 2", variant: "h2" },
  { value: "h3", label: "Heading 3", variant: "h3" },
  { value: "h4", label: "Heading 4", variant: "h4" },
  { value: "h5", label: "Heading 5", variant: "h5" },
  { value: "h6", label: "Heading 6", variant: "h6" },
  { value: "subtitle1", label: "Subtitle 1", variant: "subtitle1" },
  { value: "subtitle2", label: "Subtitle 2", variant: "subtitle2" },
  { value: "body1", label: "Body 1", variant: "body1" },
  { value: "body2", label: "Body 2", variant: "body2" },
  { value: "caption", label: "Caption", variant: "caption" },
];

export const SERVICE_REQUEST_BULLET_ITEMS = [
  "Service restarts and upgrades",
  "Certificate management",
  "Infrastructure scaling",
  "Configuration changes",
  "Log and information requests",
  "Security updates",
] as const;

/**
 * Valid keys for all conversations statistics (no values yet).
 */
export type AllConversationsStatKey =
  | "resolved"
  | "open"
  | "abandoned"
  | "totalChats";

/**
 * Configuration for the all conversations statistics cards (no values displayed yet).
 */
export const ALL_CONVERSATIONS_STAT_CONFIGS: SupportStatConfig<AllConversationsStatKey>[] =
  [
    {
      icon: CircleCheck,
      iconColor: "success",
      key: "resolved",
      label: "Resolved",
    },
    { icon: Clock, iconColor: "info", key: "open", label: "Open" },
    {
      icon: CircleAlert,
      iconColor: "warning",
      key: "abandoned",
      label: "Abandoned",
    },
    { icon: Bot, iconColor: "info", key: "totalChats", label: "Total Chats" },
  ];

/**
 * Configuration for the all conversations filters (state only).
 */
export const ALL_CONVERSATIONS_FILTER_DEFINITIONS: AllConversationsFilterDefinition[] =
  [
    {
      filterKey: "stateId",
      id: "state",
      metadataKey: "conversationStates",
    },
  ];

/**
 * Configuration for the all cases filters.
 * Uses caseStates, caseTypes, severities, issueTypes, deploymentTypes from useGetProjectFilters.
 */
export const ALL_CASES_FILTER_DEFINITIONS: AllCasesFilterDefinition[] = [
  {
    filterKey: "statusIds",
    id: "status",
    metadataKey: "caseStates",
    multiSelect: true,
  },
  {
    filterKey: "severityIds",
    id: "severity",
    metadataKey: "severities",
    multiSelect: true,
  },
  {
    filterKey: "issueTypes",
    id: "category",
    metadataKey: "issueTypes",
    multiSelect: true,
  },
  {
    filterKey: "deploymentIds",
    id: "deployment",
    metadataKey: "deploymentTypes",
    multiSelect: true,
  },
  {
    filterKey: "createdBy",
    id: "createdBy",
    multiSelect: true,
  },
];

/**
 * Announcement page stat keys (hardcoded values for now).
 */
export type AnnouncementStatKey =
  | "unread"
  | "critical"
  | "actionRequired"
  | "total";

/**
 * Hardcoded announcement stats (sample values).
 */
export const ANNOUNCEMENT_STATS_HARDCODED: Record<AnnouncementStatKey, number> =
  {
    unread: 3,
    critical: 1,
    actionRequired: 3,
    total: 8,
  };

/**
 * Configuration for announcement statistics cards.
 */
export const ANNOUNCEMENT_STAT_CONFIGS: SupportStatConfig<AnnouncementStatKey>[] =
  [
    { icon: Bell, iconColor: "warning", key: "unread", label: "Unread" },
    {
      icon: TriangleAlert,
      iconColor: "error",
      key: "critical",
      label: "Critical",
    },
    {
      icon: CircleAlert,
      iconColor: "warning",
      key: "actionRequired",
      label: "Action Required",
    },
    {
      icon: FileText,
      iconColor: "info",
      key: "total",
      label: "Total",
    },
  ];

/**
 * Valid keys for pending-verification list statistics. Limited to what the
 * search response actually aggregates over the full filtered set
 * (totalRecords/autoClosedCount/manualCount) — no full-set "verified" count
 * is exposed by the backend, so that stat is deliberately not included here
 * rather than showing a misleading current-page-only number.
 */
export type PendingVerificationStatKey = "total" | "autoClosed" | "manual" | "verified";

/**
 * Configuration for the pending-verification list's statistics cards.
 * "verified" reads the same deduped, per-case total the Verified tab badge
 * uses (see 15-dedupe-verified-tab.md) — already fetched regardless of
 * which tab is active, so this tile costs no extra request.
 */
export const PENDING_VERIFICATION_STAT_CONFIGS: SupportStatConfig<PendingVerificationStatKey>[] =
  [
    { icon: ShieldCheck, iconColor: "info", key: "total", label: "Total Pending Verification" },
    { icon: RotateCcw, iconColor: "warning", key: "autoClosed", label: "Auto-closed Pending Verification" },
    { icon: FileText, iconColor: "secondary", key: "manual", label: "Manual Pending Verification" },
    { icon: CircleCheck, iconColor: "success", key: "verified", label: "Verified" },
  ];

/**
 * Announcement filter definitions (status only).
 */
export const ANNOUNCEMENT_FILTER_DEFINITIONS: AnnouncementFilterDefinition[] = [
  {
    filterKey: "statusIds",
    id: "status",
    metadataKey: "caseStates",
    multiSelect: true,
  },
];

/**
 * Constants for comment types when posting a comment to a case.
 */
export const CommentType = {
  COMMENT: "comments",
} as const;
export type CommentType = (typeof CommentType)[keyof typeof CommentType];

// Line count threshold for showing expand button in support activity section.
export const COLLAPSE_LINE_THRESHOLD = 4;

// Case type input.
export type CaseTypeInput = CaseTypeObject | string | null | undefined;

/**
 * Valid keys for service requests statistics.
 */
export type ServiceRequestStatKey =
  | "pending"
  | "inProgress"
  | "completed"
  | "rejected";

/**
 * Configuration for the service requests statistics cards.
 */
export const SERVICE_REQUEST_STAT_CONFIGS: SupportStatConfig<ServiceRequestStatKey>[] =
  [
    {
      icon: Clock,
      iconColor: "warning",
      key: "pending",
      label: "Pending",
    },
    {
      icon: Info,
      iconColor: "info",
      key: "inProgress",
      label: "In Progress",
    },
    {
      icon: CircleCheck,
      iconColor: "success",
      key: "completed",
      label: "Completed",
    },
    {
      icon: XCircle,
      iconColor: "error",
      key: "rejected",
      label: "Rejected",
    },
  ];

/**
 * Valid keys for operations statistics.
 */
export type OperationsStatKey =
  | "actionRequiredServiceRequests"
  | "outstandingServiceRequests"
  | "actionRequiredChangeRequests"
  | "upcomingChanges";

/**
 * Configuration for the operations statistics cards.
 */
export const OPERATIONS_STAT_CONFIGS: SupportStatConfig<OperationsStatKey>[] = [
  {
    icon: AlertCircle,
    iconColor: "info",
    key: "actionRequiredServiceRequests",
    label: "Action Required Service Requests",
  },
  {
    icon: Clock,
    iconColor: "primary",
    key: "outstandingServiceRequests",
    label: "Outstanding Service Requests",
  },
  {
    icon: TriangleAlert,
    iconColor: "warning",
    key: "actionRequiredChangeRequests",
    label: "Action Required Changes",
  },
  {
    icon: Calendar,
    iconColor: "warning",
    key: "upcomingChanges",
    label: "Upcoming Changes",
  },
];
