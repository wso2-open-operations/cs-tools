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

import type { JSX, ReactNode } from "react";
import type { CaseMetadataResponse } from "@features/support/types/cases";
import type { ProjectDeploymentItem } from "@features/project-details/types/deployments";
import type { ProjectContact } from "@features/settings/types/users";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListFilters from "@components/list-view/ListFilters";
import { countListSearchAndFilters } from "@features/support/utils/support";

export interface ListSearchPanelProps {
  searchTerm: string;
  searchPlaceholder?: string;
  onSearchChange: (value: string) => void;
  isFiltersOpen: boolean;
  onFiltersToggle: () => void;
  filters: {
    statusIds?: string[];
    severityIds?: string[];
    issueTypes?: string[];
    deploymentIds?: string[];
    createdBy?: string[];
    startCreatedDate?: string;
    endCreatedDate?: string;
    startUpdatedDate?: string;
    endUpdatedDate?: string;
  };
  filterMetadata: CaseMetadataResponse | undefined;
  deployments?: ProjectDeploymentItem[];
  contacts?: ProjectContact[];
  isContactsLoading?: boolean;
  onFilterChange: (field: string, value: string | string[]) => void;
  onClearFilters: () => void;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  hideSeverityFilter?: boolean;
  hideStatusFilter?: boolean;
  hideDeploymentFilter?: boolean;
  hideCategoryFilter?: boolean;
  hideCreatedByFilter?: boolean;
  hideFiltersButton?: boolean;
  hideDateFilters?: boolean;
  isProjectContextLoading?: boolean;
  onLoadMoreDeployments?: () => void;
  hasMoreDeployments?: boolean;
  isFetchingMoreDeployments?: boolean;
  excludeFromCount?: string[];
  actionsBeforeClearFilters?: ReactNode;
}

/**
 * ListSearchPanel wraps ListSearchBar with the ListFilters panel.
 *
 * @param {ListSearchPanelProps} props - Search and filter props.
 * @returns {JSX.Element} The rendered search bar.
 */
export default function ListSearchPanel({
  searchTerm,
  searchPlaceholder = "Search cases by ID, title, or description...",
  onSearchChange,
  isFiltersOpen,
  onFiltersToggle,
  filters,
  filterMetadata,
  deployments,
  contacts,
  isContactsLoading = false,
  onFilterChange,
  onClearFilters,
  excludeS0 = false,
  restrictSeverityToLow = false,
  hideSeverityFilter = false,
  hideStatusFilter = false,
  hideDeploymentFilter = false,
  hideCategoryFilter = false,
  hideCreatedByFilter = false,
  hideFiltersButton = false,
  hideDateFilters = false,
  isProjectContextLoading = false,
  onLoadMoreDeployments,
  hasMoreDeployments = false,
  isFetchingMoreDeployments = false,
  excludeFromCount = [],
  actionsBeforeClearFilters,
}: ListSearchPanelProps): JSX.Element {
  const excluded = Object.fromEntries(excludeFromCount.map((k) => [k, undefined]));
  const filtersForCount = {
    ...filters,
    ...(hideSeverityFilter ? { severityIds: undefined } : {}),
    ...(hideStatusFilter ? { statusIds: undefined } : {}),
    ...(hideDeploymentFilter ? { deploymentIds: undefined } : {}),
    ...(hideCategoryFilter ? { issueTypes: undefined } : {}),
    ...(hideCreatedByFilter ? { createdBy: undefined } : {}),
    // Collapse each date range pair into a single "active" sentinel so each pair counts as 1.
    startCreatedDate: undefined,
    endCreatedDate: undefined,
    startUpdatedDate: undefined,
    endUpdatedDate: undefined,
    ...(!hideDateFilters
      ? {
          _createdDateRange:
            filters.startCreatedDate || filters.endCreatedDate
              ? "active"
              : undefined,
          _updatedDateRange:
            filters.startUpdatedDate || filters.endUpdatedDate
              ? "active"
              : undefined,
        }
      : {}),
    ...excluded,
  };
  return (
    <ListSearchBar
      searchPlaceholder={searchPlaceholder}
      searchTerm={searchTerm}
      onSearchChange={onSearchChange}
      isFiltersOpen={isFiltersOpen}
      onFiltersToggle={onFiltersToggle}
      activeFiltersCount={countListSearchAndFilters(
        "",
        filtersForCount,
      )}
      onClearFilters={onClearFilters}
      hideFiltersButton={hideFiltersButton}
      isLoading={isProjectContextLoading}
      actionsBeforeClearFilters={actionsBeforeClearFilters}
      filtersContent={
        <ListFilters
          filters={filters}
          filterMetadata={filterMetadata}
          deployments={deployments}
          contacts={contacts}
          isContactsLoading={isContactsLoading}
          onFilterChange={onFilterChange}
          excludeS0={excludeS0}
          restrictSeverityToLow={restrictSeverityToLow}
          hideSeverityFilter={hideSeverityFilter}
          hideStatusFilter={hideStatusFilter}
          hideDeploymentFilter={hideDeploymentFilter}
          hideCategoryFilter={hideCategoryFilter}
          hideCreatedByFilter={hideCreatedByFilter}
          hideDateFilters={hideDateFilters}
          onLoadMoreDeployments={onLoadMoreDeployments}
          hasMoreDeployments={hasMoreDeployments}
          isFetchingMoreDeployments={isFetchingMoreDeployments}
        />
      }
    />
  );
}
