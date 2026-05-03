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

import { useParams, useLocation } from "react-router";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useSessionState } from "@hooks/useSessionState";
import { Button, CircularProgress, Divider, Stack } from "@wso2/oxygen-ui";
import { Download } from "@wso2/oxygen-ui-icons-react";
import type { ChangeRequestFilterValues, ChangeRequestItem } from "@features/operations/types/changeRequests";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetChangeRequests, {
  useGetChangeRequestsInfinite,
} from "@features/operations/api/useGetChangeRequests";
import { useGetProjectChangeRequestStats } from "@features/operations/api/useGetProjectChangeRequestStats";
import { CHANGE_REQUEST_FILTER_DEFINITIONS } from "@features/operations/constants/operationsConstants";
import ListPageHeader from "@components/list-view/ListPageHeader";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListFiltersPanel from "@components/list-view/ListFiltersPanel";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ChangeRequestsList from "@features/operations/components/change-requests/ChangeRequestsList";
import ChangeRequestsCalendarView from "@features/operations/components/change-requests/ChangeRequestsCalendarView";
import TabBar from "@components/tab-bar/TabBar";
import { generateChangeRequestsSchedulePdf } from "@features/operations/utils/changeRequestsSchedulePdf";
import { hasListSearchOrFilters, countListSearchAndFilters } from "@features/support/utils/support";
import {
  ChangeRequestFilterDefinitionId,
  ChangeRequestsViewMode,
} from "@features/operations/types/changeRequests";
import {
  CHANGE_REQUESTS_ENTITY_LABEL,
  CHANGE_REQUESTS_EXPORT_EXPORTING_LABEL,
  CHANGE_REQUESTS_EXPORT_SCHEDULE_LABEL,
  CHANGE_REQUESTS_PAGE_DESCRIPTION,
  CHANGE_REQUESTS_PAGE_DESCRIPTION_ACTION_REQUIRED,
  CHANGE_REQUESTS_PAGE_DESCRIPTION_OUTSTANDING,
  CHANGE_REQUESTS_PAGE_DESCRIPTION_SCHEDULED,
  CHANGE_REQUESTS_PAGE_TITLE,
  CHANGE_REQUESTS_PAGE_TITLE_ACTION_REQUIRED,
  CHANGE_REQUESTS_PAGE_TITLE_OUTSTANDING,
  CHANGE_REQUESTS_PAGE_TITLE_SCHEDULED,
  CHANGE_REQUESTS_SEARCH_PLACEHOLDER,
  CHANGE_REQUESTS_VIEW_TABS_CONFIG,
  OPERATIONS_LIST_BACK_LABEL,
  OPERATIONS_LIST_PAGE_SIZE,
} from "@features/operations/constants/operationsConstants";
import {
  buildChangeRequestSearchRequest,
  flattenChangeRequestInfinitePages,
  getOperationsNavSegment,
  resolveChangeRequestFilterListOptions,
} from "@features/operations/utils/operationsPages";

/**
 * ChangeRequestsPage component to display all change requests with stats, filters, and search.
 *
 * @returns {JSX.Element} The rendered Change Requests page.
 */
