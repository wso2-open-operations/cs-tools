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
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import {
  Box,
  Button,
  Stack,
  Typography,
  Pagination,
  CircularProgress,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  FileText,
  Calendar as CalendarIcon,
  Download,
} from "@wso2/oxygen-ui-icons-react";
import type {
  ChangeRequestFilterValues,
  ChangeRequestItem,
} from "@models/responses";
import type { ChangeRequestSearchRequest } from "@models/requests";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetChangeRequests, {
  useGetChangeRequestsInfinite,
} from "@api/useGetChangeRequests";
import ChangeRequestsStatCards from "@components/support/change-requests/ChangeRequestsStatCards";
import ChangeRequestsSearchBar from "@components/support/change-requests/ChangeRequestsSearchBar";
import ChangeRequestsList from "@components/support/change-requests/ChangeRequestsList";
import ChangeRequestsCalendarView from "@components/support/change-requests/ChangeRequestsCalendarView";
import TabBar from "@components/common/tab-bar/TabBar";
import { generateChangeRequestsSchedulePdf } from "@utils/changeRequestsSchedulePdf";

/**
 * ChangeRequestsPage component to display all change requests with stats, filters, and search.
 *
 * @returns {JSX.Element} The rendered Change Requests page.
 */
export default function ChangeRequestsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [viewMode, setViewMode] = useState<"list" | "calendar">("list");
  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<ChangeRequestFilterValues>({});
  const [page, setPage] = useState(1);
  const [isExporting, setIsExporting] = useState(false);
  const pageSize = 10;

  // Fetch filter metadata (deployments etc.)
  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  // Build API request (following cases listing pattern)
  const changeRequestSearchRequest = useMemo<
    Omit<ChangeRequestSearchRequest, "pagination">
  >(
    () => ({
      filters: {
        searchQuery: searchTerm.trim() || undefined,
        stateKeys: filters.stateId ? [Number(filters.stateId)] : undefined,
        impactKey: filters.impactId ? Number(filters.impactId) : undefined,
      },
    }),
    [searchTerm, filters],
  );

  // Fetch change requests from API - different approaches for list vs calendar
  const offset = (page - 1) * pageSize;

  // List view: use regular query with pagination
  const {
    data: listData,
    isLoading: isListLoading,
    isError: isListError,
  } = useGetChangeRequests(
    projectId || "",
    changeRequestSearchRequest,
    offset,
    pageSize,
    {
      enabled: !!projectId && viewMode === "list",
    },
  );

  // Infinite query to fetch all data in batches (enabled for calendar view or export)
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
      enabled: !!projectId && (viewMode === "calendar" || isExporting),
    },
  );

  // Auto-fetch all pages for calendar view and export
  useEffect(() => {
    if (
      (viewMode === "calendar" || isExporting) &&
      hasNextPage &&
      !isFetchingNextPage
    ) {
      void fetchNextPage();
    }
  }, [viewMode, isExporting, hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Combine data based on view mode
  const changeRequests = useMemo(() => {
    if (viewMode === "list") {
      return listData?.changeRequests || [];
    } else {
      // Flatten all pages for calendar view
      return (
        infiniteData?.pages.flatMap(
          (page: { changeRequests: ChangeRequestItem[] }) =>
            page.changeRequests,
        ) || []
      );
    }
  }, [viewMode, listData, infiniteData]);

  const isLoading =
    viewMode === "list"
      ? isListLoading
      : isInfiniteLoading || isFetchingNextPage;
  const isError = viewMode === "list" ? isListError : isInfiniteError;
  const totalRecords =
    viewMode === "list"
      ? listData?.totalRecords || 0
      : infiniteData?.pages[0]?.totalRecords || 0;
  const totalPages = Math.ceil(totalRecords / pageSize);

  // Mock stats for now - TODO: implement stats API
  const stats = useMemo(
    () => ({
      totalRequests: totalRecords,
      scheduled: 0,
      inProgress: 0,
      completed: 0,
    }),
    [totalRecords],
  );

  // Handle export completion when all data is fetched
  useEffect(() => {
    if (isExporting) {
      if (isInfiniteError) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setIsExporting(false);
      } else if (
        !isInfiniteLoading &&
        !hasNextPage &&
        !isFetchingNextPage &&
        infiniteData
      ) {
        // Success case - all data fetched
        const allChangeRequests =
          infiniteData.pages.flatMap(
            (page: { changeRequests: ChangeRequestItem[] }) =>
              page.changeRequests,
          ) || [];
        generateChangeRequestsSchedulePdf(allChangeRequests, stats);
        setIsExporting(false);
      }
    }
  }, [
    isExporting,
    isInfiniteLoading,
    isInfiniteError,
    hasNextPage,
    isFetchingNextPage,
    infiniteData,
    stats,
  ]);

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

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const handleChangeRequestClick = (item: ChangeRequestItem): void => {
    navigate(`/${projectId}/support/change-requests/${item.id}`);
  };

  const handleExportSchedule = () => {
    setIsExporting(true);
  };

  const viewTabs = useMemo(
    () => [
      { id: "list", label: "List View", icon: FileText },
      { id: "calendar", label: "Calendar View", icon: CalendarIcon },
    ],
    [],
  );

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
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
          }}
        >
          <Box>
            <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
              Change Requests
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Track and manage deployment changes and updates
            </Typography>
          </Box>
          <Button
            variant="contained"
            color="primary"
            size="medium"
            onClick={handleExportSchedule}
            startIcon={
              isExporting ? (
                <CircularProgress size={16} sx={{ color: "white" }} />
              ) : (
                <Download />
              )
            }
            disabled={isExporting}
            sx={{
              bgcolor: "#ea580c",
              "&:hover": {
                bgcolor: "#c2410c",
              },
            }}
          >
            {isExporting ? "Exporting..." : "Export Schedule"}
          </Button>
        </Box>
      </Box>

      {/* Stat cards */}
      <ChangeRequestsStatCards
        isLoading={isLoading}
        isError={isError}
        stats={stats}
      />

      {/* Search bar and filters */}
      <ChangeRequestsSearchBar
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
      />

      {/* View selector */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 2,
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {changeRequests.length} of {totalRecords} change requests
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
          <TabBar
            tabs={viewTabs}
            activeTab={viewMode}
            onTabChange={(tabId) => {
              setViewMode(tabId as "list" | "calendar");
              setPage(1);
            }}
            sx={{ mb: 0, height: 32 }}
          />
        </Box>
      </Box>

      {/* List View or Calendar View */}
      {viewMode === "list" ? (
        <>
          <ChangeRequestsList
            changeRequests={changeRequests}
            isLoading={isLoading}
            isError={isError}
            onChangeRequestClick={handleChangeRequestClick}
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
        </>
      ) : (
        <ChangeRequestsCalendarView
          changeRequests={changeRequests}
          isLoading={isLoading}
          isError={isError}
          onChangeRequestClick={handleChangeRequestClick}
        />
      )}
    </Stack>
  );
}
