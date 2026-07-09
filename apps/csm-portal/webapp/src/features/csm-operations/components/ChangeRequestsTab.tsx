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
  Chip,
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
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchChangeRequests } from "@features/csm-operations/api/useSearchChangeRequests";
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";
import ChangeRequestsFilterBar, {
  DEFAULT_CR_FILTERS,
  type ChangeRequestFilters,
} from "@features/csm-operations/components/ChangeRequestsFilterBar";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "short",
      day: "numeric",
    }) ?? "—"
  );
}

/** Convert a YYYY-MM-DD date picker value to an ISO 8601 string at midnight UTC. */
function toISOStart(date: string): string {
  return `${date}T00:00:00Z`;
}

/** Convert a YYYY-MM-DD date picker value to an ISO 8601 string at end-of-day UTC. */
function toISOEnd(date: string): string {
  return `${date}T23:59:59Z`;
}

/**
 * Change-requests listing for the Operations → Change requests tab. Searches
 * `POST /change-requests/search` with server-side pagination and filters
 * (state, impact, closed date range, free-text search).
 */
export default function ChangeRequestsTab(): JSX.Element {
  const navigate = useNavTransition();
  const [filters, setFilters] = useState<ChangeRequestFilters>(DEFAULT_CR_FILTERS);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: {
        ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
        ...(filters.states.length > 0 && { states: filters.states }),
        ...(filters.impacts.length > 0 && { impacts: filters.impacts }),
        ...(filters.closedStartDate && {
          closedStartDate: toISOStart(filters.closedStartDate),
        }),
        ...(filters.closedEndDate && {
          closedEndDate: toISOEnd(filters.closedEndDate),
        }),
      },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [debouncedSearch, filters.states, filters.impacts, filters.closedStartDate, filters.closedEndDate, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching } =
    useSearchChangeRequests(payload);

  const changeRequests = data?.changeRequests ?? [];
  const total = data?.total ?? 0;

  const handleFiltersChange = (next: ChangeRequestFilters): void => {
    setFilters(next);
    setPage(0);
  };

  const handleReset = (): void => {
    setFilters(DEFAULT_CR_FILTERS);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <ChangeRequestsFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((prev: boolean) => !prev)}
      />

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>Project</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Impact</TableCell>
                <TableCell>Planned start</TableCell>
                <TableCell>Updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="85%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={72} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={60} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <QueryErrorState
                      message={`Failed to load change requests: ${error instanceof Error ? error.message : "unknown error"}`}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : changeRequests.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No change requests found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                changeRequests.map((cr) => (
                  <TableRow
                    key={cr.id}
                    hover
                    onClick={() => navigate(`/operations/change-requests/${cr.id}`)}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>{cr.number || "—"}</TableCell>
                    <TableCell sx={{ maxWidth: 360 }}>
                      <Typography variant="body2" noWrap title={cr.subject ?? undefined}>
                        {cr.subject || "—"}
                      </Typography>
                    </TableCell>
                    <TableCell>{cr.project?.name || "—"}</TableCell>
                    <TableCell>
                      {cr.state ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={changeRequestStateColor(cr.state)}
                          label={changeRequestStateLabel(cr.state)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {cr.impact ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={changeRequestImpactColor(cr.impact)}
                          label={changeRequestImpactLabel(cr.impact)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>{formatDate(cr.plannedStartOn)}</TableCell>
                    <TableCell>{formatDate(cr.updatedOn)}</TableCell>
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
          onPageChange={(_: unknown, newPage: number) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
