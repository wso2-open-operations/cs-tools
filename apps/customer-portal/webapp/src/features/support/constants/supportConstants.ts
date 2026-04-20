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
  CalendarDays,
  CircleAlert,
  CircleCheck,
  CirclePause,
  CircleQuestionMark,
  CircleX,
  Clock,
  FileText,
  Info,
  MessageCircle,
  MessageSquare,
  Paperclip,
  Phone,
  Server,
  TriangleAlert,
  TrendingUp,
  RotateCcw,
  XCircle,
  FileCheck,
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
  S4: "S4",
} as const;

export type CaseSeverityLevel =
  (typeof CaseSeverityLevel)[keyof typeof CaseSeverityLevel];

// Case type values for case creation and stats filters.
export const CaseType = {
  DEFAULT_CASE: "default_case",
  SERVICE_REQUEST: "service_request",
  ENGAGEMENT: "engagement",
  SECURITY_REPORT_ANALYSIS: "security_report_analysis",
  ANNOUNCEMENT: "announcement",
} as const;

export type CaseType = (typeof CaseType)[keyof typeof CaseType];

// Maximum allowed attachment file size in bytes.
export const MAX_ATTACHMENT_SIZE_BYTES = 15 * 1024 * 1024;

// Maximum allowed embedded image size in bytes (15MB for base64 images in rich text).
export const MAX_IMAGE_SIZE_BYTES = 15 * 1024 * 1024;

// Initial limit for case attachments list.
export const CASE_ATTACHMENTS_INITIAL_LIMIT = 50;

/** Tooltip when delete is disabled because the case is closed. */
export const ATTACHMENT_DELETE_TOOLTIP_CASE_CLOSED =
  "Attachments cannot be deleted on a closed case.";

/** Tooltip when delete is disabled because the current user did not upload the file. */
export const ATTACHMENT_DELETE_TOOLTIP_NOT_OWNER =
  "Only the person who uploaded this attachment can delete it.";

/** Tooltip for deployment documents when delete is disabled for non-owners. */
export const DEPLOYMENT_DOCUMENT_DELETE_TOOLTIP_NOT_OWNER =
  "Only the person who uploaded this document can delete it.";

/** Tooltip for deployment documents when edit is disabled for non-owners. */
export const DEPLOYMENT_DOCUMENT_EDIT_TOOLTIP_NOT_OWNER =
  "Only the person who uploaded this document can edit it.";

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
    label: "Active Cases",
    secondaryIcon: TrendingUp,
  },
  {
    icon: FileCheck,
    iconColor: "success",
    key: "resolvedPast30DaysCasesCount",
    label: "Resolved Recently (Last 30d)",
  },
  {
    icon: CircleCheck,
    iconColor: "info",
    key: "resolvedChats",
    label: "Resolved via Chat",
  },
  {
    icon: Clock,
    iconColor: "warning",
    key: "activeChats",
    label: "Active Chats",
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
];

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
    filterKey: "statusId",
    id: "status",
    metadataKey: "caseStates",
  },
  {
    filterKey: "severityId",
    id: "severity",
    metadataKey: "severities",
  },
  {
    filterKey: "issueTypes",
    id: "category",
    metadataKey: "issueTypes",
  },
  {
    filterKey: "deploymentId",
    id: "deployment",
    metadataKey: "deploymentTypes",
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
 * Announcement filter definitions (status only).
 */
export const ANNOUNCEMENT_FILTER_DEFINITIONS: AnnouncementFilterDefinition[] = [
  {
    filterKey: "statusId",
    id: "status",
    metadataKey: "caseStates",
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
  | "activeServiceRequests"
  | "activeChangeRequests"
  | "completedThisMonth"
  | "upcomingChanges";

/**
 * Configuration for the operations statistics cards.
 */
export const OPERATIONS_STAT_CONFIGS: SupportStatConfig<OperationsStatKey>[] = [
  {
    icon: Server,
    iconColor: "info",
    key: "activeServiceRequests",
    label: "Active Service Requests",
  },
  {
    icon: CalendarDays,
    iconColor: "primary",
    key: "activeChangeRequests",
    label: "Active Change Requests",
  },
  {
    icon: CircleCheck,
    iconColor: "success",
    key: "completedThisMonth",
    label: "Completed This Month",
  },
  {
    icon: Calendar,
    iconColor: "warning",
    key: "upcomingChanges",
    label: "Upcoming Changes",
  },
];
