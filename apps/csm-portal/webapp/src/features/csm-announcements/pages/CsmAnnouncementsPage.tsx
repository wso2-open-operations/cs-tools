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
  IconButton,
  InputAdornment,
  LinearProgress,
  Skeleton,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tabs,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent, type ReactNode } from "react";
import { Link as RouterLink, useSearchParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
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
import { useSearchAnnouncementRegistry } from "@features/csm-announcements/api/useSearchAnnouncementRegistry";
import { useSearchAnnouncementRequests } from "@features/csm-announcements/api/useSearchAnnouncementRequests";
import AnnouncementRequestDialog from "@features/csm-announcements/components/AnnouncementRequestDialog";
import {
  DEFAULT_ANNOUNCEMENT_FILTERS,
  type AnnouncementFilters,
} from "@features/csm-announcements/types/csmAnnouncements";
import type {
  AnnouncementRegistryCaseMember,
  AnnouncementRegistryRow,
} from "@features/csm-announcements/types/announcementRegistry";
import type { AnnouncementRequestState } from "@features/csm-announcements/types/announcementRequests";
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

type RegistryTabId = "announcements" | "pending";

/**
 * The "Pending" tab's own state sub-filter. `published` was originally
 * excluded here on the reasoning that a published request already shows as
 * a real case in the "Announcements" tab, so listing it here too would just
 * be a duplicate — but that tab's own row link only ever opens the case
 * itself (`/announcements/:caseId`), not `AnnouncementRequestDialog`, so a
 * published request had no click-path at all to its own "Post an
 * update"/"Past updates" panel (which only requires `state === "published"`
 * and only ever renders inside this dialog). Kept, not dropped: `published`
 * is now included so that request-level view stays reachable, alongside the
 * case view the Announcements tab already provides. The backend's search
 * only accepts one `state` at a time (no `in` list — see
 * `SearchAnnouncementRequestsPayload`), so this is a single-select rather
 * than the multi-select the Announcements tab's own state filter uses;
 * there's no single call that can show every state merged into one
 * paginated list.
 */
const PENDING_STATE_OPTIONS: { value: AnnouncementRequestState; label: string }[] = [
  { value: "draft", label: "Draft" },
  { value: "pending_approval", label: "Pending approval" },
  { value: "approved", label: "Approved" },
  { value: "published", label: "Published" },
];

const PENDING_ROWS_PER_PAGE = 10;

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

// row.kind decides what each column actually shows: a "case" row is exactly
// what CsmAnnouncementRow used to render (one case, one project, one state);
// a "batch" row represents every case a published announcement created
// collapsed into one row, so columns with no single-value meaning across a
// batch (Number, Reference, State) show "—", and Project shows the count
// instead of a single project name.
function renderRegistryCell(id: AnnouncementColumnId, row: AnnouncementRegistryRow): ReactNode {
  switch (id) {
    case "number":
      return row.kind === "case" ? row.caseNumber || "—" : "—";
    case "wso2CaseId":
      return row.kind === "case" ? row.wso2CaseId || "—" : "—";
    case "subject":
      return (
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <Typography
            variant="body2"
            title={row.subject}
            sx={{
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {row.subject}
          </Typography>
          {row.isSecurityAnnouncement && (
            <Chip size="small" color="warning" label="Security" sx={{ flexShrink: 0 }} />
          )}
        </Box>
      );
    case "project":
      return row.kind === "batch"
        ? `${row.projectCount ?? 0} project${row.projectCount === 1 ? "" : "s"}`
        : row.projectName;
    case "state":
      if (row.kind === "batch") {
        return <SemanticChip role="success" label="Published" variant="outlined" />;
      }
      return row.state ? (
        <SemanticChip
          role={announcementStateRole(row.state as CaseState)}
          label={STATE_LABEL[row.state as CaseState] ?? row.state}
          variant="outlined"
        />
      ) : (
        "—"
      );
    case "createdBy":
      return row.createdBy || "—";
    case "createdAt":
      return formatDate(row.createdOn);
    case "updatedAt":
      return formatDate(row.updatedOn);
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
  const { canWrite } = usePortalAccess();
  const [searchParams] = useSearchParams();
  // Seeded once from `?tab=pending` (e.g. the create form's post-save
  // redirect landing straight on the request just saved), not kept in sync
  // afterward — clicking the tabs below only updates local state, matching
  // this page's existing non-URL-driven tab pattern (see PENDING_STATE_OPTIONS'
  // own doc comment on why this page doesn't use the nav-tree tab system).
  const [tab, setTab] = useState<RegistryTabId>(
    searchParams.get("tab") === "pending" ? "pending" : "announcements",
  );
  const [filters, setFilters] = useState<AnnouncementFilters>(DEFAULT_ANNOUNCEMENT_FILTERS);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchAnnouncementRegistry({ ...filters, search: debouncedSearch }, page, rowsPerPage);

  const registryRows = data?.rows ?? [];
  const total = data?.total ?? 0;

  const [pendingState, setPendingState] = useState<AnnouncementRequestState>("pending_approval");
  const [pendingPage, setPendingPage] = useState(0);
  const [selectedRequestId, setSelectedRequestId] = useState<string | null>(null);
  // Only ever populated by clicking a batch row below — the registry search
  // result is the one place this data exists (see AnnouncementRegistryRow's
  // own doc comment); a dialog opened from the Pending tab has none, and the
  // dialog itself already treats an empty/undefined list as "nothing to
  // show" the same way it already does for a legacy published request with
  // no publishedCaseIds at all.
  const [selectedCaseMembers, setSelectedCaseMembers] = useState<AnnouncementRegistryCaseMember[]>([]);
  const pendingSearch = useSearchAnnouncementRequests(pendingState, pendingPage, PENDING_ROWS_PER_PAGE);
  const pendingRequests = pendingSearch.data?.requests ?? [];
  const pendingTotal = pendingSearch.data?.total ?? 0;

  const openBatchRow = (row: AnnouncementRegistryRow): void => {
    if (!row.announcementRequestId) return;
    setSelectedCaseMembers(row.cases ?? []);
    setSelectedRequestId(row.announcementRequestId);
  };
  const openPendingRow = (id: string): void => {
    setSelectedCaseMembers([]);
    setSelectedRequestId(id);
  };

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
          {canWrite && (
            <Button
              variant="contained"
              color="primary"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => navigate("/announcements/new")}
            >
              New announcement
            </Button>
          )}
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

      <Tabs
        value={tab}
        onChange={(_, v) => setTab(v as RegistryTabId)}
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        <Tab value="announcements" label="Announcements" />
        <Tab value="pending" label="Requests" />
      </Tabs>

      {tab === "announcements" && (
        <>
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
              ) : registryRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={visibleColumnIds.length} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No announcements found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                registryRows.map((row) =>
                  // A "case" row is an individual case with no owning
                  // announcement request — a real anchor (not a click-handler
                  // row) so it supports cmd/middle-click "open in new tab" and
                  // exposes a copyable URL, same rationale as CasesList's row
                  // links (ISSU-031). A "batch" row represents every case a
                  // published announcement request created collapsed into
                  // one row — clicking it opens that request's own dialog
                  // (the same one the Pending tab already uses) rather than
                  // navigating to any single case.
                  row.kind === "case" ? (
                    <TableRow
                      key={`case-${row.caseId}`}
                      hover
                      component={RouterLink}
                      to={`/announcements/${row.caseId}`}
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
                          {renderRegistryCell(id, row)}
                        </TableCell>
                      ))}
                    </TableRow>
                  ) : (
                    <TableRow
                      key={`batch-${row.announcementRequestId}`}
                      hover
                      onClick={() => openBatchRow(row)}
                      onKeyDown={(e: KeyboardEvent<HTMLTableRowElement>) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          openBatchRow(row);
                        }
                      }}
                      tabIndex={0}
                      aria-label={`View announcement request: ${row.subject || "(no subject)"}`}
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
                          {renderRegistryCell(id, row)}
                        </TableCell>
                      ))}
                    </TableRow>
                  ),
                )
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
        </>
      )}

      {tab === "pending" && (
        <>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap", alignItems: "center" }}>
            <Box sx={{ flex: "1 1 220px", minWidth: 200 }}>
              <MultiSelectField
                id="pending-announcement-requests-filter-state"
                label="State"
                values={[pendingState]}
                options={PENDING_STATE_OPTIONS}
                // Single-select in practice — see PENDING_STATE_OPTIONS' own
                // doc comment on why the backend can't merge multiple states
                // into one paginated list. Picking a second value swaps to
                // it rather than adding to a selection.
                onChange={(next) => {
                  const last = next[next.length - 1];
                  if (last) {
                    setPendingState(last);
                    setPendingPage(0);
                  }
                }}
              />
            </Box>
          </Box>

          <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
            <Box sx={{ height: 2 }}>
              {pendingSearch.isFetching && !pendingSearch.isLoading && (
                <LinearProgress sx={{ height: 2 }} />
              )}
            </Box>
            <TableContainer>
              <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
                <TableHead>
                  <TableRow sx={{ bgcolor: "action.hover" }}>
                    <TableCell sx={{ width: "36%" }}>Subject</TableCell>
                    <TableCell>Kind</TableCell>
                    <TableCell>Created by</TableCell>
                    <TableCell>Created</TableCell>
                    <TableCell>Updated</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {pendingSearch.isLoading ? (
                    Array.from({ length: PENDING_ROWS_PER_PAGE }).map((_, i) => (
                      <TableRow key={i}>
                        {Array.from({ length: 5 }).map((_v, j) => (
                          <TableCell key={j}>
                            <Skeleton variant="rounded" width="80%" height={18} />
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  ) : pendingSearch.isError ? (
                    <TableRow>
                      <TableCell colSpan={5} align="center">
                        <QueryErrorState
                          message={
                            pendingSearch.error instanceof Error && pendingSearch.error.message.trim()
                              ? pendingSearch.error.message
                              : "Failed to load pending announcement requests."
                          }
                          error={pendingSearch.error}
                        />
                      </TableCell>
                    </TableRow>
                  ) : pendingRequests.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                        <Typography variant="body2" color="text.secondary">
                          No {PENDING_STATE_OPTIONS.find((o) => o.value === pendingState)?.label.toLowerCase()}{" "}
                          requests.
                        </Typography>
                      </TableCell>
                    </TableRow>
                  ) : (
                    pendingRequests.map((r) => (
                      <TableRow
                        key={r.id}
                        hover
                        onClick={() => openPendingRow(r.id)}
                        onKeyDown={(e: KeyboardEvent<HTMLTableRowElement>) => {
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            openPendingRow(r.id);
                          }
                        }}
                        tabIndex={0}
                        aria-label={`View announcement request: ${r.subject || "(no subject)"}`}
                        sx={{
                          cursor: "pointer",
                          "&:focus-visible": {
                            outline: "2px solid",
                            outlineColor: "primary.main",
                            outlineOffset: -2,
                          },
                        }}
                      >
                        <TableCell sx={{ maxWidth: 360 }}>
                          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                            <Typography
                              variant="body2"
                              title={r.subject}
                              sx={{
                                display: "-webkit-box",
                                WebkitLineClamp: 2,
                                WebkitBoxOrient: "vertical",
                                overflow: "hidden",
                              }}
                            >
                              {r.subject || "(no subject)"}
                            </Typography>
                            {r.isSecurityAnnouncement && (
                              <Chip size="small" color="warning" label="Security" sx={{ flexShrink: 0 }} />
                            )}
                          </Box>
                        </TableCell>
                        <TableCell>{r.kind === "eol" ? "EOL" : "Customer"}</TableCell>
                        <TableCell>{r.createdByEmail || r.createdBy || "—"}</TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDate(r.createdAt)}</TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDate(r.updatedAt)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
            <TablePagination
              component="div"
              count={pendingTotal}
              page={pendingPage}
              onPageChange={(_, newPage) => setPendingPage(newPage)}
              rowsPerPage={PENDING_ROWS_PER_PAGE}
              rowsPerPageOptions={[PENDING_ROWS_PER_PAGE]}
            />
          </Box>
        </>
      )}

      {selectedRequestId && (
        <AnnouncementRequestDialog
          requestId={selectedRequestId}
          caseMembers={selectedCaseMembers}
          onClose={() => setSelectedRequestId(null)}
        />
      )}
    </Box>
  );
}
