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
import { useLocation, useNavigate } from "react-router";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import type { EngagementsStatKey } from "@features/engagements/types/engagements";
import EngagementsListSection from "@features/engagements/components/EngagementsListSection";
import EngagementsStatCards from "@features/engagements/components/EngagementsStatCards";
import { useEngagementsPageState } from "@features/engagements/hooks/useEngagementsPageState";

const ENGAGEMENT_STAT_FILTER_INFO: Record<EngagementsStatKey, { title: string; subtitle: string }> = {
  total: { title: "All Engagements", subtitle: "All engagement cases" },
  active: { title: "Outstanding Engagements", subtitle: "Active engagements without closed state" },
  completed: { title: "Completed Engagements", subtitle: "Engagements in closed state" },
  onHold: { title: "On Hold Engagements", subtitle: "Engagements awaiting info or action" },
};

/**
 * Engagements list: stats, search, filters, sort, and paginated cases.
 *
 * @returns {JSX.Element} Engagements page.
 */
export default function EngagementsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

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
    rowsPerPage,
    paginatedCases,
    isCasesAreaLoading,
    isCasesError,
    listHasRefinement,
    totalItems,
    handlePageChange,
    handleRowsPerPageChange,
    handleFilterChange,
    handleClearFilters,
    handleSortChange,
    handleSortFieldUiChange,
    handleSearchChange,
    handleStatCardClick,
    isStatFiltered,
    activeStatKey,
    clearStatFilter,
    onCaseClick,
  } = useEngagementsPageState();

  return (
    <Stack spacing={3}>
      {(returnTo || isStatFiltered) && (
        <Box>
          <Button
            startIcon={<ArrowLeft size={16} />}
            onClick={() => {
              if (isStatFiltered) {
                clearStatFilter();
              } else if (returnTo) {
                navigate(returnTo);
              }
            }}
            variant="text"
          >
            Back
          </Button>
        </Box>
      )}
      {isStatFiltered ? (
        <Box>
          <Typography variant="h5" color="text.primary" sx={{ mb: 0.5 }}>
            {activeStatKey ? ENGAGEMENT_STAT_FILTER_INFO[activeStatKey].title : ""}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {activeStatKey ? ENGAGEMENT_STAT_FILTER_INFO[activeStatKey].subtitle : ""}
          </Typography>
        </Box>
      ) : (
        <EngagementsStatCards
          stats={stats}
          isLoading={isStatsLoading}
          isError={isStatsError}
          onStatClick={handleStatCardClick}
        />
      )}
      <EngagementsListSection
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
        hideFiltersButton={isStatFiltered}
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
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handlePageChange}
        onRowsPerPageChange={handleRowsPerPageChange}
      />
    </Stack>
  );
}
