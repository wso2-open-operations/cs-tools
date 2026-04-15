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
  CalendarDays,
  FileText,
  Server,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";
import type { DashboardMockStats } from "@features/support/types/cases";
import { colors } from "@wso2/oxygen-ui";

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
  icon: ComponentType<{ size?: number }>;
  iconColor: StatCardColor;
  tooltipText: string;
}

/** Case type labels to filter stats by (Incident, Query, Service Request, Security Report Analysis). */
export const DASHBOARD_CASE_TYPE_LABELS = [
  "Incident",
  "Query",
  "Service Request",
  "Security Report Analysis",
] as const;

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

/** Maps severity API label to display name (S0-S4) for charts and table. */
export const SEVERITY_LABEL_TO_DISPLAY: Record<string, string> = {
  "Catastrophic (P0)": "S0",
  "Critical (P1)": "S1",
  "High (P2)": "S2",
  "Medium (P3)": "S3",
  "Low (P4)": "S4",
  "0 - Catastrophic": "S0",
  "1 - Critical": "S1",
  "2 - High": "S2",
  "3 - Moderate": "S3",
  "4 - Low": "S4",
};

/** S0 (Catastrophic) severity API labels - used when filtering out S0 for non-Managed Cloud Subscription projects. */
export const S0_SEVERITY_LABELS = [
  "Catastrophic (P0)",
  "0 - Catastrophic",
  "0 - catastrophic",
] as const;

/** Returns true if the severity label corresponds to S0 (Catastrophic). */
export function isS0SeverityLabel(label?: string | null): boolean {
  if (!label?.trim()) return false;
  const trimmed = label.trim().toLowerCase();
  return S0_SEVERITY_LABELS.some((l) => l.toLowerCase() === trimmed);
}

// Legend display format: "S{n} - {Severity}". Same order for Outstanding Engagements and Cases Trend.
// Order: S0 - Catastrophic (highest), S1 - Critical, S2 - High, S3 - Medium, S4 - Low (lowest).
// Matches SEVERITY_LABEL_TO_DISPLAY: Catastrophic (P0)→S0, Critical (P1)→S1, etc.
export const SEVERITY_LEGEND_ORDER = [
  {
    key: "catastrophic",
    label: "Catastrophic (P0)",
    displayName: "S0 - Catastrophic",
    color: colors.red[500],
  },
  {
    key: "critical",
    label: "Critical (P1)",
    displayName: "S1 - Critical",
    color: colors.orange[500],
  },
  {
    key: "high",
    label: "High (P2)",
    displayName: "S2 - High",
    color: colors.yellow[700],
  },
  {
    key: "medium",
    label: "Medium (P3)",
    displayName: "S3 - Medium",
    color: colors.blue[500],
  },
  {
    key: "low",
    label: "Low (P4)",
    displayName: "S4 (Queries) - Low",
    color: colors.green[500],
  },
] as const;

export const OUTSTANDING_INCIDENTS_CHART_DATA = SEVERITY_LEGEND_ORDER;

/** API severity labels in chart order (catastrophic, critical, high, medium, low) for casesTrend mapping. */
export const SEVERITY_API_LABELS = SEVERITY_LEGEND_ORDER.map(
  (item) => item.label,
);

/** Alternate severity labels for announcements (e.g. "1 - Critical", "2 - High") mapped to legend keys. */
export const SEVERITY_ALT_TO_LEGEND_KEY: Record<string, string> = {
  "0 - catastrophic": "catastrophic",
  "1 - critical": "critical",
  "2 - high": "high",
  "3 - moderate": "medium",
  "4 - low": "low",
};

/** Maps S0-S4 / legend key to friendly label for chips (Critical, High, etc.). */
export const SEVERITY_FRIENDLY_LABEL: Record<string, string> = {
  catastrophic: "Catastrophic",
  critical: "Critical",
  high: "High",
  medium: "Medium",
  low: "Low",
};

/**
 * Returns the friendly severity label for display in chips (e.g. "Critical", "High").
 *
 * @param label - API severity label (e.g. "1 - Critical", "High (P2)").
 * @returns {string} Friendly label or original.
 */
