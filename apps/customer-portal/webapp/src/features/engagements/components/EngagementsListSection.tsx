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
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ListItems from "@components/list-view/ListItems";
import ListSearchPanel from "@components/list-view/ListSearchPanel";
import {
  ENGAGEMENTS_LIST_ENTITY_LABEL,
  ENGAGEMENTS_SEARCH_PLACEHOLDER,
  ENGAGEMENTS_SORT_OPTIONS,
} from "@/features/engagements/constants/engagements";
import type { EngagementsListSectionProps } from "@features/engagements/types/engagements";

/**
 * Search, filters, sort, list rows, and pagination for the engagements page.
 *
 * @param props - List section props.
 * @returns {JSX.Element} Engagements list stack.
 */
export default function EngagementsListSection({
  searchTerm,
  onSearchChange,
  isFiltersOpen,
  onFiltersToggle,
  filters,
  filterMetadata,
  onFilterChange,
  onClearFilters,
  excludeS0,
  restrictSeverityToLow,
  isProjectContextLoading,
  sortField,
  onSortFieldChange,
  sortOrder,
  onSortOrderChange,
  paginatedCases,
  isCasesAreaLoading,
  isCasesError,
  listHasRefinement,
  totalItems,
  onCaseClick,
  totalPages,
  page,
  onPageChange,
}: EngagementsListSectionProps): JSX.Element {
  return (
    <>
      <ListSearchPanel
        searchTerm={searchTerm}
        searchPlaceholder={ENGAGEMENTS_SEARCH_PLACEHOLDER}
        onSearchChange={onSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={onFiltersToggle}
        filters={filters}
        filterMetadata={filterMetadata}
        deployments={undefined}
        onFilterChange={onFilterChange}
        onClearFilters={onClearFilters}
        excludeS0={excludeS0}
        restrictSeverityToLow={restrictSeverityToLow}
        hideSeverityFilter
        isProjectContextLoading={isProjectContextLoading}
      />

      <ListResultsBar
        shownCount={paginatedCases.length}
        totalCount={totalItems}
        entityLabel={ENGAGEMENTS_LIST_ENTITY_LABEL}
        sortFieldOptions={[...ENGAGEMENTS_SORT_OPTIONS]}
        sortField={sortField}
        onSortFieldChange={onSortFieldChange}
        sortOrder={sortOrder}
        onSortOrderChange={onSortOrderChange}
      />

      <ListItems
        cases={paginatedCases}
        isLoading={isCasesAreaLoading}
        isError={isCasesError}
        hasListRefinement={listHasRefinement}
        entityName={ENGAGEMENTS_LIST_ENTITY_LABEL}
        onCaseClick={onCaseClick}
        hideSeverity
      />

      <ListPagination
        totalPages={totalPages}
        page={page}
        onChange={onPageChange}
      />
    </>
  );
}
