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
  FormControl,
  IconButton,
  InputAdornment,
  InputLabel,
  Link,
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
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from "@wso2/oxygen-ui";
import { Inbox, Plus, Search, X } from "@wso2/oxygen-ui-icons-react";
import { Fragment, useEffect, useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useSearchParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import {
  useCountStandardChangeProposals,
  useCreateStandardChangeProposal,
  useSearchStandardChangeProposals,
  useSearchStandardChangeTemplates,
} from "@features/csm-operations/api/useStandardChanges";
import { changeRequestImpactLabel } from "@features/csm-operations/utils/changeRequests";
import {
  CHANGE_REQUESTS_PATH,
  changeRequestRiskLabel,
  proposalScope,
  proposalStateFilter,
  STANDARD_CHANGE_PROPOSAL_STATES,
  standardChangeProposalKindLabel,
  standardChangeProposalPath,
  standardChangeProposalStateColor,
  standardChangeProposalStateLabel,
  standardChangesSearch,
  standardChangeTemplatePath,
  standardChangeTemplateStateColor,
  standardChangeTemplateStateLabel,
  type ProposalScope,
  type StandardChangesShow,
} from "@features/csm-operations/utils/standardChanges";
import type {
  BeSearchStandardChangeProposalsPayload,
  BeSearchStandardChangeTemplatesPayload,
  BeStandardChangeProposalState,
  BeStandardChangeTemplateState,
} from "@api/backend/types";

const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];
const DEFAULT_ROWS_PER_PAGE = 20;
const SEARCH_DEBOUNCE_MS = 300;

/** The catalogue's state filter. "live" is the server's default: active and suspended. */
type TemplateStateFilter = "live" | BeStandardChangeTemplateState | "all";
const TEMPLATE_STATE_FILTERS: ReadonlyArray<{ value: TemplateStateFilter; label: string }> = [
  { value: "live", label: "Active and suspended" },
  { value: "active", label: "Active" },
  { value: "suspended", label: "Suspended" },
  { value: "retired", label: "Retired" },
  { value: "all", label: "All" },
];

function templateStates(filter: TemplateStateFilter): BeStandardChangeTemplateState[] | undefined {
  if (filter === "live") return undefined;
  if (filter === "all") return ["active", "suspended", "retired"];
  return [filter];
}

/** Whose proposals the proposals view lists (`?scope=`). */
const PROPOSAL_SCOPE_OPTIONS: ReadonlyArray<{ value: ProposalScope; label: string }> = [
  { value: "all", label: "All proposals" },
  { value: "mine", label: "Proposed by me" },
  { value: "awaiting", label: "Awaiting my decision" },
];

function formatDate(value?: string | null): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium" }) ?? "—";
}

function errorText(err: unknown, fallback: string): string {
  return err instanceof Error && err.message.trim() ? err.message : fallback;
}

/** The search box's text, sent once typing pauses. */
function useDebounced(value: string): string {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [value]);
  return debounced;
}

function SearchField({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }): JSX.Element {
  return (
    <TextField
      size="small"
      placeholder={`${label}…`}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      sx={{ flex: 1, minWidth: 240 }}
      slotProps={{
        htmlInput: { "aria-label": label },
        input: {
          startAdornment: (
            <InputAdornment position="start">
              <Search size={16} />
            </InputAdornment>
          ),
          endAdornment: value ? (
            <InputAdornment position="end">
              <IconButton size="small" edge="end" onClick={() => onChange("")} aria-label="Clear search">
                <X size={16} />
              </IconButton>
            </InputAdornment>
          ) : undefined,
        },
      }}
    />
  );
}

/** What every view's filter bar starts and ends with: the Templates | Proposals switch, and Propose. */
interface ToolbarSlots {
  leading: ReactNode;
  trailing: ReactNode;
}

