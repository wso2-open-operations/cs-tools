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
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft, ChevronDown, ChevronUp, ListFilter, X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent, type ReactNode } from "react";
import { useLocation, useSearchParams } from "react-router";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import QueryErrorState from "@components/QueryErrorState";
import MultiSelectField from "@components/MultiSelectField";
import UserRefLink from "@components/UserRefLink";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
  type ColumnOption,
} from "@hooks/useColumnPreferences";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchProjects } from "@features/csm-projects/api/useSearchProjects";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import ProjectOnboardingStatusChip from "@features/csm-projects/components/ProjectOnboardingStatusChip";
import ProjectsAdvancedFilters, {
  ProjectFilterChips,
} from "@features/csm-projects/components/ProjectsAdvancedFilters";
import { ONBOARDING_STATUS_OPTIONS } from "@features/csm-projects/utils/projectAdvancedFields";
import {
  countActiveProjectFilters,
  EMPTY_PROJECT_FILTERS,
  projectFiltersToRequest,
  readProjectFilters,
  writeProjectFilters,
  type ProjectFilters,
} from "@features/csm-projects/utils/projectFilters";
import RefreshButton from "@components/RefreshButton";
import { formatUtcDateForDisplay } from "@utils/dateTime";
import type { Project, SearchProjectsRequest } from "@features/csm-projects/types/csmProjects";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

type ProjectColumnId =
  | "name"
  | "key"
  | "closureState"
  | "startDate"
  | "endDate"
  | "onboardingStatus"
  | "onboardingOwner"
  | "goLivePlanDate"
  | "goLiveDate"
  | "onboardingExpiryDate"
  | "productionReadinessCompletionDate";

const PROJECT_COLUMNS: { id: ProjectColumnId; label: string }[] = [
  { id: "name", label: "Name" },
  { id: "key", label: "Project key" },
  { id: "closureState", label: "State" },
  { id: "startDate", label: "Start" },
  { id: "endDate", label: "End" },
  { id: "onboardingStatus", label: "Onboarding status" },
  { id: "onboardingOwner", label: "Onboarding owner" },
  { id: "goLivePlanDate", label: "Go-live plan date" },
  { id: "goLiveDate", label: "Go-live date" },
  { id: "onboardingExpiryDate", label: "Onboarding expiry date" },
  { id: "productionReadinessCompletionDate", label: "Production readiness completion date" },
];

// Matches the table's original, always-shown set. Onboarding status/owner are
// opt-in, same as the accounts table's own account manager/technical owner/CRE
// team columns — most projects have no onboarding engagement at all, so they'd
// show as a bare dash for the common case. The four onboarding date columns
// are opt-in for the same reason.
const DEFAULT_PROJECT_COLUMN_IDS: ProjectColumnId[] = [
  "name",
  "key",
  "closureState",
  "startDate",
  "endDate",
];

/** The onboarding dates are calendar days stored at midnight UTC, so show the
 *  UTC day (not the viewer's local one, which can be the day before). */
function formatOnboardingDate(value?: string | null): string {
  return formatUtcDateForDisplay(value) ?? "—";
}

