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
  MenuItem,
  Paper,
  Select,
  Skeleton,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tabs,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Plus, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, type JSX } from "react";
import { useLocation, useSearchParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { usePortalAccess } from "@context/current-user/usePortalAccess";
import { useNavTransition } from "@hooks/useNavTransition";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import {
  useCreateStandardChangeProposal,
  useSearchStandardChangeProposals,
  useSearchStandardChangeTemplates,
} from "@features/csm-operations/api/useStandardChanges";
import { changeRequestImpactLabel } from "@features/csm-operations/utils/changeRequests";
import {
  changeRequestRiskLabel,
  STANDARD_CHANGE_PROPOSAL_STATES,
  standardChangeProposalKindLabel,
  standardChangeProposalPath,
  standardChangeProposalStateColor,
  standardChangeProposalStateLabel,
  standardChangeTemplatePath,
  standardChangeTemplateStateColor,
  standardChangeTemplateStateLabel,
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

/** Whose proposals the Proposals view lists. */
type ProposalScope = "mine" | "awaiting" | "all";
const PROPOSAL_SCOPES: ReadonlyArray<{ value: ProposalScope; label: string }> = [
  { value: "mine", label: "Proposed by me" },
  { value: "awaiting", label: "Awaiting my decision" },
  { value: "all", label: "All proposals" },
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

function TemplateCatalogue(): JSX.Element {
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

function ProposalList(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [search, setSearch] = useState("");
  const query = useDebounced(search.trim());
  const [scope, setScope] = useState<ProposalScope>("mine");
  const [stateFilter, setStateFilter] = useState<BeStandardChangeProposalState | "">("");
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
              setScope(e.target.value as ProposalScope);
              setPage(0);
            }}
          >
            {PROPOSAL_SCOPES.map((o) => (
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
              setStateFilter(v === "any" ? "" : (v as BeStandardChangeProposalState));
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

/**
 * Operations -> Standard Changes: the catalogue of CAB-approved standard change
 * templates (a standard change can only be raised from an active one) and the
 * proposals that create, revise or retire them. The view (`?view=proposals`) is
 * in the URL, so Back from a proposal returns to the list it was opened from.
 */
export default function StandardChangesTab(): JSX.Element {
  const { canWrite } = usePortalAccess();
  const navigate = useNavTransition();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const view = searchParams.get("view") === "proposals" ? "proposals" : "catalogue";
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

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 720 }}>
          Standard changes are pre-authorised: the CAB approves a template once, and a change request raised from an
          active template needs no approval of its own. Any completed change request can be proposed as a template.
        </Typography>
        {canWrite && (
          <Button
            variant="contained"
            size="small"
            startIcon={<Plus size={16} />}
            loading={createProposal.isPending}
            onClick={propose}
          >
            Propose a standard change
          </Button>
        )}
      </Box>

      {createProposal.isError && (
        <Alert severity="error" onClose={() => createProposal.reset()}>
          {errorText(createProposal.error, "Could not start the proposal.")}
        </Alert>
      )}

      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={view}
          onChange={(_, v) => {
            const next = new URLSearchParams(searchParams);
            if (v === "proposals") next.set("view", "proposals");
            else next.delete("view");
            setSearchParams(next, { replace: true });
          }}
        >
          <Tab value="catalogue" label="Catalogue" sx={{ textTransform: "none" }} />
          <Tab value="proposals" label="Proposals" sx={{ textTransform: "none" }} />
        </Tabs>
      </Box>

      {view === "catalogue" ? <TemplateCatalogue /> : <ProposalList />}
    </Box>
  );
}
