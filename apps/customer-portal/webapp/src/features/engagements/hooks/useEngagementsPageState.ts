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

import { useNavigate, useParams } from "react-router";
import { useState, useMemo, useEffect, type ChangeEvent } from "react";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { CaseType } from "@features/support/constants/supportConstants";
import { getProjectSeverityPolicy } from "@utils/permission";
import { isS0Case } from "@features/support/utils/support";
import { hasListSearchOrFilters } from "@features/support/utils/support";
import type { AllCasesFilterValues } from "@features/support/types/cases";
import { SortOrder } from "@/types/common";
import { ENGAGEMENTS_PAGE_SIZE } from "@/features/engagements/constants/engagements";
import { EngagementsSortField } from "@features/engagements/types/engagements";
import {
  buildEngagementSearchRequest,
  buildEngagementDetailPath,
  computeEngagementsCasesAreaLoading,
  computeEngagementsInitialPageLoading,
  computeEngagementsStatsLoading,
  computeEngagementsTotalItems,
  computeEngagementsTotalPages,
  getEngagementsCurrentPageCases,
  parseEngagementsSortField,
} from "@features/engagements/utils/engagements";

/**
 * State, data fetching, and handlers for {@link EngagementsPage}.
 *
 * @returns Navigation, filters, pagination, loading flags, and case rows for the engagements list.
 */
export function useEngagementsPageState() {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortField, setSortField] = useState<EngagementsSortField>(
    EngagementsSortField.CreatedOn,
  );
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const projectReady = !isProjectLoading && project !== undefined;
  const severityPolicy = projectReady
    ? getProjectSeverityPolicy(project?.type?.label)
    : { excludeS0: false, restrictSeverityToLow: false };
  const { excludeS0, restrictSeverityToLow } = severityPolicy;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const {
    data: stats,
    isLoading: isStatsQueryLoading,
    isError: isStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.ENGAGEMENT],
    enabled: !!projectId,
  });

  const engagementSearchRequest = useMemo(
    () =>
      buildEngagementSearchRequest(filters, searchTerm, sortField, sortOrder),
    [filters, searchTerm, sortField, sortOrder],
  );

  const {
    data,
    isLoading: isCasesQueryLoading,
    isError: isCasesError,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useGetProjectCases(projectId || "", engagementSearchRequest, {
    enabled: !!projectId,
  });

  const { showLoader, hideLoader } = useLoader();

  const hasStatsResponse = stats !== undefined;
  const hasCasesResponse = data !== undefined;
  const isStatsLoading = computeEngagementsStatsLoading(
    isStatsQueryLoading,
    hasStatsResponse,
    projectId,
  );
  const isCasesAreaLoading = computeEngagementsCasesAreaLoading(
    isCasesQueryLoading,
    hasCasesResponse,
    projectId,
  );

  const isInitialPageLoading = computeEngagementsInitialPageLoading(
    isStatsLoading,
    isCasesAreaLoading,
  );

  useEffect(() => {
    if (isInitialPageLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isInitialPageLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (!data) return;
    const loadedPages = data.pages.length;
    if (page > loadedPages && hasNextPage && !isFetchingNextPage) {
      void fetchNextPage();
    }
  }, [page, data, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const currentPageCases = useMemo(
    () => getEngagementsCurrentPageCases(data, page),
    [data, page],
  );

  const apiTotalRecords = data?.pages?.[0]?.totalRecords ?? 0;

  const filteredCases = useMemo(
    () =>
      excludeS0
        ? currentPageCases.filter((c) => !isS0Case(c))
        : currentPageCases,
    [currentPageCases, excludeS0],
  );

  const totalItems = computeEngagementsTotalItems(
    apiTotalRecords,
    filteredCases.length,
  );
  const totalPages = computeEngagementsTotalPages(
    totalItems,
    ENGAGEMENTS_PAGE_SIZE,
  );
  const paginatedCases = filteredCases;

  const handlePageChange = (_e: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleFilterChange = (field: string, value: string) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value || undefined,
    }));
    setPage(1);
  };

  const handleClearFilters = () => {
    setFilters({});
    setSearchTerm("");
    setPage(1);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSortFieldUiChange = (value: string) => {
    setSortField(parseEngagementsSortField(value));
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const listHasRefinement = hasListSearchOrFilters(searchTerm, {
    ...filters,
    severityId: undefined,
  });

  const onCaseClick =
    projectId !== undefined
      ? (caseItem: { id: string }) =>
          navigate(buildEngagementDetailPath(projectId, caseItem.id))
      : undefined;

  return {
    projectId,
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
  };
}