export function getSeverityFriendlyLabel(label?: string): string {
  if (!label?.trim()) return "--";
  const trimmed = label.trim().toLowerCase();
  const entry =
    SEVERITY_LEGEND_ORDER.find(
      (item) => item.label.toLowerCase() === trimmed,
    ) ??
    (SEVERITY_ALT_TO_LEGEND_KEY[trimmed]
      ? SEVERITY_LEGEND_ORDER.find(
          (item) => item.key === SEVERITY_ALT_TO_LEGEND_KEY[trimmed],
        )
      : undefined);
  return entry ? (SEVERITY_FRIENDLY_LABEL[entry.key] ?? entry.label) : label;
}

/**
 * Returns the chart legend color for a severity label (same as Outstanding Engagements chart).
 * Supports "Critical (P1)", "1 - Critical", "2 - High", "3 - Moderate", etc.
 *
 * @param label - API severity label (e.g. "Catastrophic (P0)", "High (P2)", "1 - Critical").
 * @returns {string} Hex color from chart legend, or grey fallback.
 */
export function getSeverityLegendColor(label?: string): string {
  if (!label?.trim()) return colors.grey?.[500] ?? "#6B7280";
  const trimmed = label.trim();
  const lower = trimmed.toLowerCase();
  const entry =
    SEVERITY_LEGEND_ORDER.find((item) => item.label.toLowerCase() === lower) ??
    (SEVERITY_ALT_TO_LEGEND_KEY[lower]
      ? SEVERITY_LEGEND_ORDER.find(
          (item) => item.key === SEVERITY_ALT_TO_LEGEND_KEY[lower],
        )
      : undefined);
  return entry?.color ?? colors.grey?.[500] ?? "#6B7280";
}

/**
 * Static configuration for Outstanding Engagements category chart.
 * Used for the Outstanding Engagements donut (Onboarding, Migration, Services, Improvements).
 */
export const OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA = [
  { key: "onboarding", name: "Onboarding", color: colors.blue[500] },
  { key: "migration", name: "Migration", color: colors.orange[500] },
  { key: "services", name: "Services", color: colors.green[500] },
  { key: "follow-up", name: "Follow up", color: colors.purple[500] },
  { key: "improvements", name: "Improvements", color: colors.brown[500] },
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

/** Case type display config for Outstanding Engagements table Type column. */
export interface CaseTypeChipConfig {
  displayLabel: string;
  Icon: ComponentType<{ size?: number }>;
  bgColor: string;
  textColor: string;
  borderColor: string;
}

/** API labels that map to "Case" (Incident + Query). */
const CASE_TYPE_LABELS = ["Incident", "Query"];

/** Resolves case type chip config from API label. Incident/Query show as "Case". */
export function getCaseTypeChipConfig(
  apiLabel: string | undefined | null,
): CaseTypeChipConfig | null {
  if (!apiLabel?.trim()) return null;
  const normalized = apiLabel.trim();
  const isCase = CASE_TYPE_LABELS.some(
    (l) => l.toLowerCase() === normalized.toLowerCase(),
  );
  if (isCase) {
    return {
      displayLabel: "Case",
      Icon: FileText,
      bgColor: colors.orange?.[100] ?? "#FFF7ED",
      textColor: colors.orange?.[800] ?? "#9A3412",
      borderColor: colors.orange?.[200] ?? "#FED7AA",
    };
  }
  if (/security\s*report\s*analysis/i.test(normalized)) {
    return {
      displayLabel: "Security Report Analysis",
      Icon: Shield,
      bgColor: colors.purple?.[100] ?? "#F3E8FF",
      textColor: colors.purple?.[800] ?? "#6B21A8",
      borderColor: colors.purple?.[200] ?? "#E9D5FF",
    };
  }
  if (/service\s*request/i.test(normalized)) {
    return {
      displayLabel: "Service Request",
      Icon: Server,
      bgColor: colors.yellow?.[100] ?? "#FEF9C3",
      textColor: colors.yellow?.[800] ?? "#854D0E",
      borderColor: colors.yellow?.[200] ?? "#FDE68A",
    };
  }
  if (/change\s*request/i.test(normalized)) {
    return {
      displayLabel: "Change Request",
      Icon: CalendarDays,
      bgColor: colors.blue?.[100] ?? "#DBEAFE",
      textColor: colors.blue?.[800] ?? "#1E40AF",
      borderColor: colors.blue?.[200] ?? "#BFDBFE",
    };
  }
  return {
    displayLabel: normalized,
    Icon: FileText,
    bgColor: colors.grey?.[100] ?? "#F3F4F6",
    textColor: colors.grey?.[800] ?? "#1F2937",
    borderColor: colors.grey?.[200] ?? "#E5E7EB",
  };
}
