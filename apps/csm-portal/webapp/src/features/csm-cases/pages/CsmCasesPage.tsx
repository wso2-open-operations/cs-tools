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
  Box,
  Button,
  Chip,
  TablePagination,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import { useNavigate, useSearchParams } from "react-router";
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
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, DEFAULT_ROWS_PER_PAGE, BE_MAX_PAGE_LIMIT];

export default function CsmCasesPage(): JSX.Element {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<CasesFilters>(
    () => readCasesFiltersFromUrl(searchParams),
    [searchParams],
  );

  // Pagination is local (not URL) state, matching the projects/accounts pages.
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const setFilters = useCallback(
    (next: CasesFilters) => {
      // Any filter/search change resets to the first page.
      setPage(0);
      setSearchParams(writeCasesFiltersToUrl(next), { replace: true });
    },
    [setSearchParams],
  );

  // Search is now server-side, so debounce it to avoid a request per keystroke.
  // The filter bar still reflects the raw URL value immediately.
  const debouncedSearch = useDebouncedValue(filters.search, 300);
  const queryFilters = useMemo<CasesFilters>(
    () => ({ ...filters, search: debouncedSearch }),
    [filters, debouncedSearch],
  );

  // Load straight away with no filters: the backend sorts by last-updated
  // descending, so arriving on the page shows the most recently updated cases.
  // Filters/search narrow that set from there.
  const { data, isLoading, isError, error } = useGetCsmCases(
    queryFilters,
    page,
    rowsPerPage,
  );

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };
  const { data: directoryUsers } = useDirectoryUsers();
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load cases.", error);
    }
    if (!isError) hasShownErrorRef.current = false;
  }, [isError, error, showError]);

  // The backend (or the mock hook) already filtered, sorted and paged the rows;
  // render exactly what came back for the current page.
  const cases = data?.cases ?? [];

  // Derive option lists for the searchable filters. Assignees come from the
  // user directory (so typing finds anyone, not only owners present in the
  // loaded cases). Projects and products are still derived from the case
  // rows in scope. All sorted and de-duplicated; the lists are small.
  const availableAssigneeUsers = useMemo(() => {
    const list = (directoryUsers ?? [])
      .filter((u) => u.name)
      .map((u) => ({ name: u.name, email: u.email }));
    list.sort((a, b) => a.name.localeCompare(b.name));
    return list;
  }, [directoryUsers]);

  // The project filter searches the backend as the user types, so we no longer
  // preload the whole catalogue. These are only the projects on the loaded
  // cases — used to label an already-selected project id (e.g. from a shared
  // URL) until a search resolves its name.
  const availableProjects = useMemo(() => {
    const byId = new Map<string, string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.projectId) byId.set(c.projectId, c.projectName || c.projectId);
    });
    return Array.from(byId, ([id, name]) => ({ id, name }));
  }, [data?.cases]);

  const availableProducts = useMemo(() => {
    const set = new Set<string>();
    (data?.cases ?? []).forEach((c) => {
      if (c.product) set.add(c.product);
    });
    return Array.from(set).sort();
  }, [data?.cases]);

  const total = data?.total ?? 0;
  // If the total shrinks (a background refetch, or rows closing) while we're on
  // a later page, clamp back to the last valid page so the table never shows an
  // empty out-of-range page with a misleading range. Adjusting state during
  // render (React's documented pattern) rather than in an effect: React
  // re-renders with the corrected page and the hook refetches the right slice.
  const lastPage = total === 0 ? 0 : Math.ceil(total / rowsPerPage) - 1;
  if (page > lastPage) {
    setPage(lastPage);
  }

  // Counts reflect the current page only (the backend exposes no SLA/assignee
  // breakdown across pages); both are inert in LIVE where that data is absent.
  const breachedCount = cases.filter(
    (c) => c.minutesToBreach < 0 && c.state !== "closed",
  ).length;
  const myCount = cases.filter(
    (c) => c.assigneeIsMe && c.state !== "closed",
  ).length;
  const rangeStart = total === 0 ? 0 : page * rowsPerPage + 1;
  const rangeEnd = page * rowsPerPage + cases.length;

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
          <Typography variant="h5">Cases</Typography>
          <Typography variant="body2" color="text.secondary">
            {isLoading
              ? "Loading…"
              : total === 0
                ? "No cases"
                : `Showing ${rangeStart}–${rangeEnd} of ${total}`}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {breachedCount > 0 && (
            <Chip
              size="small"
              color="error"
              label={`${breachedCount} breached`}
            />
          )}
          {myCount > 0 && (
            <Chip
              size="small"
              color="primary"
              variant="outlined"
              label={`${myCount} mine`}
            />
          )}
          <Button
            variant="contained"
            color="primary"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={() => navigate("/cases/new")}
          >
            Create case
          </Button>
        </Box>
      </Box>

      <CasesFilterBar
        filters={filters}
        onChange={setFilters}
        onReset={() =>
          setFilters({ ...DEFAULT_CASES_FILTERS, scope: filters.scope })
        }
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((v) => !v)}
        availableAssigneeUsers={availableAssigneeUsers}
        availableProjects={availableProjects}
        availableProducts={availableProducts}
      />

      <CasesList cases={cases} isLoading={isLoading} />

      <TablePagination
        component="div"
        count={total}
        page={page}
        onPageChange={(_, newPage) => setPage(newPage)}
        rowsPerPage={rowsPerPage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        labelRowsPerPage="Cases per page"
        showFirstButton
        showLastButton
      />
    </Box>
  );
}
