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
import { useSessionState } from "@hooks/useSessionState";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { Box, Button, Divider, Stack, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Plus } from "@wso2/oxygen-ui-icons-react";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import {
  hasListSearchOrFilters,
  isS0Case,
} from "@features/support/utils/support";
import {
  getProjectPermissions,
  getProjectSeverityPolicy,
} from "@utils/permission";
import { SortOrder } from "@/types/common";
import type { AllCasesFilterValues } from "@features/support/types/cases";
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
  SERVICE_REQUESTS_PAGE_DESCRIPTION_ACTION_REQUIRED,
  SERVICE_REQUESTS_PAGE_DESCRIPTION_ALL,
  SERVICE_REQUESTS_PAGE_DESCRIPTION_MINE,
  SERVICE_REQUESTS_PAGE_DESCRIPTION_OUTSTANDING,
  SERVICE_REQUESTS_PAGE_TITLE_ACTION_REQUIRED,
  SERVICE_REQUESTS_PAGE_TITLE_ALL,
  SERVICE_REQUESTS_PAGE_TITLE_MINE,
  SERVICE_REQUESTS_PAGE_TITLE_OUTSTANDING,
  SERVICE_REQUESTS_SEARCH_PLACEHOLDER,
  SERVICE_REQUESTS_SORT_FIELD_OPTIONS,
  SERVICE_REQUESTS_ERROR_ENTITY_NAME,
} from "@features/operations/constants/operationsConstants";
import {
  buildServiceRequestsPageCaseSearchRequest,
  getOperationsNavSegment,
} from "@features/operations/utils/operationsPages";
import { resolveCasesTableDefaultStatusIds } from "@features/dashboard/utils/casesTable";
import { CaseStatus } from "@features/support/constants/supportConstants";

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
  const returnTo = (location.state as { returnTo?: string; outstandingOnly?: boolean; actionRequired?: boolean } | null)?.returnTo;
  const outstandingOnly = (location.state as { outstandingOnly?: boolean } | null)?.outstandingOnly ?? false;
  const actionRequired = (location.state as { actionRequired?: boolean } | null)?.actionRequired ?? false;

  const listMode = createdByMe ? "mine" : "all";
  const sessionPrefix = `${projectId ?? "unknown"}-service-requests-${listMode}`;
  const [searchTerm, setSearchTerm] = useSessionState(`${sessionPrefix}-search`, "", undefined, { popOnly: true });
  const [filters, setFilters] = useSessionState<AllCasesFilterValues>(`${sessionPrefix}-filters`, {}, undefined, { popOnly: true });
  const [isFiltersOpen, setIsFiltersOpen] = useState(
    () => hasListSearchOrFilters(searchTerm, filters),
  );
  const [sortField, setSortField] = useSessionState<ServiceRequestCaseSortField>(`${sessionPrefix}-sortField`, ServiceRequestCaseSortField.CreatedOn, undefined, { popOnly: true });
  const [sortOrder, setSortOrder] = useSessionState<SortOrder>(`${sessionPrefix}-sortOrder`, SortOrder.DESC, undefined, { popOnly: true });
  const [page, setPage] = useSessionState<number>(`${sessionPrefix}-page`, 1, undefined, { popOnly: true });
  const [rowsPerPage, setRowsPerPage] = useSessionState<number>(`${sessionPrefix}-rowsPerPage`, OPERATIONS_LIST_PAGE_SIZE, undefined, { popOnly: true });

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const { data: projectFeatures } = useGetProjectFeatures(projectId || "");
  const projectDetailsReady = !isProjectLoading && project !== undefined;

  const permissions = useMemo(() => {
    if (!projectDetailsReady || !project) {
      return getProjectPermissions(undefined, { projectFeatures: null });
    }
    return getProjectPermissions(project.type?.label, {
      projectFeatures,
    });
  }, [projectDetailsReady, project, projectFeatures]);

  const severityPolicy = useMemo(
    () =>
      projectDetailsReady && project
        ? getProjectSeverityPolicy(project.type?.label, { projectFeatures })
        : { excludeS0: false, restrictSeverityToLow: false },
    [projectDetailsReady, project, projectFeatures],
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

  const outstandingStatusIds = useMemo(() => {
    if (actionRequired) {
      if (!filterMetadata) return undefined;
      return (filterMetadata.caseStates ?? [])
        .filter(
          (s) =>
            s.label === CaseStatus.AWAITING_INFO ||
            s.label === CaseStatus.SOLUTION_PROPOSED,
        )
        .map((s) => Number(s.id));
    }
    if (outstandingOnly) {
      if (!filterMetadata) return undefined;
      return resolveCasesTableDefaultStatusIds(filterMetadata.caseStates);
    }
    return [];
  }, [actionRequired, outstandingOnly, filterMetadata]);

  const caseSearchRequest = useMemo(
    () =>
      buildServiceRequestsPageCaseSearchRequest(
        filters,
        searchTerm,
        sortField,
        sortOrder,
        createdByMe,
        outstandingStatusIds,
      ),
    [filters, searchTerm, sortField, sortOrder, createdByMe, outstandingStatusIds],
  );

  const {
    data,
    isLoading: isCasesQueryLoading,
    isError: isCasesError,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useGetProjectCases(projectId || "", caseSearchRequest, {
    enabled:
      !!projectId &&
      permissions.hasSR &&
      (!(actionRequired || outstandingOnly) || filterMetadata !== undefined),
    pageSize: rowsPerPage,
  });

  const { showLoader, hideLoader } = useLoader();

  const hasCasesResponse = data !== undefined;
  const isProjectContextLoading = isProjectLoading;
  const canLoadServiceRequests = !projectDetailsReady || permissions.hasSR;

  const isCasesAreaLoading =
    canLoadServiceRequests &&
    (isCasesQueryLoading ||
      (!!projectId && !hasCasesResponse) ||
      isFetchingNextPage);

  const isInitialPageLoading = isCasesAreaLoading;

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

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleRowsPerPageChange = (newSize: number) => {
    setRowsPerPage(newSize);
    setPage(1);
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
  }, [
    projectDetailsReady,
    permissions.hasDeployments,
    filters.deploymentId,
    setFilters,
  ]);

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
        title={(() => {
          return actionRequired
            ? SERVICE_REQUESTS_PAGE_TITLE_ACTION_REQUIRED
            : outstandingOnly
            ? SERVICE_REQUESTS_PAGE_TITLE_OUTSTANDING
            : createdByMe
            ? SERVICE_REQUESTS_PAGE_TITLE_MINE
            : SERVICE_REQUESTS_PAGE_TITLE_ALL;
        })()}
        description={
          actionRequired
            ? SERVICE_REQUESTS_PAGE_DESCRIPTION_ACTION_REQUIRED
            : outstandingOnly
            ? SERVICE_REQUESTS_PAGE_DESCRIPTION_OUTSTANDING
            : createdByMe
            ? SERVICE_REQUESTS_PAGE_DESCRIPTION_MINE
            : SERVICE_REQUESTS_PAGE_DESCRIPTION_ALL
        }
        backLabel={OPERATIONS_LIST_BACK_LABEL}
        onBack={() => (returnTo ? navigate(returnTo) : navigate(".."))}
        actions={newRequestButton}
      />

      {outstandingOnly || actionRequired ? (
        <Divider />
      ) : (
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
          hideDeploymentFilter={!permissions.hasDeployments}
          isProjectContextLoading={isProjectContextLoading}
        />
      )}

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
        showInternalId
        onCaseClick={(c) =>
          navigate(
            `/projects/${projectId}/${navSegment}/service-requests/${c.id}`,
          )
        }
      />

      <ListPagination
        totalRecords={totalItems}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handlePageChange}
        onRowsPerPageChange={handleRowsPerPageChange}
      />
    </Stack>
  );
}
