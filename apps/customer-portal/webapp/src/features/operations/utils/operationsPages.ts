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
import { ChangeRequestStates } from "@features/operations/constants/operationsConstants";
import type { MetadataItem, SortOrder } from "@/types/common";

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

/** Labels excluded from customer-facing "allowed" states (internal pre-approval workflow). */
const EXCLUDED_ALLOWED_CR_STATE_LABELS = new Set<string>([
  ChangeRequestStates.NEW,
  ChangeRequestStates.ASSESS,
  ChangeRequestStates.AUTHORIZE,
]);

/** Labels of CR states excluded from the "outstanding" view. */
const EXCLUDED_OUTSTANDING_CR_STATE_LABELS = new Set<string>([
  ChangeRequestStates.ROLLBACK,
  ChangeRequestStates.CLOSED,
  ChangeRequestStates.CANCELED,
]);

/** Labels of CR states in the "action required" group. */
const ACTION_REQUIRED_CR_STATE_LABELS = new Set<string>([
  ChangeRequestStates.CUSTOMER_APPROVAL,
  ChangeRequestStates.CUSTOMER_REVIEW,
]);

/** Labels of CR states in the "scheduled" group. */
const SCHEDULED_CR_STATE_LABELS = new Set<string>([
  ChangeRequestStates.SCHEDULED,
]);

/** Labels of CR states in the "closed" group. */
const CLOSED_CR_STATE_LABELS = new Set<string>([
  ChangeRequestStates.CLOSED,
]);

/**
 * Derives all customer-visible CR state IDs from the project filters response by excluding
 * internal pre-approval workflow states (New, Assess, Authorize).
 *
 * @param changeRequestStates - `changeRequestStates` array from `useGetProjectFilters`.
 * @returns Array of numeric state IDs, or `undefined` if metadata is not yet loaded.
 */
export function resolveAllowedCrStateIds(
  changeRequestStates: MetadataItem[] | undefined,
): number[] | undefined {
  if (!changeRequestStates) return undefined;
  return changeRequestStates
    .filter((s) => !EXCLUDED_ALLOWED_CR_STATE_LABELS.has(s.label))
    .map((s) => Number(s.id));
}

/**
 * Derives outstanding CR state IDs from the project filters response by excluding
 * states labeled "Rollback", "Closed", and "Canceled".
 *
 * @param changeRequestStates - `changeRequestStates` array from `useGetProjectFilters`.
 * @returns Array of numeric state IDs, or `undefined` if metadata is not yet loaded.
 */
export function resolveOutstandingCrStateIds(
  changeRequestStates: MetadataItem[] | undefined,
): number[] | undefined {
  if (!changeRequestStates) return undefined;
  return changeRequestStates
    .filter((s) => !EXCLUDED_OUTSTANDING_CR_STATE_LABELS.has(s.label))
    .map((s) => Number(s.id));
}

/**
 * Derives action-required CR state IDs (Customer Approval + Customer Review) from filter metadata.
 *
 * @param changeRequestStates - `changeRequestStates` array from `useGetProjectFilters`.
 * @returns Array of numeric state IDs, or `undefined` if metadata is not yet loaded.
 */
export function resolveActionRequiredCrStateIds(
  changeRequestStates: MetadataItem[] | undefined,
): number[] | undefined {
  if (!changeRequestStates) return undefined;
  return changeRequestStates
    .filter((s) => ACTION_REQUIRED_CR_STATE_LABELS.has(s.label))
    .map((s) => Number(s.id));
}

/**
 * Derives scheduled CR state IDs (Scheduled) from filter metadata.
 *
 * @param changeRequestStates - `changeRequestStates` array from `useGetProjectFilters`.
 * @returns Array of numeric state IDs, or `undefined` if metadata is not yet loaded.
 */
export function resolveScheduledCrStateIds(
  changeRequestStates: MetadataItem[] | undefined,
): number[] | undefined {
  if (!changeRequestStates) return undefined;
  return changeRequestStates
    .filter((s) => SCHEDULED_CR_STATE_LABELS.has(s.label))
    .map((s) => Number(s.id));
}

/**
 * Derives closed CR state IDs (Closed) from filter metadata.
 *
 * @param changeRequestStates - `changeRequestStates` array from `useGetProjectFilters`.
 * @returns Array of numeric state IDs, or `undefined` if metadata is not yet loaded.
 */
export function resolveClosedCrStateIds(
  changeRequestStates: MetadataItem[] | undefined,
): number[] | undefined {
  if (!changeRequestStates) return undefined;
  return changeRequestStates
    .filter((s) => CLOSED_CR_STATE_LABELS.has(s.label))
    .map((s) => Number(s.id));
}

/**
 * Builds the change-request search payload for the list/calendar/export flows.
 * All CR state IDs are derived from `changeRequestStates` filter metadata using
 * label-based resolution — no hardcoded numeric IDs.
 *
 * @param filters - UI filter state.
 * @param searchTerm - Raw search string.
 * @param outstandingOnly - Restrict to outstanding states.
 * @param actionRequired - Restrict to action-required states.
 * @param scheduledOnly - Restrict to scheduled state.
 * @param changeRequestStates - Raw states metadata from `useGetProjectFilters`.
 * @returns Request body without pagination.
 */
export function buildChangeRequestSearchRequest(
  filters: ChangeRequestFilterValues,
  searchTerm: string,
  outstandingOnly: boolean = false,
  actionRequired: boolean = false,
  scheduledOnly: boolean = false,
  changeRequestStates?: MetadataItem[],
): Omit<ChangeRequestSearchRequest, "pagination"> {
  const selectedStateId = filters.stateId ? Number(filters.stateId) : undefined;
  const resolvedIds = actionRequired
    ? resolveActionRequiredCrStateIds(changeRequestStates)
    : scheduledOnly
      ? resolveScheduledCrStateIds(changeRequestStates)
      : outstandingOnly
        ? resolveOutstandingCrStateIds(changeRequestStates)
        : resolveAllowedCrStateIds(changeRequestStates);
  const allowedStateIds = resolvedIds ?? [];
  const stateKeys =
    selectedStateId === undefined
      ? allowedStateIds
      : allowedStateIds.includes(selectedStateId)
        ? [selectedStateId]
        : [];

  return {
    filters: {
      searchQuery: searchTerm.trim() || undefined,
      stateKeys,
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
  outstandingStatusIds?: number[],
): Omit<CaseSearchRequest, "pagination"> {
  const normalizedSortField =
    sortField === ServiceRequestCaseSortField.Severity
      ? ServiceRequestCaseSortField.CreatedOn
      : sortField;

  const resolvedStatusIds: number[] | undefined = filters.statusId
    ? [Number(filters.statusId)]
    : outstandingStatusIds?.length
      ? outstandingStatusIds
      : undefined;

  return {
    filters: {
      caseTypes: [CaseType.SERVICE_REQUEST],
      statusIds: resolvedStatusIds,
      issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
      deploymentId: filters.deploymentId || undefined,
      searchQuery: searchTerm.trim() || undefined,
      createdByMe: createdByMe || undefined,
    },
    sortBy: {
      field: normalizedSortField,
      order: sortOrder,
    },
  };
}
