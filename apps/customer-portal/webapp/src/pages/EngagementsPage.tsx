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
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import {
  Box,
  Stack,
  Pagination,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Typography,
} from "@wso2/oxygen-ui";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { CaseType } from "@constants/supportConstants";
import { PROJECT_TYPE_LABELS } from "@constants/projectDetailsConstants";
import type { AllCasesFilterValues } from "@models/responses";
import { isS0Case } from "@utils/support";
import AllCasesList from "@components/support/all-cases/AllCasesList";
import AllCasesSearchBar from "@components/support/all-cases/AllCasesSearchBar";
import EngagementsStatCards from "@components/support/engagements/EngagementsStatCards";

/**
 * EngagementsPage component to display engagements with stats, filters, and search.
 *
 * @returns {JSX.Element} The rendered Engagements page.
 */
export default function EngagementsPage(): JSX.Element {
  const navigate = useNavigate();

  const { projectId } = useParams<{ projectId: string }>();

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortOrder, setSortOrder] = useState<"desc" | "asc">("desc");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const projectReady = !isProjectLoading && project !== undefined;
  const isManagedCloudSubscription =
    project?.type?.label === PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION;
  const excludeS0 = projectReady ? !isManagedCloudSubscription : false;

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
    () => ({
      filters: {
        caseTypes: [CaseType.ENGAGEMENT],
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
        searchQuery: searchTerm.trim() || undefined,
      },
      sortBy: {
        field: "createdOn",
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortOrder],
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
  const isStatsLoading =
    isStatsQueryLoading || (!!projectId && !hasStatsResponse);
  const isCasesAreaLoading =
    isCasesQueryLoading || (!!projectId && !hasCasesResponse);

  const isInitialPageLoading = isStatsLoading || isCasesAreaLoading;

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

  const currentPageCases = useMemo(() => {
    if (!data || data.pages.length === 0) return [];
    const requestedPageIndex = page - 1;
    if (requestedPageIndex < 0 || requestedPageIndex >= data.pages.length) {
      return [];
    }
    return data.pages[requestedPageIndex]?.cases ?? [];
  }, [data, page]);

  const apiTotalRecords = data?.pages?.[0]?.totalRecords ?? 0;

  const filteredCases = useMemo(
    () =>
      excludeS0 ? currentPageCases.filter((c) => !isS0Case(c)) : currentPageCases,
    [currentPageCases, excludeS0],
  );

  const totalItems = apiTotalRecords || filteredCases.length;
  const totalPages = Math.ceil(totalItems / pageSize);
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
    setPage(1);
  };

  const handleSortChange = (value: "desc" | "asc") => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  return (
    <Stack spacing={3}>
      <EngagementsStatCards
        stats={stats}
        isLoading={isStatsLoading}
        isError={isStatsError}
      />

      <AllCasesSearchBar
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        deployments={undefined}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
        excludeS0={excludeS0}
      />

      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedCases.length} of {totalItems} engagements
        </Typography>

        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="sort-label">Sort</InputLabel>
            <Select<"desc" | "asc">
              labelId="sort-label"
              id="sort"
              value={sortOrder}
              label="Sort"
              onChange={(e) => handleSortChange(e.target.value as "desc" | "asc")}
            >
              <MenuItem value="desc">
                <Typography variant="body2">Newest First</Typography>
              </MenuItem>
              <MenuItem value="asc">
                <Typography variant="body2">Oldest First</Typography>
              </MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Box>

      <AllCasesList
        cases={paginatedCases}
        isLoading={isCasesAreaLoading}
        isError={isCasesError}
        onCaseClick={
          projectId
            ? (caseItem) =>
                navigate(`/projects/${projectId}/engagements/${caseItem.id}`)
            : undefined
        }
      />

      {totalPages > 1 && (
        <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
          <Pagination
            page={page}
            count={totalPages}
            onChange={handlePageChange}
          />
        </Box>
      )}
    </Stack>
  );
}
