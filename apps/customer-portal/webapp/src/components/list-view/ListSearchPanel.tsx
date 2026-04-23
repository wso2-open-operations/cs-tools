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

import type { JSX } from "react";
import type { CaseMetadataResponse } from "@features/support/types/cases";
import type { ProjectDeploymentItem } from "@features/project-details/types/deployments";
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
    statusId?: string;
    severityId?: string;
    issueTypes?: string;
    deploymentId?: string;
  };
  filterMetadata: CaseMetadataResponse | undefined;
  deployments?: ProjectDeploymentItem[];
  onFilterChange: (field: string, value: string) => void;
  onClearFilters: () => void;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  hideSeverityFilter?: boolean;
  hideDeploymentFilter?: boolean;
  isProjectContextLoading?: boolean;
  onLoadMoreDeployments?: () => void;
  hasMoreDeployments?: boolean;
  isFetchingMoreDeployments?: boolean;
  excludeFromCount?: string[];
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
  onFilterChange,
  onClearFilters,
  excludeS0 = false,
  restrictSeverityToLow = false,
  hideSeverityFilter = false,
  hideDeploymentFilter = false,
  isProjectContextLoading = false,
  onLoadMoreDeployments,
  hasMoreDeployments = false,
  isFetchingMoreDeployments = false,
  excludeFromCount = [],
}: ListSearchPanelProps): JSX.Element {
  const excluded = Object.fromEntries(excludeFromCount.map((k) => [k, undefined]));
  const filtersForCount = {
    ...filters,
    ...(hideSeverityFilter ? { severityId: undefined } : {}),
    ...(hideDeploymentFilter ? { deploymentId: undefined } : {}),
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
        searchTerm,
        filtersForCount,
      )}
      onClearFilters={onClearFilters}
      isLoading={isProjectContextLoading}
      filtersContent={
        <ListFilters
          filters={filters}
          filterMetadata={filterMetadata}
          deployments={deployments}
          onFilterChange={onFilterChange}
          excludeS0={excludeS0}
          restrictSeverityToLow={restrictSeverityToLow}
          hideSeverityFilter={hideSeverityFilter}
          hideDeploymentFilter={hideDeploymentFilter}
          onLoadMoreDeployments={onLoadMoreDeployments}
          hasMoreDeployments={hasMoreDeployments}
          isFetchingMoreDeployments={isFetchingMoreDeployments}
        />
      }
    />
  );
}
