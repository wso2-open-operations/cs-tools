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

import type { ChangeRequestStatsResponse } from "@features/operations/types/changeRequests";
import type { ProjectCasesStats } from "@features/support/types/cases";
import { colors } from "@wso2/oxygen-ui";
import {
  CalendarDays,
  FileText,
  Server,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import {
  CASE_CHIP_INCIDENT_QUERY_LABELS,
  CASE_TYPE_CHIP_DISPLAY_LABEL,
  S0_SEVERITY_LABELS,
  SEVERITY_ALT_TO_LEGEND_KEY,
  SEVERITY_FRIENDLY_LABEL,
  SEVERITY_LEGEND_ORDER,
} from "@/features/dashboard/constants/dashboard";
import {
  CaseTypeChipKind,
  SeverityLegendKey,
  type CaseTypeChipConfig,
  type SeverityLegendEntry,
} from "@features/dashboard/types/dashboard";
import type { NavigateFunction } from "react-router";
import { CASES_TABLE_CLEAR_FILTERS_LABEL } from "../constants/casesTable";

function resolveSeverityLegendEntry(
  lower: string,
): SeverityLegendEntry | undefined {
  const direct = SEVERITY_LEGEND_ORDER.find(
    (item) => item.label.toLowerCase() === lower,
  );
  if (direct) {
    return direct;
  }
  const altKey = SEVERITY_ALT_TO_LEGEND_KEY[lower];
  switch (altKey) {
    case SeverityLegendKey.Catastrophic:
    case SeverityLegendKey.Critical:
    case SeverityLegendKey.High:
    case SeverityLegendKey.Medium:
    case SeverityLegendKey.Low:
      return SEVERITY_LEGEND_ORDER.find((item) => item.key === altKey);
    default:
      return undefined;
  }
}

/** Returns true if the severity label corresponds to S0 (Catastrophic). */
export function isS0SeverityLabel(label?: string | null): boolean {
  if (!label?.trim()) return false;
  const trimmed = label.trim().toLowerCase();
  switch (trimmed) {
    case S0_SEVERITY_LABELS[0].toLowerCase():
    case S0_SEVERITY_LABELS[1].toLowerCase():
    case S0_SEVERITY_LABELS[2].toLowerCase():
      return true;
    default:
      return false;
  }
}

/**
 * Returns the friendly severity label for display in chips (e.g. "Critical", "High").
 *
 * @param label - API severity label (e.g. "1 - Critical", "High (P2)").
 * @returns {string} Friendly label or original.
 */
export function getSeverityFriendlyLabel(label?: string): string {
  if (!label?.trim()) return "--";
  const trimmed = label.trim().toLowerCase();
  const entry = resolveSeverityLegendEntry(trimmed);
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
  const lower = label.trim().toLowerCase();
  const entry = resolveSeverityLegendEntry(lower);
  return entry?.color ?? colors.grey?.[500] ?? "#6B7280";
}

function classifyCaseTypeChipKind(normalized: string): CaseTypeChipKind {
  const lower = normalized.toLowerCase();
  switch (true) {
    case CASE_CHIP_INCIDENT_QUERY_LABELS.some((l) => l.toLowerCase() === lower):
      return CaseTypeChipKind.IncidentOrQuery;
    case /security\s*report\s*analysis/i.test(normalized):
      return CaseTypeChipKind.SecurityReportAnalysis;
    case /service\s*request/i.test(normalized):
      return CaseTypeChipKind.ServiceRequest;
    case /change\s*request/i.test(normalized):
      return CaseTypeChipKind.ChangeRequest;
    default:
      return CaseTypeChipKind.Fallback;
  }
}

/** Resolves case type chip config from API label. Incident/Query show as "Case". */
export function getCaseTypeChipConfig(
  apiLabel: string | undefined | null,
): CaseTypeChipConfig | null {
  if (!apiLabel?.trim()) return null;
  const normalized = apiLabel.trim();
  switch (classifyCaseTypeChipKind(normalized)) {
    case CaseTypeChipKind.IncidentOrQuery:
      return {
        displayLabel:
          CASE_TYPE_CHIP_DISPLAY_LABEL[CaseTypeChipKind.IncidentOrQuery],
        Icon: FileText,
        bgColor: colors.orange?.[100] ?? "#FFF7ED",
        textColor: colors.orange?.[800] ?? "#9A3412",
        borderColor: colors.orange?.[200] ?? "#FED7AA",
      };
    case CaseTypeChipKind.SecurityReportAnalysis:
      return {
        displayLabel:
          CASE_TYPE_CHIP_DISPLAY_LABEL[CaseTypeChipKind.SecurityReportAnalysis],
        Icon: Shield,
        bgColor: colors.purple?.[100] ?? "#F3E8FF",
        textColor: colors.purple?.[800] ?? "#6B21A8",
        borderColor: colors.purple?.[200] ?? "#E9D5FF",
      };
    case CaseTypeChipKind.ServiceRequest:
      return {
        displayLabel:
          CASE_TYPE_CHIP_DISPLAY_LABEL[CaseTypeChipKind.ServiceRequest],
        Icon: Server,
        bgColor: colors.yellow?.[100] ?? "#FEF9C3",
        textColor: colors.yellow?.[800] ?? "#854D0E",
        borderColor: colors.yellow?.[200] ?? "#FDE68A",
      };
    case CaseTypeChipKind.ChangeRequest:
      return {
        displayLabel:
          CASE_TYPE_CHIP_DISPLAY_LABEL[CaseTypeChipKind.ChangeRequest],
        Icon: CalendarDays,
        bgColor: colors.blue?.[100] ?? "#DBEAFE",
        textColor: colors.blue?.[800] ?? "#1E40AF",
        borderColor: colors.blue?.[200] ?? "#BFDBFE",
      };
    default:
      return {
        displayLabel: normalized,
        Icon: FileText,
        bgColor: colors.grey?.[100] ?? "#F3F4F6",
        textColor: colors.grey?.[800] ?? "#1F2937",
        borderColor: colors.grey?.[200] ?? "#E5E7EB",
      };
  }
}

/**
 * Whether the combined cases + change-request dashboard card is still loading data.
 */
export function computeCrCardIsCardLoading(
  includeCrStats: boolean,
  combinedCasesStats: ProjectCasesStats | undefined,
  changeRequestStats: ChangeRequestStatsResponse | undefined,
  isCombinedCasesLoading: boolean,
  isChangeRequestStatsLoading: boolean,
  isErrorCombinedCases: boolean,
  isErrorChangeRequestStats: boolean,
): boolean {
  switch (includeCrStats) {
    case true:
      return (
        !isErrorCombinedCases &&
        !isErrorChangeRequestStats &&
        ((!combinedCasesStats && isCombinedCasesLoading) ||
          (!changeRequestStats && isChangeRequestStatsLoading))
      );
    case false:
      return (
        !isErrorCombinedCases && isCombinedCasesLoading && !combinedCasesStats
      );
    default:
      return false;
  }
}

/**
 * Whether the combined cases + change-request dashboard card is in an error state.
 */
export function computeCrCardIsCardError(
  includeCrStats: boolean,
  isCardLoading: boolean,
  combinedCasesStats: ProjectCasesStats | undefined,
  changeRequestStats: ChangeRequestStatsResponse | undefined,
  isErrorCombinedCases: boolean,
  isErrorChangeRequestStats: boolean,
): boolean {
  switch (includeCrStats) {
    case true:
      return (
        !isCardLoading &&
        (isErrorCombinedCases ||
          isErrorChangeRequestStats ||
          !combinedCasesStats ||
          !changeRequestStats)
      );
    case false:
      return !isCardLoading && (isErrorCombinedCases || !combinedCasesStats);
    default:
      return false;
  }
}

export function normalizeEngagementLabel(label: string): string {
  return label;
}

export function getDashboardChartsLoadingState(params: {
  isDashboardLoading: boolean;
  isDefaultCaseLoading: boolean;
  showOpsChart: boolean;
  isServiceRequestLoading: boolean;
  isEngagementLoading: boolean;
  includeCrStats: boolean;
  isChangeRequestStatsLoading: boolean;
  includeEngagementStats?: boolean;
}): boolean {
  const {
    isDashboardLoading,
    isDefaultCaseLoading,
    showOpsChart,
    isServiceRequestLoading,
    isEngagementLoading,
    includeCrStats,
    isChangeRequestStatsLoading,
    includeEngagementStats = true,
  } = params;

  if (isDashboardLoading || isDefaultCaseLoading) {
    return true;
  }
  if (includeEngagementStats && isEngagementLoading) {
    return true;
  }
  if (showOpsChart && isServiceRequestLoading) {
    return true;
  }
  if (includeCrStats && isChangeRequestStatsLoading) {
    return true;
  }

  return false;
}

export function getAllCoreFailedState(params: {
  isErrorCombinedCases: boolean;
  isErrorDefaultCase: boolean;
  isErrorEngagement: boolean;
  showOpsChart: boolean;
  isErrorServiceRequest: boolean;
  includeCrStats: boolean;
  isErrorChangeRequestStats: boolean;
  includeEngagementStats?: boolean;
}): boolean {
  const {
    isErrorCombinedCases,
    isErrorDefaultCase,
    isErrorEngagement,
    showOpsChart,
    isErrorServiceRequest,
    includeCrStats,
    isErrorChangeRequestStats,
    includeEngagementStats = true,
  } = params;

  if (!isErrorCombinedCases || !isErrorDefaultCase) {
    return false;
  }
  if (includeEngagementStats && !isErrorEngagement) {
    return false;
  }
  if (showOpsChart && !isErrorServiceRequest) {
    return false;
  }
  if (includeCrStats && !isErrorChangeRequestStats) {
    return false;
  }

  return true;
}

export function formatCasesTableClearFiltersLabel(activeCount: number): string {
  return `${CASES_TABLE_CLEAR_FILTERS_LABEL} (${activeCount})`;
}

export function navigateToCreateCase(
  navigate: NavigateFunction,
  projectId: string,
  hasAgent: boolean,
): void {
  switch (hasAgent) {
    case true:
      navigate(`/projects/${projectId}/support/chat/describe-issue`);
      return;
    default:
      navigate(`/projects/${projectId}/support/chat/create-case`, {
        state: { skipChat: true },
      });
  }
}
