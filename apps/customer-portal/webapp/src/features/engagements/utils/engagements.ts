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

import type { InfiniteData } from "@tanstack/react-query";
import { SortOrder } from "@/types/common";
import {
  CaseType,
  SUPPORT_STATE_AWAITING_INFO,
  SUPPORT_STATE_CLOSED,
  SUPPORT_STATE_WAITING_ON_WSO2,
} from "@features/support/constants/supportConstants";
import type {
  AllCasesFilterValues,
  CaseListItem,
  CaseSearchRequest,
  CaseSearchResponse,
  ProjectCasesStats,
} from "@features/support/types/cases";
import {
  EngagementsSortField,
  type EngagementsStatKey,
} from "@features/engagements/types/engagements";

/**
 * Maps list UI sort value to API field (guards unknown strings).
 *
 * @param value - Raw value from sort dropdown.
 * @returns {EngagementsSortField} Valid sort field.
 */
export function parseEngagementsSortField(value: string): EngagementsSortField {
  switch (value) {
    case EngagementsSortField.CreatedOn:
    case EngagementsSortField.UpdatedOn:
    case EngagementsSortField.State:
      return value;
    case EngagementsSortField.Severity:
      return EngagementsSortField.CreatedOn;
    default:
      return EngagementsSortField.CreatedOn;
  }
}

/**
 * Builds the engagements case search payload (without pagination).
 *
 * @param filters - Filter bar state.
 * @param searchTerm - Free-text search.
 * @param sortField - Sort column.
 * @param sortOrder - Ascending or descending.
 * @returns {Omit<CaseSearchRequest, "pagination">} Body for `useGetProjectCases`.
 */
export function buildEngagementSearchRequest(
  filters: AllCasesFilterValues,
  searchTerm: string,
  sortField: EngagementsSortField,
  sortOrder: SortOrder,
): Omit<CaseSearchRequest, "pagination"> {
  const normalizedSortField =
    sortField === EngagementsSortField.Severity
      ? EngagementsSortField.CreatedOn
      : sortField;

  return {
    filters: {
      caseTypes: [CaseType.ENGAGEMENT],
      statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
      issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
      deploymentId: filters.deploymentId || undefined,
      searchQuery: searchTerm.trim() || undefined,
    },
    sortBy: {
      field: normalizedSortField,
      order: sortOrder,
    },
  };
}

/**
 * Whether stats cards should show loading (query in flight or no payload yet).
 */
export function computeEngagementsStatsLoading(
  isStatsQueryLoading: boolean,
  hasStatsResponse: boolean,
  projectId: string | undefined,
): boolean {
  switch (true) {
    case Boolean(isStatsQueryLoading):
    case Boolean(projectId && !hasStatsResponse):
      return true;
    default:
      return false;
  }
}

/**
 * Whether the cases list area should show loading.
 */
export function computeEngagementsCasesAreaLoading(
  isCasesQueryLoading: boolean,
  hasCasesResponse: boolean,
  projectId: string | undefined,
): boolean {
  switch (true) {
    case Boolean(isCasesQueryLoading):
    case Boolean(projectId && !hasCasesResponse):
      return true;
    default:
      return false;
  }
}

/**
 * Whether the global loader should run (stats + list area).
 */
export function computeEngagementsInitialPageLoading(
  isStatsLoading: boolean,
  isCasesAreaLoading: boolean,
): boolean {
  switch (true) {
    case isStatsLoading:
    case isCasesAreaLoading:
      return true;
    default:
      return false;
  }
}

/**
 * Returns cases for the current page from infinite-query pages (1-based `page`).
 *
 * @param data - Infinite query data.
 * @param page - Current page (1-based).
 * @returns {CaseListItem[]} Rows for that page.
 */
export function getEngagementsCurrentPageCases(
  data: InfiniteData<CaseSearchResponse> | undefined,
  page: number,
): CaseListItem[] {
  if (!data || data.pages.length === 0) return [];
  const requestedPageIndex = page - 1;
  if (requestedPageIndex < 0 || requestedPageIndex >= data.pages.length) {
    return [];
  }
  return data.pages[requestedPageIndex]?.cases ?? [];
}

/**
 * Total for pagination (prefer API total when present).
 */
export function computeEngagementsTotalItems(
  apiTotalRecords: number,
  filteredRowCount: number,
): number {
  switch (true) {
    case apiTotalRecords > 0:
      return apiTotalRecords;
    default:
      return filteredRowCount;
  }
}

/**
 * Total pages from item count and page size.
 *
 * @param totalItems - Total rows.
 * @param pageSize - Rows per page.
 * @returns {number} Page count (minimum 0).
 */
export function computeEngagementsTotalPages(
  totalItems: number,
  pageSize: number,
): number {
  return Math.ceil(totalItems / pageSize);
}

/**
 * Route path for a single engagement case under a project.
 *
 * @param projectId - Project id.
 * @param caseId - Case id.
 * @returns {string} App path.
 */
export function buildEngagementDetailPath(
  projectId: string,
  caseId: string,
): string {
  return `/projects/${projectId}/engagements/${caseId}`;
}

/**
 * Maps {@link ProjectCasesStats} for engagement stat cards (total, active, completed, on hold).
 *
 * @param stats - Project stats for engagement case type.
 * @returns {Partial<Record<EngagementsStatKey, number>>} Values for stat grid.
 */
export function computeEngagementsStatValues(
  stats: ProjectCasesStats | undefined,
): Partial<Record<EngagementsStatKey, number>> {
  if (!stats) {
    return {};
  }
  const completed =
    stats.stateCount?.find((s) => s.label === SUPPORT_STATE_CLOSED)?.count ?? 0;
  const onHold =
    (stats.stateCount?.find((s) => s.label === SUPPORT_STATE_AWAITING_INFO)
      ?.count ?? 0) +
    (stats.stateCount?.find((s) => s.label === SUPPORT_STATE_WAITING_ON_WSO2)
      ?.count ?? 0);
  return {
    total: stats.totalCount ?? 0,
    active: stats.activeCount ?? 0,
    completed,
    onHold,
  };
}
