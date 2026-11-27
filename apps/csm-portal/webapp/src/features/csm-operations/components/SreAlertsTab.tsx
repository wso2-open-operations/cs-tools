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
  Chip,
  Link,
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
import { useCallback, useMemo, useState, type ChangeEvent, type JSX, type MouseEvent } from "react";
import { Link as RouterLink, useLocation, useSearchParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useFilterBarCollapsed } from "@hooks/useFilterBarCollapsed";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { getColumnPreferencesUserKey } from "@hooks/useColumnPreferences";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type { BeSreAlert, BeSreAlertSortField } from "@api/backend/types";
import { useSearchSreAlerts, useSreAlertFacets } from "@features/csm-operations/api/useSreAlerts";
import SreAlertsFilterBar from "@features/csm-operations/components/SreAlertsFilterBar";
import SortableHeaderCell from "@features/csm-operations/components/SortableHeaderCell";
import type { ListSort } from "@features/csm-operations/utils/listSort";
import { incidentStateColor, incidentStateLabel } from "@features/csm-operations/utils/incidents";
import {
  buildSreAlertSearchFilters,
  DEFAULT_SRE_ALERT_FILTERS,
  readSreAlertFiltersFromUrl,
  SRE_ALERT_FILTER_PARAM_KEYS,
  sreAlertIncidentState,
  sreAlertSeverityColor,
  sreAlertSeverityLabel,
  sreAlertTierLabel,
  writeSreAlertFiltersToUrl,
  type SreAlertFilters,
} from "@features/csm-operations/utils/sreAlerts";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

interface Column {
  id: string;
  label: string;
  sortField?: BeSreAlertSortField;
}

const COLUMNS: Column[] = [
  { id: "number", label: "Number", sortField: "number" },
  { id: "receivedOn", label: "Received", sortField: "receivedOn" },
  { id: "severity", label: "Severity", sortField: "severity" },
  { id: "service", label: "Service", sortField: "service" },
  { id: "environment", label: "Environment", sortField: "environment" },
  { id: "metricName", label: "Metric" },
  { id: "source", label: "Source" },
  { id: "tier", label: "Tier" },
  { id: "incident", label: "Incident" },
];

function formatReceived(value: string): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—";
}

/** A free-text cell capped in width, so one long value can't widen the whole table. */
function TextCell({ value, maxWidth = 220 }: { value: string; maxWidth?: number }): JSX.Element {
  return (
    <TableCell sx={{ maxWidth }}>
      <Typography variant="body2" noWrap title={value || undefined}>
        {value || "—"}
      </Typography>
    </TableCell>
  );
}

/**
 * Operations → Alerts: the SRE alerts stored by entity-service
 * (`POST /sre-alerts/search`), with server-side search, sorting and pagination.
 * Filters live in the URL (`alt...` params) like the Incidents tab's, so a
 * filtered list survives a tab switch and can be shared. A row opens the
 * alert's page with `{ from, parentState }`, so its Back returns here.
 */
export default function SreAlertsTab(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo(() => readSreAlertFiltersFromUrl(searchParams), [searchParams]);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);
  const [sort, setSort] = useState<ListSort<BeSreAlertSortField>>({ field: "receivedOn", order: "desc" });

  const currentUserEmail = useIdTokenClaims()?.email;
  const currentUserId = useCurrentUser().user?.id;
  const [isFiltersOpen, setIsFiltersOpen] = useFilterBarCollapsed(
    "sre-alerts",
    getColumnPreferencesUserKey({ id: currentUserId, email: currentUserEmail }),
    true,
  );

  const payload = useMemo(
    () => ({
      filters: buildSreAlertSearchFilters(filters, debouncedSearch),
      sortBy: sort,
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [filters, debouncedSearch, sort, page, rowsPerPage],
  );
  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } = useSearchSreAlerts(payload);
  const { data: facets } = useSreAlertFacets();
  const alerts = data?.alerts ?? [];
  const total = data?.total ?? 0;

  const setFilters = useCallback(
    (next: SreAlertFilters) => {
      setPage(0);
      // Keep every other param (another tab's filters, a dashboard's state) as it is.
      const merged = new URLSearchParams(searchParams);
      SRE_ALERT_FILTER_PARAM_KEYS.forEach((k) => merged.delete(k));
      writeSreAlertFiltersToUrl(next).forEach((v, k) => merged.append(k, v));
      setSearchParams(merged, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const handleSort = (next: ListSort<BeSreAlertSortField>): void => {
    setSort(next);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const navState = {
    from: `${location.pathname}${location.search}`,
    parentState: location.state ?? null,
  };

  const openAlert = (alert: BeSreAlert): void => {
    navigate(`/operations/alerts/${encodeURIComponent(alert.id)}`, { state: navState });
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 1 }}>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh alerts"
        />
      </Box>

      <SreAlertsFilterBar
        filters={filters}
        facets={facets}
        onChange={setFilters}
        onReset={() => setFilters(DEFAULT_SRE_ALERT_FILTERS)}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
      />

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                {COLUMNS.map((c) =>
                  c.sortField ? (
                    <SortableHeaderCell key={c.id} label={c.label} field={c.sortField} sort={sort} onSort={handleSort} />
                  ) : (
                    <TableCell key={c.id}>{c.label}</TableCell>
                  ),
                )}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {COLUMNS.map((c) => (
                      <TableCell key={c.id}>
                        <Skeleton variant="rounded" width="80%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load alerts."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : alerts.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={COLUMNS.length} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No alerts found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                alerts.map((alert) => (
                  <TableRow
                    key={alert.id}
                    hover
                    onClick={() => openAlert(alert)}
                    sx={{ cursor: "pointer" }}
                    aria-label={`View alert ${alert.number}`}
                  >
                    <TableCell sx={{ fontFamily: "monospace", whiteSpace: "nowrap" }}>{alert.number || "—"}</TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>{formatReceived(alert.receivedOn)}</TableCell>
                    <TableCell>
                      {alert.severity ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={sreAlertSeverityColor(alert.severity)}
                          label={sreAlertSeverityLabel(alert.severity)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TextCell value={alert.service} />
                    <TextCell value={alert.environment} maxWidth={140} />
                    <TextCell value={alert.metricName} />
                    <TextCell value={alert.source} maxWidth={160} />
                    <TableCell>
                      {alert.tier ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={alert.tier === "priority" ? "primary" : "default"}
                          label={sreAlertTierLabel(alert.tier)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {alert.incident ? (
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                          <Link
                            component={RouterLink}
                            to={`/operations/incidents/${encodeURIComponent(alert.incident.id)}`}
                            state={navState}
                            // The row opens the alert; this link opens the incident instead.
                            onClick={(e: MouseEvent) => e.stopPropagation()}
                            sx={{ fontFamily: "monospace" }}
                            title={alert.incident.subject || undefined}
                          >
                            {alert.incident.number || alert.incident.id}
                          </Link>
                          {alert.incident.state && (
                            <Chip
                              size="small"
                              variant="outlined"
                              color={incidentStateColor(sreAlertIncidentState(alert.incident.state))}
                              label={incidentStateLabel(sreAlertIncidentState(alert.incident.state))}
                              sx={{ height: 20, fontSize: 11 }}
                            />
                          )}
                        </Box>
                      ) : (
                        "—"
                      )}
                    </TableCell>
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
