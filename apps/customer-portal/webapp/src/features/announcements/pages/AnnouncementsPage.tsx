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

import { useParams, useNavigate } from "react-router";
import { useState, useMemo, type JSX, type ChangeEvent } from "react";
import { Stack } from "@wso2/oxygen-ui";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import type { AnnouncementFilterValues } from "@features/support/constants/supportConstants";
import { ANNOUNCEMENT_FILTER_DEFINITIONS } from "@features/support/constants/supportConstants";
import AnnouncementList from "@features/announcements/components/AnnouncementList";
import {
  hasListSearchOrFilters,
  countListSearchAndFilters,
} from "@features/support/utils/support";
import { SortOrder } from "@/types/common";
import ListPageHeader from "@components/list-view/ListPageHeader";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListFiltersPanel from "@components/list-view/ListFiltersPanel";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import { AnnouncementSortField } from "@features/announcements/types/announcements";
import {
  ANNOUNCEMENTS_BACK_LABEL,
  ANNOUNCEMENTS_PAGE_DESCRIPTION,
  ANNOUNCEMENTS_PAGE_SIZE,
  ANNOUNCEMENTS_PAGE_TITLE,
  ANNOUNCEMENTS_SEARCH_PLACEHOLDER,
  ANNOUNCEMENTS_LIST_ENTITY_LABEL,
  ANNOUNCEMENTS_SORT_FIELD_OPTIONS,
} from "@features/announcements/constants/announcementsConstants";
import {
  buildAnnouncementCaseSearchRequest,
  getAnnouncementTotalPages,
  resolveAnnouncementListFilterOptions,
} from "@features/announcements/utils/announcements";

/**
 * AnnouncementsPage component to display announcements with search, filters, and pagination.
 *
 * @returns {JSX.Element} The rendered Announcements page.
 */
export default function AnnouncementsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AnnouncementFilterValues>({});
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const pageSize = ANNOUNCEMENTS_PAGE_SIZE;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const caseSearchRequest = useMemo(
    () =>
      buildAnnouncementCaseSearchRequest(filters, searchTerm, sortOrder),
    [filters, searchTerm, sortOrder],
  );

  const offset = (page - 1) * pageSize;

  const { data, isLoading: isCasesQueryLoading } = useGetProjectCasesPage(
    projectId || "",
    caseSearchRequest,
    offset,
    pageSize,
  );

  const cases = data?.cases ?? [];
  const totalRecords = data?.totalRecords ?? 0;
  const totalPages = getAnnouncementTotalPages(totalRecords, pageSize);

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const handleFilterChange = (field: string, value: string) => {
    setFilters((prev) => ({ ...prev, [field]: value || undefined }));
    setPage(1);
  };

  const handleClearFilters = () => {
    setFilters({});
    setSearchTerm("");
    setPage(1);
  };

  const listHasRefinement = hasListSearchOrFilters(searchTerm, filters);

  return (
    <Stack spacing={3}>
      <ListPageHeader
        title={ANNOUNCEMENTS_PAGE_TITLE}
        description={ANNOUNCEMENTS_PAGE_DESCRIPTION}
        backLabel={ANNOUNCEMENTS_BACK_LABEL}
        onBack={() => navigate("..")}
      />

      <ListSearchBar
        searchPlaceholder={ANNOUNCEMENTS_SEARCH_PLACEHOLDER}
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        activeFiltersCount={countListSearchAndFilters(searchTerm, filters)}
        onClearFilters={handleClearFilters}
        filtersContent={
          <ListFiltersPanel
            filterDefinitions={ANNOUNCEMENT_FILTER_DEFINITIONS}
            filters={filters}
            resolveOptions={(def) =>
              resolveAnnouncementListFilterOptions(def, filterMetadata)
            }
            onFilterChange={handleFilterChange}
          />
        }
      />

      <ListResultsBar
        shownCount={cases.length}
        totalCount={totalRecords}
        entityLabel={ANNOUNCEMENTS_LIST_ENTITY_LABEL}
        sortFieldOptions={ANNOUNCEMENTS_SORT_FIELD_OPTIONS}
        sortField={AnnouncementSortField.CreatedOn}
        onSortFieldChange={() => undefined}
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
      />

      <AnnouncementList
        cases={cases}
        isLoading={isCasesQueryLoading}
        hasListRefinement={listHasRefinement}
        onCaseClick={(c) =>
          navigate(`/projects/${projectId}/announcements/${c.id}`)
        }
      />

      <ListPagination
        totalPages={totalPages}
        page={page}
        onChange={handlePageChange}
      />
    </Stack>
  );
}
