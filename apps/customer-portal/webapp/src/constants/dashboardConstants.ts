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
  Clock,
  AlertCircle,
  CheckCircle,
  Activity,
} from "@wso2/oxygen-ui-icons-react";
import { type DashboardMockStats } from "@/models/responses";
import { colors } from "@wso2/oxygen-ui";
import { type FilterField } from "@/components/common/filterPanel/FilterPopover";

// Valid color types for the stat card icons.
export type StatCardColor =
  | "primary"
  | "secondary"
  | "error"
  | "warning"
  | "info"
  | "success";

// Configuration for a single statistic card.
export interface StatConfigItem {
  id: Exclude<keyof DashboardMockStats, "casesTrend">;
  label: string;
  icon: any;
  iconColor: StatCardColor;
  tooltipText: string;
}

// Dashboard statistics list.
export const DASHBOARD_STATS: StatConfigItem[] = [
  {
    id: "totalCases",
    label: "Total Cases",
    icon: Clock,
    iconColor: "primary",
    tooltipText: "Total number of cases reported for this project",
  },
  {
    id: "openCases",
    label: "Open Cases",
    icon: AlertCircle,
    iconColor: "warning",
    tooltipText: "Currently active and unresolved cases",
  },
  {
    id: "resolvedCases",
    label: "Resolved Cases",
    icon: CheckCircle,
    iconColor: "success",
    tooltipText: "Successfully closed and resolved cases",
  },
  {
    id: "avgResponseTime",
    label: "Avg. Response Time",
    icon: Activity,
    iconColor: "info",
    tooltipText: "Average time taken to first respond to a case",
  },
];

// Configuration for Active Cases Chart data mapping.
export const ACTIVE_CASES_CHART_DATA = [
  {
    name: "Work in progress",
    key: "workInProgress",
    color: colors.blue[500],
  },
  {
    name: "Waiting on client",
    key: "waitingOnClient",
    color: colors.green[500],
  },
  {
    name: "Waiting on WSO2",
    key: "waitingOnWso2",
    color: colors.orange[500],
  },
] as const;

// Configuration for Outstanding Incidents Chart data mapping.
export const OUTSTANDING_INCIDENTS_CHART_DATA = [
  {
    name: "Medium",
    key: "medium",
    color: colors.blue[500],
  },
  {
    name: "High",
    key: "high",
    color: colors.orange[500],
  },
  {
    name: "Critical",
    key: "critical",
    color: colors.red[500],
  },
] as const;

/**
 * Type definition for Cases Trend Chart data item.
 */
export interface CasesTrendChartDataItem {
  name: string;
  key: string;
  color: string;
  radius?: [number, number, number, number];
  border?: boolean;
}

// Configuration for Cases Trend Chart data mapping.
export const CASES_TREND_CHART_DATA: CasesTrendChartDataItem[] = [
  {
    name: "Type A",
    key: "TypeA",
    color: colors.blue[500],
    radius: [0, 0, 4, 4],
  },
  {
    name: "Type B",
    key: "TypeB",
    color: colors.green[500],
  },
  {
    name: "Type C",
    key: "TypeC",
    color: colors.orange[500],
  },
  {
    name: "Type D",
    key: "TypeD",
    color: colors.yellow[600],
    radius: [4, 4, 0, 0],
    border: true,
  },
];

// Filter fields for the cases table.
export const FILTER_FIELDS: FilterField[] = [
  {
    id: "deploymentId",
    label: "Deployment",
    type: "select",
    options: [
      { label: "Development", value: "Development" },
      { label: "Production", value: "production" },
      { label: "QA", value: "QA" },
      { label: "Staging", value: "Staging" },
    ],
  },
  {
    id: "severityId",
    label: "Severity",
    type: "select",
    options: [
      { label: "Critical", value: "0" },
      { label: "High", value: "1" },
      { label: "Medium", value: "2" },
      { label: "Low", value: "3" },
      { label: "Minimal", value: "4" },
    ],
  },
  {
    id: "statusId",
    label: "Status",
    type: "select",
    options: [
      { label: "Open", value: "0" },
      { label: "In Progress", value: "1" },
      { label: "Awaiting Response", value: "2" },
      { label: "Resolved", value: "3" },
      { label: "Closed", value: "4" },
    ],
  },
  {
    id: "caseTypes",
    label: "Case Type",
    type: "select",
    options: [
      "Total Outage",
      "Partial Outage",
      "Performance Degradation",
      "Question",
      "Security or Compliance",
      "Error",
    ],
  },
];
