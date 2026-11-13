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
  Autocomplete,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  Grid,
  IconButton,
  InputAdornment,
  InputLabel,
  Link,
  Menu,
  MenuItem,
  Paper,
  Select,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, Download, ExternalLink, ListFilter, Pencil, Plus, Search, Trash2, Upload, X } from "@wso2/oxygen-ui-icons-react";
import { downloadCsv } from "@utils/csvExport";
import { saveBlob } from "@utils/saveBlob";
import { useMemo, useRef, useState, type ChangeEvent, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import {
  useCreateRunbook,
  useDeleteRunbook,
  useImportRunbooks,
  useRunbooks,
  useUpdateRunbook,
} from "@features/csm-operations/api/useRunbooks";
import {
  activeRunbookFilterCount,
  cleanAlertNames,
  DEFAULT_RUNBOOK_FILTERS,
  filterRunbooks,
  findAlertsContaining,
  findRunbookForIncident,
  isValidRunbookUrl,
  parseRunbookImport,
  RUNBOOK_CSV_HEADER,
  runbooksToCsvRows,
  runbooksToImportJson,
  type Runbook,
  type RunbookFilters,
} from "@features/csm-operations/utils/runbooks";
import { errorText } from "@features/csm-operations/utils/changeRequests";

const DEFAULT_ROWS_PER_PAGE = 20;
// A Select treats "" as no selection and leaves its label inside the box, so "Anyone" needs a real value.
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

interface RunbookDialogState {
  open: boolean;
  editing?: Runbook;
}

/** runbooks-YYYY-MM-DD.<ext>, in the viewer's local date. */
function exportFileName(ext: "json" | "csv"): string {
  const d = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  return `runbooks-${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}.${ext}`;
}

/**
 * Operations -> Runbooks: named runbook links, each with the alert names it covers.
 * An incident whose subject contains one of those alert names shows the runbook in
 * its Related tab.
 */
export default function RunbooksTab(): JSX.Element {
  const { canWrite } = usePortalAccess();
  const { data: runbooks = [], isLoading, isError, error, refetch } = useRunbooks();
  const deleteRunbook = useDeleteRunbook();
  const importRunbooks = useImportRunbooks();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [exportAnchor, setExportAnchor] = useState<HTMLElement | null>(null);
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<RunbookFilters>(DEFAULT_RUNBOOK_FILTERS);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [dialog, setDialog] = useState<RunbookDialogState>({ open: false });
  const [pendingDelete, setPendingDelete] = useState<Runbook | null>(null);
  const [notice, setNotice] = useState<{ severity: "success" | "error"; text: string } | null>(null);

  const linkedCount = runbooks.filter((r) => r.runbookUrl).length;
  const alertCount = runbooks.reduce((n, r) => n + r.alerts.length, 0);

  const filtered = useMemo(() => filterRunbooks(runbooks, search, filters), [runbooks, search, filters]);
  const activeFilters = activeRunbookFilterCount(filters);

  const setFilter = <K extends keyof RunbookFilters>(key: K, value: RunbookFilters[K]): void => {
    setFilters((f) => ({ ...f, [key]: value }));
    setPage(0);
  };

  const pageRows = filtered.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage);
  const columnCount = canWrite ? 6 : 5;

  const handleDelete = (): void => {
    if (!pendingDelete) return;
    deleteRunbook.mutate(pendingDelete.id, {
      onSuccess: () => setPendingDelete(null),
    });
  };

  const handleImportFile = async (e: ChangeEvent<HTMLInputElement>): Promise<void> => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    try {
      const entries = parseRunbookImport(await file.text());
      if (entries.length === 0) {
        setNotice({ severity: "error", text: "The file has no runbooks." });
        return;
      }
      importRunbooks.mutate(entries, {
        onSuccess: (res) =>
          setNotice({
            severity: "success",
            text:
              `Imported ${res.created} new and updated ${res.updated} existing runbooks.` +
              (res.removed?.length
                ? ` Removed ${res.removed.map((r) => `${r.name} (${r.number}, ${r.runbookUrl ?? "no link"})`).join(", ")}: all of their alerts moved to another runbook.`
                : ""),
          }),
        onError: (err) => setNotice({ severity: "error", text: errorText(err, "Failed to import runbooks.") }),
      });
    } catch (err) {
      setNotice({ severity: "error", text: errorText(err, "The file could not be read.") });
    }
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="body2" color="text.secondary">
          {isLoading
            ? "Loading runbooks…"
            : `${runbooks.length} runbooks covering ${alertCount} alerts, ${runbooks.length - linkedCount} without a link`}
        </Typography>
        <Box sx={{ display: "flex", gap: 1 }}>
          <Button
            variant="outlined"
            size="small"
            startIcon={<Download size={16} />}
            endIcon={<ChevronDown size={16} />}
            disabled={isLoading || filtered.length === 0}
            aria-haspopup="menu"
            aria-expanded={exportAnchor !== null}
            onClick={(e) => setExportAnchor(e.currentTarget)}
          >
            Export
          </Button>
          <Menu anchorEl={exportAnchor} open={exportAnchor !== null} onClose={() => setExportAnchor(null)}>
            <MenuItem
              onClick={() => {
                setExportAnchor(null);
                saveBlob(new Blob([runbooksToImportJson(filtered)], { type: "application/json" }), exportFileName("json"));
              }}
            >
              JSON (can be imported again)
            </MenuItem>
            <MenuItem
              onClick={() => {
                setExportAnchor(null);
                downloadCsv(RUNBOOK_CSV_HEADER, runbooksToCsvRows(filtered), exportFileName("csv"));
              }}
            >
              CSV
            </MenuItem>
          </Menu>
          {canWrite && (
            <>
              <input
                ref={fileInputRef}
                type="file"
                accept="application/json,.json"
                hidden
                onChange={(e) => void handleImportFile(e)}
                data-testid="runbook-import-input"
              />
              <Button
                variant="outlined"
                size="small"
                startIcon={<Upload size={16} />}
                disabled={importRunbooks.isPending}
                onClick={() => fileInputRef.current?.click()}
              >
                {importRunbooks.isPending ? "Importing…" : "Import JSON"}
              </Button>
              <Button
                variant="contained"
                color="primary"
                size="small"
                startIcon={<Plus size={16} />}
                onClick={() => setDialog({ open: true })}
              >
                Add runbook
              </Button>
            </>
          )}
        </Box>
      </Box>

      {notice && (
        <Alert severity={notice.severity} onClose={() => setNotice(null)}>
          {notice.text}
        </Alert>
      )}

      <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <Box sx={{ flex: 1, minWidth: 240 }}>
            <TextField
              size="small"
              placeholder="Search by number, name, alert name or runbook link…"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setPage(0);
              }}
              fullWidth
              slotProps={{
                htmlInput: { "aria-label": "Search runbooks by number, name, alert name or runbook link" },
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={16} />
                    </InputAdornment>
                  ),
                  endAdornment: search ? (
                    <InputAdornment position="end">
                      <IconButton size="small" edge="end" onClick={() => setSearch("")} aria-label="Clear search">
                        <X size={16} />
                      </IconButton>
                    </InputAdornment>
                  ) : undefined,
                },
              }}
            />
          </Box>

          <Button
            variant="outlined"
            size="small"
            color="primary"
            onClick={() => setIsFiltersOpen((open) => !open)}
            startIcon={<ListFilter size={16} />}
            endIcon={isFiltersOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
          >
            {activeFilters > 0 ? `Filters (${activeFilters})` : "Filters"}
          </Button>
          {activeFilters > 0 && (
            <Button
              variant="text"
              size="small"
              color="primary"
              onClick={() => {
                setFilters(DEFAULT_RUNBOOK_FILTERS);
                setPage(0);
              }}
              startIcon={<X size={16} />}
            >
              Clear filters
            </Button>
          )}
        </Box>

        {isFiltersOpen && (
          <>
            <Divider />
            <Grid container spacing={2} alignItems="center">
              <Grid size={{ xs: 12, sm: 4 }}>
                <FormControl fullWidth size="small">
                  <InputLabel id="runbook-filter-coverage-label">Runbook link</InputLabel>
                  <Select
                    labelId="runbook-filter-coverage-label"
                    id="runbook-filter-coverage"
                    label="Runbook link"
                    value={filters.coverage}
                    onChange={(e) => setFilter("coverage", e.target.value as RunbookFilters["coverage"])}
                  >
                    <MenuItem value="all">All</MenuItem>
                    <MenuItem value="linked">Has a link</MenuItem>
                    <MenuItem value="missing">No link yet</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid size={{ xs: 12, sm: 4 }}>
                <FormControl fullWidth size="small">
                  <InputLabel id="runbook-filter-alerts-label">Alerts</InputLabel>
                  <Select
                    labelId="runbook-filter-alerts-label"
                    id="runbook-filter-alerts"
                    label="Alerts"
                    value={filters.alertCount}
                    onChange={(e) => setFilter("alertCount", e.target.value as RunbookFilters["alertCount"])}
                  >
                    <MenuItem value="all">All</MenuItem>
                    <MenuItem value="single">One alert</MenuItem>
                    <MenuItem value="multiple">Several alerts</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid size={{ xs: 12, sm: 4 }}>
                <FormControl fullWidth size="small">
                  <InputLabel id="runbook-filter-updated-within-label">Last updated</InputLabel>
                  <Select
                    labelId="runbook-filter-updated-within-label"
                    id="runbook-filter-updated-within"
                    label="Last updated"
                    value={filters.updatedWithin}
                    onChange={(e) => setFilter("updatedWithin", e.target.value as RunbookFilters["updatedWithin"])}
                  >
                    <MenuItem value="all">Any time</MenuItem>
                    <MenuItem value="7">Past 7 days</MenuItem>
                    <MenuItem value="30">Past 30 days</MenuItem>
                    <MenuItem value="90">Past 90 days</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
            </Grid>
          </>
        )}
      </Paper>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell sx={{ width: 120 }}>Number</TableCell>
                <TableCell>Name</TableCell>
                <TableCell sx={{ width: 180 }}>Runbook</TableCell>
                <TableCell sx={{ width: 170 }}>Created</TableCell>
                <TableCell sx={{ width: 170 }}>Updated</TableCell>
                {canWrite && <TableCell align="right" sx={{ width: 96 }}>Actions</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={110} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={100} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={100} height={18} /></TableCell>
                    {canWrite && <TableCell><Skeleton variant="rounded" width={56} height={18} /></TableCell>}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={columnCount} align="center">
                    <QueryErrorState message={errorText(error, "Failed to load runbooks.")} error={error} onRetry={() => void refetch()} />
                  </TableCell>
                </TableRow>
              ) : pageRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columnCount} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No runbooks found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                pageRows.map((rb) => (
                  <TableRow key={rb.id} hover>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      <Typography variant="body2">
                        {rb.number}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ wordBreak: "break-word" }}>
                      <Typography variant="body2">{rb.name}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {rb.alerts.length === 1 ? "1 alert" : `${rb.alerts.length} alerts`}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      {rb.runbookUrl ? (
                        <Link
                          href={rb.runbookUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          sx={{ display: "inline-flex", alignItems: "center", gap: 0.5 }}
                        >
                          Open runbook
                          <ExternalLink size={14} />
                        </Link>
                      ) : (
                        <Chip size="small" variant="outlined" color="warning" label="No runbook" />
                      )}
                    </TableCell>
                    <TableCell>
                      <RunbookDate on={rb.createdOn} />
                    </TableCell>
                    <TableCell>
                      <RunbookDate on={rb.updatedOn} />
                    </TableCell>
                    {canWrite && (
                      <TableCell align="right">
                        <Tooltip title="Edit">
                          <IconButton size="small" aria-label={`Edit ${rb.number}`} onClick={() => setDialog({ open: true, editing: rb })}>
                            <Pencil size={16} />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Delete">
                          <IconButton size="small" aria-label={`Delete ${rb.number}`} onClick={() => setPendingDelete(rb)}>
                            <Trash2 size={16} />
                          </IconButton>
                        </Tooltip>
                      </TableCell>
                    )}
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={filtered.length}
          page={page}
          onPageChange={(_: unknown, next: number) => setPage(next)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e: ChangeEvent<HTMLInputElement>) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          labelDisplayedRows={({ from, to, count }) => `Showing ${from}–${to} of ${count}`}
          showFirstButton
          showLastButton
        />
      </Box>

      <RunbookAlertSearch runbooks={runbooks} />

      {dialog.open && (
        <RunbookDialog
          editing={dialog.editing}
          existing={runbooks}
          onClose={() => setDialog({ open: false })}
        />
      )}

      <Dialog open={pendingDelete !== null} onClose={() => setPendingDelete(null)} maxWidth="xs" fullWidth>
        <DialogTitle>Delete runbook?</DialogTitle>
        <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="body2" sx={{ wordBreak: "break-word" }}>
            Incidents matching the {pendingDelete?.alerts.length === 1 ? "alert" : `${pendingDelete?.alerts.length ?? 0} alerts`} of{" "}
            <strong>{pendingDelete?.name}</strong> ({pendingDelete?.number}) will no longer show this runbook.
          </Typography>
          {deleteRunbook.isError && (
            <Alert severity="error">{errorText(deleteRunbook.error, "Failed to delete the runbook.")}</Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingDelete(null)}>Go back</Button>
          <Button color="error" variant="contained" onClick={handleDelete} disabled={deleteRunbook.isPending}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}

interface RunbookDialogProps {
  editing?: Runbook;
  existing: Runbook[];
  onClose: () => void;
}

/** A date, with the full date and time on hover. */
function RunbookDate({ on }: { on: string }): JSX.Element {
  const date = formatBackendTimestampForDisplay(on, { month: "short", day: "numeric", year: "numeric" });
  const full = formatBackendTimestampForDisplay(on, { dateStyle: "medium", timeStyle: "short" });
  return (
    <Tooltip title={full ?? ""}>
      <Typography variant="body2" component="span">
        {date ?? "—"}
      </Typography>
    </Tooltip>
  );
}

function RunbookDialog({ editing, existing, onClose }: RunbookDialogProps): JSX.Element {
  const createRunbook = useCreateRunbook();
  const updateRunbook = useUpdateRunbook();
  const [name, setName] = useState(editing?.name ?? "");
  const [runbookUrl, setRunbookUrl] = useState(editing?.runbookUrl ?? "");
  const [alerts, setAlerts] = useState<string[]>(editing?.alerts ?? []);
  const [pendingAlert, setPendingAlert] = useState("");

  const saving = createRunbook.isPending || updateRunbook.isPending;
  const saveError = createRunbook.error ?? updateRunbook.error;
  const trimmedName = name.trim();
  const trimmedUrl = runbookUrl.trim();
  // A typed alert not yet confirmed with Enter is still saved.
  const alertsToSave = cleanAlertNames([...alerts, pendingAlert]);
  const others = existing.filter((r) => r.id !== editing?.id);
  const duplicateName = others.some((r) => r.name.trim().toLowerCase() === trimmedName.toLowerCase());
  const takenAlerts = alertsToSave.flatMap((a) => {
    const owner = others.find((r) => r.alerts.some((b) => b.trim().toLowerCase() === a.toLowerCase()));
    return owner ? [`${a} (in ${owner.name})`] : [];
  });
  const urlInvalid = !isValidRunbookUrl(runbookUrl);
  const canSave =
    trimmedName.length > 0 && !duplicateName && alertsToSave.length > 0 && takenAlerts.length === 0 && !urlInvalid && !saving;

  const handleSave = (): void => {
    const input = { name: trimmedName, runbookUrl: trimmedUrl || null, alerts: alertsToSave };
    if (editing) {
      updateRunbook.mutate({ id: editing.id, input }, { onSuccess: onClose });
    } else {
      createRunbook.mutate(input, { onSuccess: onClose });
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>{editing ? `Edit ${editing.number}` : "Add runbook"}</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: "8px !important" }}>
        <TextField
          label="Name"
          required
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={duplicateName}
          helperText={duplicateName ? "Another runbook already has this name." : undefined}
        />
        <TextField
          label="Runbook link"
          placeholder="https://docs.google.com/document/d/…"
          value={runbookUrl}
          onChange={(e) => setRunbookUrl(e.target.value)}
          error={urlInvalid}
          helperText={urlInvalid ? "Enter a full http(s) link." : "Leave empty to record that no runbook exists yet."}
        />
        <Autocomplete
          multiple
          freeSolo
          options={[]}
          value={alerts}
          inputValue={pendingAlert}
          onInputChange={(_e, value, reason) => {
            if (reason !== "reset") setPendingAlert(value);
          }}
          onChange={(_e, value) => {
            setAlerts(cleanAlertNames(value as string[]));
            setPendingAlert("");
          }}
          renderTags={(value, getTagProps) =>
            value.map((option, index) => {
              const { key, ...tagProps } = getTagProps({ index });
              return <Chip key={key} size="small" label={option} {...tagProps} />;
            })
          }
          renderInput={(params) => (
            <TextField
              {...params}
              label="Alerts"
              required
              placeholder={alerts.length === 0 ? "Type an alert name and press Enter" : undefined}
              error={takenAlerts.length > 0}
              helperText={
                takenAlerts.length > 0
                  ? `Already used by another runbook: ${takenAlerts.join(", ")}`
                  : "Press Enter after each alert name. An incident whose subject contains one gets this runbook (case-insensitive)."
              }
            />
          )}
        />
        {saveError && <Alert severity="error">{errorText(saveError, "Failed to save the runbook.")}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" disabled={!canSave} onClick={handleSave}>
          {editing ? "Save" : "Add"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function RunbookAlertSearch({ runbooks }: { runbooks: Runbook[] }): JSX.Element {
  const [query, setQuery] = useState("");
  const match = query.trim() ? findRunbookForIncident(runbooks, query) : undefined;
  // No alert is contained in the text: it may be part of an alert name, so list the alerts that contain it.
  const partial = query.trim() && !match ? findAlertsContaining(runbooks, query) : [];

  return (
    <Paper sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box>
        <Typography variant="subtitle2">Search runbook with alert name</Typography>
        <Typography variant="body2" color="text.secondary">
          Enter an alert name, or paste an incident subject, to find the runbook that covers it.
        </Typography>
      </Box>
      <TextField
        size="small"
        placeholder="example-gw-CriticalHTTP5xxCountLimitExceeded-prod-001"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        slotProps={{ htmlInput: { "aria-label": "Search runbook with alert name" } }}
      />
      {query.trim() && (
        <Alert severity={match?.runbook.runbookUrl ? "success" : partial.length > 0 ? "info" : "warning"}>
          {match ? (
            <Typography variant="body2" sx={{ wordBreak: "break-word" }}>
              {match.runbook.name} ({match.runbook.number}), matched alert: {match.alertName}
            </Typography>
          ) : partial.length > 0 ? (
            <>
              <Typography variant="body2">No alert matches this whole text. Alerts that contain it:</Typography>
              <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2.5 }}>
                {partial.map((p) => (
                  <Typography component="li" variant="body2" key={p.alertName} sx={{ wordBreak: "break-word" }}>
                    {p.runbook.name} ({p.runbook.number}): {p.alertName}
                    {p.runbook.runbookUrl ? (
                      <>
                        {" "}
                        <Link href={p.runbook.runbookUrl} target="_blank" rel="noopener noreferrer">
                          Open runbook
                        </Link>
                      </>
                    ) : (
                      " (no runbook yet)"
                    )}
                  </Typography>
                ))}
              </Box>
            </>
          ) : (
            <Typography variant="body2">No runbook covers this alert.</Typography>
          )}
          {match?.runbook.runbookUrl ? (
            <Link
              href={match.runbook.runbookUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="body2"
              sx={{ mt: 0.5, display: "inline-flex", alignItems: "center", gap: 0.5, fontWeight: 600, wordBreak: "break-all" }}
            >
              {match.runbook.runbookUrl}
              <ExternalLink size={14} />
            </Link>
          ) : match ? (
            <Typography variant="body2" sx={{ mt: 0.5, fontWeight: 600 }}>
              No runbook available for this alert.
            </Typography>
          ) : null}
        </Alert>
      )}
    </Paper>
  );
}
