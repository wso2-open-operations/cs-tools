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

import { Box, Chip, TablePagination, Typography } from "@wso2/oxygen-ui";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
  type ReactNode,
} from "react";
import { useSearchParams } from "react-router";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import CasesFilterBar, {
  type CasesFilters,
} from "@features/csm-cases/components/CasesFilterBar";
import CasesList from "@features/csm-cases/components/CasesList";
import { useGetCsmCases } from "@features/csm-cases/api/useGetCsmCases";
import { useDirectoryUsers } from "@api/useDirectoryUsers";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import {
  DEFAULT_CASES_FILTERS,
  readCasesFiltersFromUrl,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, DEFAULT_ROWS_PER_PAGE, BE_MAX_PAGE_LIMIT];

// URL params owned by the filter state; cleared/rewritten on change while any
// other params (e.g. a `tab` selection) are preserved.
const FILTER_PARAM_KEYS = [
  "q",
  "severities",
  "states",
  "types",
  "assignees",
  "projects",
  "engagementTypes",
  "products",
] as const;

interface CsmIssuesViewProps {
  /** Optional heading shown on the left of the header row. */
  title?: string;
  /** Optional right-aligned actions (e.g. a "Create" button). */
  actions?: ReactNode;
  /** Plural noun for the count subtitle / empty states. Default "cases". */
  entityNoun?: string;
  /** Filter values forced onto every query and hidden from the user (e.g. a
   *  locked case type or project). Merged over the user's URL filters. */
  lockedFilters?: Partial<CasesFilters>;
  /** Hide the case-type filter control (use when `lockedFilters` fixes it). */
  hideTypeFilter?: boolean;
  /** Hide the project filter control (use when the view is project-scoped). */
  hideProjectFilter?: boolean;
  /** Show the engagement-type sub-filter (pass when the view is locked to engagement cases). */
  showEngagementTypeFilter?: boolean;
  /** Base path for row detail links. Defaults to "/cases". */
  detailBasePath?: string;
}

/**
 * Shared issues list: the cases filter bar + list + pagination, backed by
 * `POST /cases/search`. Reused for the all-cases page, the per-type lists
 * (service requests, security reports) and the project-scoped issues tab —
 * each just supplies `lockedFilters` (and hides the now-fixed control) so the
 * one component covers every "list issues of kind X" surface.
 */
export default function CsmIssuesView({
  title,
  actions,
  entityNoun = "cases",
  lockedFilters,
  hideTypeFilter,
  hideProjectFilter,
  showEngagementTypeFilter,
  detailBasePath,
}: CsmIssuesViewProps): JSX.Element {
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<CasesFilters>(
    () => readCasesFiltersFromUrl(searchParams),
    [searchParams],
  );

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);

  const setFilters = useCallback(
    (next: CasesFilters) => {
      setPage(0);
      // Preserve any non-filter params (e.g. the active project-detail tab).
      const merged = new URLSearchParams(searchParams);
      FILTER_PARAM_KEYS.forEach((k) => merged.delete(k));
      writeCasesFiltersToUrl(next).forEach((v, k) => merged.set(k, v));
      setSearchParams(merged, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const debouncedSearch = useDebouncedValue(filters.search, 300);
  // User filters (debounced search) with the locked overrides applied last so
  // the fixed type/project can't be widened by a stale URL value.
  const queryFilters = useMemo<CasesFilters>(
    () => ({ ...filters, search: debouncedSearch, ...lockedFilters }),
    [filters, debouncedSearch, lockedFilters],
  );

  const { data, isLoading, isError, error } = useGetCsmCases(
    queryFilters,
    page,
    rowsPerPage,
  );

  const { data: directoryUsers } = useDirectoryUsers();
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError(`Could not load ${entityNoun}.`, error);
    }
    if (!isError) hasShownErrorRef.current = false;
  }, [isError, error, showError, entityNoun]);

  const cases = data?.cases ?? [];

  const availableAssigneeUsers = useMemo(() => {
    const list = (directoryUsers ?? [])
      .filter((u) => u.name)
      .map((u) => ({ name: u.name, email: u.email }));
    list.sort((a, b) => a.name.localeCompare(b.name));
    return list;
  }, [directoryUsers]);

  const availableProjects = useMemo(() => {
    const byId = new Map<string, string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.projectId) byId.set(c.projectId, c.projectName || c.projectId);
    });
    return Array.from(byId, ([id, name]) => ({ id, name }));
  }, [data?.cases]);

  const total = data?.total ?? 0;
  const lastPage = total === 0 ? 0 : Math.ceil(total / rowsPerPage) - 1;
  // Clamp to the last valid page when the loaded set shrinks (filter change, rows
  // closing). React's documented pattern for adjusting state from changed inputs
  // is a guarded set during render — not an effect (the lint rule forbids
  // setState in effects); React re-renders before committing, so it's not a
  // user-visible extra paint.
  if (data !== undefined && !data.hasMore && page > lastPage) {
    setPage(lastPage);
  }
  const paginationCount = data === undefined || data.hasMore ? -1 : total;

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const breachedCount = cases.filter(
    (c) => c.minutesToBreach < 0 && c.state !== "closed",
  ).length;
  const myCount = cases.filter(
    (c) => c.assigneeIsMe && c.state !== "closed",
  ).length;
  const rangeStart = total === 0 ? 0 : page * rowsPerPage + 1;
  const rangeEnd = page * rowsPerPage + cases.length;

  const subtitle =
    isLoading
      ? null
      : total === 0
        ? `No ${entityNoun}`
        : `Showing ${rangeStart}–${rangeEnd} of ${total}`;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box>
          {title && <Typography variant="h5">{title}</Typography>}
          {subtitle != null && (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {breachedCount > 0 && (
            <Chip size="small" color="error" label={`${breachedCount} breached`} />
          )}
          {myCount > 0 && (
            <Chip
              size="small"
              color="primary"
              variant="outlined"
              label={`${myCount} mine`}
            />
          )}
          {actions}
        </Box>
      </Box>

      <CasesFilterBar
        filters={filters}
        onChange={setFilters}
        onReset={() => setFilters(DEFAULT_CASES_FILTERS)}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((v) => !v)}
        availableAssigneeUsers={availableAssigneeUsers}
        availableProjects={availableProjects}
        hideTypeFilter={hideTypeFilter}
        hideProjectFilter={hideProjectFilter}
        showEngagementTypeFilter={showEngagementTypeFilter}
      />

      <CasesList cases={cases} isLoading={isLoading} detailBasePath={detailBasePath} />

      <TablePagination
        component="div"
        count={paginationCount}
        page={page}
        onPageChange={(_, newPage) => setPage(newPage)}
        rowsPerPage={rowsPerPage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        labelRowsPerPage={`${entityNoun[0].toUpperCase()}${entityNoun.slice(1)} per page`}
        showFirstButton
        showLastButton
      />
    </Box>
  );
}
