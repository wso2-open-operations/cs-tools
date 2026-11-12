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
import { useLocation, useSearchParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import FilteredExportMenu from "@components/FilteredExportMenu";
import { ApiQueryKeys } from "@constants/apiConstants";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { getColumnPreferencesUserKey, useColumnPreferences } from "@hooks/useColumnPreferences";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useFilterBarCollapsed } from "@hooks/useFilterBarCollapsed";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { useBackendApi } from "@api/backend/client";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchChangeRequests } from "@features/csm-operations/api/useSearchChangeRequests";
import {
  buildChangeRequestSearchFilters,
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
  DEFAULT_CR_FILTERS,
  isChangeRequestTerminal,
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";
import {
  assignChangeRequestToMePlan,
  bulkChangeRequestPlan,
  type BulkChangeRequestPlan,
} from "@features/csm-operations/utils/bulkChangeRequestEdit";
import {
  mapWithConcurrency,
  runBulkUpdate,
  type BulkFailure,
  type BulkResult,
} from "@features/csm-operations/utils/bulkUpdate";
import { changeRequestDetailQuery } from "@features/csm-operations/api/useGetChangeRequest";
import { refreshChangeRequestRecord } from "@features/csm-operations/api/usePatchChangeRequest";
import { useRowSelection } from "@features/csm-operations/hooks/useRowSelection";
import BulkSelectionBar from "@features/csm-operations/components/BulkSelectionBar";
import BulkResultAlert from "@features/csm-operations/components/BulkResultAlert";
import BulkEditChangeRequestsDialog from "@features/csm-operations/components/BulkEditChangeRequestsDialog";
import {
  CR_FILTER_PARAM_KEYS,
  readChangeRequestFiltersFromUrl,
  writeChangeRequestFiltersToUrl,
} from "@features/csm-operations/utils/changeRequestsFiltersUrl";
import {
  CHANGE_REQUEST_OPTIONAL_COLUMNS,
  DEFAULT_VISIBLE_CHANGE_REQUEST_COLUMNS,
  type ChangeRequestOptionalColumnId,
} from "@features/csm-operations/utils/changeRequestListColumns";
import ChangeRequestsFilterBar from "@features/csm-operations/components/ChangeRequestsFilterBar";
import SortableHeaderCell from "@features/csm-operations/components/SortableHeaderCell";
import type { ListSort } from "@features/csm-operations/utils/listSort";
import RefreshButton from "@components/RefreshButton";
import type {
  BeChangeRequestSearchPayload,
  BeChangeRequestSearchResponse,
  BeChangeRequestDetail,
  BeChangeRequestSearchView,
  BePatchChangeRequestPayload,
  BePatchChangeRequestResponse,
} from "@api/backend/types";

const changeRequestId = (cr: BeChangeRequestSearchView): string | null | undefined => cr.id;
// A closed, canceled or rolled-back change request is over: it takes no bulk edit.
const changeRequestSelectable = (cr: BeChangeRequestSearchView): boolean => !isChangeRequestTerminal(cr.state);

const CHANGE_REQUEST_OPTIONAL_COLUMN_IDS = Object.keys(
  CHANGE_REQUEST_OPTIONAL_COLUMNS,
) as ChangeRequestOptionalColumnId[];

function renderOptionalCell(
  id: ChangeRequestOptionalColumnId,
  cr: BeChangeRequestSearchView,
): JSX.Element {
  switch (id) {
    case "project":
      return <>{cr.project?.name || "—"}</>;
    case "impact":
      return cr.impact ? (
        <Chip
          size="small"
          variant="outlined"
          color={changeRequestImpactColor(cr.impact)}
          label={changeRequestImpactLabel(cr.impact)}
        />
      ) : (
        <>—</>
      );
    case "plannedStart":
      return <>{formatDate(cr.plannedStartOn)}</>;
    case "plannedEnd":
      return <>{formatDate(cr.plannedEndOn)}</>;
    case "product":
      return <>{cr.product?.name || "—"}</>;
    case "assignedEngineer":
      return <>{cr.assignedEngineer?.name || "—"}</>;
    case "assignedTeam":
      return <>{cr.assignedTeam?.name || "—"}</>;
    case "type":
      return <>{cr.type || "—"}</>;
    case "case":
      return <>{cr.case?.name || "—"}</>;
    case "createdOn":
      return <>{formatDate(cr.createdOn)}</>;
  }
}

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

/** A backend timestamp as a short date ("Nov 4, 2026"), or "—" when it is missing or cannot be read. */
function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

type CrSortField = "createdOn" | "updatedOn" | "plannedStartOn" | "plannedEndOn";

/** The optional columns that are dates, and the sort field each one sorts by. */
const CR_SORT_FIELD_BY_COLUMN: Partial<Record<ChangeRequestOptionalColumnId, CrSortField>> = {
  plannedStart: "plannedStartOn",
  plannedEnd: "plannedEndOn",
  createdOn: "createdOn",
};

/**
 * Change-requests listing for the Operations → Change requests tab. Searches
 * `POST /change-requests/search` with server-side pagination and filters
 * (state, impact, closed date range, free-text search). A row opens its change
 * request with `{ from, parentState }`, so its Back returns here with this
 * page's own Back intact.
 */
export default function ChangeRequestsTab(): JSX.Element {
  const navigate = useNavTransition();
  const api = useBackendApi();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<ChangeRequestFilters>(
    () => readChangeRequestFiltersFromUrl(searchParams),
    [searchParams],
  );
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  // Date columns sort by a click on their header; newest created first until then.
  const [sort, setSort] = useState<ListSort<CrSortField>>({ field: "createdOn", order: "desc" });
  const handleSort = (next: ListSort<CrSortField>): void => {
    setSort(next);
    setPage(0);
  };

  const payload = useMemo(
    () => ({
      filters: buildChangeRequestSearchFilters(filters, debouncedSearch),
      sortBy: sort,
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [filters, debouncedSearch, sort, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching, refetch, dataUpdatedAt } =
    useSearchChangeRequests(payload);

  const changeRequests = useMemo(() => data?.changeRequests ?? [], [data]);
  const total = data?.total ?? 0;

  // "Customise columns" — see `caseListColumns.ts`'s equivalent doc comment;
  // the default visible set below is exactly the table's pre-existing fixed
  // set so a returning user sees no change until they open the picker.
  const currentUserEmail = useIdTokenClaims()?.email;
  const currentUserId = useCurrentUser().user?.id;
  const [isFiltersOpen, setIsFiltersOpen] = useFilterBarCollapsed(
    "change-requests",
    getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    true,
  );
  const columnPrefs = useColumnPreferences({
    viewId: "change-requests",
    userKey: getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    columns: CHANGE_REQUEST_OPTIONAL_COLUMN_IDS.map((id) => ({
      id,
      label: CHANGE_REQUEST_OPTIONAL_COLUMNS[id].label,
    })),
    defaultVisibleIds: DEFAULT_VISIBLE_CHANGE_REQUEST_COLUMNS,
  });
  const visibleOptionalColumns = columnPrefs.visibleColumns.map(
    (c) => c.id as ChangeRequestOptionalColumnId,
  );
  // Bulk actions (Edit, Assign to me) on the selected rows, for those who may write.
  const { canWrite } = usePortalAccess();
  const queryClient = useQueryClient();
  const selection = useRowSelection<BeChangeRequestSearchView>(
    changeRequestId,
    changeRequestSelectable,
    changeRequests,
  );
  const { clear: clearSelection, remove: removeFromSelection } = selection;
  const selectAllRef = useRef<HTMLInputElement>(null);
  const [bulkEditOpen, setBulkEditOpen] = useState(false);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkResult, setBulkResult] = useState<{ result: BulkResult; verb: string } | null>(null);
  /**
   * One bulk action over the selection. Each change request is planned from
   * what it is now (its detail, read again just before the run), not from the
   * row as it was when picked: one approved in the meantime then gets only its
   * state, hold and notes, as promised, and one whose detail can't be read is
   * skipped with the reason. After
   * the run the dialog is closed first, so its detail queries are gone before
   * the records are refreshed the way a single edit refreshes them.
   */
  const runBulk = async (planFor: (cr: BeChangeRequestDetail) => BulkChangeRequestPlan, verb: string): Promise<void> => {
    setBulkBusy(true);
    const rows = selection.selected;
    try {
      // Read now, never from the cache: the plan follows each one's state, hold and customer steps as they are.
      const current = await mapWithConcurrency(rows, (cr) =>
        queryClient.fetchQuery({ ...changeRequestDetailQuery(api, cr.id), staleTime: 0 }),
      );
      const skipped: BulkFailure[] = [];
      const toSend: Array<{ id: string; number?: string; patch: BePatchChangeRequestPayload }> = [];
      rows.forEach((row, i) => {
        const number = row.number || row.id;
        if (current[i] === null) {
          skipped.push({ id: row.id, number, message: "Not found: it may have been deleted." });
          return;
        }
        // Without its detail its hold, type and customer steps are unknown: nothing is sent blind.
        const detail = current[i];
        if (!detail) {
          skipped.push({ id: row.id, number, message: "Couldn't read it just now, so it was not changed. Try again." });
          return;
        }
        const plan = planFor(detail);
        if ("skip" in plan) skipped.push({ id: row.id, number, message: plan.skip });
        else toSend.push({ id: row.id, number: row.number, patch: plan.patch });
      });
      const patches = new Map(toSend.map((t) => [t.id, t.patch]));
      const outcome = await runBulkUpdate(toSend, (t) =>
        api.patch<BePatchChangeRequestPayload, BePatchChangeRequestResponse>(
          `/change-requests/${encodeURIComponent(t.id)}`,
          patches.get(t.id)!,
        ),
      );
      flushSync(() => {
        setBulkEditOpen(false);
        setBulkResult({ result: { ...outcome, skipped }, verb });
      });
      removeFromSelection(rows.map((r) => r.id));
      void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CHANGE_REQUESTS] });
      for (const t of toSend) refreshChangeRequestRecord(queryClient, t.id, t.patch);
    } finally {
      setBulkBusy(false);
    }
  };
  const clearAndRefocus = (): void => {
    clearSelection();
    // The bar (and the button just pressed) goes away with the selection.
    selectAllRef.current?.focus();
  };

  const columnCount = (canWrite ? 5 : 4) + visibleOptionalColumns.length; // [select,] Number, Subject, State, Updated + optional

  const setFilters = useCallback(
    (next: ChangeRequestFilters) => {
      setPage(0);
      // A selection is of the rows a filter showed: a new filter starts afresh.
      clearSelection();
      // Preserve any non-filter params (e.g. the active operations tab) and
      // any other tab's own filter params (e.g. the incidents tab's), rather
      // than resetting the whole query string.
      const merged = new URLSearchParams(searchParams);
      CR_FILTER_PARAM_KEYS.forEach((k) => merged.delete(k));
      writeChangeRequestFiltersToUrl(next).forEach((v, k) => merged.set(k, v));
      // `replace: true` so switching tabs / paging doesn't spam browser
      // history — same rationale as the shared cases list view.
      setSearchParams(merged, { replace: true });
    },
    [searchParams, setSearchParams, clearSelection],
  );

  const handleFiltersChange = (next: ChangeRequestFilters): void => {
    setFilters(next);
  };

  const handleReset = (): void => {
    setFilters(DEFAULT_CR_FILTERS);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  // Pages `/change-requests/search` with the currently applied filters
  // (same `filters` as `payload` above) until the full filtered result set
  // has been fetched — see `useFilteredCsvExport`/`fetchAllPages`. The CR
  // search response carries no `hasMore` (unlike incidents/cases), but
  // `fetchAllPages` only ever needs `total`, so that's not a problem here.
  const fetchChangeRequestsPage = useCallback(
    async (offset: number, limit: number) => {
      const res = await api.post<BeChangeRequestSearchPayload, BeChangeRequestSearchResponse>(
        "/change-requests/search",
        {
          filters: payload.filters,
          // The export follows the list's own order.
          sortBy: payload.sortBy,
          pagination: { offset, limit },
        },
      );
      return { items: res.changeRequests ?? [], total: res.total ?? 0 };
    },
    [api, payload.filters, payload.sortBy],
  );

  const changeRequestToCsvRow = useCallback(
    (cr: BeChangeRequestSearchView): string[] => [
      cr.number ?? "",
      cr.subject ?? "",
      cr.project?.name ?? "",
      changeRequestStateLabel(cr.state),
      changeRequestImpactLabel(cr.impact),
      formatDate(cr.plannedStartOn),
      formatDate(cr.updatedOn),
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
          label="Refresh change requests"
        />
        <FilteredExportMenu<BeChangeRequestSearchView>
          entityName="change-requests"
          title="Change requests"
          header={["Number", "Subject", "Project", "State", "Impact", "Planned start", "Updated"]}
          toRow={changeRequestToCsvRow}
          fetchPage={fetchChangeRequestsPage}
          disabled={isError}
        />
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/operations/change-requests/new")}
        >
          Create change request
        </Button>
      </Box>

      <ChangeRequestsFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
      />

      {bulkResult && (
        <BulkResultAlert
          result={bulkResult.result}
          nounPlural="change requests"
          verb={bulkResult.verb}
          onClose={() => setBulkResult(null)}
        />
      )}
      {canWrite && (
        <BulkSelectionBar
          count={selection.count}
          noun="change request"
          busy={bulkBusy}
          onEdit={() => setBulkEditOpen(true)}
          onAssignToMe={() => {
            if (currentUserId) void runBulk((cr) => assignChangeRequestToMePlan(cr, currentUserId), "Assigned to you");
          }}
          assignToMeDisabledReason={currentUserId ? null : "Your user record is still loading."}
          onClear={clearAndRefocus}
        />
      )}
      {bulkEditOpen && (
        <BulkEditChangeRequestsDialog
          changeRequests={selection.selected}
          isApplying={bulkBusy}
          onClose={() => setBulkEditOpen(false)}
          onApply={(edit) => void runBulk((cr) => bulkChangeRequestPlan(cr, edit), "Updated")}
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
          label="Customise change request columns"
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
                      checked={selection.pageState(changeRequests).all}
                      indeterminate={selection.pageState(changeRequests).some}
                      // While a page loads the rows still held are the previous page's.
                      disabled={isFetching || bulkBusy || !changeRequests.some(changeRequestSelectable)}
                      onChange={() => selection.togglePage(changeRequests)}
                      inputProps={{ "aria-label": "Select all change requests on this page" }}
                    />
                  </TableCell>
                )}
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                {visibleOptionalColumns.map((id) => (
                  CR_SORT_FIELD_BY_COLUMN[id] ? (
                    <SortableHeaderCell
                      key={id}
                      label={CHANGE_REQUEST_OPTIONAL_COLUMNS[id].label}
                      field={CR_SORT_FIELD_BY_COLUMN[id]!}
                      sort={sort}
                      onSort={handleSort}
                    />
                  ) : (
                    <TableCell key={id}>{CHANGE_REQUEST_OPTIONAL_COLUMNS[id].label}</TableCell>
                  )
                ))}
                <TableCell>State</TableCell>
                <SortableHeaderCell label="Updated" field="updatedOn" sort={sort} onSort={handleSort} />
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {canWrite && <TableCell padding="checkbox" />}
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    {visibleOptionalColumns.map((id) => (
                      <TableCell key={id}><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    ))}
                    <TableCell><Skeleton variant="rounded" width={72} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={columnCount} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load change requests."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : changeRequests.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columnCount} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No change requests found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                changeRequests.map((cr) => (
                  <TableRow
                    key={cr.id}
                    hover
                    onClick={() =>
                      navigate(`/operations/change-requests/${cr.id}`, {
                        // `parentState` keeps this tab's own state (e.g.
                        // `{ from: <dashboard> }`) for the detail page's Back.
                        state: {
                          from: `${location.pathname}${location.search}`,
                          parentState: location.state ?? null,
                        },
                      })
                    }
                    selected={selection.isSelected(cr)}
                    sx={{ cursor: "pointer" }}
                  >
                    {canWrite && (
                      <TableCell padding="checkbox" onClick={(e) => e.stopPropagation()}>
                        {changeRequestSelectable(cr) || selection.isSelected(cr) ? (
                          <Checkbox
                            size="small"
                            checked={selection.isSelected(cr)}
                            disabled={bulkBusy}
                            onChange={() => selection.toggle(cr)}
                            inputProps={{
                              "aria-label": changeRequestSelectable(cr)
                                ? `Select ${cr.number ?? "change request"}`
                                : // Picked while open, since finished: it can still be taken out.
                                  `Deselect ${cr.number ?? "change request"}, now ${changeRequestStateLabel(cr.state).toLowerCase()}`,
                            }}
                          />
                        ) : (
                          <Tooltip title="A closed, canceled or rolled-back change request can't be edited.">
                            <span>
                              <Checkbox
                                size="small"
                                disabled
                                inputProps={{
                                  "aria-label": `${cr.number ?? "Change request"} is ${changeRequestStateLabel(cr.state).toLowerCase()} and can't be selected`,
                                }}
                              />
                            </span>
                          </Tooltip>
                        )}
                      </TableCell>
                    )}
                    <TableCell>{cr.number || "—"}</TableCell>
                    <TableCell sx={{ maxWidth: 360 }}>
                      <Typography variant="body2" noWrap title={cr.subject ?? undefined}>
                        {cr.subject || "—"}
                      </Typography>
                    </TableCell>
                    {visibleOptionalColumns.map((id) => (
                      <TableCell key={id}>{renderOptionalCell(id, cr)}</TableCell>
                    ))}
                    <TableCell>
                      {cr.state ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={changeRequestStateColor(cr.state)}
                          label={changeRequestStateLabel(cr.state)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>{formatDate(cr.updatedOn)}</TableCell>
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
