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

import {
  useParams,
  useNavigate,
  useSearchParams,
  useLocation,
} from "react-router";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Plus } from "@wso2/oxygen-ui-icons-react";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import {
  hasListSearchOrFilters,
  isS0Case,
} from "@features/support/utils/support";
import {
  CaseType,
  ALL_CASES_STAT_CONFIGS,
  getAllCasesFlattenedStats,
} from "@features/support/constants/supportConstants";
import {
  getProjectPermissions,
  getProjectSeverityPolicy,
} from "@utils/permission";
import { SortOrder } from "@/types/common";
import type { AllCasesFilterValues } from "@features/support/types/cases";
import ListStatGrid from "@components/list-view/ListStatGrid";
import ListPageHeader from "@components/list-view/ListPageHeader";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ListSearchPanel from "@components/list-view/ListSearchPanel";
import ListItems from "@components/list-view/ListItems";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { ServiceRequestCaseSortField } from "@features/operations/types/serviceRequests";
import {
  OPERATIONS_LIST_BACK_LABEL,
  OPERATIONS_LIST_PAGE_SIZE,
  SERVICE_REQUESTS_ENTITY_LABEL,
  SERVICE_REQUESTS_NEW_BUTTON_LABEL,
  SERVICE_REQUESTS_PAGE_DESCRIPTION_ALL,
  SERVICE_REQUESTS_PAGE_DESCRIPTION_MINE,
  SERVICE_REQUESTS_PAGE_TITLE_ALL,
  SERVICE_REQUESTS_PAGE_TITLE_MINE,
  SERVICE_REQUESTS_SEARCH_PLACEHOLDER,
  SERVICE_REQUESTS_SORT_FIELD_OPTIONS,
  SERVICE_REQUESTS_STAT_ENTITY_NAME,
  SERVICE_REQUESTS_ERROR_ENTITY_NAME,
} from "@features/operations/constants/operationsConstants";
import {
  buildServiceRequestsPageCaseSearchRequest,
  getOperationsNavSegment,
} from "@features/operations/utils/operationsPages";

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
  const navSegment = getOperationsNavSegment(location.pathname);
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortField, setSortField] = useState<ServiceRequestCaseSortField>(
    ServiceRequestCaseSortField.CreatedOn,
  );
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const pageSize = OPERATIONS_LIST_PAGE_SIZE;

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const projectDetailsReady = !isProjectLoading && project !== undefined;

  const permissions = useMemo(() => {
    if (!projectDetailsReady || !project) {
      return getProjectPermissions(undefined);
    }
    return getProjectPermissions(project.type?.label, {
      hasPdpSubscription: project.hasPdpSubscription,
    });
  }, [projectDetailsReady, project]);

  const severityPolicy = useMemo(
    () =>
      projectDetailsReady && project
        ? getProjectSeverityPolicy(project.type?.label)
        : { excludeS0: false, restrictSeverityToLow: false },
    [projectDetailsReady, project],
  );
  const { excludeS0, restrictSeverityToLow } = severityPolicy;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(
    projectId || "",
    {
      pageSize: 10,
      enabled: !!projectId,
    },
  );
  const deploymentsList =
    deploymentsQuery.data?.pages.flatMap((p) => p.deployments ?? []) ?? [];

  const {
    data: stats,
    isLoading: isStatsQueryLoading,
    isError: isStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SERVICE_REQUEST],
    createdByMe: createdByMe || undefined,
    enabled: !!projectId && permissions.hasSR,
  });

  const caseSearchRequest = useMemo(
    () =>
      buildServiceRequestsPageCaseSearchRequest(
        filters,
        searchTerm,
        sortField,
        sortOrder,
        createdByMe,
      ),
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
    enabled: !!projectId && permissions.hasSR,
  });

  const { showLoader, hideLoader } = useLoader();

  const hasStatsResponse = stats !== undefined;
  const hasCasesResponse = data !== undefined;
  const isProjectContextLoading = isProjectLoading;
  const canLoadServiceRequests = !projectDetailsReady || permissions.hasSR;
  const isStatsLoading =
    canLoadServiceRequests &&
    (isProjectContextLoading ||
      isStatsQueryLoading ||
      (!!projectId && !hasStatsResponse && !isStatsError));

  const isCasesAreaLoading =
    canLoadServiceRequests &&
    (isCasesQueryLoading ||
      (!!projectId && !hasCasesResponse) ||
      isFetchingNextPage);

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

  const handleSortFieldChange = (value: string) => {
    setSortField(value as ServiceRequestCaseSortField);
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

  const listHasRefinement = hasListSearchOrFilters(searchTerm, {
    ...filters,
    severityId: undefined,
  });

  const handleNewServiceRequest = () => {
    navigate(`/projects/${projectId}/${navSegment}/service-requests/create`);
  };

  if (projectDetailsReady && !permissions.hasSR) {
    return (
      <Stack spacing={3}>
        <Box>
          <Button
            startIcon={<ArrowLeft size={16} />}
            onClick={() => (returnTo ? navigate(returnTo) : navigate(".."))}
            sx={{ mb: 2 }}
            variant="text"
          >
            {OPERATIONS_LIST_BACK_LABEL}
          </Button>
          <Typography variant="body2" color="text.secondary">
            Service requests are not available for this project.
          </Typography>
        </Box>
      </Stack>
    );
  }

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
            {OPERATIONS_LIST_BACK_LABEL}
          </Button>
          <ErrorIndicator entityName={SERVICE_REQUESTS_ERROR_ENTITY_NAME} size="medium" />
        </Box>
      </Stack>
    );
  }

  const newRequestButton = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<Plus size={16} />}
      onClick={handleNewServiceRequest}
      sx={{ mt: { xs: 0, sm: 4 } }}
    >
      {SERVICE_REQUESTS_NEW_BUTTON_LABEL}
    </Button>
  );

  return (
    <Stack spacing={3}>
      <ListPageHeader
        title={
          createdByMe
            ? SERVICE_REQUESTS_PAGE_TITLE_MINE
            : SERVICE_REQUESTS_PAGE_TITLE_ALL
        }
        description={
          createdByMe
            ? SERVICE_REQUESTS_PAGE_DESCRIPTION_MINE
            : SERVICE_REQUESTS_PAGE_DESCRIPTION_ALL
        }
        backLabel={OPERATIONS_LIST_BACK_LABEL}
        onBack={() => (returnTo ? navigate(returnTo) : navigate(".."))}
        actions={newRequestButton}
      />

      <Box sx={{ mb: 3 }}>
        <ListStatGrid
          isLoading={isStatsLoading}
          isError={isStatsError}
          entityName={SERVICE_REQUESTS_STAT_ENTITY_NAME}
          configs={ALL_CASES_STAT_CONFIGS}
          stats={getAllCasesFlattenedStats(stats)}
        />
      </Box>

      <ListSearchPanel
        searchTerm={searchTerm}
        searchPlaceholder={SERVICE_REQUESTS_SEARCH_PLACEHOLDER}
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
        restrictSeverityToLow={restrictSeverityToLow}
        hideSeverityFilter
        isProjectContextLoading={isProjectContextLoading}
      />

      <ListResultsBar
        shownCount={paginatedCases.length}
        totalCount={totalItems}
        entityLabel={SERVICE_REQUESTS_ENTITY_LABEL}
        sortFieldOptions={SERVICE_REQUESTS_SORT_FIELD_OPTIONS}
        sortField={sortField}
        onSortFieldChange={handleSortFieldChange}
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
      />

      <ListItems
        cases={paginatedCases}
        isLoading={isCasesAreaLoading && !isCasesError}
        isError={isCasesError}
        hasListRefinement={listHasRefinement}
        entityName={SERVICE_REQUESTS_ENTITY_LABEL}
        hideSeverity
        onCaseClick={(c) =>
          navigate(
            `/projects/${projectId}/${navSegment}/service-requests/${c.id}`,
          )
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
