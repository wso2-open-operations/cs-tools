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
  IconButton,
  InputAdornment,
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
import { Plus, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useLocation } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useAlertMuteRuleCounts, useSearchAlertMuteRules } from "@features/csm-operations/api/useAlertMuteRules";
import type { AlertMuteRuleSearchPayload, AlertMuteRuleStateFilter } from "@features/csm-operations/types/alertMuteRules";
import {
  ALERT_MUTE_RULE_STATE_FILTERS,
  alertMuteRuleState,
  conditionsSummary,
  formatDuration,
  useNow,
} from "@features/csm-operations/utils/alertMuteRules";
import AlertMuteRuleStateChip from "@features/csm-operations/components/AlertMuteRuleStateChip";
import AlertMuteRuleActions from "@features/csm-operations/components/AlertMuteRuleActions";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];
const COLUMN_COUNT = 8;

function formatDateTime(value: string): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "short" }) ?? "—";
}

/**
 * Operations → Alert mute rules: windows during which matching SRE alerts are
 * muted. Laid out like CloudWatch's alarm list: state filter with counts, a
 * search box, and one row per rule with its actions in a ⋮ menu. Searching,
 * filtering and paging run on the backend (`POST /alert-mute-rules/search`).
 */
export default function AlertMuteRulesTab(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const now = useNow();
  const [stateFilter, setStateFilter] = useState<AlertMuteRuleStateFilter>("all");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const searchQuery = useDebouncedValue(search.trim(), 300);
  const payload = useMemo<AlertMuteRuleSearchPayload>(
    () => ({
      filters: {
        ...(stateFilter !== "all" && { states: [stateFilter] }),
        ...(searchQuery && { searchQuery }),
      },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [stateFilter, searchQuery, page, rowsPerPage],
  );
  const { data, isLoading, isError, error, isFetching, refetch, dataUpdatedAt } = useSearchAlertMuteRules(payload);
  const counts = useAlertMuteRuleCounts(searchQuery);
  const returnTo = `${location.pathname}${location.search}`;
  const pageRows = (data?.alertMuteRules ?? []).map((rule) => ({ rule, state: alertMuteRuleState(rule, now) }));
  const total = data?.total ?? 0;

  const openRule = (id: string): void => navigate(`/operations/alert-mute-rules/${id}`, { state: { from: returnTo } });

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <ToggleButtonGroup
          exclusive
          size="small"
          color="primary"
          value={stateFilter}
          aria-label="Filter by state"
          onChange={(_, next: AlertMuteRuleStateFilter | null) => {
            if (next) {
              setStateFilter(next);
              setPage(0);
            }
          }}
        >
          {ALERT_MUTE_RULE_STATE_FILTERS.map((f) => (
            <ToggleButton key={f.value} value={f.value} sx={{ textTransform: "none", px: 1.5 }}>
              {counts ? `${f.label} (${counts[f.value]})` : f.label}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
        <TextField
          size="small"
          placeholder="Search by name, number, condition value or description…"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
          sx={{ flex: 1, minWidth: 240 }}
          slotProps={{
            htmlInput: { "aria-label": "Search alert mute rules" },
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
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh alert mute rules"
        />
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/operations/alert-mute-rules/new", { state: { from: returnTo } })}
        >
          Create alert mute rule
        </Button>
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Name</TableCell>
                <TableCell>Conditions</TableCell>
                <TableCell>Window</TableCell>
                <TableCell>Duration</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Muted</TableCell>
                <TableCell>Created by</TableCell>
                <TableCell align="right">
                  <Box component="span" sx={{ position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0 0 0 0)" }}>
                    Actions
                  </Box>
                </TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: COLUMN_COUNT }).map((__, j) => (
                      <TableCell key={j}>
                        <Skeleton variant="rounded" width="80%" height={18} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={COLUMN_COUNT} align="center">
                    <QueryErrorState message={error?.message || "Failed to load alert mute rules."} error={error} />
                  </TableCell>
                </TableRow>
              ) : pageRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      {stateFilter === "all" && !searchQuery
                        ? "No alert mute rules yet. Create one to mute a service's alerts during planned work."
                        : "No alert mute rules match these filters."}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                pageRows.map(({ rule, state }) => (
                  <TableRow
                    key={rule.id}
                    hover
                    tabIndex={0}
                    aria-label={`View alert mute rule ${rule.name}`}
                    onClick={() => openRule(rule.id)}
                    onKeyDown={(e) => {
                      if (e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) {
                        e.preventDefault();
                        openRule(rule.id);
                      }
                    }}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell sx={{ maxWidth: 260 }}>
                      <Typography variant="body2" noWrap title={rule.name}>
                        {rule.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {rule.number}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ maxWidth: 320 }}>
                      <Typography variant="body2" noWrap title={conditionsSummary(rule.conditions)}>
                        {conditionsSummary(rule.conditions)}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      <Typography variant="body2">{formatDateTime(rule.startOn)}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {`→ ${formatDateTime(rule.endedOn ?? rule.endOn)}`}
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {formatDuration(Date.parse(rule.endOn) - Date.parse(rule.startOn))}
                    </TableCell>
                    <TableCell>
                      <AlertMuteRuleStateChip state={state} />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary" title="Shown once alert-core uses these rules">
                        —
                      </Typography>
                    </TableCell>
                    <TableCell sx={{ maxWidth: 200 }}>
                      <Typography variant="body2" noWrap title={rule.createdBy}>
                        {rule.createdBy}
                      </Typography>
                    </TableCell>
                    <TableCell align="right">
                      <AlertMuteRuleActions rule={rule} state={state} variant="menu" returnTo={returnTo} />
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
    </Box>
  );
}
