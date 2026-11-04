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
  MenuItem,
  Paper,
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
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX, type MouseEvent } from "react";
import { Link as RouterLink, useNavigate } from "react-router";
import SeverityChip from "@components/SeverityChip";
import { severityFromBe, priorityFromSeverity } from "@api/backend/mappers";
import { caseIdLabel } from "@features/csm-cases/utils/caseIdentity";
import {
  ALL_CASE_TYPES,
  CASE_TYPE_COLOR,
  CASE_TYPE_LABEL,
  caseTypeDetailBasePath,
  caseTypeHasSeverity,
} from "@features/csm-cases/utils/caseType";
import type { BeCaseType } from "@api/backend/types";
import {
  SLA_ELAPSED_BUCKET_LABEL,
  useActiveSlaStatuses,
  useRequestSlaRecalculation,
  type ActiveSlaStatusFilters,
  type SlaElapsedBucket,
} from "@features/csm-admin/sla-engine/useSlaEngine";
import { SEVERITY_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import { useDebouncedValue } from "@hooks/useDebouncedValue";

const DEFAULT_ROWS_PER_PAGE = 25;
const ROWS_PER_PAGE_OPTIONS = [25, 50, 100];
const TABLE_COLUMN_COUNT = 8;
const SLA_ENGINE_PAGE_PATH = "/admin/sla-engine";
/** Only S0-S4 -- "unset" has no entity-service-side filter value. */
const SEVERITY_FILTER_OPTIONS: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const ELAPSED_BUCKET_FILTER_OPTIONS: SlaElapsedBucket[] = ["under50", "50to75", "75to100", "over100"];
const SEARCH_DEBOUNCE_MS = 400;

function formatTimestamp(value: string | null): string {
  if (!value) return "—";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? "—" : parsed.toLocaleString();
}

/** entity-service sends SLAStatus.CaseType uppercase (e.g. "SERVICE_REQUEST"); CASE_TYPE_LABEL/caseTypeDetailBasePath are keyed lowercase. */
function lowerCaseType(caseType: string | undefined): BeCaseType | undefined {
  return caseType ? (caseType.toLowerCase() as BeCaseType) : undefined;
}

/** Where this row's case/SR/SRA/etc. detail page lives -- each type has its own route, not just `/cases`. */
function caseDetailPath(caseId: string, caseType: string | undefined): string {
  return `${caseTypeDetailBasePath(lowerCaseType(caseType))}/${caseId}`;
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
  const navigate = useNavigate();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const [severityFilter, setSeverityFilter] = useState<Severity | "">("");
  const [caseTypeFilter, setCaseTypeFilter] = useState<BeCaseType | "">("");
  const [elapsedBucketFilter, setElapsedBucketFilter] = useState<SlaElapsedBucket | "">("");
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(searchInput, SEARCH_DEBOUNCE_MS);

  const hasActiveFilters = severityFilter !== "" || caseTypeFilter !== "" || elapsedBucketFilter !== "" || searchInput !== "";
  const handleClearFilters = (): void => {
    setSeverityFilter("");
    setCaseTypeFilter("");
    setElapsedBucketFilter("");
    setSearchInput("");
    setPage(0);
  };
  const handleSeverityChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSeverityFilter(e.target.value as Severity | "");
    setPage(0);
  };
  const handleCaseTypeChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setCaseTypeFilter(e.target.value as BeCaseType | "");
    setPage(0);
  };
  const handleElapsedBucketChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setElapsedBucketFilter(e.target.value as SlaElapsedBucket | "");
    setPage(0);
  };
  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const filters: ActiveSlaStatusFilters = useMemo(
    () => ({
      severity: severityFilter ? priorityFromSeverity(severityFilter) : undefined,
      caseType: caseTypeFilter || undefined,
      elapsedBucket: elapsedBucketFilter || undefined,
      search: debouncedSearch.trim() || undefined,
    }),
    [severityFilter, caseTypeFilter, elapsedBucketFilter, debouncedSearch],
  );
  const request = useMemo(
    () => ({ limit: rowsPerPage, offset: page * rowsPerPage, ...filters }),
    [page, rowsPerPage, filters],
  );
  const activeStatuses = useActiveSlaStatuses(request);
  const recalculate = useRequestSlaRecalculation();

  // The total can shrink between fetches (a clock completes, a recalculation
  // changes what's active) -- if that leaves `page` past the new last page,
  // the table would show "No active SLA clocks right now" with rows still on
  // an earlier page. Snap back to the last valid page -- a guarded set during
  // render, React's documented pattern for adjusting state from changed
  // inputs, not an effect (see CsmIssuesView.tsx's identical pattern/comment).
  if (activeStatuses.data !== undefined) {
    const lastPage = Math.max(0, Math.ceil(activeStatuses.data.total / rowsPerPage) - 1);
    if (page > lastPage) setPage(lastPage);
  }

  const handleConfirm = (): void => {
    setConfirmOpen(false);
    recalculate.mutate();
  };

  const handleRowsPerPageChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
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

        <Box sx={{ display: "flex", flexWrap: "wrap", gap: 2, mb: 2, alignItems: "flex-start" }}>
          <TextField
            label="Search (case id, number, or WSO2 case id)"
            value={searchInput}
            onChange={handleSearchChange}
            size="small"
            sx={{ minWidth: 260 }}
          />
          <TextField select label="Severity" value={severityFilter} onChange={handleSeverityChange} size="small" sx={{ minWidth: 160 }}>
            <MenuItem value="">All</MenuItem>
            {SEVERITY_FILTER_OPTIONS.map((sev) => (
              <MenuItem key={sev} value={sev}>
                {SEVERITY_LABEL[sev]}
              </MenuItem>
            ))}
          </TextField>
          <TextField select label="Type" value={caseTypeFilter} onChange={handleCaseTypeChange} size="small" sx={{ minWidth: 180 }}>
            <MenuItem value="">All</MenuItem>
            {ALL_CASE_TYPES.map((type) => (
              <MenuItem key={type} value={type}>
                {CASE_TYPE_LABEL[type]}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            label="Elapsed"
            value={elapsedBucketFilter}
            onChange={handleElapsedBucketChange}
            size="small"
            sx={{ minWidth: 150 }}
          >
            <MenuItem value="">All</MenuItem>
            {ELAPSED_BUCKET_FILTER_OPTIONS.map((bucket) => (
              <MenuItem key={bucket} value={bucket}>
                {SLA_ELAPSED_BUCKET_LABEL[bucket]}
              </MenuItem>
            ))}
          </TextField>
          {hasActiveFilters && (
            <Button size="small" onClick={handleClearFilters} sx={{ alignSelf: "center" }}>
              Clear filters
            </Button>
          )}
        </Box>

        {activeStatuses.isError && <Alert severity="error">Failed to load active SLA clocks.</Alert>}
        {activeStatuses.isLoading && <Skeleton variant="rounded" height={200} />}
        {activeStatuses.data && (
          <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
            <TableContainer>
              <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
                <TableHead>
                  <TableRow sx={{ bgcolor: "action.hover" }}>
                    <TableCell>Case</TableCell>
                    <TableCell sx={{ maxWidth: 260 }}>Title</TableCell>
                    <TableCell>Type</TableCell>
                    <TableCell>Severity</TableCell>
                    <TableCell>Clock type</TableCell>
                    <TableCell>Elapsed</TableCell>
                    <TableCell>Status</TableCell>
                    <TableCell>Started on</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {activeStatuses.isFetching ? (
                    Array.from({ length: rowsPerPage }).map((_, i) => (
                      <TableRow key={i}>
                        {Array.from({ length: TABLE_COLUMN_COUNT }).map((__, j) => (
                          <TableCell key={j}>
                            <Skeleton variant="text" />
                          </TableCell>
                        ))}
                      </TableRow>
                    ))
                  ) : activeStatuses.data.statuses.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={TABLE_COLUMN_COUNT}>
                        <Typography variant="body2" color="text.secondary">
                          No active SLA clocks right now.
                        </Typography>
                      </TableCell>
                    </TableRow>
                  ) : (
                    activeStatuses.data.statuses.map((status) => {
                      const detailPath = caseDetailPath(status.caseId, status.caseType);
                      const lowerType = lowerCaseType(status.caseType);
                      return (
                        <TableRow
                          key={`${status.caseId}-${status.clockType}`}
                          onClick={(e: MouseEvent<HTMLTableRowElement>) => {
                            // The case-id cell below is a real RouterLink: a plain
                            // click on it already navigates via its own handler
                            // (which calls preventDefault), and that click still
                            // bubbles up here -- do nothing then, or this pushes a
                            // second, duplicate history entry for the same
                            // destination. A modified click (cmd/ctrl/shift/alt)
                            // deliberately does not preventDefault -- react-router
                            // leaves it to the browser to open a new tab -- so skip
                            // here too, or the current tab would also navigate away
                            // while a new tab is opening.
                            if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                            navigate(detailPath, { state: { from: SLA_ENGINE_PAGE_PATH } });
                          }}
                          sx={{ cursor: "pointer", "&:hover": { bgcolor: "action.hover" } }}
                        >
                          <TableCell sx={{ fontFamily: "monospace" }}>
                            <Box
                              component={RouterLink}
                              to={detailPath}
                              state={{ from: SLA_ENGINE_PAGE_PATH }}
                              sx={{
                                color: "inherit",
                                textDecoration: "none",
                                "&:hover": { textDecoration: "underline" },
                              }}
                            >
                              {caseIdLabel(status) || status.caseId}
                            </Box>
                          </TableCell>
                          <TableCell sx={{ maxWidth: 260 }}>
                            <Typography variant="body2" noWrap title={status.caseTitle || undefined}>
                              {status.caseTitle || "—"}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            {lowerType ? (
                              <Chip
                                size="small"
                                variant="outlined"
                                color={CASE_TYPE_COLOR[lowerType]}
                                label={CASE_TYPE_LABEL[lowerType] ?? status.caseType}
                              />
                            ) : (
                              "—"
                            )}
                          </TableCell>
                          <TableCell>
                            {caseTypeHasSeverity(lowerType) ? (
                              <SeverityChip severity={severityFromBe(status.priority)} />
                            ) : (
                              "—"
                            )}
                          </TableCell>
                          <TableCell>{status.clockType}</TableCell>
                          <TableCell>{Math.round(status.businessElapsedPercent)}%</TableCell>
                          <TableCell>
                            {status.isPaused && (
                              <Chip size="small" variant="outlined" label="Paused" sx={{ mr: 0.5 }} />
                            )}
                            {status.hasBreached && (
                              <Chip size="small" variant="outlined" color="error" label="Breached" />
                            )}
                            {!status.isPaused && !status.hasBreached && (
                              <Chip size="small" variant="outlined" color="success" label="In progress" />
                            )}
                          </TableCell>
                          <TableCell>{formatTimestamp(status.startedOn)}</TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </TableContainer>
            <TablePagination
              component="div"
              count={activeStatuses.data.total}
              page={page}
              onPageChange={(_, newPage) => setPage(newPage)}
              rowsPerPage={rowsPerPage}
              onRowsPerPageChange={handleRowsPerPageChange}
              rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
              showFirstButton
              showLastButton
            />
          </Box>
        )}
      </Box>
    </Box>
  );
}
