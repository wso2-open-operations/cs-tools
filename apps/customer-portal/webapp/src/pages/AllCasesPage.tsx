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

import { useParams, useNavigate, useSearchParams, useLocation } from "react-router";
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
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import { hasListSearchOrFilters, isS0Case } from "@utils/support";
import { CaseType } from "@constants/supportConstants";
import { SortOrder } from "@/types/common";
import DOMPurify from "dompurify";
import {
  getProjectPermissions,
  shouldExcludeS0,
} from "@utils/subscriptionUtils";
import type { AllCasesFilterValues } from "@/types/cases";
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
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams] = useSearchParams();
  const createdByMe = searchParams.get("createdByMe") === "true";

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortField, setSortField] = useState<
    "createdOn" | "updatedOn" | "severity" | "state"
  >("createdOn");
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const projectDetailsReady = !isProjectLoading && project !== undefined;

  const permissions = useMemo(() => {
    if (!projectDetailsReady || !project) {
      return getProjectPermissions(undefined);
    }
    return getProjectPermissions(project.type?.label);
  }, [projectDetailsReady, project]);

  const excludeS0 = useMemo(() => {
    if (!projectDetailsReady || !project) {
      return false;
    }
    return shouldExcludeS0(project.type?.label);
  }, [projectDetailsReady, project]);

  // Fetch filter metadata first to get Incident and Query IDs for stats API
  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  // Fetch deployments for the deployment filter (10 at a time)
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(projectId || "", {
    pageSize: 10,
    enabled: !!projectId,
  });
  const deploymentsList =
    deploymentsQuery.data?.pages.flatMap((p) => p.deployments ?? []) ?? [];

  const {
    data: stats,
    isLoading: isStatsQueryLoading,
    isError: isStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.DEFAULT_CASE],
    createdByMe: createdByMe || undefined,
    enabled: !!projectId,
  });

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId:
          permissions.hasDeployments ? filters.deploymentId || undefined : undefined,
        searchQuery: searchTerm.trim() || undefined,
        createdByMe: createdByMe || undefined,
      },
      sortBy: {
        field: sortField,
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortField, sortOrder, createdByMe, permissions.hasDeployments],
  );

  // Fetch all cases using infinite query (runs in parallel with stats when projectId and auth are ready)
  const {
    data,
    isLoading: isCasesQueryLoading,
    isError: isCasesError,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useGetProjectCases(projectId || "", caseSearchRequest, {
    enabled: !!projectId,
  });

  const { showLoader, hideLoader } = useLoader();

  // Show loader only for initial load (until first stats + cases response), not for background refetches or fetchNextPage.
  const hasStatsResponse = stats !== undefined;
  const hasCasesResponse = data !== undefined;
  const isProjectContextLoading = isProjectLoading;
  const isStatsLoading =
    isProjectContextLoading ||
    isStatsQueryLoading ||
    (!!projectId && !hasStatsResponse);

  const isCasesAreaLoading =
    isCasesQueryLoading ||
    (!!projectId && !hasCasesResponse) ||
    isFetchingNextPage;

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

  const filteredAndSearchedCases = useMemo(
    () =>
      excludeS0
        ? currentPageCases.filter((c) => !isS0Case(c))
        : currentPageCases,
    [currentPageCases, excludeS0],
  );

  const totalItems = apiTotalRecords || filteredAndSearchedCases.length;

  const paginatedCases = filteredAndSearchedCases;

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
    setSearchTerm("");
    setPage(1);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSortFieldChange = (
    value: "createdOn" | "updatedOn" | "severity" | "state",
  ) => {
    setSortField(value);
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  // Note: deploymentId is ignored in API request when user lacks permissions.

  const listHasRefinement = hasListSearchOrFilters(searchTerm, filters);

  return (
    <Stack spacing={3}>
      {/* Back button and header */}
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
            onClick={() => (returnTo ? navigate(returnTo) : navigate(".."))}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back to Support Center
        </Button>
        <Box>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            {createdByMe ? "My Cases" : "All Cases"}
          </Typography>
          <Typography
            variant="body2"
            color="text.secondary"
            component="div"
            // biome-ignore lint/security/noDangerouslySetInnerHtml: trusted static copy rendered as HTML by request
            dangerouslySetInnerHTML={{
              __html: DOMPurify.sanitize(
                createdByMe
                  ? "Manage and track your support cases"
                  : "Manage and track all your support cases",
              ),
            }}
          />
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
        deployments={
          projectDetailsReady && permissions.hasDeployments
            ? deploymentsList
            : []
        }
        onLoadMoreDeployments={() => {
          if (
            deploymentsQuery.hasNextPage &&
            !deploymentsQuery.isFetchingNextPage
          ) {
            void deploymentsQuery.fetchNextPage();
          }
        }}
        hasMoreDeployments={!!deploymentsQuery.hasNextPage}
        isFetchingMoreDeployments={deploymentsQuery.isFetchingNextPage}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
        excludeS0={excludeS0}
        isProjectContextLoading={isProjectContextLoading}
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
            <InputLabel id="sort-by-label">Sort by</InputLabel>
            <Select<"createdOn" | "updatedOn" | "severity" | "state">
              labelId="sort-by-label"
              id="sort-by"
              value={sortField}
              label="Sort by"
              onChange={(e) =>
                handleSortFieldChange(
                  e.target.value as
                    | "createdOn"
                    | "updatedOn"
                    | "severity"
                    | "state",
                )
              }
            >
              <MenuItem value="createdOn">
                <Typography variant="body2">Created on</Typography>
              </MenuItem>
              <MenuItem value="updatedOn">
                <Typography variant="body2">Updated on</Typography>
              </MenuItem>
              <MenuItem value="severity">
                <Typography variant="body2">Severity</Typography>
              </MenuItem>
              <MenuItem value="state">
                <Typography variant="body2">State</Typography>
              </MenuItem>
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="order-by-label">Order by</InputLabel>
            <Select<SortOrder>
              labelId="order-by-label"
              id="order-by"
              value={sortOrder}
              label="Order by"
              onChange={(e) =>
                handleSortChange(e.target.value as SortOrder)
              }
            >
              <MenuItem value={SortOrder.DESC}>
                <Typography variant="body2">Newest first</Typography>
              </MenuItem>
              <MenuItem value={SortOrder.ASC}>
                <Typography variant="body2">Oldest first</Typography>
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
        hasListRefinement={listHasRefinement}
        onCaseClick={(c) =>
          navigate(`/projects/${projectId}/support/cases/${c.id}`)
        }
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