function renderProjectCell(id: ProjectColumnId, p: Project): ReactNode {
  switch (id) {
    case "name":
      return (
        <Typography variant="body2" noWrap title={p.name}>
          {p.name}
        </Typography>
      );
    case "key":
      return p.key;
    case "closureState":
      return <ClosureStateChip closureState={p.closureState} emptyFallback="—" />;
    case "startDate":
      return formatDate(p.startDate);
    case "endDate":
      return formatDate(p.endDate);
    case "goLivePlanDate":
      return formatOnboardingDate(p.goLivePlanDate);
    case "goLiveDate":
      return formatOnboardingDate(p.goLiveDate);
    case "onboardingExpiryDate":
      return formatOnboardingDate(p.onboardingExpiryDate);
    case "productionReadinessCompletionDate":
      return formatOnboardingDate(p.productionReadinessCompletionDate);
    case "onboardingStatus":
      return <ProjectOnboardingStatusChip onboardingStatus={p.onboardingStatus} emptyFallback="—" />;
    case "onboardingOwner":
      // Gated on onboardingStatus too, matching the detail page's own
      // "onboarding enabled" condition — an owner left over on a row whose
      // status reads "Not-Applicable" (or is missing) must not render,
      // since that status means there's no active onboarding engagement.
      return p.onboardingStatus && p.onboardingStatus !== "Not-Applicable" && p.onboardingOwner ? (
        // UserRefLink renders its own link to the owner's profile; without
        // this, a click (or Enter while it's focused) bubbles up into the
        // row's own onClick/onKeyDown below and navigates to the project
        // instead of the owner's profile.
        <span onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
          <UserRefLink
            name={p.onboardingOwner.name}
            email={p.onboardingOwner.email || undefined}
            userId={p.onboardingOwner.id}
          />
        </span>
      ) : (
        "—"
      );
  }
}

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleDateString("en-US", {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
}

export default function CsmProjectsPage(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  // Set by a dashboard widget's click-through, since this page has no
  // dashboard context of its own.
  const backState = location.state as { from?: string } | undefined;
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [filtersOpen, setFiltersOpen] = useState(false);
  // Derived straight from the URL on every render (not a `useState`
  // initializer, which only runs once at mount): a same-route navigation that
  // doesn't remount this page (one dashboard tile's click-through followed by
  // another, both landing on /customers/projects, or browser back/forward)
  // still reads the URL's current filters. `handleFiltersChange` writes the
  // controls' choices to the URL rather than to separate local state, so there
  // is only one source of truth and nothing to re-sync.
  const searchParamsKey = searchParams.toString();
  const filters = useMemo(
    () => readProjectFilters(new URLSearchParams(searchParamsKey)),
    [searchParamsKey],
  );
  const activeFilterCount = countActiveProjectFilters(filters);
  // Resets pagination whenever the URL's filters change for any reason,
  // including a same-route external navigation `handleFiltersChange` below
  // didn't cause (and so didn't already reset itself) — syncing page state to
  // that external source is exactly what the QuickNav.tsx precedent below
  // disables this rule for.
  const filtersKey = useMemo(
    () => writeProjectFilters(new URLSearchParams(), filters).toString(),
    [filters],
  );
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs page state to an external source, the URL's filter params, same pattern as QuickNav.tsx's initial q/goto sync
    setPage(0);
  }, [filtersKey]);

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const handleFiltersChange = (next: ProjectFilters): void => {
    setPage(0);
    setSearchParams((prev) => writeProjectFilters(prev, next), { replace: true });
  };

  const request = useMemo<SearchProjectsRequest>(
    () => ({
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      searchQuery: debouncedSearch.trim() || undefined,
      ...projectFiltersToRequest(filters),
    }),
    [debouncedSearch, filters, page, rowsPerPage],
  );

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchProjects(request);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const projects = data?.projects ?? [];
  const total = data?.total ?? 0;

  const columnOptions = useMemo<ColumnOption[]>(
    () => PROJECT_COLUMNS.map(({ id, label }) => ({ id, label })),
    [],
  );
  const currentUser = useCurrentUser().user;
  const currentUserEmail = useIdTokenClaims()?.email;
  const columnPrefs = useColumnPreferences({
    viewId: "projects",
    userKey: getColumnPreferencesUserKey({ id: currentUser?.id, email: currentUserEmail }),
    columns: columnOptions,
    defaultVisibleIds: DEFAULT_PROJECT_COLUMN_IDS,
  });
  const visibleColumnIds = columnPrefs.visibleColumns.map((c) => c.id as ProjectColumnId);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backState?.from && (
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backState.from as string)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
      )}

      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Search across project name, project key, and subscription type.
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label="Refresh projects"
          />
          <ColumnCustomizerButton
            allColumns={columnPrefs.allColumns}
            isVisible={columnPrefs.isVisible}
            onToggle={columnPrefs.toggleColumn}
            onMove={columnPrefs.moveColumn}
            onReorder={columnPrefs.reorderColumn}
            onReset={columnPrefs.resetToDefault}
            label="Customise projects columns"
          />
        </Box>
      </Box>

      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2 }}>
        <TextField
          size="small"
          label="Search projects"
          placeholder="Search projects by name, key, or subscription"
          value={searchInput}
          onChange={handleSearchChange}
          slotProps={{ htmlInput: { "aria-label": "Search projects by name, key, or subscription type" } }}
          sx={{ maxWidth: 480, flex: "1 1 320px" }}
        />
        <Box sx={{ width: 240 }}>
          <MultiSelectField
            id="projects-filter-onboarding-status"
            label="Onboarding status"
            values={filters.onboardingStatus}
            options={ONBOARDING_STATUS_OPTIONS}
            onChange={(next) => handleFiltersChange({ ...filters, onboardingStatus: next })}
          />
        </Box>
        <Button
          variant="outlined"
          size="small"
          onClick={() => setFiltersOpen((o) => !o)}
          aria-expanded={filtersOpen}
          startIcon={<ListFilter size={16} />}
          endIcon={filtersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        >
          {activeFilterCount > 0 ? `Filters (${activeFilterCount})` : "Filters"}
        </Button>
        {activeFilterCount > 0 && (
          <Button
            variant="text"
            size="small"
            startIcon={<X size={16} />}
            onClick={() => handleFiltersChange(EMPTY_PROJECT_FILTERS)}
          >
            Clear filters
          </Button>
        )}
      </Box>

      <ProjectFilterChips filters={filters} panelOpen={filtersOpen} onChange={handleFiltersChange} />

      {filtersOpen && <ProjectsAdvancedFilters filters={filters} onChange={handleFiltersChange} />}

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                {visibleColumnIds.map((id) => (
                  <TableCell key={id}>{PROJECT_COLUMNS.find((c) => c.id === id)?.label}</TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {visibleColumnIds.map((id) => (
                      <TableCell key={id}>
                        <Skeleton variant="rounded" width="70%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load projects."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : projects.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No projects found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                projects.map((p) => {
                  const goToProject = (): void =>
                    navigate(`/customers/projects/${p.id}`, {
                      state: { from: `${location.pathname}${location.search}` },
                    });
                  const handleRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>): void => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      goToProject();
                    }
                  };
                  return (
                    <TableRow
                      key={p.id}
                      hover
                      onClick={goToProject}
                      onKeyDown={handleRowKeyDown}
                      tabIndex={0}
                      aria-label={`View project ${p.name}`}
                      sx={{
                        cursor: "pointer",
                        "&:focus-visible": {
                          outline: "2px solid",
                          outlineColor: "primary.main",
                          outlineOffset: -2,
                        },
                      }}
                    >
                      {visibleColumnIds.map((id) => (
                        <TableCell key={id} sx={id === "name" ? { maxWidth: 320 } : undefined}>
                          {renderProjectCell(id, p)}
                        </TableCell>
                      ))}
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
