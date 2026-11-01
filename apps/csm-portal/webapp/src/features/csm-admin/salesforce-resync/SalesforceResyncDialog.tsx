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
  Checkbox,
  Chip,
  CircularProgress,
  Collapse,
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
import { ChevronDown, ChevronRight, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useId, useMemo, useRef, useState, type JSX } from "react";
import {
  DELIBERATE_COMPONENT_KEYS,
  salesforceResyncErrorMessage,
  useSalesforceResyncApply,
  useSalesforceResyncPreview,
  type SalesforceResyncApplyComponent,
  type SalesforceResyncApplyStatus,
  type SalesforceResyncComponent,
  type SalesforceResyncComponentKey,
  type SalesforceResyncComponentStatus,
  type SalesforceResyncRecord,
  type SalesforceResyncRecordState,
  type SalesforceResyncScope,
} from "@features/csm-admin/salesforce-resync/useSalesforceResync";

/** Records shown per component before "Show more"; keeps a component with hundreds of records usable. */
const RECORD_PAGE_SIZE = 25;
/** Longest value shown inline; the full text is always in the tooltip. */
const MAX_VALUE_CHARS = 200;
/** Apply errors listed per component; the rest are summarised as a count. */
const MAX_ERRORS_SHOWN = 3;

const COMPONENT_STATUS: Record<
  SalesforceResyncComponentStatus,
  { label: string; color: "default" | "warning" | "success" }
> = {
  in_sync: { label: "In sync", color: "success" },
  differs: { label: "Differs", color: "warning" },
  skipped: { label: "Skipped", color: "default" },
};

const RECORD_STATE: Record<SalesforceResyncRecordState, { label: string; color: "default" | "warning" | "info" }> = {
  differs: { label: "Differs", color: "warning" },
  missing_in_csm: { label: "Missing in CSM", color: "warning" },
  only_in_csm: { label: "Only in CSM", color: "info" },
};

const APPLY_STATUS: Record<SalesforceResyncApplyStatus, { label: string; color: "default" | "error" | "success" }> = {
  applied: { label: "Applied", color: "success" },
  failed: { label: "Failed", color: "error" },
  skipped: { label: "Skipped", color: "default" },
};

function isDeliberate(key: SalesforceResyncComponentKey): boolean {
  return DELIBERATE_COMPONENT_KEYS.includes(key);
}

/** The components ticked when a comparison arrives: every differing one, except those that change access. */
function defaultSelection(components: SalesforceResyncComponent[]): Set<SalesforceResyncComponentKey> {
  return new Set(components.filter((c) => c.status === "differs" && !isDeliberate(c.key)).map((c) => c.key));
}

function pluralize(count: number, singular: string, plural = `${singular}s`): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

/** One value cell: "(empty)" for null/blank, a visually cut value with the full text on hover otherwise. */
function ValueText({ value }: { value: string | null | undefined }): JSX.Element {
  if (value === null || value === undefined || value === "") {
    return (
      <Typography variant="body2" color="text.secondary" component="span" sx={{ fontStyle: "italic" }}>
        (empty)
      </Typography>
    );
  }
  const cut = value.length > MAX_VALUE_CHARS;
  return (
    <Typography
      variant="body2"
      component="span"
      title={cut ? value : undefined}
      sx={{ wordBreak: "break-word", whiteSpace: "pre-wrap" }}
    >
      {cut ? `${value.slice(0, MAX_VALUE_CHARS)}…` : value}
    </Typography>
  );
}

