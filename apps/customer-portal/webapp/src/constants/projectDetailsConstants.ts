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
import { getSystemHealthColor } from "@utils/projectStats";

export interface Contact {
  role: string;
  email: string;
  icon: ElementType<{ size?: number }>;
  bgColor: string;
}

export interface Stat {
  label: string;
  icon: ElementType<{ size?: number }>;
  iconColor: "primary" | "success" | "warning";
  key: keyof NonNullable<ProjectStatsResponse["projectStats"]>;
}

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
  chipColor?: "success" | "warning" | "error" | "default" | "primary" | "info";
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
): ActivityItem[] => [
  {
    label: "Total Time Logged",
    value:
      activity?.totalTimeLogged !== undefined
        ? `${activity.totalTimeLogged}h`
        : "N/A",
    type: "text",
  },
  {
    label: "Billable Hours",
    value:
      activity?.billableHours !== undefined
        ? `${activity.billableHours}h`
        : "N/A",
    type: "text",
  },
  {
    label: "Last Deployment",
    value: activity?.lastDeploymentOn
      ? new Date(activity.lastDeploymentOn).toLocaleDateString()
      : "N/A",
    type: "text",
  },
  {
    label: "System Health",
    value: activity?.systemHealth || "N/A",
    type: "chip",
    chipColor: getSystemHealthColor(activity?.systemHealth || ""),
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
  GOOD: "All Good",
  BAD: "Bad",
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

export type CaseStatus = (typeof CASE_STATUS)[keyof typeof CASE_STATUS];
