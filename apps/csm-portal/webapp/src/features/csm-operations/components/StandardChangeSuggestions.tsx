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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  Link,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, FilePlus, Undo2 } from "@wso2/oxygen-ui-icons-react";
import { Fragment, useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import QueryErrorState from "@components/QueryErrorState";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import {
  useCreateStandardChangeProposal,
  useDismissStandardChangeSuggestion,
  useRestoreStandardChangeSuggestion,
  useSearchStandardChangeCustomersForSelect,
  useStandardChangeSuggestions,
  useUpdateStandardChangeProposal,
} from "@features/csm-operations/api/useStandardChanges";
import {
  MAX_SUGGESTION_DISMISS_REASON_LENGTH,
  suggestionActions,
  suggestionCriteriaText,
  suggestionProposalEvidence,
  suggestionUpdatedText,
  SUGGESTIONS_NOT_COMPUTED_YET,
} from "@features/csm-operations/utils/standardChangeSuggestions";
import { standardChangeProposalPath } from "@features/csm-operations/utils/standardChanges";
import type { BeEntityRef, BeStandardChangeSuggestion } from "@api/backend/types";

const COLUMNS = 6;

function formatDate(value?: string | null): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium" }) ?? "—";
}

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message.trim() ? err.message : fallback;
}

function changeRequestPath(id: string): string {
  return `/operations/change-requests/${encodeURIComponent(id)}`;
}

