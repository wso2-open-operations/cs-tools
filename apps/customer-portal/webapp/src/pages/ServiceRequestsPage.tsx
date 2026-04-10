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
import { ArrowLeft, Plus } from "@wso2/oxygen-ui-icons-react";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import DOMPurify from "dompurify";
import { hasListSearchOrFilters, isS0Case } from "@utils/support";
import { CaseType } from "@constants/supportConstants";
import {
  getProjectPermissions,
  shouldExcludeS0,
} from "@utils/subscriptionUtils";
import type { AllCasesFilterValues } from "@models/responses";
import AllCasesStatCards from "@components/support/all-cases/AllCasesStatCards";
import AllCasesSearchBar from "@components/support/all-cases/AllCasesSearchBar";
import AllCasesList from "@components/support/all-cases/AllCasesList";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";

/**
 * ServiceRequestsPage lists service requests using the same filters, sort,
 * and pagination pattern as All Cases; only the case type filter differs.
 *
 * @returns {JSX.Element} The rendered Service Requests page.
 */
export default function ServiceRequestsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams] = useSearchParams();
  const createdByMe = searchParams.get("createdByMe") === "true";
  const basePath = location.pathname.includes("/operations/")
    ? "operations"
    : "support";
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortField, setSortField] = useState<
    "createdOn" | "updatedOn" | "severity" | "state"
  >("createdOn");
  const [sortOrder, setSortOrder] = useState<"desc" | "asc">("desc");
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

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");
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
    caseTypes: [CaseType.SERVICE_REQUEST],
    createdByMe: createdByMe || undefined,
    enabled: !!projectId,
  });

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.SERVICE_REQUEST],
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
        searchQuery: searchTerm.trim() || undefined,
        createdByMe: createdByMe || undefined,
      },
      sortBy: {
        field: sortField,
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortField, sortOrder, createdByMe],
  );

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

  const hasStatsResponse = stats !== undefined;
  const hasCasesResponse = data !== undefined;
  const isProjectContextLoading = isProjectLoading;
  const isStatsLoading =
    isProjectContextLoading ||
    isStatsQueryLoading ||
    (!!projectId && !hasStatsResponse && !isStatsError);

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

  const handleSortChange = (value: "desc" | "asc") => {
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

  useEffect(() => {
    if (!projectDetailsReady) {
      return;
    }
    if (
      projectDetailsReady &&
      !permissions.hasDeployments &&
      filters.deploymentId
    ) {
      setFilters((prev) => ({ ...prev, deploymentId: undefined }));
    }
  }, [projectDetailsReady, permissions.hasDeployments, filters.deploymentId]);

  const listHasRefinement = hasListSearchOrFilters(searchTerm, filters);

  const handleNewServiceRequest = () => {
    navigate(`/projects/${projectId}/${basePath}/service-requests/create`);
  };

  if (isCasesError) {
    return (
      <Stack spacing={3}>
        <Box>
          <Button
            startIcon={<ArrowLeft size={16} />}
            onClick={() => (returnTo ? navigate(returnTo) : navigate(".."))}
            sx={{ mb: 2 }}
            variant="text"
          >
            Back
          </Button>
          <ErrorIndicator entityName="service requests" size="medium" />
        </Box>
      </Stack>
    );
  }

  return (
    <Stack spacing={3}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
        }}
      >
        <Box>
          <Button
            startIcon={<ArrowLeft size={16} />}
            onClick={() => navigate("..")}
            sx={{ mb: 2 }}
            variant="text"
          >
            Back
          </Button>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            {createdByMe ? "My Service Requests" : "All Service Requests"}
          </Typography>
          <Typography
            variant="body2"
            color="text.secondary"
            component="div"
            // biome-ignore lint/security/noDangerouslySetInnerHtml: trusted static copy rendered as HTML by request
            dangerouslySetInnerHTML={{
              __html: DOMPurify.sanitize(
                createdByMe
                  ? "Manage and track your service requests"
                  : "Manage deployments, operations, infrastructure change, and service configurations",
              ),
            }}
          />
        </Box>
        <Button
          variant="contained"
          color="primary"
          startIcon={<Plus size={16} />}
          onClick={handleNewServiceRequest}
          sx={{ mt: 4 }}
        >
          New Service Request
        </Button>
      </Box>

      <AllCasesStatCards
        isLoading={isStatsLoading}
        isError={isStatsError}
        stats={stats}
        statEntityName="service request"
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

      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedCases.length} of {totalItems} service requests
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="sr-sort-by-label">Sort by</InputLabel>
            <Select<"createdOn" | "updatedOn" | "severity" | "state">
              labelId="sr-sort-by-label"
              id="sr-sort-by"
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
            <InputLabel id="sr-order-by-label">Order by</InputLabel>
            <Select<"desc" | "asc">
              labelId="sr-order-by-label"
              id="sr-order-by"
              value={sortOrder}
              label="Order by"
              onChange={(e) =>
                handleSortChange(e.target.value as "desc" | "asc")
              }
            >
              <MenuItem value="desc">
                <Typography variant="body2">Newest first</Typography>
              </MenuItem>
              <MenuItem value="asc">
                <Typography variant="body2">Oldest first</Typography>
              </MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Box>

      <AllCasesList
        cases={paginatedCases}
        isLoading={isCasesAreaLoading && !isCasesError}
        isError={isCasesError}
        hasListRefinement={listHasRefinement}
        onCaseClick={(c) =>
          navigate(
            `/projects/${projectId}/${basePath}/service-requests/${c.id}`,
          )
        }
      />

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
