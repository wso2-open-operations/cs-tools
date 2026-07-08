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
  Chip,
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
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { Link as RouterLink } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchProjects } from "@features/csm-projects/api/useSearchProjects";
import type { SearchProjectsRequest } from "@features/csm-projects/types/csmProjects";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleDateString();
}

function formatSubscriptionType(value: string): string {
  return value.replace(/_/g, " ");
}

export default function CsmProjectsPage(): JSX.Element {
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

  const { data, isLoading, isFetching, isError, error } = useSearchProjects(request);

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
      <Typography variant="body2" color="text.secondary">
        Search across project name, project key, and subscription type.
      </Typography>

      <TextField
        size="small"
        label="Search projects"
        placeholder="Search projects by name, key, or subscription"
        value={searchInput}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search projects by name, key, or subscription type" } }}
        sx={{ maxWidth: 480 }}
      />

      {isError && (
        <Alert severity="error">
          Failed to load projects: {error instanceof Error ? error.message : "unknown error"}
        </Alert>
      )}

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Name</TableCell>
                <TableCell>Project key</TableCell>
                <TableCell>Subscription</TableCell>
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
              ) : projects.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No projects found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                projects.map((p) => (
                  <TableRow key={p.id} hover>
                    <TableCell>
                      <Typography
                        component={RouterLink}
                        to={`/customers/projects/${p.id}`}
                        variant="body2"
                        sx={(t) => ({
                          textDecoration: "none",
                          color: t.palette.primary.dark,
                          ...t.applyStyles("dark", { color: t.palette.primary.main }),
                          "&:hover": { textDecoration: "underline" },
                        })}
                      >
                        {p.name}
                      </Typography>
                    </TableCell>
                    <TableCell>{p.key}</TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        label={formatSubscriptionType(p.subscriptionType)}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell>{formatDate(p.startDate)}</TableCell>
                    <TableCell>{formatDate(p.endDate)}</TableCell>
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
