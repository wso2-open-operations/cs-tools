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
  useSearchParams,
  useLocation,
} from "react-router";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useSessionState } from "@hooks/useSessionState";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { Stack, Divider } from "@wso2/oxygen-ui";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import {
  hasListSearchOrFilters,
  isS0Case,
  getLast30DaysUtcRange,
} from "@features/support/utils/support";
import {
  buildDashboardCaseSearchFilters,
  getDashboardOutstandingCasesDescription,
  getDashboardOutstandingCasesTitle,
} from "@features/dashboard/utils/dashboardNavigation";
import { CaseStatus, CaseType } from "@features/support/constants/supportConstants";
import { SortOrder } from "@/types/common";
import {
  getProjectPermissions,
  getProjectSeverityPolicy,
} from "@utils/permission";
import type { AllCasesFilterValues } from "@features/support/types/cases";
import ListPageHeader from "@components/list-view/ListPageHeader";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ListSearchPanel from "@components/list-view/ListSearchPanel";
import ListItems from "@components/list-view/ListItems";

/**
 * AllCasesPage component to display all cases with stats, filters, and search.
 *
 * @returns {JSX.Element} The rendered All Cases page.
 */
export default function AllCasesPage(): JSX.Element {
  const navigate = useModifierAwareNavigate();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const createdByMe = searchParams.get("createdByMe") === "true";
  const initialSeverityId = searchParams.get("severityId");
  const rawStatusFilter = searchParams.get("statusFilter");
  const statusFilter: "active" | "resolved" | null =
    rawStatusFilter === "active" || rawStatusFilter === "resolved"
      ? rawStatusFilter
      : null;
  const isDashboardSeverityNavigation = Boolean(initialSeverityId);

  const sessionPrefix = `${projectId ?? "unknown"}-cases`;
  const [searchTerm, setSearchTerm] = useSessionState(`${sessionPrefix}-search`, "", undefined, { popOnly: true });
  const [filters, setFilters] = useSessionState<AllCasesFilterValues>(
    `${sessionPrefix}-filters`,
    initialSeverityId ? { severityId: initialSeverityId } : {},
    undefined,
    { popOnly: true },
  );
  const [isFiltersOpen, setIsFiltersOpen] = useState(
    () => hasListSearchOrFilters(searchTerm, filters),
  );
  const [sortField, setSortField] = useSessionState<"createdOn" | "updatedOn" | "severity" | "state">(`${sessionPrefix}-sortField`, "createdOn", undefined, { popOnly: true });
  const [sortOrder, setSortOrder] = useSessionState<SortOrder>(`${sessionPrefix}-sortOrder`, SortOrder.DESC, undefined, { popOnly: true });
  const [page, setPage] = useSessionState<number>(`${sessionPrefix}-page`, 1, undefined, { popOnly: true });
  const [rowsPerPage, setRowsPerPage] = useSessionState<number>(`${sessionPrefix}-rowsPerPage`, 10, undefined, { popOnly: true });

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const { data: projectFeatures } = useGetProjectFeatures(projectId || "");
  const projectDetailsReady = !isProjectLoading && project !== undefined;

  const permissions = useMemo(() => {
    if (!projectDetailsReady || !project) {
      return getProjectPermissions(undefined, { projectFeatures: null });
    }
    return getProjectPermissions(project.type?.label, { projectFeatures });
  }, [projectDetailsReady, project, projectFeatures]);

  const severityPolicy = useMemo(
    () =>
      projectDetailsReady && project
        ? getProjectSeverityPolicy(project.type?.label, { projectFeatures })
        : { excludeS0: false, restrictSeverityToLow: false },
    [projectDetailsReady, project, projectFeatures],
  );
  const { excludeS0, restrictSeverityToLow } = severityPolicy;

  // Fetch filter metadata first to get Incident and Query IDs for stats API
  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  // When navigating from Dashboard with ?statusFilter=resolved, force the Closed status into the filter
  // so the list only shows resolved cases. We guard on "already equals closed id" rather than "any truthy
  // statusId" to avoid skipping the update when a prior session filter is a different status.
  useEffect(() => {
    if (statusFilter !== "resolved" || !filterMetadata?.caseStates) return;
    const closedState = filterMetadata.caseStates.find((s) => s.label === CaseStatus.CLOSED);
    if (closedState && filters.statusId !== String(closedState.id)) {
      setFilters((prev) => ({ ...prev, statusId: String(closedState.id) }));
    }
  }, [statusFilter, filterMetadata, filters.statusId, setFilters]);

  // Fetch deployments for the deployment filter (10 at a time)
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(
    projectId || "",
    {
      pageSize: 10,
      enabled: !!projectId,
    },
  );
  const deploymentsList =
    deploymentsQuery.data?.pages.flatMap((p) => p.deployments ?? []) ?? [];

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.DEFAULT_CASE],
        ...buildDashboardCaseSearchFilters({
          statusId: filters.statusId,
          severityId: filters.severityId,
          issueTypes: filters.issueTypes,
          deploymentId: permissions.hasDeployments
            ? filters.deploymentId || undefined
            : undefined,
          searchQuery: searchTerm,
          createdByMe: createdByMe || undefined,
          caseStates: filterMetadata?.caseStates,
          isDashboardSeverityNavigation:
            (isDashboardSeverityNavigation &&
              filters.severityId === initialSeverityId) ||
            statusFilter === "active",
        }),
        ...(statusFilter === "resolved" ? getLast30DaysUtcRange() : {}),
      },
      sortBy: {
        field: sortField,
        order: sortOrder,
      },
    }),
    [
      statusFilter,
      filters,
      searchTerm,
      sortField,
      sortOrder,
      createdByMe,
      permissions.hasDeployments,
      filterMetadata?.caseStates,
      isDashboardSeverityNavigation,
      initialSeverityId,
    ],
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
    pageSize: rowsPerPage,
  });

  const { showLoader, hideLoader } = useLoader();

  const hasCasesResponse = data !== undefined;
  const isProjectContextLoading = isProjectLoading;

  const isCasesAreaLoading =
    isCasesQueryLoading ||
    (!!projectId && !hasCasesResponse) ||
    isFetchingNextPage;

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

  const totalItems = excludeS0
    ? filteredAndSearchedCases.length
    : (apiTotalRecords || filteredAndSearchedCases.length);

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
    if (statusFilter) {
      setSearchParams((prev) => {
        const next = new URLSearchParams(prev);
        next.delete("statusFilter");
        return next;
      }, { replace: true });
    }
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
    <Stack spacing={3} sx={{ minWidth: 0 }}>
      <ListPageHeader
        title={(() => {
          const dashboardTitle = getDashboardOutstandingCasesTitle(initialSeverityId);
          if (dashboardTitle) return dashboardTitle;
          if (createdByMe) return "My Cases";
          if (statusFilter === "active") return "Outstanding Cases";
          if (statusFilter === "resolved") return "Resolved Cases (Last 30d)";
          return "All Cases";
        })()}
        description={
          (() => {
            const dashboardDescription =
              getDashboardOutstandingCasesDescription(initialSeverityId);
            if (dashboardDescription) {
              return dashboardDescription;
            }
            if (statusFilter === "active") return "Cases that are currently in progress";
            if (statusFilter === "resolved") return "Cases that have been resolved during the last 30 days";
            return createdByMe
              ? "Manage and track your support cases"
              : "Manage and track all your support cases";
          })()
        }
        backLabel={
          returnTo?.endsWith("/support") ? "Back to Support Center" : "Back"
        }
        onBack={() => {
          if (returnTo) {
            setFilters({});
            navigate(returnTo);
          } else {
            navigate("..");
          }
        }}
      />

      {statusFilter || isDashboardSeverityNavigation ? (
        <Divider />
      ) : (
        <ListSearchPanel
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
          restrictSeverityToLow={restrictSeverityToLow}
          hideSeverityFilter={isDashboardSeverityNavigation}
          hideDeploymentFilter={!permissions.hasDeployments}
          isProjectContextLoading={isProjectContextLoading}
          excludeFromCount={
            initialSeverityId && filters.severityId === initialSeverityId
              ? ["severityId"]
              : []
          }
        />
      )}

      <ListResultsBar
        shownCount={paginatedCases.length}
        totalCount={totalItems}
        entityLabel="cases"
        sortFieldOptions={[
          { value: "createdOn", label: "Created on" },
          { value: "updatedOn", label: "Updated on" },
          { value: "severity", label: "Severity", kind: "ordinal" as const },
          { value: "state", label: "Status", kind: "ordinal" as const },
        ]}
        sortField={sortField}
        onSortFieldChange={(v) =>
          handleSortFieldChange(
            v as "createdOn" | "updatedOn" | "severity" | "state",
          )
        }
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
      />

      <ListItems
        cases={paginatedCases}
        isLoading={isCasesAreaLoading && !isCasesError}
        isError={isCasesError}
        hasListRefinement={listHasRefinement}
        entityName="cases"
        onCaseClick={(c) =>
          navigate(`/projects/${projectId}/support/cases/${c.id}`)
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
