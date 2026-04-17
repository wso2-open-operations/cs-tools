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
import { Stack } from "@wso2/oxygen-ui";
import EngagementsStatCards from "@features/engagements/components/EngagementsStatCards";
import EngagementsListSection from "@features/engagements/components/EngagementsListSection";
import { useEngagementsPageState } from "@features/engagements/hooks/useEngagementsPageState";

/**
 * Engagements list: stats, search, filters, sort, and paginated cases.
 *
 * @returns {JSX.Element} Engagements page.
 */
export default function EngagementsPage(): JSX.Element {
  const {
    projectReady,
    excludeS0,
    restrictSeverityToLow,
    filterMetadata,
    stats,
    isStatsLoading,
    isStatsError,
    searchTerm,
    isFiltersOpen,
    setIsFiltersOpen,
    filters,
    sortField,
    sortOrder,
    page,
    paginatedCases,
    isCasesAreaLoading,
    isCasesError,
    listHasRefinement,
    totalItems,
    totalPages,
    handlePageChange,
    handleFilterChange,
    handleClearFilters,
    handleSortChange,
    handleSortFieldUiChange,
    handleSearchChange,
    onCaseClick,
  } = useEngagementsPageState();

  return (
    <Stack spacing={3}>
      <EngagementsStatCards
        stats={stats}
        isLoading={isStatsLoading}
        isError={isStatsError}
      />

      <EngagementsListSection
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
        excludeS0={excludeS0}
        restrictSeverityToLow={restrictSeverityToLow}
        isProjectContextLoading={!projectReady}
        sortField={sortField}
        onSortFieldChange={handleSortFieldUiChange}
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
        paginatedCases={paginatedCases}
        isCasesAreaLoading={isCasesAreaLoading}
        isCasesError={isCasesError}
        listHasRefinement={listHasRefinement}
        totalItems={totalItems}
        onCaseClick={onCaseClick}
        totalPages={totalPages}
        page={page}
        onPageChange={handlePageChange}
      />
    </Stack>
  );
}
