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
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent } from "react";
import { useLocation } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchProjects } from "@features/csm-projects/api/useSearchProjects";
import ClosureStateChip from "@features/csm-projects/components/ClosureStateChip";
import RefreshButton from "@components/RefreshButton";
import type { SearchProjectsRequest } from "@features/csm-projects/types/csmProjects";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleDateString("en-US", {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
}

export default function CsmProjectsPage(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  // Set by a dashboard widget's click-through, since this page has no
  // dashboard context of its own.
  const backState = location.state as { from?: string } | undefined;
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const request = useMemo<SearchProjectsRequest>(
    () => ({
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      searchQuery: debouncedSearch.trim() || undefined,
    }),
    [debouncedSearch, page, rowsPerPage],
  );

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchProjects(request);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const projects = data?.projects ?? [];
  const total = data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {backState?.from && (
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate(backState.from as string)}
          sx={{ alignSelf: "flex-start" }}
        >
          Back
        </Button>
      )}

      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Search across project name, project key, and subscription type.
        </Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh projects"
        />
      </Box>

      <TextField
        size="small"
        label="Search projects"
        placeholder="Search projects by name, key, or subscription"
        value={searchInput}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search projects by name, key, or subscription type" } }}
        sx={{ maxWidth: 480 }}
      />

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Name</TableCell>
                <TableCell>Project key</TableCell>
                <TableCell>State</TableCell>
                <TableCell>Start</TableCell>
                <TableCell>End</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="55%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={80} height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={5} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load projects."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : projects.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No projects found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                projects.map((p) => {
                  const goToProject = (): void =>
                    navigate(`/customers/projects/${p.id}`, {
                      state: { from: `${location.pathname}${location.search}` },
                    });
                  const handleRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>): void => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      goToProject();
                    }
                  };
                  return (
                    <TableRow
                      key={p.id}
                      hover
                      onClick={goToProject}
                      onKeyDown={handleRowKeyDown}
                      tabIndex={0}
                      aria-label={`View project ${p.name}`}
                      sx={{
                        cursor: "pointer",
                        "&:focus-visible": {
                          outline: "2px solid",
                          outlineColor: "primary.main",
                          outlineOffset: -2,
                        },
                      }}
                    >
                      <TableCell>
                        <Typography variant="body2" noWrap>
                          {p.name}
                        </Typography>
                      </TableCell>
                      <TableCell>{p.key}</TableCell>
                      <TableCell>
                        <ClosureStateChip
                          closureState={p.closureState}
                          emptyFallback="—"
                        />
                      </TableCell>
                      <TableCell>{formatDate(p.startDate)}</TableCell>
                      <TableCell>{formatDate(p.endDate)}</TableCell>
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
          onPageChange={(_, newPage) => setPage(newPage)}
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