/** One line of the expanded details: a label and what it says. */
function DetailLine({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
      <Typography variant="body2" color="text.secondary" sx={{ minWidth: 140 }}>
        {label}
      </Typography>
      <Box sx={{ flex: 1, minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function RefLinks({ refs, path }: { refs: BeEntityRef[]; path: (id: string) => string }): JSX.Element {
  return (
    <Typography variant="body2" component="span">
      {refs.map((r, i) => (
        <Fragment key={r.id}>
          {i > 0 && ", "}
          <Link component={RouterLink} to={path(r.id)} underline="hover">
            {r.name || r.id}
          </Link>
        </Fragment>
      ))}
    </Typography>
  );
}

function breakdownText(b: BeStandardChangeSuggestion["risk"], middle: string): string {
  const parts = [
    b.low && `${b.low} low`,
    b.moderate && `${b.moderate} ${middle}`,
    b.high && `${b.high} high`,
    b.unknown && `${b.unknown} unknown`,
  ].filter(Boolean);
  return parts.length ? parts.join(", ") : "—";
}

function SuggestionDetails({ s }: { s: BeStandardChangeSuggestion }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75, py: 1 }}>
      {s.unmet.length > 0 && (
        <DetailLine label="Not yet a candidate">
          <Typography variant="body2">{s.unmet.join("; ")}</Typography>
        </DetailLine>
      )}
      <DetailLine label="Risk">
        <Typography variant="body2">{breakdownText(s.risk, "moderate")}</Typography>
      </DetailLine>
      <DetailLine label="Impact">
        <Typography variant="body2">{breakdownText(s.impact, "medium")}</Typography>
      </DetailLine>
      {s.incidents.length > 0 && (
        <DetailLine label="Incidents">
          <RefLinks refs={s.incidents} path={(id) => `/operations/incidents/${encodeURIComponent(id)}`} />
        </DetailLine>
      )}
      {s.openProposals.length > 0 && (
        <DetailLine label="Already proposed in">
          <RefLinks refs={s.openProposals} path={standardChangeProposalPath} />
        </DetailLine>
      )}
      {s.dismissal && (
        <DetailLine label="Dismissed">
          <Typography variant="body2">
            {`${s.dismissal.reason} (${s.dismissal.dismissedBy?.name || "someone"}, ${formatDate(s.dismissal.dismissedOn)})`}
          </Typography>
        </DetailLine>
      )}
      <DetailLine label={`Change requests (${s.memberCount})`}>
        <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
          {s.members.map((m) => (
            <li key={m.id}>
              <Typography variant="body2" component="span">
                <Link component={RouterLink} to={changeRequestPath(m.id)} underline="hover">
                  {m.number}
                </Link>
                {` · ${m.subject} · ${m.state === "rollback" ? "rolled back" : "closed"} ${formatDate(m.completedOn)}`}
                {m.joinedBy === "clone" ? " · a clone" : ""}
                {m.hasDeviation ? " · deviated from its plan" : ""}
              </Typography>
            </li>
          ))}
        </Box>
        {s.memberCount > s.members.length && (
          <Typography variant="caption" color="text.secondary">{`The newest ${s.members.length} are listed.`}</Typography>
        )}
      </DetailLine>
    </Box>
  );
}

/** The reason a suggestion is dismissed with: required, at most 1000 characters. */
function DismissDialog({
  suggestion,
  pending,
  error,
  onCancel,
  onConfirm,
}: {
  suggestion: BeStandardChangeSuggestion;
  pending: boolean;
  error: string | null;
  onCancel: () => void;
  onConfirm: (reason: string) => void;
}): JSX.Element {
  const [reason, setReason] = useState("");
  const trimmed = reason.trim();
  return (
    <Dialog open onClose={pending ? undefined : onCancel} maxWidth="sm" fullWidth aria-labelledby="dismiss-suggestion-title">
      <DialogTitle id="dismiss-suggestion-title">Dismiss this suggestion</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="body2">
          {`"${suggestion.subject}" for ${suggestion.customer.name || "this customer"} will no longer be suggested, however often it is done again. It can be restored from "Show dismissed".`}
        </Typography>
        <TextField
          label="Why is this not a standard change?"
          required
          multiline
          minRows={2}
          value={reason}
          onChange={(e) => setReason(e.target.value.slice(0, MAX_SUGGESTION_DISMISS_REASON_LENGTH))}
          helperText={`${MAX_SUGGESTION_DISMISS_REASON_LENGTH - reason.length} characters left`}
          autoFocus
        />
        {error && <Alert severity="error">{error}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={pending}>
          Go back
        </Button>
        <Button variant="contained" onClick={() => onConfirm(trimmed)} disabled={pending || !trimmed}>
          Dismiss
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/**
 * Suggested standard changes: clusters of one customer's completed change
 * requests that look like the same change done again. entity-service finds and
 * judges them once a day and `GET /standard-change-suggestions` returns the
 * last snapshot (`computedOn`, shown as "Updated …"); nothing is computed or
 * decided here. Each row offers Propose (a new standard change from its newest
 * clean change request, citing the others) and Dismiss with a reason. It is not
 * related to the customer's existing templates, on purpose (see
 * suggestionActions).
 */
export default function StandardChangeSuggestions({ leading, trailing }: { leading: ReactNode; trailing: ReactNode }): JSX.Element {
  const { canWrite } = usePortalAccess();
  const navigate = useNavTransition();
  const location = useLocation();
  const { showError } = useErrorBanner();
  const [customer, setCustomer] = useState<{ id: string; label?: string }>({ id: "" });
  const [includeIneligible, setIncludeIneligible] = useState(false);
  const [includeDismissed, setIncludeDismissed] = useState(false);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [dismissing, setDismissing] = useState<BeStandardChangeSuggestion | null>(null);
  const [dismissError, setDismissError] = useState<string | null>(null);
  const [busyKey, setBusyKey] = useState<string | null>(null);

  const { data, isLoading, isError, error, refetch, isFetching } = useStandardChangeSuggestions({
    accountId: customer.id || undefined,
    includeIneligible,
    includeDismissed,
  });
  const createProposal = useCreateStandardChangeProposal();
  const citeEvidence = useUpdateStandardChangeProposal();
  const dismiss = useDismissStandardChangeSuggestion();
  const restore = useRestoreStandardChangeSuggestion();
  const suggestions = data?.suggestions ?? [];
  const from = { from: `${location.pathname}${location.search}` };
  const rowKey = (s: BeStandardChangeSuggestion): string => `${s.customer.id}|${s.key}`;

  /** Start a proposal, then cite the evidence; the proposal opens either way. */
  const startProposal = (
    s: BeStandardChangeSuggestion,
    body: Parameters<typeof createProposal.mutate>[0],
    evidence: string[],
    what: string,
  ): void => {
    setBusyKey(rowKey(s));
    createProposal.mutate(body, {
      onSuccess: (proposal) => {
        const open = (): void => {
          setBusyKey(null);
          navigate(standardChangeProposalPath(proposal.id), { state: from });
        };
        if (evidence.length === 0) {
          open();
          return;
        }
        citeEvidence.mutate(
          { id: proposal.id, patch: { evidenceChangeRequestIds: evidence } },
          {
            onError: (err) =>
              showError(errorText(err, `The ${what} was started, but its evidence could not be cited: add it on the proposal.`), err),
            onSettled: open,
          },
        );
      },
      onError: (err) => {
        setBusyKey(null);
        showError(errorText(err, `Could not start the ${what}.`), err);
      },
    });
  };

  const confirmDismiss = (reason: string): void => {
    if (!dismissing) return;
    setDismissError(null);
    dismiss.mutate(
      { accountId: dismissing.customer.id, key: dismissing.key, reason },
      {
        onSuccess: () => setDismissing(null),
        onError: (err) => setDismissError(errorText(err, "Could not dismiss the suggestion.")),
      },
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Paper sx={{ p: 2, display: "flex", gap: 1.5, flexWrap: "wrap", alignItems: "center" }}>
        {leading}
        <Box sx={{ minWidth: 220, flex: "0 1 260px" }}>
          <AsyncEntitySelect<BeEntityRef>
            id="std-suggestion-customer"
            label="Customer"
            placeholder="All customers"
            shrinkLabel
            value={customer.id}
            knownLabel={customer.label}
            onChange={(next, item) => setCustomer({ id: next, label: item?.name })}
            useSearch={useSearchStandardChangeCustomersForSelect}
            getId={(a) => a.id}
            getLabel={(a) => a.name}
          />
        </Box>
        <FormControlLabel
          control={<Checkbox size="small" checked={includeIneligible} onChange={(e) => setIncludeIneligible(e.target.checked)} />}
          label="Show near misses"
        />
        <FormControlLabel
          control={<Checkbox size="small" checked={includeDismissed} onChange={(e) => setIncludeDismissed(e.target.checked)} />}
          label="Show dismissed"
        />
        {trailing}
      </Paper>

      {data && (
        <Typography variant="body2" color="text.secondary" role="status">
          {data.computedOn
            ? `${suggestionUpdatedText(data.computedOn)}. ${suggestionCriteriaText(data)}`
            : SUGGESTIONS_NOT_COMPUTED_YET}
        </Typography>
      )}

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden", opacity: isFetching && !isLoading ? 0.7 : 1 }}>
        <TableContainer>
          <Table size="small" aria-label="Suggested standard changes" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell sx={{ width: 48 }} />
                <TableCell>Change</TableCell>
                <TableCell sx={{ width: 180 }}>Customer</TableCell>
                <TableCell sx={{ width: 130 }}>Done</TableCell>
                <TableCell sx={{ width: 120 }}>Last done</TableCell>
                <TableCell sx={{ width: 260 }}>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: COLUMNS }).map((__, c) => (
                      <TableCell key={c}>
                        <Skeleton variant="rounded" width={c === 1 ? "70%" : 60} height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={COLUMNS} align="center">
                    <QueryErrorState
                      message={errorText(error, "Failed to load standard change suggestions.")}
                      error={error}
                      onRetry={() => void refetch()}
                    />
                  </TableCell>
                </TableRow>
              ) : suggestions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={COLUMNS} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      {data && !data.computedOn
                        ? SUGGESTIONS_NOT_COMPUTED_YET
                        : "No change is done often enough, and reliably enough, to suggest a standard change."}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                suggestions.map((s) => {
                  const key = rowKey(s);
                  const open = !!expanded[key];
                  const actions = suggestionActions(s, canWrite);
                  const busy = busyKey === key;
                  return (
                    <Fragment key={key}>
                      <TableRow sx={{ "& > td": { borderBottom: open ? "none" : undefined }, opacity: s.dismissal ? 0.7 : 1 }}>
                        <TableCell>
                          <IconButton
                            size="small"
                            aria-label={open ? `Hide the details of ${s.subject}` : `Show the details of ${s.subject}`}
                            aria-expanded={open}
                            onClick={() => setExpanded((prev) => ({ ...prev, [key]: !open }))}
                          >
                            {open ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                          </IconButton>
                        </TableCell>
                        <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>
                          <Typography variant="body2">{s.subject}</Typography>
                          <Box sx={{ display: "flex", gap: 0.5, flexWrap: "wrap", mt: 0.5 }}>
                            {!s.eligible && <Chip size="small" variant="outlined" color="default" label="Near miss" />}
                            {s.dismissal && <Chip size="small" variant="outlined" color="default" label="Dismissed" />}
                            {s.incidents.length > 0 && (
                              <Chip size="small" variant="outlined" color="error" label={`${s.incidents.length} incident${s.incidents.length === 1 ? "" : "s"}`} />
                            )}
                          </Box>
                        </TableCell>
                        <TableCell sx={{ maxWidth: 200, wordBreak: "break-word" }}>{s.customer.name || "—"}</TableCell>
                        <TableCell>
                          <Typography variant="body2">{`${s.closedCount} closed`}</Typography>
                          <Typography variant="caption" color="text.secondary">
                            {`${s.rollbackCount} rolled back · ${Math.round(s.successRate * 100)}%`}
                          </Typography>
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDate(s.lastCompletedOn)}</TableCell>
                        <TableCell>
                          <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap", alignItems: "center" }}>
                            {actions.propose && (
                              <Tooltip title={actions.propose.blockedReason ?? ""}>
                                <span>
                                  <Button
                                    size="small"
                                    variant="contained"
                                    startIcon={<FilePlus size={14} />}
                                    disabled={!!actions.propose.blockedReason || busy}
                                    aria-label={actions.propose.blockedReason ? `Propose: ${actions.propose.blockedReason}` : undefined}
                                    onClick={() =>
                                      startProposal(
                                        s,
                                        { kind: "new", sourceChangeRequestId: s.proposeFrom!.id },
                                        suggestionProposalEvidence(s),
                                        "proposal",
                                      )
                                    }
                                  >
                                    Propose
                                  </Button>
                                </span>
                              </Tooltip>
                            )}
                            {actions.dismiss && (
                              <Button size="small" variant="text" disabled={busy} onClick={() => setDismissing(s)}>
                                Dismiss
                              </Button>
                            )}
                            {actions.restore && s.dismissal && (
                              <Button
                                size="small"
                                variant="text"
                                startIcon={<Undo2 size={14} />}
                                disabled={restore.isPending}
                                onClick={() =>
                                  restore.mutate(s.dismissal!.id, {
                                    onError: (err) => showError(errorText(err, "Could not restore the suggestion."), err),
                                  })
                                }
                              >
                                Restore
                              </Button>
                            )}
                          </Box>
                        </TableCell>
                      </TableRow>
                      {open && (
                        <TableRow>
                          <TableCell />
                          <TableCell colSpan={COLUMNS - 1}>
                            <SuggestionDetails s={s} />
                          </TableCell>
                        </TableRow>
                      )}
                    </Fragment>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>

      {dismissing && (
        <DismissDialog
          suggestion={dismissing}
          pending={dismiss.isPending}
          error={dismissError}
          onCancel={() => {
            setDismissing(null);
            setDismissError(null);
          }}
          onConfirm={confirmDismiss}
        />
      )}
    </Box>
  );
}
