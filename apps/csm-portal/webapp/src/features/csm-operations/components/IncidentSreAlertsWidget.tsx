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
  Card,
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
import { BellRing } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import RefreshButton from "@components/RefreshButton";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type { BeSreAlertSearchPayload } from "@api/backend/types";
import { useSearchSreAlerts } from "@features/csm-operations/api/useSreAlerts";
import { sreAlertSeverityColor, sreAlertSeverityLabel } from "@features/csm-operations/utils/sreAlerts";

const COLUMNS = ["Number", "Received", "Severity", "Metric", "Environment"];
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

interface IncidentSreAlertsWidgetProps {
  /** UUID of the incident whose alerts are listed. */
  incidentId: string;
}

/**
 * The SRE alerts linked to one incident (`POST /sre-alerts/search` with
 * `filters.incidentId`), newest first, on the incident's Alerts tab. A row
 * opens the alert with Back returning to this tab. Same card/table pattern as
 * `ChildIncidentsWidget`.
 */
export default function IncidentSreAlertsWidget({ incidentId }: IncidentSreAlertsWidgetProps): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const payload = useMemo<BeSreAlertSearchPayload>(
    () => ({
      filters: { incidentId },
      sortBy: { field: "receivedOn", order: "desc" },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [incidentId, page, rowsPerPage],
  );
  const { data, isLoading, isError, isFetching, refetch, dataUpdatedAt } = useSearchSreAlerts(payload);
  const alerts = data?.alerts ?? [];
  const total = data?.total ?? 0;
  // Back from the alert returns to this tab, with whatever state brought us to the incident.
  const navState = { from: `${location.pathname}${location.search}`, parentState: location.state ?? null };

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <BellRing size={16} />
          <Typography variant="subtitle2">
            Alerts{!isLoading && !isError && total > 0 ? ` (${total})` : ""}
          </Typography>
        </Box>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh alerts"
        />
      </Box>

      {isError ? (
        <Typography variant="body2" color="error">
          Could not load the alerts of this incident.
        </Typography>
      ) : (
        <>
          <TableContainer>
            <Table size="small" sx={{ width: "100%" }}>
              <TableHead>
                <TableRow>
                  {COLUMNS.map((col) => (
                    <TableCell key={col} sx={{ whiteSpace: "nowrap" }}>
                      {col}
                    </TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoading ? (
                  [0, 1].map((i) => (
                    <TableRow key={i}>
                      {COLUMNS.map((col) => (
                        <TableCell key={col}>
                          <Skeleton variant="text" />
                        </TableCell>
                      ))}
                    </TableRow>
                  ))
                ) : alerts.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={COLUMNS.length} align="center">
                      <Typography variant="body2" color="text.secondary">
                        No alerts are linked to this incident.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  alerts.map((a) => {
                    const alertPath = `/operations/alerts/${encodeURIComponent(a.id)}`;
                    return (
                      <TableRow
                        key={a.id}
                        hover
                        onClick={() => navigate(alertPath, { state: navState })}
                        sx={{ cursor: "pointer" }}
                      >
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          <Typography
                            component={RouterLink}
                            to={alertPath}
                            state={navState}
                            variant="body2"
                            onClick={(e) => e.stopPropagation()}
                            sx={{ color: "inherit", textDecoration: "none", fontFamily: "monospace" }}
                          >
                            {a.number || a.id}
                          </Typography>
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {formatBackendTimestampForDisplay(a.receivedOn, { dateStyle: "medium", timeStyle: "short" }) ??
                            "—"}
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {a.severity ? (
                            <Chip
                              size="small"
                              variant="outlined"
                              color={sreAlertSeverityColor(a.severity)}
                              label={sreAlertSeverityLabel(a.severity)}
                            />
                          ) : (
                            "—"
                          )}
                        </TableCell>
                        <TableCell sx={{ maxWidth: 240 }}>
                          <Typography variant="body2" noWrap title={a.metricName || undefined}>
                            {a.metricName || "—"}
                          </Typography>
                        </TableCell>
                        <TableCell sx={{ maxWidth: 160 }}>
                          <Typography variant="body2" noWrap title={a.environment || undefined}>
                            {a.environment || "—"}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </TableContainer>
          {total > ROWS_PER_PAGE_OPTIONS[0] && (
            <TablePagination
              component="div"
              count={total}
              page={page}
              onPageChange={(_: unknown, next: number) => setPage(next)}
              rowsPerPage={rowsPerPage}
              onRowsPerPageChange={(e: ChangeEvent<HTMLInputElement>) => {
                setRowsPerPage(parseInt(e.target.value, 10));
                setPage(0);
              }}
              rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
              showFirstButton
              showLastButton
            />
          )}
        </>
      )}
    </Card>
  );
}
