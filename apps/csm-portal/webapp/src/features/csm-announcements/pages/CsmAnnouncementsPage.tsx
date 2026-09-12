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
  IconButton,
  InputAdornment,
  LinearProgress,
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
import { Plus, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX, type ReactNode } from "react";
import { Link as RouterLink } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import MultiSelectField from "@components/MultiSelectField";
import QueryErrorState from "@components/QueryErrorState";
import SemanticChip from "@components/SemanticChip";
import AsyncProjectMultiSelect from "@features/csm-cases/components/AsyncProjectMultiSelect";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
  type ColumnOption,
} from "@hooks/useColumnPreferences";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchAnnouncements } from "@features/csm-announcements/api/useSearchAnnouncements";
import {
  DEFAULT_ANNOUNCEMENT_FILTERS,
  type AnnouncementFilters,
  type CsmAnnouncementRow,
} from "@features/csm-announcements/types/csmAnnouncements";
import { announcementStateRole } from "@features/csm-announcements/utils/announcementState";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";
import RefreshButton from "@components/RefreshButton";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

// Every field `CsmAnnouncementRow` carries is offered as a column below
// except `id` (a raw UUID, never human-facing). Fields on the underlying
// case search view that never made it into `CsmAnnouncementRow` at all —
// `severity` (announcements carry no severity, see `AnnouncementFilters`'s
// doc), `issueType`, `deployment`/`deployedProduct`/`product`, and
// `assignedEngineer` — aren't meaningful for an announcement (a broadcast,
// not a worked case with an owner or an affected deployment), so there was
// nothing there worth adding as a column either.
type AnnouncementColumnId =
  | "number"
  | "wso2CaseId"
  | "subject"
  | "project"
  | "state"
  | "createdBy"
  | "createdAt"
  | "updatedAt";

const ANNOUNCEMENT_COLUMNS: { id: AnnouncementColumnId; label: string }[] = [
  { id: "number", label: "Number" },
  { id: "wso2CaseId", label: "Reference" },
  { id: "subject", label: "Subject" },
  { id: "project", label: "Project" },
  { id: "state", label: "State" },
  { id: "createdBy", label: "Created by" },
  { id: "createdAt", label: "Created" },
  { id: "updatedAt", label: "Updated" },
];

// Matches the list's original, always-shown set — "Created" is the one
// available-but-not-default column (the row already carries `createdAt`,
// just not surfaced until now).
const DEFAULT_ANNOUNCEMENT_COLUMN_IDS: AnnouncementColumnId[] = [
  "number",
  "subject",
  "project",
  "state",
  "createdBy",
  "updatedAt",
];

// `reopened` is intentionally excluded — it only appears as a `nextStates`
// signal, never as a case's own state (see CaseState's doc).
const STATE_OPTIONS: { value: CaseState; label: string }[] = (
  [
    "open",
    "work_in_progress",
    "solution_proposed",
    "awaiting_info",
    "waiting_on_wso2",
    "closed",
  ] as CaseState[]
).map((s) => ({ value: s, label: STATE_LABEL[s] }));

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

