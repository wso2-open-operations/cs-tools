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

/**
 * Valid color types for the stat card icons.
 */
export type StatCardColor =
  | "primary"
  | "secondary"
  | "error"
  | "warning"
  | "info"
  | "success";

/**
 * Configuration for a single statistic card.
 */
export interface StatConfigItem {
  /**
   * Unique identifier matching the keys in DashboardMockStats.
   */
  id: keyof DashboardMockStats;
  /**
   * Display label for the statistic.
   */
  label: string;
  /**
   * Icon component to display.
   */
  icon: any;
  /**
   * Color theme for the icon.
   */
  iconColor: StatCardColor;
  /**
   * Text to display in the tooltip.
   */
  tooltipText: string;
}

/**
 * Dashboard statistics list.
 */
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
