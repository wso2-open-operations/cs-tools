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

import { resolveCasesTableDefaultStatusIds } from "@features/dashboard/utils/casesTable";
import type { MetadataItem } from "@/types/common";
import type { CaseSearchFilters } from "@features/support/types/cases";

const DASHBOARD_SEVERITY_ID_TO_LABEL: Record<string, string> = {
  "14": "S0",
  "10": "S1",
  "11": "S2",
  "12": "S3",
  "13": "S4",
};

/**
 * Resolves dashboard severity heading label from query param severity id.
 *
 * @param severityId - Severity ID from URL query string.
 * @returns {string | undefined} Severity heading label (S0-S4) if known.
 */
export function getDashboardSeverityHeadingLabel(
  severityId?: string | null,
): string | undefined {
  if (!severityId) {
    return undefined;
  }
  return DASHBOARD_SEVERITY_ID_TO_LABEL[severityId];
}

/**
 * Resolves All Cases dashboard-driven title text.
 *
 * @param severityId - Severity ID from query params.
 * @returns {string | undefined} Formatted title for severity-specific dashboard flow.
 */
export function getDashboardOutstandingCasesTitle(
  severityId?: string | null,
): string | undefined {
  const severityLabel = getDashboardSeverityHeadingLabel(severityId);
  if (!severityLabel) {
    return undefined;
  }
  return `Outstanding ${severityLabel} Cases`;
}

/**
 * Resolves dashboard-driven All Cases description text.
 *
 * @param severityId - Severity ID from query params.
 * @returns {string | undefined} Formatted description for severity-specific dashboard flow.
 */
export function getDashboardOutstandingCasesDescription(
  severityId?: string | null,
): string | undefined {
  const severityLabel = getDashboardSeverityHeadingLabel(severityId);
  if (!severityLabel) {
    return undefined;
  }
  return `Manage and track ${severityLabel} outstanding support cases`;
}

/**
 * Builds case search filters for dashboard-origin severity navigation.
 *
 * @param params - Source values and metadata for filter construction.
 * @returns {CaseSearchFilters} Normalized case search filters.
 */
export function buildDashboardCaseSearchFilters(params: {
  statusId?: string;
  severityId?: string;
  issueTypes?: string;
  deploymentId?: string;
  searchQuery?: string;
  createdByMe?: boolean;
  caseStates?: MetadataItem[];
  isDashboardSeverityNavigation?: boolean;
}): CaseSearchFilters {
  const {
    statusId,
    severityId,
    issueTypes,
    deploymentId,
    searchQuery,
    createdByMe,
    caseStates,
    isDashboardSeverityNavigation = false,
  } = params;

  const normalizedSearchQuery = searchQuery?.trim() || undefined;
  const explicitStatusId = statusId ? Number(statusId) : undefined;
  const normalizedSeverityId = severityId ? Number(severityId) : undefined;
  const normalizedIssueId = issueTypes ? Number(issueTypes) : undefined;

  switch (true) {
    case Boolean(explicitStatusId):
      return {
        statusIds: [explicitStatusId as number],
        severityId: normalizedSeverityId,
        issueId: normalizedIssueId,
        deploymentId,
        searchQuery: normalizedSearchQuery,
        createdByMe,
      };
    case isDashboardSeverityNavigation:
      return {
        statusIds: resolveCasesTableDefaultStatusIds(caseStates),
        severityId: normalizedSeverityId,
        issueId: normalizedIssueId,
        deploymentId,
        searchQuery: normalizedSearchQuery,
        createdByMe,
      };
    default:
      return {
        severityId: normalizedSeverityId,
        issueId: normalizedIssueId,
        deploymentId,
        searchQuery: normalizedSearchQuery,
        createdByMe,
      };
  }
}

