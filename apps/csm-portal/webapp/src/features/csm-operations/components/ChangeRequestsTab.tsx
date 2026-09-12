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
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useLocation, useSearchParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import FilteredCsvExportButton from "@components/FilteredCsvExportButton";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { getColumnPreferencesUserKey, useColumnPreferences } from "@hooks/useColumnPreferences";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
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
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";
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
import RefreshButton from "@components/RefreshButton";
import type {
  BeChangeRequestSearchPayload,
  BeChangeRequestSearchResponse,
  BeChangeRequestSearchView,
} from "@api/backend/types";

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

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

/**
 * Change-requests listing for the Operations → Change requests tab. Searches
 * `POST /change-requests/search` with server-side pagination and filters
 * (state, impact, closed date range, free-text search).
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
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: buildChangeRequestSearchFilters(filters, debouncedSearch),
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [filters, debouncedSearch, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching, refetch, dataUpdatedAt } =
    useSearchChangeRequests(payload);

  const changeRequests = data?.changeRequests ?? [];
  const total = data?.total ?? 0;

  // "Customise columns" — see `caseListColumns.ts`'s equivalent doc comment;
  // the default visible set below is exactly the table's pre-existing fixed
  // set so a returning user sees no change until they open the picker.
  const currentUserEmail = useIdTokenClaims()?.email;
  const currentUserId = useCurrentUser().user?.id;
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
  const columnCount = 4 + visibleOptionalColumns.length; // Number, Subject, State, Updated + optional

  const setFilters = useCallback(
    (next: ChangeRequestFilters) => {
      setPage(0);
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
    [searchParams, setSearchParams],
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
          pagination: { offset, limit },
        },
      );
      return { items: res.changeRequests ?? [], total: res.total ?? 0 };
    },
    [api, payload.filters],
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
        <FilteredCsvExportButton<BeChangeRequestSearchView>
          entityName="change-requests"
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
        onFiltersToggle={() => setIsFiltersOpen((prev: boolean) => !prev)}
      />

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
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                {visibleOptionalColumns.map((id) => (
                  <TableCell key={id}>{CHANGE_REQUEST_OPTIONAL_COLUMNS[id].label}</TableCell>
                ))}
                <TableCell>State</TableCell>
                <TableCell>Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
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
                        state: { from: `${location.pathname}${location.search}` },
                      })
                    }
                    sx={{ cursor: "pointer" }}
                  >
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
