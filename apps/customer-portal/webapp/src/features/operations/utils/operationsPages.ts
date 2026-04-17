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

import type { FilterDefinition } from "@components/list-view/ListFiltersPanel";
import { CaseType } from "@features/support/constants/supportConstants";
import type { AllCasesFilterValues } from "@features/support/types/cases";
import type { CaseMetadataResponse } from "@features/support/types/cases";
import type { CaseSearchRequest } from "@features/support/types/cases";
import { formatImpactLabel } from "@features/operations/utils/changeRequestUi";
import type {
  ChangeRequestFilterValues,
  ChangeRequestItem,
  ChangeRequestSearchRequest,
} from "@features/operations/types/changeRequests";
import {
  ChangeRequestFilterDefinitionId,
  type ChangeRequestFilterOption,
} from "@features/operations/types/changeRequests";
import {
  OperationsNavSegment,
  ServiceRequestCaseSortField,
} from "@features/operations/types/serviceRequests";
import type { SortOrder } from "@/types/common";

/**
 * Resolves `operations` vs `support` from the current pathname.
 *
 * @param pathname - `location.pathname`.
 * @returns Segment used in `/projects/:id/{segment}/...` routes.
 */
export function getOperationsNavSegment(pathname: string): OperationsNavSegment {
  return pathname.includes("/operations/")
    ? OperationsNavSegment.Operations
    : OperationsNavSegment.Support;
}

/**
 * Subtitle for the operations hub Service Requests overview card.
 *
 * @param limit - Max rows shown (e.g. {@link OPERATIONS_OVERVIEW_LIST_LIMIT}).
 * @returns e.g. `Latest 5 service requests`.
 */
export function formatOperationsOverviewServiceRequestsSubtitle(
  limit: number,
): string {
  return `Latest ${limit} service requests`;
}

/**
 * Subtitle for the operations hub Change Requests overview card.
 *
 * @param limit - Max rows shown.
 * @returns e.g. `Latest 5 change requests`.
 */
export function formatOperationsOverviewChangeRequestsSubtitle(
  limit: number,
): string {
  return `Latest ${limit} change requests`;
}

/**
 * Builds the change-request search payload for the list/calendar/export flows.
 *
 * @param filters - UI filter state.
 * @param searchTerm - Raw search string.
 * @returns Request body without pagination.
 */
export function buildChangeRequestSearchRequest(
  filters: ChangeRequestFilterValues,
  searchTerm: string,
): Omit<ChangeRequestSearchRequest, "pagination"> {
  return {
    filters: {
      searchQuery: searchTerm.trim() || undefined,
      stateKeys: filters.stateId ? [Number(filters.stateId)] : undefined,
      impactKey: filters.impactId ? Number(filters.impactId) : undefined,
    },
  };
}

/**
 * Maps project filter metadata into options for the change requests `ListFiltersPanel`.
 *
 * @param def - Filter definition (`CHANGE_REQUEST_FILTER_DEFINITIONS` entry).
 * @param filterMetadata - Project filters API payload.
 * @returns Select options (impact labels formatted).
 */
export function resolveChangeRequestFilterListOptions(
  def: FilterDefinition,
  filterMetadata: CaseMetadataResponse | undefined,
): ChangeRequestFilterOption[] {
  const raw = filterMetadata?.[def.metadataKey as keyof CaseMetadataResponse];
  if (!Array.isArray(raw)) return [];
  switch (def.id) {
    case ChangeRequestFilterDefinitionId.Impact:
      return raw.map((item: { label: string; id: string }) => ({
        label: formatImpactLabel(item.label),
        value: item.id,
      }));
    case ChangeRequestFilterDefinitionId.State:
    default:
      return raw.map((item: { label: string; id: string }) => ({
        label: item.label,
        value: item.id,
      }));
  }
}

/**
 * Flattens infinite-query pages into a single change request array.
 *
 * @param pages - Pages from `useGetChangeRequestsInfinite`.
 */
export function flattenChangeRequestInfinitePages(
  pages: Array<{ changeRequests: ChangeRequestItem[] }> | undefined,
): ChangeRequestItem[] {
  return pages?.flatMap((p) => p.changeRequests) ?? [];
}

/**
 * Builds the case search payload for the service requests list page.
 *
 * @param filters - All-cases filter state.
 * @param searchTerm - Raw search string.
 * @param sortField - Active sort field.
 * @param sortOrder - Sort direction.
 * @param createdByMe - From query `createdByMe=true`.
 * @returns Case search request without pagination.
 */
export function buildServiceRequestsPageCaseSearchRequest(
  filters: AllCasesFilterValues,
  searchTerm: string,
  sortField: ServiceRequestCaseSortField,
  sortOrder: SortOrder,
  createdByMe: boolean,
): Omit<CaseSearchRequest, "pagination"> {
  return {
    filters: {
      caseTypes: [CaseType.SERVICE_REQUEST],
      statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
      severityId: filters.severityId ? Number(filters.severityId) : undefined,
      issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
      deploymentId: filters.deploymentId || undefined,
      searchQuery: searchTerm.trim() || undefined,
      createdByMe: createdByMe || undefined,
    },
    sortBy: {
      field: sortField,
      order: sortOrder,
    },
  };
}
