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

import { useParams, useNavigate, useSearchParams } from "react-router";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import {
  Box,
  Button,
  Stack,
  Select,
  MenuItem,
  Typography,
  FormControl,
  InputLabel,
  Pagination,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetDeployments } from "@api/useGetDeployments";
import { getIncidentAndQueryIds, isS0Case } from "@utils/support";
import { CaseType } from "@constants/supportConstants";
import { PROJECT_TYPE_LABELS } from "@constants/projectDetailsConstants";
import type { AllCasesFilterValues } from "@models/responses";
import AllCasesStatCards from "@components/support/all-cases/AllCasesStatCards";
import AllCasesSearchBar from "@components/support/all-cases/AllCasesSearchBar";
import AllCasesList from "@components/support/all-cases/AllCasesList";

/**
 * AllCasesPage component to display all cases with stats, filters, and search.
 *
 * @returns {JSX.Element} The rendered All Cases page.
 */
export default function AllCasesPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams] = useSearchParams();
  const createdByMe = searchParams.get("createdByMe") === "true";

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortOrder, setSortOrder] = useState<"desc" | "asc">("desc");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const {
    data: project,
    isLoading: isProjectLoading,
  } = useGetProjectDetails(projectId || "");
  const projectReady = !isProjectLoading && project !== undefined;
  const isManagedCloudSubscription =
    project?.type?.label === PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION;
  const excludeS0 = projectReady ? !isManagedCloudSubscription : false;

  // Fetch filter metadata first to get Incident and Query IDs for stats API
  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  // Fetch deployments for the deployment filter
  const { data: deploymentsData } = useGetDeployments(projectId || "");

  const { incidentId, queryId } = useMemo(
    () => getIncidentAndQueryIds(filterMetadata?.caseTypes),
    [filterMetadata?.caseTypes],
  );

  const {
    data: stats,
    isLoading: isStatsQueryLoading,
    isError: isStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    incidentId,
    queryId,
    enabled: !!incidentId && !!queryId,
  });

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: filters.caseTypeId
          ? [filters.caseTypeId]
          : [CaseType.DEFAULT_CASE],
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
        searchQuery: searchTerm.trim() || undefined,
        createdByMe: createdByMe || undefined,
      },
      sortBy: {
        field: "createdOn",
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortOrder, createdByMe],
  );

  // Fetch all cases using infinite query (runs in parallel with stats when projectId and auth are ready)
  const {
    data,
    isLoading: isCasesQueryLoading,
    isError: isCasesError,
    hasNextPage,
    fetchNextPage,
  } = useGetProjectCases(projectId || "", caseSearchRequest, {
    enabled: !!projectId,
  });

  const { showLoader, hideLoader } = useLoader();

  // Show loader only for initial load (until first stats + cases response), not for background refetches or fetchNextPage.
  const statsQueryExpected = !!incidentId && !!queryId;
  const hasStatsResponse = stats !== undefined;
  const hasCasesResponse = data !== undefined;
  const isStatsLoading =
    isStatsQueryLoading ||
    (statsQueryExpected && !!projectId && !hasStatsResponse);
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

  // Background-load all remaining pages so search/filters work on full dataset.
  useEffect(() => {
    if (!data || !hasNextPage) {
      return;
    }

    void fetchNextPage();
  }, [data, hasNextPage, fetchNextPage]);

  const rawCases = useMemo(
    () => data?.pages.flatMap((page) => page.cases) ?? [],
    [data],
  );
  const apiTotalRecords = data?.pages?.[0]?.totalRecords ?? 0;

  const filteredAndSearchedCases = useMemo(
    () => (excludeS0 ? rawCases.filter((c) => !isS0Case(c)) : rawCases),
    [rawCases, excludeS0],
  );

  const totalItems = excludeS0
    ? filteredAndSearchedCases.length
    : (apiTotalRecords || filteredAndSearchedCases.length);

  // Pagination logic
  const paginatedCases = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return filteredAndSearchedCases.slice(startIndex, startIndex + pageSize);
  }, [filteredAndSearchedCases, page]);

  const totalPages = Math.ceil(totalItems / pageSize);

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
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
      {/* Back button and header */}
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("..")}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back to Support Center
        </Button>
        <Box>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            {createdByMe ? "My Cases" : "All Cases"}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {createdByMe
              ? "Manage and track your support cases"
              : "Manage and track all your support cases"}
          </Typography>
        </Box>
      </Box>

      {/* Stat cards */}
      <AllCasesStatCards
        isLoading={isStatsLoading}
        isError={isStatsError}
        stats={stats}
      />

      <AllCasesSearchBar
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        deployments={deploymentsData?.deployments}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
        excludeS0={excludeS0}
      />

      {/* Sort and results count */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedCases.length} of {totalItems} cases
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="sort-label">Sort</InputLabel>
            <Select<"desc" | "asc">
              labelId="sort-label"
              id="sort"
              value={sortOrder}
              label="Sort"
              onChange={(e) =>
                handleSortChange(e.target.value as "desc" | "asc")
              }
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

      {/* Cases list */}
      <AllCasesList
        cases={paginatedCases}
        isLoading={isCasesAreaLoading && !isCasesError}
        isError={isCasesError}
        onCaseClick={(c) => navigate(`/${projectId}/support/cases/${c.id}`)}
      />

      {/* Pagination */}
      {totalPages > 1 && (
        <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 4 }}>
          <Pagination
            count={totalPages}
            page={page}
            onChange={handlePageChange}
            color="primary"
            variant="outlined"
            shape="rounded"
          />
        </Box>
      )}
    </Stack>
  );
}
