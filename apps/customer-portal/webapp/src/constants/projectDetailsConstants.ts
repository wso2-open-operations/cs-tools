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
import { User, Shield } from "@wso2/oxygen-ui-icons-react";
import type { ElementType } from "react";
import type { TabOption } from "@components/common/tab-bar/TabBar";
import { colors } from "@wso2/oxygen-ui";
import type { ProjectStatsResponse } from "@models/responses";
import { convertMinutesToHours } from "@utils/projectDetails";

export interface Contact {
  role: string;
  email: string | null;
  icon: ElementType<{ size?: number }>;
  bgColor: string;
}

export interface Stat {
  label: string;
  icon: ElementType<{ size?: number }>;
  iconColor: "primary" | "success" | "warning";
  key: keyof NonNullable<ProjectStatsResponse["projectStats"]>;
}

/** Project type labels for conditional UI visibility. */
export const PROJECT_TYPE_LABELS = {
  MANAGED_CLOUD_SUBSCRIPTION: "Managed Cloud Subscription",
  CLOUD_SUPPORT: "Cloud Support",
  CLOUD_EVALUATION_SUPPORT: "Cloud Evaluation Support",
} as const;

export const PROJECT_DETAILS_TABS: TabOption[] = [
  {
    id: "overview",
    label: "Overview",
    icon: Info,
  },
  {
    id: "deployments",
    label: "Deployments",
    icon: Server,
  },
  {
    id: "time-tracking",
    label: "Time Tracking",
    icon: Clock,
  },
];

export const contacts: Contact[] = [
  {
    role: "Account Manager",
    email: "TODO:[EMAIL_ADDRESS]",
    icon: User,
    bgColor: colors.blue[700],
  },
  {
    role: "Technical Owner",
    email: "TODO:[EMAIL_ADDRESS]",
    icon: Shield,
    bgColor: colors.purple[400],
  },
];

export interface ActivityItem {
  label: string;
  value: string;
  type?: "text" | "chip";
  chipColor?: ProjectStatusChipColor;
}

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

export const getRecentActivityItems = (
  activity?: ProjectStatsResponse["recentActivity"],
  projectTypeLabel?: string | null,
): ActivityItem[] => {
  const hideTimeTracking =
    projectTypeLabel === PROJECT_TYPE_LABELS.CLOUD_SUPPORT ||
    projectTypeLabel === PROJECT_TYPE_LABELS.CLOUD_EVALUATION_SUPPORT;

  const formatDateTime = (dateString: string): string => {
    if (!dateString) return "";
    try {
      const date = new Date(dateString.replace(" ", "T"));
      if (isNaN(date.getTime())) return "";
      const dateStr = date.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });
      const timeStr = date.toLocaleTimeString("en-US", {
        hour: "numeric",
        minute: "2-digit",
        hour12: true,
      });
      return `${dateStr} at ${timeStr}`;
    } catch {
      return "";
    }
  };

  const items: ActivityItem[] = [];

  if (!hideTimeTracking) {
    items.push(
      {
        label: "Total Time Logged",
        value:
          activity?.totalHours !== undefined
            ? `${convertMinutesToHours(activity.totalHours)} hrs`
            : "N/A",
        type: "text",
      },
      {
        label: "Billable Hours",
        value:
          activity?.billableHours !== undefined
            ? `${convertMinutesToHours(activity.billableHours)} hrs`
            : "N/A",
        type: "text",
      },
    );
  }

  items.push({
    label: "Last Deployment",
    value: activity?.lastDeploymentOn
      ? formatDateTime(activity.lastDeploymentOn)
      : "N/A",
    type: "text",
  });

  return items;
};

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

export type SupportTier = (typeof SUPPORT_TIER)[keyof typeof SUPPORT_TIER];

export const PROJECT_TYPE = {
  SUBSCRIPTION: "Subscription",
  FREE: "Free",
} as const;

export type ProjectType = (typeof PROJECT_TYPE)[keyof typeof PROJECT_TYPE];

export const SYSTEM_HEALTH = {
  HEALTHY: "Healthy",
  CRITICAL: "Critical",
} as const;

export type SystemHealth = (typeof SYSTEM_HEALTH)[keyof typeof SYSTEM_HEALTH];

export const SLA_STATUS = {
  ALL_GOOD: "All Good",
  NEEDS_ATTENTION: "Needs attention",
} as const;

export type SLAStatus = (typeof SLA_STATUS)[keyof typeof SLA_STATUS];

export const CASE_SEVERITY = {
  S0: "S0",
  S1: "S1",
  S2: "S2",
  S3: "S3",
  S4: "S4",
} as const;

export type CaseSeverity = (typeof CASE_SEVERITY)[keyof typeof CASE_SEVERITY];

export const CASE_STATUS = {
  OPEN: "Open",
  WORK_IN_PROGRESS: "Work In Progress",
  AWAITING_INFO: "Awaiting Info",
  WAITING_ON_WSO2: "Waiting On WSO2",
  SOLUTION_PROPOSED: "Solution Proposed",
  CLOSED: "Closed",
  REOPENED: "Reopened",
} as const;

export const CASE_TYPES = {
  INCIDENT: "Incident",
  QUERY: "Query",
  SERVICE_REQUEST: "Service Request",
  SECURITY_REPORT_ANALYSIS: "Security Report Analysis",
} as const;

export type CaseStatus = (typeof CASE_STATUS)[keyof typeof CASE_STATUS];

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

export type TimeCardState =
  (typeof TIME_CARD_STATE)[keyof typeof TIME_CARD_STATE];

export const DEPLOYMENT_STATUS = {
  HEALTHY: "Healthy",
  WARNING: "Warning",
  ERROR: "Error",
} as const;

export type DeploymentStatus =
  (typeof DEPLOYMENT_STATUS)[keyof typeof DEPLOYMENT_STATUS];

export const PRODUCT_SUPPORT_STATUS = {
  ACTIVE: "Active Support",
  END_OF_LIFE: "End of Life",
  DEPRECATED: "Deprecated",
  LIMITED: "Limited Support",
  EXTENDED: "Extended Support",
} as const;

export type ProductSupportStatus =
  (typeof PRODUCT_SUPPORT_STATUS)[keyof typeof PRODUCT_SUPPORT_STATUS];

export type ProjectStatusChipColor =
  | "default"
  | "primary"
  | "secondary"
  | "error"
  | "info"
  | "success"
  | "warning";
