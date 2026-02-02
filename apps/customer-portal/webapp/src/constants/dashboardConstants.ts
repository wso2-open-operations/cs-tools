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
    color: "#3B82F6",
  },
  {
    name: "Waiting on client",
    key: "waitingOnClient",
    color: "#22C55E",
  },
  {
    name: "Waiting on WSO2",
    key: "waitingOnWso2",
    color: "#EAB308",
  },
] as const;

// Configuration for Outstanding Incidents Chart data mapping.
export const OUTSTANDING_INCIDENTS_CHART_DATA = [
  {
    name: "Medium",
    key: "medium",
    color: "#3B82F6",
  },
  {
    name: "High",
    key: "high",
    color: "#F97316",
  },
  {
    name: "Critical",
    key: "critical",
    color: "#EF4444",
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
    color: "#3B82F6",
    radius: [0, 0, 4, 4],
  },
  {
    name: "Type B",
    key: "TypeB",
    color: "#22C55E",
  },
  {
    name: "Type C",
    key: "TypeC",
    color: "#F97316",
  },
  {
    name: "Type D",
    key: "TypeD",
    color: "#EAB308",
    radius: [4, 4, 0, 0],
    border: true,
  },
];
