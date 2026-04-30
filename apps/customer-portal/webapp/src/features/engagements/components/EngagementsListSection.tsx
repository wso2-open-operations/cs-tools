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
import {
  Divider,
  FormControl,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ListItems from "@components/list-view/ListItems";
import { countListSearchAndFilters } from "@features/support/utils/support";
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
  hideFiltersButton = false,
  isStatFiltered = false,
  engagementTypeOptions = [],
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
  page,
  rowsPerPage,
  onPageChange,
  onRowsPerPageChange,
}: EngagementsListSectionProps): JSX.Element {
  const activeFiltersCount = countListSearchAndFilters(searchTerm, {
    statusId: filters.statusId,
    engagementTypeKey: filters.engagementTypeKey,
  });

  return (
    <>
      {isStatFiltered ? (
        <Divider />
      ) : (
        <ListSearchBar
          searchPlaceholder={ENGAGEMENTS_SEARCH_PLACEHOLDER}
          searchTerm={searchTerm}
          onSearchChange={onSearchChange}
          isFiltersOpen={isFiltersOpen}
          onFiltersToggle={onFiltersToggle}
          activeFiltersCount={activeFiltersCount}
          onClearFilters={onClearFilters}
          hideFiltersButton={hideFiltersButton}
          isLoading={isProjectContextLoading}
          filtersContent={
            <Grid container spacing={2} sx={{ mt: 1 }}>
              {filterMetadata?.caseStates && (
                <Grid size={{ xs: 12, sm: 6, md: 3 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel id="eng-status-label">Status</InputLabel>
                    <Select
                      labelId="eng-status-label"
                      value={filters.statusId ?? ""}
                      label="Status"
                      onChange={(e: SelectChangeEvent<string>) =>
                        onFilterChange("statusId", e.target.value)
                      }
                    >
                      <MenuItem value="">
                        <Typography variant="body2">All Statuses</Typography>
                      </MenuItem>
                      {filterMetadata.caseStates.map((s) => (
                        <MenuItem key={s.id} value={s.id}>
                          <Typography variant="body2">{s.label}</Typography>
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Grid>
              )}
              {engagementTypeOptions.length > 0 && (
                <Grid size={{ xs: 12, sm: 6, md: 3 }}>
                  <FormControl fullWidth size="small">
                    <InputLabel id="eng-type-label">Engagement Type</InputLabel>
                    <Select
                      labelId="eng-type-label"
                      value={filters.engagementTypeKey ?? ""}
                      label="Engagement Type"
                      onChange={(e: SelectChangeEvent<string>) =>
                        onFilterChange("engagementTypeKey", e.target.value)
                      }
                    >
                      <MenuItem value="">
                        <Typography variant="body2">All Types</Typography>
                      </MenuItem>
                      {engagementTypeOptions.map((opt) => (
                        <MenuItem key={opt.value} value={opt.value}>
                          <Typography variant="body2">{opt.label}</Typography>
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Grid>
              )}
            </Grid>
          }
        />
      )}

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
        totalRecords={totalItems}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={onPageChange}
        onRowsPerPageChange={onRowsPerPageChange}
      />
    </>
  );
}
