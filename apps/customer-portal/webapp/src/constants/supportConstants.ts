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
  Info,
  MessageCircle,
  MessageSquare,
  Paperclip,
  Phone,
  TriangleAlert,
  TrendingUp,
  Zap,
  RotateCcw,
  CheckCircle,
  XCircle,
  PlayCircle,
  Eye,
  ShieldCheck,
  UserCheck,
  CalendarCheck,
} from "@wso2/oxygen-ui-icons-react";
import { colors, alpha } from "@wso2/oxygen-ui";
import { type ComponentType } from "react";
import type {
  ProjectSupportStats,
  ProjectCasesStats,
  CaseMetadataResponse,
  AllCasesFilterValues,
  AllConversationsFilterValues,
  ChangeRequestFilterValues,
} from "@models/responses";

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

// Call request status types.
export const CallRequestStatus = {
  CANCELED: "Canceled",
  COMPLETED: "Completed",
  PENDING: "Pending",
  REJECTED: "Rejected",
  SCHEDULED: "Scheduled",
} as const;

export type CallRequestStatus =
  (typeof CallRequestStatus)[keyof typeof CallRequestStatus];

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

// Case type values for case creation.
export const CaseType = {
  DEFAULT_CASE: "default_case",
  SERVICE_REQUEST: "service_request",
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

// Interface for support statistics card configuration.
export interface SupportStatConfig<Key = keyof ProjectSupportStats> {
  iconColor: "primary" | "secondary" | "success" | "error" | "info" | "warning";
  icon: ComponentType;
  key: Key;
  label: string;
  secondaryIcon?: ComponentType;
}

/**
 * Valid keys for project time tracking statistics.
 */
export type TimeTrackingStatKey =
  | "totalHours"
  | "billableHours"
  | "nonBillableHours";

/**
 * Configuration for the time tracking statistics cards.
 */
export const TIME_TRACKING_STAT_CONFIGS: SupportStatConfig<TimeTrackingStatKey>[] =
  [
    {
      icon: Clock,
      iconColor: "primary",
      key: "totalHours",
      label: "Total Hours",
    },
    {
      icon: Zap,
      iconColor: "success",
      key: "billableHours",
      label: "Billable Hours",
    },
    {
      icon: Activity,
      iconColor: "info",
      key: "nonBillableHours",
      label: "Non-Billable Hours",
    },
  ];

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
    label: "Ongoing Cases",
    secondaryIcon: TrendingUp,
  },
  {
    icon: MessageSquare,
    iconColor: "success",
    key: "sessionChats",
    label: "Chat Sessions",
    secondaryIcon: Bot,
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
export interface CaseDetailsTabConfig {
  label: string;
  Icon: ComponentType<{ size?: number }>;
}

export const CASE_DETAILS_TABS: CaseDetailsTabConfig[] = [
  { label: "Activity", Icon: MessageSquare },
  { label: "Details", Icon: Info },
  { label: "Attachments (0)", Icon: Paperclip },
  { label: "Calls (0)", Icon: Phone },
  { label: "Knowledge Base (0)", Icon: BookOpen },
];

//Palette intent for case status action buttons.
export type CaseStatusPaletteIntent = "error" | "warning" | "success" | "info";

// Case status action (e.g. Escalate, Mark as Resolved) for the action row.
export interface CaseStatusAction {
  label: string;
  Icon: ComponentType<{ size?: number }>;
  paletteIntent: CaseStatusPaletteIntent;
}

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
export const SUPPORT_OVERVIEW_CHAT_LIMIT = 5;

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

export const CHANGE_REQUEST_BULLET_ITEMS = [
  "Formal approval process",
  "Scheduled implementation",
  "Customer review and approval",
  "Rollback capabilities",
  "Calendar visualization",
  "Complete audit trail",
  "Impact and risk assessment",
  "Post implementation verification",
] as const;
/**
 * Interface for all cases filter configuration.
 */
export interface AllCasesFilterDefinition {
  id: string;
  metadataKey: keyof CaseMetadataResponse;
  filterKey: keyof AllCasesFilterValues;
  useLabelAsValue?: boolean;
}

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
 * Interface for conversations filter configuration.
 */
export interface AllConversationsFilterDefinition {
  id: string;
  metadataKey: keyof CaseMetadataResponse;
  filterKey: keyof AllConversationsFilterValues;
}

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
    filterKey: "caseTypeId",
    id: "caseType",
    metadataKey: "caseTypes",
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
 * Filter values for announcements page.
 */
export interface AnnouncementFilterValues {
  statusId?: string;
}

/**
 * Announcement filter definitions (status only).
 */
export const ANNOUNCEMENT_FILTER_DEFINITIONS: Array<{
  filterKey: keyof AnnouncementFilterValues;
  id: string;
  metadataKey: keyof CaseMetadataResponse;
  useLabelAsValue?: boolean;
}> = [
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

// Case type object interface.
export interface CaseTypeObject {
  id: string;
  label: string;
}

// Case type input.
export type CaseTypeInput = CaseTypeObject | string | null | undefined;

/**
 * Change Request Status types.
 */
export const ChangeRequestStatus = {
  SCHEDULED: "Scheduled",
  IN_PROGRESS: "In Progress",
  COMPLETED: "Completed",
  CANCELLED: "Cancelled",
  PENDING_APPROVAL: "Pending Approval",
} as const;

export type ChangeRequestStatus =
  (typeof ChangeRequestStatus)[keyof typeof ChangeRequestStatus];

/**
 * Change Request Impact types.
 */
export const ChangeRequestImpact = {
  HIGH: "High",
  MEDIUM: "Medium",
  LOW: "Low",
} as const;

export type ChangeRequestImpact =
  (typeof ChangeRequestImpact)[keyof typeof ChangeRequestImpact];

/**
 * Change Request State labels.
 */
export const ChangeRequestStates = {
  NEW: "New",
  ASSESS: "Assess",
  AUTHORIZE: "Authorize",
  CUSTOMER_APPROVAL: "Customer Approval",
  SCHEDULED: "Scheduled",
  IMPLEMENT: "Implement",
  REVIEW: "Review",
  CUSTOMER_REVIEW: "Customer Review",
  ROLLBACK: "Rollback",
  CLOSED: "Closed",
  CANCELED: "Canceled",
} as const;

export type ChangeRequestState =
  (typeof ChangeRequestStates)[keyof typeof ChangeRequestStates];

/**
 * Helper: Get color palette object for state label using exact matching.
 */
function getStateColorPalette(stateLabel: string | undefined) {
  if (!stateLabel) return colors.grey;

  switch (stateLabel) {
    case ChangeRequestStates.NEW:
      return colors.blue;
    case ChangeRequestStates.ASSESS:
      return colors.purple;
    case ChangeRequestStates.AUTHORIZE:
      return colors.pink;
    case ChangeRequestStates.CUSTOMER_APPROVAL:
      return colors.amber;
    case ChangeRequestStates.SCHEDULED:
      return colors.cyan;
    case ChangeRequestStates.IMPLEMENT:
      return colors.orange;
    case ChangeRequestStates.REVIEW:
      return colors.indigo;
    case ChangeRequestStates.CUSTOMER_REVIEW:
      return colors.yellow;
    case ChangeRequestStates.ROLLBACK:
      return colors.red;
    case ChangeRequestStates.CLOSED:
      return colors.green;
    case ChangeRequestStates.CANCELED:
      return colors.red;
    default:
      return colors.grey;
  }
}

/**
 * Get color for change request state.
 */
export function getChangeRequestStateColor(
  stateLabel: string | undefined,
): string {
  const palette = getStateColorPalette(stateLabel);

  if (stateLabel === ChangeRequestStates.CUSTOMER_REVIEW) {
    return palette[600];
  } else if (stateLabel === ChangeRequestStates.CANCELED) {
    return palette[700];
  } else if (!stateLabel) {
    return palette[400];
  }

  return palette[500];
}

/**
 * Get icon component for change request state.
 */
export function getChangeRequestStateIcon(
  stateLabel: string | undefined,
): ComponentType<{ size?: number }> {
  if (!stateLabel) return CircleQuestionMark;

  switch (stateLabel) {
    case ChangeRequestStates.NEW:
      return FileText;
    case ChangeRequestStates.ASSESS:
      return Activity;
    case ChangeRequestStates.AUTHORIZE:
      return ShieldCheck;
    case ChangeRequestStates.CUSTOMER_APPROVAL:
      return UserCheck;
    case ChangeRequestStates.SCHEDULED:
      return CalendarCheck;
    case ChangeRequestStates.IMPLEMENT:
      return PlayCircle;
    case ChangeRequestStates.REVIEW:
      return Eye;
    case ChangeRequestStates.CUSTOMER_REVIEW:
      return UserCheck;
    case ChangeRequestStates.ROLLBACK:
      return RotateCcw;
    case ChangeRequestStates.CLOSED:
      return CheckCircle;
    case ChangeRequestStates.CANCELED:
      return XCircle;
    default:
      return CircleQuestionMark;
  }
}

/**
 * Change Request Impact labels.
 */
export const ChangeRequestImpactLabels = {
  HIGH: "1 - High",
  MEDIUM: "2 - Medium",
  LOW: "3 - Low",
} as const;

/**
 * Helper: Get color palette object for impact label using exact matching.
 */
function getImpactColorPalette(impactLabel: string | undefined) {
  if (!impactLabel) return colors.grey;

  switch (impactLabel) {
    case ChangeRequestImpactLabels.HIGH:
      return colors.red;
    case ChangeRequestImpactLabels.MEDIUM:
      return colors.orange;
    case ChangeRequestImpactLabels.LOW:
      return colors.green;
    default:
      return colors.grey;
  }
}

/**
 * Get color for change request impact level.
 */
export function getChangeRequestImpactColor(
  impactLabel: string | undefined,
): string {
  const palette = getImpactColorPalette(impactLabel);
  return !impactLabel ? palette[400] : palette[500];
}

/**
 * Get color shadesfor change request impact level.
 */
export function getChangeRequestImpactColorShades(
  impactLabel: string | undefined,
): { bg: string; text: string; border: string } {
  const colorShades = getImpactColorPalette(impactLabel);

  return {
    bg: alpha(colorShades[500], 0.1),
    text: colorShades[800],
    border: alpha(colorShades[500], 0.2),
  };
}

/**
 * Get color shades (bg, text, border) for change request state.
 */
export function getChangeRequestStateColorShades(
  stateLabel: string | undefined,
): { bg: string; text: string; border: string } {
  const colorShades = getStateColorPalette(stateLabel);

  // Match the special-case shades used by getChangeRequestStateColor
  if (stateLabel === ChangeRequestStates.CUSTOMER_REVIEW) {
    return {
      bg: alpha(colorShades[600], 0.1),
      text: colorShades[800],
      border: alpha(colorShades[600], 0.2),
    };
  } else if (stateLabel === ChangeRequestStates.CANCELED) {
    return {
      bg: alpha(colorShades[700], 0.1),
      text: colorShades[800],
      border: alpha(colorShades[700], 0.2),
    };
  }

  return {
    bg: alpha(colorShades[500], 0.1),
    text: colorShades[800],
    border: alpha(colorShades[500], 0.2),
  };
}

/**
 * Format impact label by removing the numeric prefix.
 * "1 - High" -> "High"
 */
export function formatImpactLabel(impactLabel: string | undefined): string {
  if (!impactLabel) return "Not Available";

  // Remove "1 - ", "2 - ", "3 - " prefix
  return impactLabel.replace(/^\d+\s*-\s*/, "");
}

/**
 * Valid keys for change requests statistics.
 */
export type ChangeRequestStatKey =
  | "totalRequests"
  | "scheduled"
  | "inProgress"
  | "completed";

/**
 * Configuration for the change requests statistics cards.
 */
export const CHANGE_REQUEST_STAT_CONFIGS: SupportStatConfig<ChangeRequestStatKey>[] =
  [
    {
      icon: FileText,
      iconColor: "info",
      key: "totalRequests",
      label: "Total Requests",
    },
    {
      icon: Calendar,
      iconColor: "primary",
      key: "scheduled",
      label: "Scheduled",
    },
    {
      icon: Clock,
      iconColor: "warning",
      key: "inProgress",
      label: "In Progress",
    },
    {
      icon: CircleCheck,
      iconColor: "success",
      key: "completed",
      label: "Completed",
    },
  ];

/**
 * Change request filter definitions.
 */
export const CHANGE_REQUEST_FILTER_DEFINITIONS: Array<{
  filterKey: keyof ChangeRequestFilterValues;
  id: string;
  metadataKey: keyof CaseMetadataResponse;
}> = [
  {
    filterKey: "stateId",
    id: "state",
    metadataKey: "changeRequestStates",
  },
  {
    filterKey: "impactId",
    id: "impact",
    metadataKey: "changeRequestImpacts",
  },
];
