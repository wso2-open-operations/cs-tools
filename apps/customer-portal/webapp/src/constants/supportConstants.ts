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
  BookOpen,
  Bot,
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
} from "@wso2/oxygen-ui-icons-react";
import { type ComponentType } from "react";
import type {
  ProjectSupportStats,
  ProjectCasesStats,
  CaseMetadataResponse,
  AllCasesFilterValues,
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
  STILL_OPEN: "Still Open",
  ABANDONED: "Abandoned",
} as const;

export type ChatStatus = (typeof ChatStatus)[keyof typeof ChatStatus];

// Maximum allowed attachment file size in bytes.
export const MAX_ATTACHMENT_SIZE_BYTES = 15 * 1024 * 1024;

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
    label: "Open",
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
 *
 * @param {ProjectCasesStats | undefined} stats - The original stats.
 * @returns {Record<AllCasesStatKey, number | undefined>} The flattened stats.
 */
export const getAllCasesFlattenedStats = (
  stats: ProjectCasesStats | undefined,
): Record<AllCasesStatKey, number | undefined> => ({
  openCases: stats?.openCases,
  waitingOnClient: stats?.activeCases?.waitingOnClient,
  waitingOnWso2: stats?.activeCases?.waitingOnWso2,
  workInProgress: stats?.activeCases?.workInProgress,
});

// Configuration for the support statistics cards.
export const SUPPORT_STAT_CONFIGS: SupportStatConfig[] = [
  {
    icon: FileText,
    iconColor: "error",
    key: "totalCases",
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

// Case status actions shown in the case details action row.
export const CASE_STATUS_ACTIONS: CaseStatusAction[] = [
  { label: "Escalate Case", Icon: TriangleAlert, paletteIntent: "error" },
  { label: "Waiting on WSO2", Icon: CirclePause, paletteIntent: "warning" },
  { label: "Mark as Resolved", Icon: CircleCheck, paletteIntent: "success" },
  { label: "Close Case", Icon: CircleX, paletteIntent: "info" },
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
 * Configuration for the all cases filters.
 */
export const ALL_CASES_FILTER_DEFINITIONS: AllCasesFilterDefinition[] = [
  {
    filterKey: "statusId",
    id: "status",
    metadataKey: "statuses",
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
    metadataKey: "deployments",
  },
];
