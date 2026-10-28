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
  Checkbox,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useMemo, useRef, useState, type ChangeEvent, type JSX } from "react";
import { flushSync } from "react-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useLocation, useSearchParams } from "react-router";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useNavTransition } from "@hooks/useNavTransition";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
} from "@hooks/useColumnPreferences";
import QueryErrorState from "@components/QueryErrorState";
import FilteredExportMenu from "@components/FilteredExportMenu";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useFilterBarCollapsed } from "@hooks/useFilterBarCollapsed";
import { useBackendApi } from "@api/backend/client";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchIncidents } from "@features/csm-operations/api/useSearchIncidents";
import {
  buildIncidentSearchFilters,
  DEFAULT_INCIDENT_FILTERS,
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
  isIncidentTerminal,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
import {
  mapWithConcurrency,
  runBulkUpdate,
  type BulkFailure,
  type BulkResult,
} from "@features/csm-operations/utils/bulkUpdate";
import {
  assignIncidentToMePlan,
  bulkIncidentPlan,
  bulkWatchListNeedsCurrent,
  type BulkIncidentPlan,
} from "@features/csm-operations/utils/bulkIncidentEdit";
import { refreshIncidentRecord } from "@features/csm-operations/api/usePatchIncident";
import { incidentDetailQuery } from "@features/csm-operations/api/useGetIncident";
import { useRowSelection } from "@features/csm-operations/hooks/useRowSelection";
import BulkSelectionBar from "@features/csm-operations/components/BulkSelectionBar";
import BulkResultAlert from "@features/csm-operations/components/BulkResultAlert";
import BulkEditIncidentsDialog from "@features/csm-operations/components/BulkEditIncidentsDialog";
import {
  INCIDENT_FILTER_PARAM_KEYS,
  readIncidentFiltersFromUrl,
  writeIncidentFiltersToUrl,
} from "@features/csm-operations/utils/incidentsFiltersUrl";
import {
  DEFAULT_VISIBLE_INCIDENT_COLUMNS,
  INCIDENT_OPTIONAL_COLUMNS,
  type IncidentOptionalColumnId,
} from "@features/csm-operations/utils/incidentListColumns";
import IncidentsFilterBar from "@features/csm-operations/components/IncidentsFilterBar";
import SortableHeaderCell from "@features/csm-operations/components/SortableHeaderCell";
import type { ListSort } from "@features/csm-operations/utils/listSort";
import RefreshButton from "@components/RefreshButton";
import type {
  BeIncident,
  BeIncidentSearchPayload,
  BeIncidentSearchResponse,
  BePatchIncidentResponse,
  BeUpdateIncidentPayload,
} from "@api/backend/types";

const incidentId = (incident: BeIncident): string | null | undefined => incident.id;
// A closed or cancelled incident is closed for good: it takes no bulk edit.
const incidentSelectable = (incident: BeIncident): boolean => !isIncidentTerminal(incident.state);

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

const INCIDENT_OPTIONAL_COLUMN_IDS = Object.keys(
  INCIDENT_OPTIONAL_COLUMNS,
) as IncidentOptionalColumnId[];

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

function renderOptionalCell(id: IncidentOptionalColumnId, incident: BeIncident): JSX.Element {
  switch (id) {
    case "category":
      return <>{incident.category || "—"}</>;
    case "assignmentGroup":
      return <>{incident.assignmentGroup?.name || "—"}</>;
    case "assignedTo":
      return <>{incident.assignedTo?.name || "Unassigned"}</>;
    case "parent":
      return <>{incident.parent?.name || "—"}</>;
    case "createdBy":
      return <>{incident.createdBy || "—"}</>;
    case "updatedBy":
      return <>{incident.updatedBy || "—"}</>;
    case "createdOn":
      return <>{formatDate(incident.createdOn)}</>;
  }
}

/**
 * Incidents listing for the Operations → Incidents tab. Searches
 * `POST /incidents/search` with server-side pagination, free-text search,
 * and priority / SLA-violated / created-date-range / product filters (see
 * `IncidentsFilterBar`). Filter state lives in the URL (tab-prefixed `inc...`
 * params) rather than local state, so a plain tab switch doesn't reset it and
 * a filtered list can be bookmarked or shared.
 */
type IncidentSortField = "createdOn" | "updatedOn" | "openedOn";

export default function IncidentsTab(): JSX.Element {
  const navigate = useNavTransition();
  const api = useBackendApi();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<IncidentFilters>(
    () => readIncidentFiltersFromUrl(searchParams),
    [searchParams],
  );
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);
  const currentUserEmail = useIdTokenClaims()?.email;
  const currentUserId = useCurrentUser().user?.id;
  // "Assigned to me" needs our id; until `/users/me` supplies it, searching
  // would drop the filter and list everyone's incidents.
  const awaitingCurrentUser = filters.assignedToMe && !currentUserId;

  // Date columns sort by a click on their header; newest created first until then.
  const [sort, setSort] = useState<ListSort<IncidentSortField>>({ field: "createdOn", order: "desc" });
  const handleSort = (next: ListSort<IncidentSortField>): void => {
    setSort(next);
    setPage(0);
  };

  const payload = useMemo(
    () => ({
      filters: buildIncidentSearchFilters(filters, debouncedSearch, currentUserId),
      sortBy: sort,
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [filters, debouncedSearch, currentUserId, sort, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching, refetch, dataUpdatedAt } =
    useSearchIncidents(payload, { enabled: !awaitingCurrentUser });

  const incidents = useMemo(() => data?.incidents ?? [], [data]);
  const total = data?.total ?? 0;

  // "Customise columns" — see `caseListColumns.ts`'s equivalent doc comment;
  // the default visible set is empty since none of these were columns at
  // all before this feature, so a returning user sees no change until they
  // open the picker.
  const [isFiltersOpen, setIsFiltersOpen] = useFilterBarCollapsed(
    "incidents",
    getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    true,
  );
  const columnPrefs = useColumnPreferences({
    viewId: "incidents",
    userKey: getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    columns: INCIDENT_OPTIONAL_COLUMN_IDS.map((id) => ({
      id,
      label: INCIDENT_OPTIONAL_COLUMNS[id].label,
    })),
    defaultVisibleIds: DEFAULT_VISIBLE_INCIDENT_COLUMNS,
  });
  const visibleOptionalColumns = columnPrefs.visibleColumns.map(
    (c) => c.id as IncidentOptionalColumnId,
  );
  // Bulk actions (Edit, Assign to me) on the selected rows, for those who may write.
  const { canWrite } = usePortalAccess();
  const queryClient = useQueryClient();
  const selection = useRowSelection<BeIncident>(incidentId, incidentSelectable, incidents);
  const { clear: clearSelection, remove: removeFromSelection } = selection;
  const selectAllRef = useRef<HTMLInputElement>(null);
  const [bulkEditOpen, setBulkEditOpen] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkResult, setBulkResult] = useState<{ result: BulkResult; verb: string } | null>(null);
  /**
   * One bulk action over the selection, from each incident as the list shows
   * it now (a selected incident on the current page is its refetched row). One
   * closed or cancelled since it was picked is skipped with the reason. Adding
   * people to (or removing them from) the watch list needs each incident's own
   * list, so its detail is read first (`readWatchers`). After the run the dialog
   * is closed first, then each incident is refreshed the way a single edit
   * refreshes it (detail, tasks, the flows' follow-up).
   */
  const runBulk = async (
    planFor: (incident: BeIncident, currentWatchers?: string[]) => BulkIncidentPlan,
    verb: string,
    readWatchers = false,
  ): Promise<void> => {
    setBulkBusy(true);
    const rows = selection.selected;
    try {
      const watchers = readWatchers
        ? await mapWithConcurrency(rows, async (row) => {
            // Read now, never from the cache: the list sent back replaces the whole watch list.
            const detail = await queryClient.fetchQuery({ ...incidentDetailQuery(api, row.id!), staleTime: 0 });
            return detail ? (detail.watchList ?? []).map((w) => w.id) : undefined;
          })
        : [];
      const skipped: BulkFailure[] = [];
      const toSend: Array<{ id: string; number?: string; patch: BeUpdateIncidentPayload }> = [];
      rows.forEach((row, i) => {
        const number = row.number || row.id!;
        const plan = planFor(row, readWatchers ? watchers[i] : undefined);
        if ("skip" in plan) skipped.push({ id: row.id!, number, message: plan.skip });
        else toSend.push({ id: row.id!, number: row.number ?? undefined, patch: plan.patch });
      });
      const patches = new Map(toSend.map((t) => [t.id, t.patch]));
      const outcome = await runBulkUpdate(toSend, (t) =>
        api.patch<BeUpdateIncidentPayload, BePatchIncidentResponse>(
          `/incidents/${encodeURIComponent(t.id)}`,
          patches.get(t.id)!,
        ),
      );
      flushSync(() => {
        setBulkEditOpen(false);
        setBulkResult({ result: { ...outcome, skipped }, verb });
      });
      removeFromSelection(rows.map((r) => r.id!));
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.INCIDENTS] });
      for (const t of toSend) refreshIncidentRecord(queryClient, t.id, t.patch);
    } finally {
      setBulkBusy(false);
    }
  };
  const clearAndRefocus = (): void => {
    clearSelection();
    // The bar (and the button just pressed) goes away with the selection.
    selectAllRef.current?.focus();
  };

  const columnCount = (canWrite ? 8 : 7) + visibleOptionalColumns.length; // [select,] Number, Subject, Caller, State, Priority, Opened, Updated + optional

  const setFilters = useCallback(
    (next: IncidentFilters) => {
      setPage(0);
      // A selection is of the rows a filter showed: a new filter starts afresh.
      clearSelection();
      // Preserve any non-filter params (e.g. the active operations tab) and
      // any other tab's own filter params (e.g. the change-requests tab's),
      // rather than resetting the whole query string.
      const merged = new URLSearchParams(searchParams);
      INCIDENT_FILTER_PARAM_KEYS.forEach((k) => merged.delete(k));
      writeIncidentFiltersToUrl(next).forEach((v, k) => merged.set(k, v));
      // `replace: true` so switching tabs / paging doesn't spam browser
      // history — same rationale as the shared cases list view.
      setSearchParams(merged, { replace: true });
    },
    [searchParams, setSearchParams, clearSelection],
  );

  const handleFiltersChange = (next: IncidentFilters): void => {
    setFilters(next);
  };

  const handleReset = (): void => {
    setFilters(DEFAULT_INCIDENT_FILTERS);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  // Pages `/incidents/search` with the *currently applied* filters/sort
  // (same `filters`/`sortBy` as `payload` above, just re-built per page with
  // its own offset/limit instead of the table's) until the full filtered
  // result set has been fetched — see `useFilteredCsvExport`/`fetchAllPages`.
  // Bound fresh on every render via the hook's ref pattern, so a filter
  // change is picked up even mid-typing without this identity needing to be
  // stable.
  const fetchIncidentsPage = useCallback(
    async (offset: number, limit: number) => {
      const res = await api.post<BeIncidentSearchPayload, BeIncidentSearchResponse>(
        "/incidents/search",
        {
          filters: payload.filters,
          sortBy: payload.sortBy,
          pagination: { offset, limit },
        },
      );
      return { items: res.incidents ?? [], total: res.total ?? 0 };
    },
    [api, payload.filters, payload.sortBy],
  );

  const incidentToCsvRow = useCallback(
    (incident: BeIncident): string[] => [
      incident.number ?? "",
      incident.subject ?? "",
      incident.caller?.name ?? "",
      incidentStateLabel(incident.state),
      incidentPriorityLabel(incident.priority),
      formatDate(incident.openedOn),
      formatDate(incident.updatedOn),
    ],
    [],
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 1 }}>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh incidents"
        />
        <FilteredExportMenu<BeIncident>
          entityName="incidents"
          title="Incidents"
          header={["Number", "Subject", "Caller", "State", "Priority", "Opened", "Updated"]}
          toRow={incidentToCsvRow}
          fetchPage={fetchIncidentsPage}
          disabled={isError || awaitingCurrentUser}
        />
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/operations/incidents/new")}
        >
          Create incident
        </Button>
      </Box>

      <IncidentsFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
      />

      {bulkResult && (
        <BulkResultAlert
          result={bulkResult.result}
          nounPlural="incidents"
          verb={bulkResult.verb}
          onClose={() => setBulkResult(null)}
        />
      )}
      {canWrite && (
        <BulkSelectionBar
          count={selection.count}
          noun="incident"
          busy={bulkBusy}
          onEdit={() => setBulkEditOpen(true)}
          onAssignToMe={() => {
            if (currentUserId) void runBulk((incident) => assignIncidentToMePlan(incident, currentUserId), "Assigned to you");
          }}
          assignToMeDisabledReason={currentUserId ? null : "Your user record is still loading."}
          onClear={clearAndRefocus}
        />
      )}
      {bulkEditOpen && (
        <BulkEditIncidentsDialog
          incidents={selection.selected}
          isApplying={bulkBusy}
          onClose={() => setBulkEditOpen(false)}
          onApply={(edit) =>
            void runBulk(
              (incident, currentWatchers) => bulkIncidentPlan(incident, edit, currentWatchers),
              "Updated",
              bulkWatchListNeedsCurrent(edit),
            )
          }
        />
      )}

      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <ColumnCustomizerButton
          allColumns={columnPrefs.allColumns}
          isVisible={columnPrefs.isVisible}
          onToggle={columnPrefs.toggleColumn}
          onMove={columnPrefs.moveColumn}
          onReorder={columnPrefs.reorderColumn}
          onReset={columnPrefs.resetToDefault}
          label="Customise incident columns"
        />
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                {canWrite && (
                  <TableCell padding="checkbox">
                    <Checkbox
                      size="small"
                      inputRef={selectAllRef}
                      checked={selection.pageState(incidents).all}
                      indeterminate={selection.pageState(incidents).some}
                      // While a page loads the rows still held are the previous page's.
                      disabled={isFetching || bulkBusy || !incidents.some(incidentSelectable)}
                      onChange={() => selection.togglePage(incidents)}
                      inputProps={{ "aria-label": "Select all incidents on this page" }}
                    />
                  </TableCell>
                )}
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Caller</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Priority</TableCell>
                {visibleOptionalColumns.map((id) => (
                  id === "createdOn" ? (
                    <SortableHeaderCell
                      key={id}
                      label={INCIDENT_OPTIONAL_COLUMNS[id].label}
                      field="createdOn"
                      sort={sort}
                      onSort={handleSort}
                    />
                  ) : (
                    <TableCell key={id}>{INCIDENT_OPTIONAL_COLUMNS[id].label}</TableCell>
                  )
                ))}
                <SortableHeaderCell label="Opened" field="openedOn" sort={sort} onSort={handleSort} />
                <SortableHeaderCell label="Updated" field="updatedOn" sort={sort} onSort={handleSort} />
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching || awaitingCurrentUser ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {canWrite && <TableCell padding="checkbox" />}
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="85%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={72} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={60} height={22} /></TableCell>
                    {visibleOptionalColumns.map((id) => (
                      <TableCell key={id}><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    ))}
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={columnCount} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load incidents."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : incidents.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columnCount} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No incidents found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                incidents.map((incident, index) => (
                  <TableRow
                    // incident.id is nullable — fall back to the index so
                    // multiple incidents with a null id (an edge case the
                    // type allows) still get distinct React keys.
                    key={incident.id ?? `incident-${index}`}
                    hover
                    onClick={() =>
                      incident.id &&
                      navigate(`/operations/incidents/${incident.id}`, {
                        state: { from: `${location.pathname}${location.search}` },
                      })
                    }
                    selected={selection.isSelected(incident)}
                    sx={{ cursor: "pointer" }}
                  >
                    {canWrite && (
                      <TableCell padding="checkbox" onClick={(e) => e.stopPropagation()}>
                        {incidentSelectable(incident) || selection.isSelected(incident) ? (
                          <Checkbox
                            size="small"
                            checked={selection.isSelected(incident)}
                            disabled={bulkBusy}
                            onChange={() => selection.toggle(incident)}
                            inputProps={{
                              "aria-label": incidentSelectable(incident)
                                ? `Select ${incident.number ?? "incident"}`
                                : // Picked while open, since closed: it can still be taken out.
                                  `Deselect ${incident.number ?? "incident"}, now ${incidentStateLabel(incident.state).toLowerCase()}`,
                            }}
                          />
                        ) : (
                          <Tooltip title="A closed or cancelled incident can't be edited.">
                            <span>
                              <Checkbox
                                size="small"
                                disabled
                                inputProps={{
                                  "aria-label": `${incident.number ?? "Incident"} is ${incidentStateLabel(incident.state).toLowerCase()} and can't be selected`,
                                }}
                              />
                            </span>
                          </Tooltip>
                        )}
                      </TableCell>
                    )}
                    <TableCell>{incident.number || "—"}</TableCell>
                    <TableCell sx={{ maxWidth: 360 }}>
                      <Typography variant="body2" noWrap title={incident.subject ?? undefined}>
                        {incident.subject || "—"}
                      </Typography>
                    </TableCell>
                    <TableCell>{incident.caller?.name || "—"}</TableCell>
                    <TableCell>
                      {incident.state ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={incidentStateColor(incident.state)}
                          label={incidentStateLabel(incident.state)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {incident.priority ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={incidentPriorityColor(incident.priority)}
                          label={incidentPriorityLabel(incident.priority)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    {visibleOptionalColumns.map((id) => (
                      <TableCell key={id}>{renderOptionalCell(id, incident)}</TableCell>
                    ))}
                    <TableCell>{formatDate(incident.openedOn)}</TableCell>
                    <TableCell>{formatDate(incident.updatedOn)}</TableCell>
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
          onPageChange={(_: unknown, newPage: number) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          labelDisplayedRows={({ from, to, count }) => `Showing ${from}–${to} of ${count}`}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