function renderAnnouncementCell(id: AnnouncementColumnId, a: CsmAnnouncementRow): ReactNode {
  switch (id) {
    case "number":
      return a.number || "—";
    case "wso2CaseId":
      return a.wso2CaseId || "—";
    case "subject":
      return (
        <Typography
          variant="body2"
          title={a.subject}
          sx={{
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {a.subject}
        </Typography>
      );
    case "project":
      return a.projectName;
    case "state":
      return a.state ? (
        <SemanticChip
          role={announcementStateRole(a.state)}
          label={STATE_LABEL[a.state] ?? a.state}
          variant="outlined"
        />
      ) : (
        "—"
      );
    case "createdBy":
      return a.createdBy || "—";
    case "createdAt":
      return formatDate(a.createdAt);
    case "updatedAt":
      return formatDate(a.updatedAt);
  }
}

/**
 * Announcements list. Announcements are cases of `type: "announcement"`
 * surfaced via `POST /cases/search`. Filterable by state and project (all
 * default to "show all"). Creating one is handled by
 * `CsmAnnouncementCreatePage` ("New announcement" below); unpublishing isn't
 * built yet.
 */
export default function CsmAnnouncementsPage(): JSX.Element {
  const navigate = useNavTransition();
  const [filters, setFilters] = useState<AnnouncementFilters>(DEFAULT_ANNOUNCEMENT_FILTERS);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchAnnouncements({ ...filters, search: debouncedSearch }, page, rowsPerPage);

  const announcements = data?.announcements ?? [];
  const total = data?.total ?? 0;

  const columnOptions = useMemo<ColumnOption[]>(
    () => ANNOUNCEMENT_COLUMNS.map(({ id, label }) => ({ id, label })),
    [],
  );
  const currentUser = useCurrentUser().user;
  const currentUserEmail = useIdTokenClaims()?.email;
  const columnPrefs = useColumnPreferences({
    viewId: "announcements",
    userKey: getColumnPreferencesUserKey({ id: currentUser?.id, email: currentUserEmail }),
    columns: columnOptions,
    defaultVisibleIds: DEFAULT_ANNOUNCEMENT_COLUMN_IDS,
  });
  const visibleColumnIds = columnPrefs.visibleColumns.map((c) => c.id as AnnouncementColumnId);

  // Any filter change resets to the first page.
  const patchFilters = (patch: Partial<AnnouncementFilters>): void => {
    setFilters((prev) => ({ ...prev, ...patch }));
    setPage(0);
  };

  const activeFilterCount = filters.states.length + filters.projectIds.length;

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Box>
          <Typography variant="h5">Announcements</Typography>
          <Typography variant="body2" color="text.secondary">
            Customer-facing announcements published across projects and tiers.
          </Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Button
            variant="contained"
            color="primary"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={() => navigate("/announcements/new")}
          >
            New announcement
          </Button>
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label="Refresh announcements"
          />
          <ColumnCustomizerButton
            allColumns={columnPrefs.allColumns}
            isVisible={columnPrefs.isVisible}
            onToggle={columnPrefs.toggleColumn}
            onMove={columnPrefs.moveColumn}
            onReorder={columnPrefs.reorderColumn}
            onReset={columnPrefs.resetToDefault}
            label="Customise announcements columns"
          />
        </Box>
      </Box>

      {/* Filters — search + state + project, all "show all" by default */}
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", alignItems: "center" }}>
        <Box sx={{ flex: "1 1 260px", minWidth: 220 }}>
          <TextField
            fullWidth
            size="small"
            placeholder="Search by subject or number…"
            value={filters.search}
            onChange={(e) => patchFilters({ search: e.target.value })}
            slotProps={{
              htmlInput: { "aria-label": "Search announcements" },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
                endAdornment: filters.search ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => patchFilters({ search: "" })}
                      aria-label="Clear search"
                    >
                      <X size={16} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>
        <Box sx={{ flex: "1 1 160px", minWidth: 150 }}>
          <MultiSelectField
            id="announcements-filter-state"
            label="State"
            values={filters.states}
            options={STATE_OPTIONS}
            onChange={(next) => patchFilters({ states: next })}
          />
        </Box>
        <Box sx={{ flex: "1 1 220px", minWidth: 200 }}>
          <AsyncProjectMultiSelect
            id="announcements-filter-project"
            label="Project"
            values={filters.projectIds}
            onChange={(next) => patchFilters({ projectIds: next })}
          />
        </Box>
        {activeFilterCount > 0 && (
          <Button
            variant="text"
            size="small"
            color="primary"
            startIcon={<X size={16} />}
            onClick={() => patchFilters({ states: [], projectIds: [] })}
          >
            Clear filters
          </Button>
        )}
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        {/* Thin bar during a background refetch (page / filter change) so the
            table isn't blanked to skeletons — cached rows stay visible. */}
        <Box sx={{ height: 2 }}>
          {isFetching && !isLoading && <LinearProgress sx={{ height: 2 }} />}
        </Box>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                {visibleColumnIds.map((id) => (
                  <TableCell key={id} sx={id === "subject" ? { width: "28%" } : undefined}>
                    {ANNOUNCEMENT_COLUMNS.find((c) => c.id === id)?.label}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {visibleColumnIds.map((id) => (
                      <TableCell key={id}>
                        <Skeleton variant="rounded" width="80%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load announcements."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : announcements.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No announcements found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                announcements.map((a) => (
                  // A real anchor (not a click-handler row) so it supports
                  // cmd/middle-click "open in new tab" and exposes a copyable
                  // URL — same rationale as CasesList's row links (ISSU-031).
                  <TableRow
                    key={a.id}
                    hover
                    component={RouterLink}
                    to={`/announcements/${a.id}`}
                    sx={{
                      cursor: "pointer",
                      textDecoration: "none",
                      color: "inherit",
                      "&:focus-visible": {
                        outline: (t) => `2px solid ${t.palette.primary.main}`,
                        outlineOffset: -2,
                      },
                    }}
                  >
                    {visibleColumnIds.map((id) => (
                      <TableCell
                        key={id}
                        sx={
                          id === "subject"
                            ? { width: "28%", maxWidth: 360 }
                            : id === "updatedAt" || id === "createdAt"
                              ? { whiteSpace: "nowrap" }
                              : undefined
                        }
                      >
                        {renderAnnouncementCell(id, a)}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
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
        />
      </Box>
    </Box>
  );
}