export default function ChangeRequestsPage(): JSX.Element {
  const navigate = useModifierAwareNavigate();
  const location = useLocation();
  const locationState = location.state as {
    returnTo?: string;
    outstandingOnly?: boolean;
    actionRequired?: boolean;
    scheduledOnly?: boolean;
  } | null;
  const returnTo = locationState?.returnTo;
  const outstandingOnly = locationState?.outstandingOnly ?? false;
  const actionRequired = locationState?.actionRequired ?? false;
  const scheduledOnly = locationState?.scheduledOnly ?? false;
  const { projectId } = useParams<{ projectId: string }>();
  const navSegment = getOperationsNavSegment(location.pathname);

  const [viewMode, setViewMode] = useState<ChangeRequestsViewMode>(
    ChangeRequestsViewMode.List,
  );
  const sessionPrefix = `${projectId ?? "unknown"}-change-requests`;
  const [searchTerm, setSearchTerm] = useSessionState(`${sessionPrefix}-search`, "", undefined, { popOnly: true });
  const [filters, setFilters] = useSessionState<ChangeRequestFilterValues>(`${sessionPrefix}-filters`, {}, undefined, { popOnly: true });
  const [isFiltersOpen, setIsFiltersOpen] = useState(
    () => hasListSearchOrFilters(searchTerm, filters),
  );
  const [page, setPage] = useSessionState<number>(`${sessionPrefix}-page`, 1, undefined, { popOnly: true });
  const [rowsPerPage, setRowsPerPage] = useSessionState<number>(`${sessionPrefix}-rowsPerPage`, OPERATIONS_LIST_PAGE_SIZE, undefined, { popOnly: true });
  const [isExporting, setIsExporting] = useState(false);

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const {
    data: stats,
    isLoading: isStatsLoading,
  } = useGetProjectChangeRequestStats(projectId || "", {
    enabled: !!projectId,
  });

  const changeRequestSearchRequest = useMemo(() => {
    const isPresetMode = outstandingOnly || actionRequired || scheduledOnly;
    const effectiveFilters = isPresetMode ? {} : filters;
    const effectiveSearchTerm = isPresetMode ? "" : searchTerm;
    return buildChangeRequestSearchRequest(effectiveFilters, effectiveSearchTerm, outstandingOnly, actionRequired, scheduledOnly, filterMetadata?.changeRequestStates);
  }, [searchTerm, filters, outstandingOnly, actionRequired, scheduledOnly, filterMetadata?.changeRequestStates]);

  const offset = (page - 1) * rowsPerPage;

  const {
    data: listData,
    isLoading: isListLoading,
    isError: isListError,
  } = useGetChangeRequests(
    projectId || "",
    changeRequestSearchRequest,
    offset,
    rowsPerPage,
    {
      enabled:
        !!projectId &&
        !!filterMetadata &&
        viewMode === ChangeRequestsViewMode.List,
    },
  );

  const {
    data: infiniteData,
    isLoading: isInfiniteLoading,
    isError: isInfiniteError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useGetChangeRequestsInfinite(
    projectId || "",
    changeRequestSearchRequest,
    {
      enabled:
        !!projectId &&
        !!filterMetadata &&
        (viewMode === ChangeRequestsViewMode.Calendar || isExporting),
    },
  );

  useEffect(() => {
    if (
      (viewMode === ChangeRequestsViewMode.Calendar || isExporting) &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      void fetchNextPage();
    }
  }, [viewMode, isExporting, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const changeRequests = useMemo(() => {
    if (viewMode === ChangeRequestsViewMode.List) {
      return listData?.changeRequests || [];
    }
    return flattenChangeRequestInfinitePages(infiniteData?.pages);
  }, [viewMode, listData, infiniteData]);

  const isLoading =
    viewMode === ChangeRequestsViewMode.List
      ? isListLoading
      : isInfiniteLoading || isFetchingNextPage;
  const isError =
    viewMode === ChangeRequestsViewMode.List ? isListError : isInfiniteError;
  const totalRecords =
    viewMode === ChangeRequestsViewMode.List
      ? listData?.totalRecords || 0
      : infiniteData?.pages[0]?.totalRecords || 0;

  useEffect(() => {
    if (!isExporting) return;
    if (isInfiniteError) {
      queueMicrotask(() => {
        setIsExporting(false);
      });
      return;
    }
    if (
      !isInfiniteLoading &&
      !isStatsLoading &&
      !hasNextPage &&
      !isFetchingNextPage &&
      infiniteData &&
      stats
    ) {
      const allChangeRequests =
        flattenChangeRequestInfinitePages(infiniteData.pages) || [];
      generateChangeRequestsSchedulePdf(allChangeRequests, stats);
      queueMicrotask(() => {
        setIsExporting(false);
      });
    }
  }, [
    isExporting,
    isInfiniteLoading,
    isInfiniteError,
    isStatsLoading,
    hasNextPage,
    isFetchingNextPage,
    infiniteData,
    stats,
  ]);

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

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const handleChangeRequestClick = (item: ChangeRequestItem): void => {
    navigate(
      `/projects/${projectId}/${navSegment}/change-requests/${item.id}`,
    );
  };

  const handleExportSchedule = () => {
    setIsExporting(true);
  };

  const listHasRefinement = hasListSearchOrFilters(searchTerm, filters);
  const visibleFilterDefinitions = useMemo(
    () =>
      outstandingOnly || actionRequired || scheduledOnly
        ? CHANGE_REQUEST_FILTER_DEFINITIONS.filter(
            (def) => def.id !== ChangeRequestFilterDefinitionId.State,
          )
        : CHANGE_REQUEST_FILTER_DEFINITIONS,
    [outstandingOnly, actionRequired, scheduledOnly],
  );
  const exportButton = (
    <Button
      variant="contained"
      color="warning"
      size="small"
      onClick={handleExportSchedule}
      startIcon={
        isExporting ? (
          <CircularProgress size={16} sx={{ color: "white" }} />
        ) : (
          <Download />
        )
      }
      disabled={isExporting}
      sx={{ mt: { xs: 0, sm: 4 } }}
    >
      {isExporting
        ? CHANGE_REQUESTS_EXPORT_EXPORTING_LABEL
        : CHANGE_REQUESTS_EXPORT_SCHEDULE_LABEL}
    </Button>
  );

  return (
    <Stack spacing={3}>
      <ListPageHeader
        title={
          actionRequired
            ? CHANGE_REQUESTS_PAGE_TITLE_ACTION_REQUIRED
            : scheduledOnly
            ? CHANGE_REQUESTS_PAGE_TITLE_SCHEDULED
            : outstandingOnly
            ? CHANGE_REQUESTS_PAGE_TITLE_OUTSTANDING
            : CHANGE_REQUESTS_PAGE_TITLE
        }
        description={
          actionRequired
            ? CHANGE_REQUESTS_PAGE_DESCRIPTION_ACTION_REQUIRED
            : scheduledOnly
            ? CHANGE_REQUESTS_PAGE_DESCRIPTION_SCHEDULED
            : outstandingOnly
            ? CHANGE_REQUESTS_PAGE_DESCRIPTION_OUTSTANDING
            : CHANGE_REQUESTS_PAGE_DESCRIPTION
        }
        backLabel={OPERATIONS_LIST_BACK_LABEL}
        onBack={() => (returnTo ? navigate(returnTo) : navigate(".."))}
        actions={exportButton}
      />

      {outstandingOnly || actionRequired || scheduledOnly ? (
        <Divider />
      ) : (
        <ListSearchBar
          searchPlaceholder={CHANGE_REQUESTS_SEARCH_PLACEHOLDER}
          searchTerm={searchTerm}
          onSearchChange={handleSearchChange}
          isFiltersOpen={isFiltersOpen}
          onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
          activeFiltersCount={countListSearchAndFilters("", filters)}
          onClearFilters={handleClearFilters}
          filtersContent={
            <ListFiltersPanel
              filterDefinitions={visibleFilterDefinitions}
              filters={filters}
              resolveOptions={(def) =>
                resolveChangeRequestFilterListOptions(def, filterMetadata)
              }
              onFilterChange={handleFilterChange}
              gridSize={{ xs: 12, sm: 6, md: 4 }}
            />
          }
        />
      )}

      <ListResultsBar
        shownCount={changeRequests.length}
        totalCount={totalRecords}
        entityLabel={CHANGE_REQUESTS_ENTITY_LABEL}
        rightContent={
          <TabBar
            tabs={CHANGE_REQUESTS_VIEW_TABS_CONFIG}
            activeTab={viewMode}
            onTabChange={(tabId) => {
              setViewMode(tabId as ChangeRequestsViewMode);
              setPage(1);
            }}
            sx={{ mb: 0, height: 32 }}
          />
        }
      />

      {viewMode === ChangeRequestsViewMode.List ? (
        <>
          <ChangeRequestsList
            changeRequests={changeRequests}
            isLoading={isLoading}
            isError={isError}
            hasListRefinement={listHasRefinement}
            onChangeRequestClick={handleChangeRequestClick}
          />
          <ListPagination
            totalRecords={totalRecords}
            page={page}
            rowsPerPage={rowsPerPage}
            onPageChange={handlePageChange}
            onRowsPerPageChange={handleRowsPerPageChange}
          />
        </>
      ) : (
        <ChangeRequestsCalendarView
          changeRequests={changeRequests}
          isLoading={isLoading}
          isError={isError}
          onChangeRequestClick={handleChangeRequestClick}
          legendStates={filterMetadata?.changeRequestStates}
        />
      )}
    </Stack>
  );
}
