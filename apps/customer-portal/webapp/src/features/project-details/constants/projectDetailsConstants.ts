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
  CircleAlert,
  Clock,
  Info,
  Rocket,
  Server,
} from "@wso2/oxygen-ui-icons-react";
import type { TabOption } from "@components/tab-bar/TabBar";
import { ProjectDetailsTabId } from "@features/project-details/types/projectDetails";
import type { Stat } from "@features/project-details/types/projectDetails";
import type { UpdateHistoryFormData } from "@features/project-details/types/projectDetailsComponents";

/** Generic placeholder when a project detail value is missing (labels, stats, documents). */
export const PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY = "Not Available";

export const PROJECT_DETAILS_SERVICE_HOURS_NOT_AVAILABLE =
  PROJECT_DETAILS_NOT_AVAILABLE_DISPLAY;

export const PROJECT_DETAILS_INVALID_PROJECT_ID_MESSAGE =
  "Invalid Project ID. Please check the URL.";

/** Matches common image file extensions for deployment document icons. */
export const DEPLOYMENT_DOCUMENT_IMAGE_FILE_REGEX =
  /\.(png|jpe?g|gif|webp|svg|bmp|ico)$/i;

/** Matches archive file extensions for deployment document icons. */
export const DEPLOYMENT_DOCUMENT_ARCHIVE_FILE_REGEX =
  /\.(zip|tar|gz|7z|rar|bz2)$/i;

export const PROJECT_DETAILS_TABS: TabOption[] = [
  {
    id: ProjectDetailsTabId.OVERVIEW,
    label: "Overview",
    icon: Info,
  },
  {
    id: ProjectDetailsTabId.DEPLOYMENTS,
    label: "Deployments",
    icon: Server,
  },
  {
    id: ProjectDetailsTabId.TIME_TRACKING,
    label: "Time Tracking",
    icon: Clock,
  },
];

export const statItems: Stat[] = [
  {
    label: "Open Cases",
    icon: Clock,
    iconColor: "primary",
    key: "openCases",
  },
  {
    label: "Active Chats",
    icon: CircleAlert,
    iconColor: "success",
    key: "activeChats",
  },
  {
    label: "Deployments",
    icon: Rocket,
    iconColor: "warning",
    key: "deployments",
  },
];

export const SUBSCRIPTION_STATUS = {
  EXPIRED: "Expired",
  EXPIRING_SOON: "Expiring Soon",
  ACTIVE: "Active",
} as const;

export type SubscriptionStatus =
  (typeof SUBSCRIPTION_STATUS)[keyof typeof SUBSCRIPTION_STATUS];

export const SUPPORT_TIER = {
  ENTERPRISE: "Enterprise",
  STANDARD: "Standard",
} as const;

export const PROJECT_TYPE = {
  SUBSCRIPTION: "Subscription",
  FREE: "Free",
} as const;

export const SYSTEM_HEALTH = {
  HEALTHY: "Healthy",
  CRITICAL: "Critical",
} as const;

export const CASE_STATUS = {
  OPEN: "Open",
  WORK_IN_PROGRESS: "Work In Progress",
  AWAITING_INFO: "Awaiting Info",
  WAITING_ON_WSO2: "Waiting On WSO2",
  SOLUTION_PROPOSED: "Solution Proposed",
  CLOSED: "Closed",
  REOPENED: "Reopened",
} as const;

export const PROJECT_USER_STATUSES = {
  INVITED: "invited",
  REGISTERED: "registered",
} as const;

export const TIME_TRACKING_BADGE_TYPES = {
  SUPPORT: "support",
  BILLABLE: "billable",
  CASE: "case",
  CONSULTATION: "consultation",
  MAINTENANCE: "maintenance",
} as const;

export type TimeTrackingBadgeType =
  (typeof TIME_TRACKING_BADGE_TYPES)[keyof typeof TIME_TRACKING_BADGE_TYPES];

export const TIME_CARD_STATE = {
  PENDING: "Pending",
  SUBMITTED: "Submitted",
  APPROVED: "Approved",
  REJECTED: "Rejected",
  PROCESSED: "Processed",
  RECALLED: "Recalled",
} as const;

export const DEPLOYMENT_STATUS = {
  HEALTHY: "Healthy",
  WARNING: "Warning",
  ERROR: "Error",
} as const;

export const PRODUCT_SUPPORT_STATUS = {
  ACTIVE: "Active Support",
  END_OF_LIFE: "End of Life",
  DEPRECATED: "Deprecated",
  LIMITED: "Limited Support",
  EXTENDED: "Extended Support",
} as const;

export type {
  ActivityItem,
  ProjectStatusChipColor,
} from "@features/project-details/types/projectDetails";

export const UPDATE_HISTORY_INITIAL_FORM_DATA: UpdateHistoryFormData = {
  updateLevel: "",
  date: "",
  details: "",
};
