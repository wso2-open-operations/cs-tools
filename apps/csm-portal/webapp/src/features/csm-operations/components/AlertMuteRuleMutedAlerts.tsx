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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
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
import type { ChangeEvent, JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type {
  AlertMuteRuleMutedAlert,
  AlertMuteRuleMutedAlertsResponse,
  MutedAlertOutcome,
} from "@features/csm-operations/types/alertMuteRules";
import {
  MUTED_ALERTS_ROWS_PER_PAGE_OPTIONS,
} from "@features/csm-operations/utils/alertMuteRules";

const OUTCOME_LABELS: Record<MutedAlertOutcome, string> = {
  muted: "Still muted",
  incident: "Became incident",
  closed_quietly: "Closed quietly",
  delivering: "Creating incident",
};

const OUTCOME_COLORS: Record<
  MutedAlertOutcome,
  "default" | "warning" | "error" | "success" | "info"
> = {
  muted: "warning",
  incident: "error",
  closed_quietly: "success",
  delivering: "info",
};

function formatDateTime(iso: string): string {
  return (
    formatBackendTimestampForDisplay(iso, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function OutcomeCell({
  alert,
}: {
  alert: AlertMuteRuleMutedAlert;
}): JSX.Element {
  return (
    <Box
      sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}
    >
      <Chip
        size="small"
        variant="outlined"
        color={OUTCOME_COLORS[alert.outcome]}
        label={OUTCOME_LABELS[alert.outcome]}
      />
      {alert.outcome === "incident" && alert.incidentNumber && (
        <Typography variant="body2">{alert.incidentNumber}</Typography>
      )}
    </Box>
  );
}

interface Props {
  open: boolean;
  onClose: () => void;
  title: string;
  data: AlertMuteRuleMutedAlertsResponse | undefined;
  isLoading: boolean;
  error: Error | null;
  page: number;
  rowsPerPage: number;
  onPageChange: (page: number) => void;
  onRowsPerPageChange: (rows: number) => void;
}

/** The rule page's "Muted alerts" dialog: one row per group of alerts the rule muted, and what became of it. */
export default function AlertMuteRuleMutedAlerts({
  open,
  onClose,
  title,
  data,
  isLoading,
  error,
  page,
  rowsPerPage,
  onPageChange,
  onRowsPerPageChange,
}: Props): JSX.Element {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth aria-labelledby="muted-alerts-title">
      <DialogTitle id="muted-alerts-title">
        {title}
        <Typography variant="caption" color="text.secondary" component="div">
          Alerts are grouped by fingerprint. Kept by the alerting service for 30 days.
        </Typography>
      </DialogTitle>
      <DialogContent dividers>
      {error ? (
        <QueryErrorState
          message={error.message || "Couldn't load muted alerts."}
          error={error}
        />
      ) : isLoading && !data ? (
        <Skeleton variant="rounded" height={120} />
      ) : !data || data.total === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No alerts muted by this rule.
        </Typography>
      ) : (
        <>
          <TableContainer>
            <Table
              size="small"
              sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}
            >
              <TableHead>
                <TableRow>
                  <TableCell>Alert</TableCell>
                  <TableCell>Service / environment</TableCell>
                  <TableCell>Severity</TableCell>
                  <TableCell>Source</TableCell>
                  <TableCell align="right">Alerts</TableCell>
                  <TableCell>First seen</TableCell>
                  <TableCell>Last seen</TableCell>
                  <TableCell>Outcome</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {data.items.map((alert) => (
                  <TableRow key={alert.id}>
                    <TableCell sx={{ maxWidth: 240 }}>
                      <Typography
                        variant="body2"
                        noWrap
                        title={alert.metricName}
                      >
                        {alert.metricName || "—"}
                      </Typography>
                      {alert.uniqueIdentifier && (
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          noWrap
                          title={alert.uniqueIdentifier}
                          component="div"
                        >
                          {alert.uniqueIdentifier}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {alert.service || "—"}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {alert.environment || "—"}
                      </Typography>
                    </TableCell>
                    <TableCell>{alert.severity || "—"}</TableCell>
                    <TableCell>{alert.source || "—"}</TableCell>
                    <TableCell align="right">{alert.alertCount}</TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {formatDateTime(alert.firstSeen)}
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {formatDateTime(alert.lastSeen)}
                    </TableCell>
                    <TableCell>
                      <OutcomeCell alert={alert} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
          <TablePagination
            component="div"
            count={data.total}
            page={page}
            onPageChange={(_: unknown, next: number) => onPageChange(next)}
            rowsPerPage={rowsPerPage}
            onRowsPerPageChange={(e: ChangeEvent<HTMLInputElement>) =>
              onRowsPerPageChange(parseInt(e.target.value, 10))
            }
            rowsPerPageOptions={MUTED_ALERTS_ROWS_PER_PAGE_OPTIONS}
            labelDisplayedRows={({ from, to, count }) =>
              `Showing ${from}–${to} of ${count}`
            }
          />
        </>
      )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}
