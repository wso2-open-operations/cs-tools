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
import { colors } from "@wso2/oxygen-ui";
import type {
  StatCardColor,
  StatConfigItem,
  CasesTrendChartDataItem,
  CaseTypeChipConfig,
} from "@features/dashboard/types/dashboard";
import {
  CaseTypeChipKind,
  SeverityLegendKey,
  type SeverityLegendEntry,
} from "@features/dashboard/types/dashboard";

export type { StatCardColor, StatConfigItem, CasesTrendChartDataItem, CaseTypeChipConfig };

// Case type labels to filter stats by (Incident, Query, Service Request, Security Report Analysis).
export const DASHBOARD_CASE_TYPE_LABELS = [
  "Incident",
  "Query",
  "Service Request",
  "Security Report Analysis",
] as const;

// API labels that map to the unified “Case” chip (Incident + Query).
export const CASE_CHIP_INCIDENT_QUERY_LABELS = ["Incident", "Query"] as const;

// Display labels for {@link CaseTypeChipKind} (non-fallback; fallback uses raw API text).
export const CASE_TYPE_CHIP_DISPLAY_LABEL: Record<
  Exclude<CaseTypeChipKind, CaseTypeChipKind.Fallback>,
  string
> = {
  [CaseTypeChipKind.IncidentOrQuery]: "Case",
  [CaseTypeChipKind.SecurityReportAnalysis]: "Security Report Analysis",
  [CaseTypeChipKind.ServiceRequest]: "Service Request",
  [CaseTypeChipKind.ChangeRequest]: "Change Request",
};

// Dashboard statistics list.
export const DASHBOARD_STATS: StatConfigItem[] = [
  {
    id: "totalCases",
    label: "Total Interactions",
    icon: Clock,
    iconColor: "primary",
    tooltipText: "Total number of cases reported for this project",
  },
  {
    id: "openCases",
    label: "Active Interactions",
    icon: AlertCircle,
    iconColor: "warning",
    tooltipText: "Currently active and unresolved cases",
  },
  {
    id: "resolvedCases",
    label: "Resolved Support Cases (Last 30d)",
    icon: CheckCircle,
    iconColor: "success",
    tooltipText: "Successfully closed and resolved cases",
  },
  {
    id: "avgResponseTime",
    label: "Avg. Response Time (Last 30d)",
    icon: Activity,
    iconColor: "info",
    tooltipText: "Average time taken to first respond to a case",
  },
];

// Configuration for Outstanding Operations Chart data mapping (Service Requests vs Change Requests).
export const ACTIVE_CASES_CHART_DATA = [
  {
    name: "Service Requests (SR)",
    key: "serviceRequests",
    color: colors.orange[500],
  },
  {
    name: "Change Requests (CR)",
    key: "changeRequests",
    color: colors.blue[500],
  },
] as const;

// Maps severity API label to display name (S0-S4) for charts and table.
export const SEVERITY_LABEL_TO_DISPLAY: Record<string, string> = {
  "Catastrophic (P0)": "S0",
  "Critical (P1)": "S1",
  "High (P2)": "S2",
  "Medium (P3)": "S3",
  "Low (P4)": "S4(Query)",
  "0 - Catastrophic": "S0",
  "1 - Critical": "S1",
  "2 - High": "S2",
  "3 - Moderate": "S3",
  "4 - Low": "S4(Query)",
};

// S0 (Catastrophic) severity API labels - used when filtering out S0 for non-Managed Cloud Subscription projects.
export const S0_SEVERITY_LABELS = [
  "Catastrophic (P0)",
  "0 - Catastrophic",
  "0 - catastrophic",
] as const;

// Severity legend order.
export const SEVERITY_LEGEND_ORDER: SeverityLegendEntry[] = [
  {
    key: SeverityLegendKey.Catastrophic,
    label: "Catastrophic (P0)",
    displayName: "S0 - Catastrophic",
    color: colors.red[500],
  },
  {
    key: SeverityLegendKey.Critical,
    label: "Critical (P1)",
    displayName: "S1 - Critical",
    color: colors.orange[500],
  },
  {
    key: SeverityLegendKey.High,
    label: "High (P2)",
    displayName: "S2 - High",
    color: colors.yellow[700],
  },
  {
    key: SeverityLegendKey.Medium,
    label: "Medium (P3)",
    displayName: "S3 - Medium",
    color: colors.blue[500],
  },
  {
    key: SeverityLegendKey.Low,
    label: "Low (P4)",
    displayName: "S4 (Query) - Low",
    color: colors.green[500],
  },
];

// Outstanding incidents chart data.
export const OUTSTANDING_INCIDENTS_CHART_DATA = SEVERITY_LEGEND_ORDER;

// API severity labels in chart order (catastrophic, critical, high, medium, low) for casesTrend mapping.
export const SEVERITY_API_LABELS = SEVERITY_LEGEND_ORDER.map(
  (item) => item.label,
);

// Alternate severity labels for announcements (e.g. "1 - Critical", "2 - High") mapped to legend keys.
export const SEVERITY_ALT_TO_LEGEND_KEY: Record<string, SeverityLegendKey> = {
  "0 - catastrophic": SeverityLegendKey.Catastrophic,
  "1 - critical": SeverityLegendKey.Critical,
  "2 - high": SeverityLegendKey.High,
  "3 - moderate": SeverityLegendKey.Medium,
  "4 - low": SeverityLegendKey.Low,
};

// Maps S0-S4 / legend key to friendly label for chips (Critical, High, etc.).
export const SEVERITY_FRIENDLY_LABEL: Record<SeverityLegendKey, string> = {
  [SeverityLegendKey.Catastrophic]: "Catastrophic",
  [SeverityLegendKey.Critical]: "Critical",
  [SeverityLegendKey.High]: "High",
  [SeverityLegendKey.Medium]: "Medium",
  [SeverityLegendKey.Low]: "Low",
};

// Outstanding engagements category chart data.
export const OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA = [
  { key: "onboarding", name: "Onboarding", color: colors.blue[500] },
  { key: "migration", name: "Migration", color: colors.orange[500] },
  { key: "services", name: "Services", color: colors.green[500] },
  { key: "follow-up", name: "Follow up", color: colors.purple[500] },
  { key: "improvements", name: "Improvements", color: colors.brown[500] },
] as const;

// Configuration for Cases Trend Chart (same legend order as Outstanding Engagements).
export const CASES_TREND_CHART_DATA: CasesTrendChartDataItem[] =
  SEVERITY_LEGEND_ORDER.map((item, i) => ({
    name: item.displayName,
    key: item.key,
    color: item.color,
    ...(i === 0 && {
      radius: [0, 0, 4, 4] as [number, number, number, number],
    }),
    ...(i === 4 && {
      radius: [4, 4, 0, 0] as [number, number, number, number],
      border: true,
    }),
  }));
