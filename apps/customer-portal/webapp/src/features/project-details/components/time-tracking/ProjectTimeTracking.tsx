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

import { Box, Grid, Pagination, Typography } from "@wso2/oxygen-ui";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import useSearchProjectTimeCards from "@features/usage-metrics/api/useSearchProjectTimeCards";
import useGetProjectFilters from "@api/useGetProjectFilters";
import ServiceHoursStatCards from "@time-tracking/ServiceHoursStatCards";
import TimeCardsDateFilter from "@time-tracking/TimeCardsDateFilter";
import TimeTrackingCard from "@time-tracking/TimeTrackingCard";
import TimeTrackingCardSkeleton from "@time-tracking/TimeTrackingCardSkeleton";
import TimeTrackingErrorState from "@time-tracking/TimeTrackingErrorState";
import EmptyState from "@components/empty-state/EmptyState";

import type { ProjectDetails } from "@features/project-hub/types/projects";

interface ProjectTimeTrackingProps {
  projectId: string;
  project?: ProjectDetails | null;
  isProjectLoading?: boolean;
  isProjectError?: boolean;
}

/**
 * ProjectTimeTracking component manages the display of time tracking statistics, date filter, and time cards.
 *
 * @param {ProjectTimeTrackingProps} props - Component props.
 * @returns {JSX.Element} The rendered component.
 */
export default function ProjectTimeTracking({
  projectId,
  project,
  isProjectLoading,
  isProjectError,
}: ProjectTimeTrackingProps): JSX.Element {
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [approvedStateId, setApprovedStateId] = useState<string | undefined>(
    undefined,
  );
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data: filters, isLoading: isFiltersLoading } =
    useGetProjectFilters(projectId);

  // Find approved state id from filters
  useEffect(() => {
    if (filters?.timeCardStates) {
      const approved = filters.timeCardStates.find(
        (s) => s.label.toLowerCase() === "approved",
      );
      if (approved?.id !== approvedStateId) {
        Promise.resolve().then(() => setApprovedStateId(approved?.id));
      }
    }
  }, [filters, approvedStateId]);

  const {
    data,
    isLoading: isTimeCardsLoading,
    isError: isTimeCardsError,
    hasNextPage,
    fetchNextPage,
  } = useSearchProjectTimeCards({
    projectId,
    startDate,
    endDate,
    states: approvedStateId ? [approvedStateId] : undefined,
    enabled: !isFiltersLoading && Boolean(approvedStateId),
  });

  // Auto-fetch all remaining pages in background
  useEffect(() => {
    if (!data || !hasNextPage) return;
    void fetchNextPage();
  }, [data, hasNextPage, fetchNextPage]);

  // Reset pagination when filters change
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPage(1);
  }, [projectId, startDate, endDate, approvedStateId]);

  // Flatten all pages into a single array
  const allTimeCards = useMemo(
    () => data?.pages.flatMap((page) => page.timeCards) ?? [],
    [data],
  );

  const totalItems = data?.pages?.[0]?.totalRecords ?? allTimeCards.length;

  // Client-side pagination
  const paginatedTimeCards = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return allTimeCards.slice(startIndex, startIndex + pageSize);
  }, [allTimeCards, page, pageSize]);

  const totalPages = Math.ceil(totalItems / pageSize);

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleClearDates = () => {
    setStartDate("");
    setEndDate("");
  };

  const hasDateFilters = Boolean(startDate && endDate);

  return (
    <Box>
      <ServiceHoursStatCards
        project={project}
        isLoading={isProjectLoading}
        isError={isProjectError}
      />

      <Box sx={{ mb: 3 }}>
        <TimeCardsDateFilter
          startDate={startDate}
          endDate={endDate}
          onStartDateChange={setStartDate}
          onEndDateChange={setEndDate}
          onClear={handleClearDates}
          hasFilters={hasDateFilters}
        />
      </Box>

      {/* Results count */}
      <Box sx={{ mb: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedTimeCards.length} of {totalItems} time cards
        </Typography>
      </Box>

      {isTimeCardsError ? (
        <TimeTrackingErrorState />
      ) : (
        <>
          <Grid container spacing={3}>
            {isTimeCardsLoading ? (
              Array.from({ length: 7 }).map((_, index) => (
                <Grid key={`skeleton-${index}`} size={12}>
                  <TimeTrackingCardSkeleton />
                </Grid>
              ))
            ) : paginatedTimeCards.length === 0 ? (
              <Grid size={12}>
                <EmptyState description="No time logs available." />
              </Grid>
            ) : (
              paginatedTimeCards.map((card) => (
                <Grid key={card.id} size={12}>
                  <TimeTrackingCard card={card} />
                </Grid>
              ))
            )}
          </Grid>

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
      )}
    </Box>
  );
}