function RecordRow({ record }: { record: SalesforceResyncRecord }): JSX.Element {
  const state = RECORD_STATE[record.state];
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75, py: 1 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="body2" sx={{ fontWeight: 500 }}>
          {record.label || record.sfId}
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: "monospace" }}>
          {record.sfId}
        </Typography>
        <Chip size="small" variant="outlined" color={state.color} label={state.label} />
      </Box>
      {record.state === "only_in_csm" && (
        <Typography variant="caption" color="text.secondary">
          Only the CSM database has this record. Informational: overwriting never deletes anything.
        </Typography>
      )}
      {record.state === "missing_in_csm" && (
        <Typography variant="caption" color="text.secondary">
          Salesforce has this record and the CSM database does not. Overwriting creates it.
        </Typography>
      )}
      {record.state === "differs" && record.fields.length > 0 && (
        <TableContainer sx={{ border: 1, borderColor: "divider", borderRadius: 1 }}>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Field</TableCell>
                <TableCell>CSM value</TableCell>
                <TableCell>Salesforce value</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {record.fields.map((f) => (
                <TableRow key={f.field}>
                  <TableCell sx={{ fontFamily: "monospace", verticalAlign: "top" }}>{f.field}</TableCell>
                  <TableCell sx={{ verticalAlign: "top" }}>
                    <ValueText value={f.csmValue} />
                  </TableCell>
                  <TableCell sx={{ verticalAlign: "top" }}>
                    <ValueText value={f.sfValue} />
                    {f.wouldTruncate && (
                      <Chip
                        size="small"
                        variant="outlined"
                        color="warning"
                        label="Would be cut"
                        sx={{ ml: 1 }}
                        title="The Salesforce value is longer than the CSM column and would be cut to fit"
                      />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  );
}

interface ComponentSectionProps {
  component: SalesforceResyncComponent;
  checked: boolean;
  disabled: boolean;
  onToggle: () => void;
}

function ComponentSection({ component, checked, disabled, onToggle }: ComponentSectionProps): JSX.Element {
  const [expanded, setExpanded] = useState(false);
  const [shown, setShown] = useState(RECORD_PAGE_SIZE);
  const status = COMPONENT_STATUS[component.status];
  const tickable = component.status === "differs";
  const detailId = useId();
  const { counts, records } = component;

  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Checkbox
          size="small"
          checked={checked}
          disabled={!tickable || disabled}
          onChange={onToggle}
          inputProps={{ "aria-label": `Overwrite ${component.label} from Salesforce` }}
        />
        <Typography variant="subtitle2">{component.label}</Typography>
        <Chip size="small" variant="outlined" color={status.color} label={status.label} />
        <Typography variant="caption" color="text.secondary">
          {counts.inSync} in sync, {counts.differs} differ, {counts.missingInCsm} missing in CSM, {counts.onlyInCsm}{" "}
          only in CSM
        </Typography>
      </Box>
      {isDeliberate(component.key) && tickable && (
        <Typography variant="caption" color="text.secondary" sx={{ pl: 5 }}>
          Changes who can access what, tick deliberately
        </Typography>
      )}
      {component.status === "skipped" && (
        <Typography variant="caption" color="text.secondary" sx={{ pl: 5 }}>
          {component.skippedReason || "This component was not compared."}
        </Typography>
      )}
      {records.length > 0 && (
        <Box sx={{ pl: 4 }}>
          <Button
            size="small"
            variant="text"
            startIcon={expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            aria-expanded={expanded}
            aria-controls={detailId}
            onClick={() => setExpanded((v) => !v)}
          >
            {expanded ? "Hide" : "Show"} {pluralize(records.length, "record")} for {component.label}
          </Button>
          <Collapse in={expanded} unmountOnExit>
            <Box id={detailId} sx={{ display: "flex", flexDirection: "column" }}>
              {records.slice(0, shown).map((r) => (
                <RecordRow key={`${r.state}-${r.sfId}`} record={r} />
              ))}
              {records.length > shown && (
                <Box>
                  <Button size="small" onClick={() => setShown((n) => n + RECORD_PAGE_SIZE)}>
                    Show {Math.min(RECORD_PAGE_SIZE, records.length - shown)} more ({records.length - shown} not shown)
                  </Button>
                </Box>
              )}
            </Box>
          </Collapse>
        </Box>
      )}
    </Paper>
  );
}

function ApplyResultRow({
  result,
  label,
}: {
  result: SalesforceResyncApplyComponent;
  label: string;
}): JSX.Element {
  const status = APPLY_STATUS[result.status];
  const errors = result.errors ?? [];
  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="subtitle2">{label}</Typography>
        <Chip size="small" variant="outlined" color={status.color} label={status.label} />
        <Typography variant="caption" color="text.secondary">
          {result.succeeded} succeeded, {result.failed} failed
        </Typography>
      </Box>
      {result.skippedStale > 0 && (
        <Typography variant="body2">
          {pluralize(result.skippedStale, "record")} changed since the comparison and{" "}
          {result.skippedStale === 1 ? "was" : "were"} left alone.
        </Typography>
      )}
      {result.status === "skipped" && result.skippedReason && (
        <Typography variant="body2" color="text.secondary">
          {result.skippedReason}
        </Typography>
      )}
      {errors.slice(0, MAX_ERRORS_SHOWN).map((e) => (
        <Typography key={`${e.sfId}-${e.message}`} variant="body2" color="error">
          {e.sfId}: {e.message}
        </Typography>
      ))}
      {errors.length > MAX_ERRORS_SHOWN && (
        <Typography variant="caption" color="text.secondary">
          and {errors.length - MAX_ERRORS_SHOWN} more
        </Typography>
      )}
    </Paper>
  );
}

export interface SalesforceResyncDialogProps {
  open: boolean;
  scope: SalesforceResyncScope;
  /** The CSM id of the account or project. */
  id: string;
  onClose: () => void;
}

/**
 * Admin "Compare with Salesforce" dialog for one account or project. Opening
 * it runs the read-only comparison (POST /salesforce/resync/preview); the
 * admin then ticks which components to overwrite from Salesforce
 * (POST /salesforce/resync/apply). Authorization is the backend's: this shows
 * the action and surfaces its 403, 409 and timeout answers as messages.
 *
 * Mount it only while it is open (`{open && <SalesforceResyncDialog open ... />}`)
 * so closing drops the comparison and the next open starts fresh.
 */
export default function SalesforceResyncDialog({ open, scope, id, onClose }: SalesforceResyncDialogProps): JSX.Element {
  const titleId = useId();
  const preview = useSalesforceResyncPreview();
  const apply = useSalesforceResyncApply();
  const [selected, setSelected] = useState<Set<SalesforceResyncComponentKey>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  // Guards the on-open comparison against React StrictMode's double effect run:
  // two concurrent previews would make the second one answer 409.
  const startedRef = useRef(false);

  const runPreview = (): void => {
    apply.reset();
    preview.mutate(
      { scope, id },
      { onSuccess: (data) => setSelected(defaultSelection(data.components)) },
    );
  };

  useEffect(() => {
    if (!open) {
      startedRef.current = false;
      return;
    }
    if (startedRef.current) return;
    startedRef.current = true;
    preview.mutate(
      { scope, id },
      { onSuccess: (data) => setSelected(defaultSelection(data.components)) },
    );
    // Runs once per open; `preview` is a fresh object each render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, scope, id]);

  const data = preview.data;
  const labelByKey = useMemo(
    () => new Map((data?.components ?? []).map((c) => [c.key, c.label])),
    [data],
  );
  const tickedComponents = (data?.components ?? []).filter((c) => selected.has(c.key));
  const differingCount = (data?.components ?? []).filter((c) => c.status === "differs").length;
  const busy = preview.isPending || apply.isPending;

  const toggle = (key: SalesforceResyncComponentKey): void => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  const handleConfirm = (): void => {
    setConfirmOpen(false);
    apply.mutate({ scope, id, components: tickedComponents.map((c) => c.key) });
  };

  const handleClose = (): void => {
    if (apply.isPending) return;
    onClose();
  };

  return (
    <>
      <Dialog open={open} onClose={handleClose} maxWidth="lg" fullWidth scroll="paper" aria-labelledby={titleId}>
        <DialogTitle id={titleId}>Compare with Salesforce</DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {preview.isPending && (
              <Box role="status" aria-live="polite" sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <CircularProgress size={16} />
                  <Typography variant="body2">
                    Comparing Salesforce with the CSM database. This can take up to a minute.
                  </Typography>
                </Box>
                <Skeleton variant="rounded" height={56} />
                <Skeleton variant="rounded" height={56} />
                <Skeleton variant="rounded" height={56} />
              </Box>
            )}

            {preview.isError && (
              <Alert severity="error">{salesforceResyncErrorMessage(preview.error)}</Alert>
            )}

            {apply.isPending && (
              <Box role="status" aria-live="polite" sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <CircularProgress size={16} />
                <Typography variant="body2">
                  Overwriting from Salesforce. This can take up to a minute, keep this window open.
                </Typography>
              </Box>
            )}

            {apply.isError && (
              <Alert severity="error" onClose={() => apply.reset()}>
                {salesforceResyncErrorMessage(apply.error)}
              </Alert>
            )}

            {data && !preview.isPending && !apply.data && (
              <>
                <Typography variant="body2">
                  {data.target.name || data.target.sfId}:{" "}
                  {differingCount === 0
                    ? "every compared component is in sync."
                    : `${pluralize(differingCount, "component")} of ${data.components.length} differ.`}
                </Typography>
                {data.capped && (
                  <Alert severity="warning">
                    {data.cappedReason || "The comparison stopped early, so it may be incomplete."}
                  </Alert>
                )}
                {data.components.map((c) => (
                  <ComponentSection
                    key={c.key}
                    component={c}
                    checked={selected.has(c.key)}
                    disabled={apply.isPending}
                    onToggle={() => toggle(c.key)}
                  />
                ))}
              </>
            )}

            {apply.data && (
              <>
                <Typography variant="subtitle1">Overwrite result</Typography>
                {apply.data.components.map((r) => (
                  <ApplyResultRow key={r.key} result={r} label={labelByKey.get(r.key) ?? r.key} />
                ))}
              </>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose} disabled={apply.isPending}>
            Close
          </Button>
          <Button
            startIcon={preview.isPending ? <CircularProgress size={16} color="inherit" /> : <RefreshCw size={16} />}
            onClick={runPreview}
            disabled={busy}
          >
            {apply.data ? "Run comparison again" : "Refresh comparison"}
          </Button>
          {!apply.data && (
            <Button
              variant="contained"
              onClick={() => setConfirmOpen(true)}
              disabled={busy || !data || tickedComponents.length === 0}
            >
              Overwrite selected ({tickedComponents.length})
            </Button>
          )}
        </DialogActions>
      </Dialog>

      <Dialog open={confirmOpen} onClose={() => setConfirmOpen(false)} aria-labelledby={`${titleId}-confirm`}>
        <DialogTitle id={`${titleId}-confirm`}>Overwrite from Salesforce?</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">
            <p>Salesforce values replace the CSM values for these components:</p>
            <ul>
              {tickedComponents.map((c) => (
                <li key={c.key}>{c.label}</li>
              ))}
            </ul>
            <p>Nothing is deleted. Records changed since the comparison are left alone.</p>
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleConfirm}>
            Overwrite
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