function TemplateCatalogue({ leading, trailing }: ToolbarSlots): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [search, setSearch] = useState("");
  const query = useDebounced(search.trim());
  const [stateFilter, setStateFilter] = useState<TemplateStateFilter>("live");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const payload: BeSearchStandardChangeTemplatesPayload = { pagination: { offset: page * rowsPerPage, limit: rowsPerPage } };
  const states = templateStates(stateFilter);
  if (states) payload.states = states;
  if (query) payload.query = query;
  const { data, isLoading, isError, error, refetch } = useSearchStandardChangeTemplates(payload);
  const templates = data?.templates ?? [];
  const columns = 7;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Paper sx={{ p: 2, display: "flex", gap: 1.5, flexWrap: "wrap", alignItems: "center" }}>
        {leading}
        <SearchField
          label="Search templates by number or name"
          value={search}
          onChange={(v) => {
            setSearch(v);
            setPage(0);
          }}
        />
        <FormControl size="small" sx={{ minWidth: 220 }}>
          <InputLabel id="std-template-state-label">State</InputLabel>
          <Select
            labelId="std-template-state-label"
            label="State"
            value={stateFilter}
            onChange={(e) => {
              setStateFilter(e.target.value as TemplateStateFilter);
              setPage(0);
            }}
          >
            {TEMPLATE_STATE_FILTERS.map((o) => (
              <MenuItem key={o.value} value={o.value}>
                {o.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        {trailing}
      </Paper>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="Standard change templates" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell sx={{ width: 120 }}>Number</TableCell>
                <TableCell>Name</TableCell>
                <TableCell sx={{ width: 80 }}>Version</TableCell>
                <TableCell sx={{ width: 100 }}>Risk</TableCell>
                <TableCell sx={{ width: 100 }}>Impact</TableCell>
                <TableCell sx={{ width: 150 }}>Used</TableCell>
                <TableCell sx={{ width: 120 }}>State</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: columns }).map((__, c) => (
                      <TableCell key={c}>
                        <Skeleton variant="rounded" width={c === 1 ? "70%" : 60} height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={columns} align="center">
                    <QueryErrorState
                      message={errorText(error, "Failed to load standard change templates.")}
                      error={error}
                      onRetry={() => void refetch()}
                    />
                  </TableCell>
                </TableRow>
              ) : templates.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No standard change templates found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                templates.map((t) => {
                  const path = standardChangeTemplatePath(t.id);
                  const state = { from: `${location.pathname}${location.search}` };
                  return (
                    <TableRow
                      key={t.id}
                      hover
                      tabIndex={0}
                      sx={{ cursor: "pointer" }}
                      onClick={() => navigate(path, { state })}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") navigate(path, { state });
                      }}
                    >
                      <TableCell sx={{ whiteSpace: "nowrap" }}>{t.number}</TableCell>
                      <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>
                        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                          <Typography variant="body2">{t.name}</Typography>
                          {t.reviewRequired && (
                            <Chip size="small" variant="outlined" color="warning" label="Review needed" />
                          )}
                        </Box>
                      </TableCell>
                      <TableCell>{t.currentVersion ? `v${t.currentVersion.version}` : "—"}</TableCell>
                      <TableCell>{changeRequestRiskLabel(t.currentVersion?.risk)}</TableCell>
                      <TableCell>{t.currentVersion?.impact ? changeRequestImpactLabel(t.currentVersion.impact) : "—"}</TableCell>
                      <TableCell>
                        <Typography variant="body2">{`${t.usage.total} total, ${t.usage.open} open`}</Typography>
                        {t.usage.rolledBack > 0 && (
                          <Typography variant="caption" color="text.secondary">{`${t.usage.rolledBack} rolled back`}</Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={standardChangeTemplateStateColor(t.state)}
                          label={standardChangeTemplateStateLabel(t.state)}
                        />
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={data?.total ?? 0}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(Number(e.target.value));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}

function ProposalList({ leading, trailing }: ToolbarSlots): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = useState("");
  const query = useDebounced(search.trim());
  // Both filters are in the URL, so a filtered list is linkable (the
  // attention line above the catalogue links straight to one).
  const scope = proposalScope(searchParams);
  const stateFilter = proposalStateFilter(searchParams);
  const setFilters = (next: { scope: ProposalScope; state: BeStandardChangeProposalState | "" }): void =>
    setSearchParams((prev) => standardChangesSearch(prev, "proposals", next), { replace: true });
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const payload: BeSearchStandardChangeProposalsPayload = { pagination: { offset: page * rowsPerPage, limit: rowsPerPage } };
  if (scope === "mine") payload.proposedByMe = true;
  if (scope === "awaiting") payload.awaitingMyDecision = true;
  if (stateFilter) payload.states = [stateFilter];
  if (query) payload.query = query;
  const { data, isLoading, isError, error, refetch } = useSearchStandardChangeProposals(payload);
  const proposals = data?.proposals ?? [];
  const columns = 7;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Paper sx={{ p: 2, display: "flex", gap: 1.5, flexWrap: "wrap", alignItems: "center" }}>
        {leading}
        <SearchField
          label="Search proposals by number or name"
          value={search}
          onChange={(v) => {
            setSearch(v);
            setPage(0);
          }}
        />
        <FormControl size="small" sx={{ minWidth: 200 }}>
          <InputLabel id="std-proposal-scope-label">Show</InputLabel>
          <Select
            labelId="std-proposal-scope-label"
            label="Show"
            value={scope}
            onChange={(e) => {
              setFilters({ scope: e.target.value as ProposalScope, state: stateFilter });
              setPage(0);
            }}
          >
            {PROPOSAL_SCOPE_OPTIONS.map((o) => (
              <MenuItem key={o.value} value={o.value}>
                {o.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size="small" sx={{ minWidth: 180 }}>
          <InputLabel id="std-proposal-state-label">State</InputLabel>
          <Select
            labelId="std-proposal-state-label"
            label="State"
            value={stateFilter || "any"}
            onChange={(e) => {
              const v = String(e.target.value);
              setFilters({ scope, state: v === "any" ? "" : (v as BeStandardChangeProposalState) });
              setPage(0);
            }}
          >
            <MenuItem value="any">Any state</MenuItem>
            {STANDARD_CHANGE_PROPOSAL_STATES.map((s) => (
              <MenuItem key={s} value={s}>
                {standardChangeProposalStateLabel(s)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        {trailing}
      </Paper>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="Standard change proposals" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell sx={{ width: 120 }}>Number</TableCell>
                <TableCell>Name</TableCell>
                <TableCell sx={{ width: 130 }}>Kind</TableCell>
                <TableCell sx={{ width: 120 }}>State</TableCell>
                <TableCell sx={{ width: 140 }}>Template</TableCell>
                <TableCell sx={{ width: 180 }}>Proposed by</TableCell>
                <TableCell sx={{ width: 130 }}>Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: columns }).map((__, c) => (
                      <TableCell key={c}>
                        <Skeleton variant="rounded" width={c === 1 ? "70%" : 60} height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={columns} align="center">
                    <QueryErrorState
                      message={errorText(error, "Failed to load proposals.")}
                      error={error}
                      onRetry={() => void refetch()}
                    />
                  </TableCell>
                </TableRow>
              ) : proposals.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={columns} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      {scope === "awaiting" ? "No proposals are waiting for your decision." : "No proposals found."}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                proposals.map((p) => {
                  const path = standardChangeProposalPath(p.id);
                  const state = { from: `${location.pathname}${location.search}` };
                  return (
                    <TableRow
                      key={p.id}
                      hover
                      tabIndex={0}
                      sx={{ cursor: "pointer" }}
                      onClick={() => navigate(path, { state })}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") navigate(path, { state });
                      }}
                    >
                      <TableCell sx={{ whiteSpace: "nowrap" }}>{p.number}</TableCell>
                      <TableCell sx={{ maxWidth: 360, wordBreak: "break-word" }}>{p.name || "Untitled proposal"}</TableCell>
                      <TableCell>{standardChangeProposalKindLabel(p.kind)}</TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={standardChangeProposalStateColor(p.state)}
                          label={standardChangeProposalStateLabel(p.state)}
                        />
                      </TableCell>
                      <TableCell sx={{ maxWidth: 200, wordBreak: "break-word" }}>{p.template?.name ?? "—"}</TableCell>
                      <TableCell sx={{ maxWidth: 220, wordBreak: "break-word" }}>{p.proposedBy?.name || p.proposedBy?.email || "—"}</TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>{formatDate(p.updatedOn)}</TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={data?.total ?? 0}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(Number(e.target.value));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

/**
 * "2 proposals await your CAB decision · 1 draft of yours" above the
 * catalogue, each part a link to the proposals filtered to it. Only what
 * applies to the viewer is said, and nothing at all when nothing does. It is a
 * hint, not the list: nothing is shown until both counts are in (so it does not
 * flash in piece by piece), and a count that fails is left out without a word.
 * The links are built from the current query, so the change request list's
 * `cr*` filters kept in it survive a trip through the proposals.
 */
function AttentionLine(): JSX.Element | null {
  const [searchParams] = useSearchParams();
  const proposalsHref = (filters: Parameters<typeof standardChangesSearch>[2]): string =>
    `${CHANGE_REQUESTS_PATH}?${standardChangesSearch(searchParams, "proposals", filters).toString()}`;
  const awaiting = useCountStandardChangeProposals({ awaitingMyDecision: true });
  const drafts = useCountStandardChangeProposals({ proposedByMe: true, states: ["draft"] });
  if (awaiting.isPending || drafts.isPending) return null;

  const parts: Array<{ key: string; text: string; to: string }> = [];
  const awaitingCount = awaiting.isSuccess ? awaiting.data : 0;
  const draftCount = drafts.isSuccess ? drafts.data : 0;
  if (awaitingCount > 0) {
    parts.push({
      key: "awaiting",
      text: `${plural(awaitingCount, "proposal awaits", "proposals await")} your CAB decision`,
      to: proposalsHref({ scope: "awaiting" }),
    });
  }
  if (draftCount > 0) {
    parts.push({
      key: "drafts",
      text: `${plural(draftCount, "draft", "drafts")} of yours`,
      to: proposalsHref({ scope: "mine", state: "draft" }),
    });
  }
  if (parts.length === 0) return null;

  return (
    <Alert severity="info" icon={<Inbox size={18} />} role="status" aria-label="Needs your attention">
      {parts.map((p, i) => (
        <Fragment key={p.key}>
          {i > 0 && " · "}
          <Link component={RouterLink} to={p.to} underline="hover" color="inherit" sx={{ fontWeight: 500 }}>
            {p.text}
          </Link>
        </Fragment>
      ))}
    </Alert>
  );
}

/**
 * Standard changes, the second sub-view of Operations > Change requests (see
 * `ChangeRequestsArea`, which owns the sub-navigation and the `?view=standard`
 * it is kept in). It opens on the catalogue of CAB-approved standard change
 * templates (a standard change can only be raised from an active one), under a
 * line saying what needs the viewer's attention, if anything does. The
 * proposals that create, revise or retire templates are a secondary view of the
 * same tab (`&show=proposals`, the Templates | Proposals switch beside the
 * search), with their filters in the URL too. A row opens with `state.from` set
 * to this URL, so Back from a template or proposal returns to the view it was
 * opened from.
 */
export default function StandardChangesTab({ show }: { show: StandardChangesShow }): JSX.Element {
  const { canWrite } = usePortalAccess();
  const navigate = useNavTransition();
  const location = useLocation();
  const [, setSearchParams] = useSearchParams();
  const createProposal = useCreateStandardChangeProposal();

  const propose = (): void => {
    createProposal.mutate(
      { kind: "new" },
      {
        onSuccess: (proposal) =>
          navigate(standardChangeProposalPath(proposal.id), {
            state: { from: `${location.pathname}${location.search}` },
          }),
      },
    );
  };

  const toggle = (
    <ToggleButtonGroup
      exclusive
      size="small"
      value={show}
      aria-label="Templates or proposals"
      onChange={(_, next: StandardChangesShow | null) => {
        if (next && next !== show) setSearchParams((prev) => standardChangesSearch(prev, next));
      }}
    >
      <ToggleButton value="templates" sx={{ textTransform: "none", px: 1.5 }}>
        Templates
      </ToggleButton>
      <ToggleButton value="proposals" sx={{ textTransform: "none", px: 1.5 }}>
        Proposals
      </ToggleButton>
    </ToggleButtonGroup>
  );
  const proposeButton = canWrite ? (
    <Button
      variant="contained"
      size="small"
      startIcon={<Plus size={16} />}
      loading={createProposal.isPending}
      onClick={propose}
      sx={{ ml: "auto", whiteSpace: "nowrap" }}
    >
      Propose a standard change
    </Button>
  ) : null;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {show === "templates" && <AttentionLine />}

      {createProposal.isError && (
        <Alert severity="error" onClose={() => createProposal.reset()}>
          {errorText(createProposal.error, "Could not start the proposal.")}
        </Alert>
      )}

      {show === "templates" ? (
        <TemplateCatalogue leading={toggle} trailing={proposeButton} />
      ) : (
        <ProposalList leading={toggle} trailing={proposeButton} />
      )}
    </Box>
  );
}
