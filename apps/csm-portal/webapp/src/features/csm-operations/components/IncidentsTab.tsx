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
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useNavTransition } from "@hooks/useNavTransition";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import {
  getColumnPreferencesUserKey,
  useColumnPreferences,
} from "@hooks/useColumnPreferences";
import QueryErrorState from "@components/QueryErrorState";
import FilteredCsvExportButton from "@components/FilteredCsvExportButton";
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
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
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
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
import RefreshButton from "@components/RefreshButton";
import type { BeIncident, BeIncidentSearchPayload, BeIncidentSearchResponse } from "@api/backend/types";

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
export default function IncidentsTab(): JSX.Element {
  const navigate = useNavTransition();
  const api = useBackendApi();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo<IncidentFilters>(
    () => readIncidentFiltersFromUrl(searchParams),
    [searchParams],
  );
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: buildIncidentSearchFilters(filters, debouncedSearch),
      sortBy: { field: "createdOn" as const, order: "desc" as const },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [filters, debouncedSearch, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching, refetch, dataUpdatedAt } =
    useSearchIncidents(payload);

  const incidents = data?.incidents ?? [];
  const total = data?.total ?? 0;

  // "Customise columns" — see `caseListColumns.ts`'s equivalent doc comment;
  // the default visible set is empty since none of these were columns at
  // all before this feature, so a returning user sees no change until they
  // open the picker.
  const currentUserEmail = useIdTokenClaims()?.email;
  const currentUserId = useCurrentUser().user?.id;
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
  const columnCount = 7 + visibleOptionalColumns.length; // Number, Subject, Caller, State, Priority, Opened, Updated + optional

  const setFilters = useCallback(
    (next: IncidentFilters) => {
      setPage(0);
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
    [searchParams, setSearchParams],
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
        <FilteredCsvExportButton<BeIncident>
          entityName="incidents"
          header={["Number", "Subject", "Caller", "State", "Priority", "Opened", "Updated"]}
          toRow={incidentToCsvRow}
          fetchPage={fetchIncidentsPage}
          disabled={isError}
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
        onFiltersToggle={() => setIsFiltersOpen((prev) => !prev)}
      />

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
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Caller</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Priority</TableCell>
                {visibleOptionalColumns.map((id) => (
                  <TableCell key={id}>{INCIDENT_OPTIONAL_COLUMNS[id].label}</TableCell>
                ))}
                <TableCell>Opened</TableCell>
                <TableCell>Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
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
                    sx={{ cursor: "pointer" }}
                  >
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
