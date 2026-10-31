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
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useActiveSlaStatuses, useRequestSlaRecalculation } from "@features/csm-admin/sla-engine/useSlaEngine";

function formatTimestamp(value: string | null): string {
  if (!value) return "—";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? "—" : parsed.toLocaleString();
}

/**
 * SLA Engine admin page: "Recalculate SLA" wipes and rebuilds
 * csm-notification-service's own Redis-held SLA tracking from this
 * backend's durable record (POST /sla-engine/recalculate) -- for recovering
 * from a Redis state that no longer agrees with reality. Below it, the
 * currently-active clocks (GET /sla-status/manage) so an admin can see the
 * effect, typically right after clicking Recalculate.
 */
export default function CsmSlaEnginePage(): JSX.Element {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const activeStatuses = useActiveSlaStatuses();
  const recalculate = useRequestSlaRecalculation();

  const handleConfirm = (): void => {
    setConfirmOpen(false);
    recalculate.mutate();
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3, maxWidth: 960 }}>
      <Box>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          SLA Engine
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Rebuild the notification service&apos;s own SLA tracking from this platform&apos;s durable record. Use this
          when SLA breach alerts stop matching what you see on a case or incident -- for example, after a bulk data
          change that could have left the tracking out of step.
        </Typography>
      </Box>

      <Paper variant="outlined" sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
          <Box>
            <Typography variant="subtitle1">Recalculate SLA</Typography>
            <Typography variant="body2" color="text.secondary">
              Clears and rebuilds every active SLA clock&apos;s tracked state. This runs in the background and may
              take a few minutes for a large number of clocks.
            </Typography>
          </Box>
          <Button
            variant="contained"
            startIcon={recalculate.isPending ? <CircularProgress size={16} color="inherit" /> : <RefreshCw size={16} />}
            onClick={() => setConfirmOpen(true)}
            disabled={recalculate.isPending}
          >
            Recalculate SLA
          </Button>
        </Box>

        {recalculate.isSuccess && (
          <Alert severity="success" onClose={() => recalculate.reset()}>
            Recalculation requested. The notification service rebuilds its SLA tracking in the background over the next
            few minutes. The list below reflects the platform&apos;s own records and does not change as a result of
            that rebuild.
          </Alert>
        )}
        {recalculate.isError && (
          <Alert severity="error" onClose={() => recalculate.reset()}>
            {recalculate.error.message || "Failed to start SLA recalculation."}
          </Alert>
        )}
      </Paper>

      <Dialog open={confirmOpen} onClose={() => setConfirmOpen(false)}>
        <DialogTitle>Recalculate SLA?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            This rebuilds every active SLA clock&apos;s tracked state from scratch. It does not change any case or
            incident data, and is safe to run at any time.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleConfirm}>
            Recalculate
          </Button>
        </DialogActions>
      </Dialog>

      <Box>
        <Typography variant="subtitle1" sx={{ mb: 1 }}>
          Currently active clocks
          {activeStatuses.data && ` (${activeStatuses.data.total})`}
        </Typography>
        {activeStatuses.isError && <Alert severity="error">Failed to load active SLA clocks.</Alert>}
        {activeStatuses.isLoading && <Skeleton variant="rounded" height={200} />}
        {activeStatuses.data && (
          <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
            <TableContainer>
              <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
                <TableHead>
                  <TableRow sx={{ bgcolor: "action.hover" }}>
                    <TableCell>Case ID</TableCell>
                    <TableCell>Clock type</TableCell>
                    <TableCell>Elapsed</TableCell>
                    <TableCell>Status</TableCell>
                    <TableCell>Started on</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {activeStatuses.data.statuses.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5}>
                        <Typography variant="body2" color="text.secondary">
                          No active SLA clocks right now.
                        </Typography>
                      </TableCell>
                    </TableRow>
                  ) : (
                    activeStatuses.data.statuses.map((status) => (
                      <TableRow key={`${status.caseId}-${status.clockType}`}>
                        <TableCell sx={{ fontFamily: "monospace" }}>{status.caseId}</TableCell>
                        <TableCell>{status.clockType}</TableCell>
                        <TableCell>{Math.round(status.businessElapsedPercent)}%</TableCell>
                        <TableCell>
                          {status.isPaused && <Chip size="small" variant="outlined" label="Paused" sx={{ mr: 0.5 }} />}
                          {status.hasBreached && <Chip size="small" variant="outlined" color="error" label="Breached" />}
                        </TableCell>
                        <TableCell>{formatTimestamp(status.startedOn)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        )}
      </Box>
    </Box>
  );
}
