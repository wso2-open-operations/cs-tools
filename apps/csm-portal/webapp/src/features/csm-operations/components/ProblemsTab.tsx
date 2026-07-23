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
  LinearProgress,
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
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchProblems } from "@features/csm-operations/api/useSearchProblems";
import {
  DEFAULT_PROBLEM_FILTERS,
  problemStateColor,
  problemStateLabel,
  type ProblemFilters,
} from "@features/csm-operations/utils/problems";
import ProblemsFilterBar from "@features/csm-operations/components/ProblemsFilterBar";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

/**
 * Problems listing for the Operations → Problem management tab. Searches
 * `POST /problems/search` with server-side pagination, free-text search, and
 * a state filter.
 */
export default function ProblemsTab(): JSX.Element {
  const navigate = useNavTransition();
  const [filters, setFilters] = useState<ProblemFilters>(DEFAULT_PROBLEM_FILTERS);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(filters.search.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: {
        ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
        ...(filters.states.length > 0 && { states: filters.states }),
      },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [debouncedSearch, filters.states, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching } = useSearchProblems(payload);

  const problems = data?.problems ?? [];
  const total = data?.total ?? 0;

  const handleFiltersChange = (next: ProblemFilters): void => {
    setFilters(next);
    setPage(0);
  };

  const handleReset = (): void => {
    setFilters(DEFAULT_PROBLEM_FILTERS);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <Button
          variant="contained"
          color="primary"
          size="small"
          startIcon={<Plus size={16} />}
          onClick={() => navigate("/operations/problems/new")}
        >
          Create problem
        </Button>
      </Box>

      <ProblemsFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={handleReset}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((prev) => !prev)}
      />

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        {/* A background refetch (pagination/filter change) shows this thin
            bar instead of swapping to skeleton rows — isLoading alone gates
            the skeleton, so keepPreviousData's already-loaded rows stay
            visible while the next page/filter loads. */}
        <Box sx={{ height: 2 }}>{isFetching && !isLoading && <LinearProgress sx={{ height: 2 }} />}</Box>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Number</TableCell>
                <TableCell>Subject</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Assignment group</TableCell>
                <TableCell>Assigned to</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="90%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={90} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={5} align="center">
                    <QueryErrorState
                      message={`Failed to load problems: ${error instanceof Error ? error.message : "unknown error"}`}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : problems.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No problems found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                problems.map((problem) => {
                  const detailPath = `/operations/problems/${problem.id}`;
                  return (
                  <TableRow
                    key={problem.id}
                    hover
                    onClick={() => navigate(detailPath)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        navigate(detailPath);
                      }
                    }}
                    tabIndex={0}
                    aria-label={`View problem ${problem.number || problem.id}`}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>{problem.number || "—"}</TableCell>
                    <TableCell sx={{ maxWidth: 480 }}>
                      <Typography variant="body2" noWrap title={problem.subject ?? undefined}>
                        {problem.subject || "—"}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      {problem.state ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={problemStateColor(problem.state)}
                          label={problemStateLabel(problem.state)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>{problem.assignmentGroup?.name || "—"}</TableCell>
                    <TableCell>{problem.assignedTo?.name || "—"}</TableCell>
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
          showFirstButton
          showLastButton
        />
      </Box>
    </Box>
  );
}
