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

import { colors } from "@wso2/oxygen-ui";
import { CircleAlert, Cloud, MessageSquare, Moon, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ProjectMetricKey, ProjectStatus, ProjectType } from "@src/types";
import type { ProjectMetricMeta } from "@components/features/projects";
import type { ProgressTimelineEntryProps } from "../components/features/detail";
import type { ItemCardProps } from "../components/features/support";

export const INPUT_INVALID_MSG_GATEWAY = "INPUT_INVALID_MSG_GATEWAY";

export const PROJECT_METRIC_META: Record<ProjectMetricKey, ProjectMetricMeta> = {
  outstanding: { label: "Outstanding:", color: colors.red[300], icon: CircleAlert },
  chats: { label: "Chats:", color: colors.indigo[300], icon: MessageSquare },
};

export const PROJECT_TYPE_META: Record<ProjectType, { icon: LucideIcon }> = {
  Regular: { icon: Moon },
  "Managed Cloud": { icon: Cloud },
};

export const PROJECT_STATUS_META: Record<ProjectStatus, { color: "success" | "warning" }> = {
  "All Good": { color: "success" },
  "Needs Attention": { color: "warning" },
};

export const PROJECT_SEVERITY_PIE_COLORS: Record<string, string> = {
  "10": colors.red[500],
  "11": colors.orange[500],
  "12": colors.yellow[600],
  "13": colors.blue[500],
  "14": colors.green[500],
};

export const ENGAGEMENTS_TYPE_PIE_COLORS: Record<string, string> = {
  Migration: colors.blue[500],
  Consultancy: colors.green[500],
  "New Feature / Improvement": colors.teal[400],
  "Follow up": colors.cyan[500],
  Onboarding: colors.yellow[600],
};

export const ADMIN_USER_ROLE = "sn_customerservice.customer_admin";

export const TIMELINE_META: Omit<ProgressTimelineEntryProps, "variant">[] = [
  {
    title: "New",
    description: "Change request created",
  },
  {
    title: "Assess",
    description: "Technical assessment completed",
  },
  {
    title: "Authorize",
    description: "Internal authorization obtained",
  },
  {
    title: "Customer Approval",
    description: "Customer approval received",
  },
  {
    title: "Scheduled",
    description: "Maintenance window scheduled",
  },
  {
    title: "Implement",
    description: "Change implementation",
  },
  {
    title: "Review",
    description: "Internal review",
  },
  {
    title: "Customer Review",
    description: "Customer validation",
  },
  {
    title: "Rollback",
    description: "Change rollback if needed",
  },
  {
    title: "Closed",
    description: "Change request completed",
  },
  {
    title: "Canceled",
    description: "Change request canceled",
  },
];

export const TAB_CONFIG = {
  case: { title: "Outstanding Cases", subtitle: "Active support tickets" },
  chat: { title: "Chat History", subtitle: "Recent Novera conversations" },
  service: { title: "Service Requests", subtitle: "Managed cloud service requests" },
  change: { title: "Change Requests", subtitle: "Scheduled and pending changes" },
  sra: { title: "Security Report Analysis", subtitle: "Security findings, assessments, and reviews" },
  engagement: { title: "Engagements", subtitle: "Ongoing and completed client engagements" },
  announcement: { title: "Announcements", subtitle: "View and manage announcements for your project" },
};

export const ITEM_DETAIL_PATHS: Record<ItemCardProps["type"], (id: string) => string> = {
  case: (id) => `/cases/${id}`,
  chat: (id) => `/chats/${id}`,
  service: (id) => `/services/${id}`,
  change: (id) => `/changes/${id}`,
  sra: (id) => `/sras/${id}`,
  engagement: (id) => `/engagements/${id}`,
  announcement: (id) => `/announcements/${id}`,
};

export const LOCAL_STORAGE_LAST_VISITED_PROJECT_KEY = "last-active-project-id";

export const SEARCH_PLACEHOLDER_CONFIG: Record<ItemCardProps["type"], string> = {
  case: "Search Cases",
  chat: "Search Chats",
  service: "Search Service Requests",
  change: "Search Change Requests",
  sra: "Search Security Report Analysis",
  engagement: "Search Engagement",
  announcement: "Search Announcements",
};

export const CASE_STATE_IDS = {
  OPEN: 1,
  WORK_IN_PROGRESS: 10,
  AWAITING_INFO: 18,
  WAITING_ON_WSO2: 1003,
  SOLUTION_PROPOSED: 6,
  CLOSED: 3,
  REOPENED: 1006,
} as const;

export const ACTION_REQUIRED_CASE_STATUS_IDS = [18, 6];
export const ACTION_REQUIRED_CHANGE_REQUEST_STATUS_IDS = [1, 5];

export const OUTSTANDING_CASE_STATUS_IDS = [1, 10, 18, 1003, 6, 1006];
export const OUTSTANDING_CONVERSATIONS_STATUS_IDS = [1, 2];
export const OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS = [5, -2, -1, 0, 1, 2];

export const RESOLVED_CASE_STATUS_IDS = [3];
export const RESOLVED_CHANGE_REQUEST_STATUS_IDS = [3];

export const PLURALS: Record<ItemCardProps["type"], string> = {
  case: "Cases",
  chat: "Chats",
  service: "Service Requests",
  change: "Change Requests",
  sra: "Security Report Analysis",
  engagement: "Engagements",
  announcement: "Announcements",
};

export const DEPLOYMENT_DISABLED_PROJECT_TYPES = ["Cloud Support", "Cloud Evaluation Support"];
