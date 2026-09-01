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
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Megaphone, Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useLocation } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchOutages } from "@features/csm-operations/api/useOutages";
import {
  DEFAULT_OUTAGE_FILTERS,
  outageStatusColor,
  outageStatusLabel,
  outageTypeColor,
  outageTypeLabel,
  type OutageFilters,
} from "@features/csm-operations/utils/outages";
import OutagesFilterBar from "@features/csm-operations/components/OutagesFilterBar";
import RefreshButton from "@components/RefreshButton";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

/**
 * Outages listing for the Operations → Outages tab. Searches
 * `POST /outages/search` with server-side pagination, free-text search, and
 * Type/Status/Published filters. `beginFromDefaulted` (an implicit six-month
 * lookback the backend applies when no explicit `beginFrom` is sent — the
 * table is otherwise dominated by a historical bulk load) is surfaced as an
 * inline notice rather than silently narrowing what the engineer sees.
 */
export default function OutagesTab(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  const [filters, setFilters] = useState<OutageFilters>(DEFAULT_OUTAGE_FILTERS);
  const [isFiltersOpen, setIsFiltersOpen] = useState(true);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: {
        ...(debouncedSearch.length > 0 && { searchTerm: debouncedSearch }),
        ...(filters.types.length > 0 && { types: filters.types }),
        ...(filters.statuses.length > 0 && { statuses: filters.statuses }),
        ...(filters.publishedOnly && { publishedOnly: true }),
      },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [debouncedSearch, filters.types, filters.statuses, filters.publishedOnly, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching, refetch, dataUpdatedAt } =
    useSearchOutages(payload);

  const outages = data?.outages ?? [];
  const total = data?.total ?? 0;

  const handleFiltersChange = (next: OutageFilters): void => {
    setFilters(next);
    setPage(0);
  };

  const handleReset = (): void => {
    setFilters(DEFAULT_OUTAGE_FILTERS);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", alignItems: "center", gap: 1 }}>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh outages"
        />
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/operations/outages/new")}
        >
          Create outage
        </Button>
      </Box>

      <OutagesFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((prev) => !prev)}
      />

      {data?.beginFromDefaulted && (
        <Typography variant="caption" color="text.secondary">
          Showing outages beginning on or after{" "}
          {formatDateTime(data.appliedBeginFrom)} — no start date was given, so a
          six-month lookback was applied automatically.
        </Typography>
      )}

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Number</TableCell>
                <TableCell>Short description</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Begin</TableCell>
                <TableCell>End</TableCell>
                <TableCell align="center">Public</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={90} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={90} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={24} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load outages."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : outages.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No outages found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                outages.map((outage) => {
                  const detailPath = `/operations/outages/${outage.id}`;
                  const detailState = { from: `${location.pathname}${location.search}` };
                  return (
                    <TableRow
                      key={outage.id}
                      hover
                      onClick={() => navigate(detailPath, { state: detailState })}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          navigate(detailPath, { state: detailState });
                        }
                      }}
                      tabIndex={0}
                      aria-label={`View outage ${outage.number || outage.id}`}
                      sx={{ cursor: "pointer" }}
                    >
                      <TableCell>{outage.number || "—"}</TableCell>
                      <TableCell sx={{ maxWidth: 480 }}>
                        <Typography variant="body2" noWrap title={outage.shortDescription ?? undefined}>
                          {outage.shortDescription || "—"}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={outageTypeColor(outage.type)}
                          label={outageTypeLabel(outage.type)}
                        />
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          color={outageStatusColor(outage.status)}
                          label={outageStatusLabel(outage.status)}
                        />
                      </TableCell>
                      <TableCell>{formatDateTime(outage.begin)}</TableCell>
                      <TableCell>{outage.end ? formatDateTime(outage.end) : "—"}</TableCell>
                      <TableCell align="center">
                        {outage.publishesToStatusPage && (
                          <Tooltip
                            title={`Publishes to the public status page${outage.statusPageCloud ? ` (${outage.statusPageCloud})` : ""}`}
                          >
                            <Box sx={{ display: "inline-flex", color: "warning.main" }}>
                              <Megaphone size={16} />
                            </Box>
                          </Tooltip>
                        )}
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
          count={total}
          page={page}
          onPageChange={(_: unknown, newPage: number) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          labelDisplayedRows={({ from, to, count }) => `Showing ${from}–${to} of ${count}`}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
