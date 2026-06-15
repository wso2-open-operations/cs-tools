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
  CircularProgress,
  Paper,
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
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type { SearchUsersRequest } from "@features/csm-users/types/csmUsers";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

export default function CsmUsersPage(): JSX.Element {
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const request = useMemo<SearchUsersRequest>(
    () => ({
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      searchQuery: debouncedSearch.trim() || undefined,
    }),
    [debouncedSearch, page, rowsPerPage],
  );

  const { data, isLoading, isFetching, isError, error } = useSearchUsers(request);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const users = data?.users ?? [];
  const total = data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="body2" color="text.secondary">
        Search across username and email (case-insensitive).
      </Typography>

      <TextField
        size="small"
        label="Search users"
        placeholder="Search users by username or email"
        value={searchInput}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search users by username or email" } }}
        sx={{ maxWidth: 480 }}
      />

      {isError && (
        <Alert severity="error">
          Failed to load users: {error instanceof Error ? error.message : "unknown error"}
        </Alert>
      )}

      <Paper variant="outlined">
        <TableContainer>
          <Table size="small" aria-label="Users search results">
            <TableHead>
              <TableRow>
                <TableCell>Username</TableCell>
                <TableCell>Name</TableCell>
                <TableCell>Email</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Timezone</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={24} />
                  </TableCell>
                </TableRow>
              ) : users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No users found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                users.map((u) => {
                  const fullName = `${u.firstName ?? ""} ${u.lastName ?? ""}`.trim() || "—";
                  return (
                    <TableRow key={u.id} hover>
                      <TableCell>{u.userName}</TableCell>
                      <TableCell>{fullName}</TableCell>
                      <TableCell>{u.email}</TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          label={u.userType}
                          color={u.userType === "internal" ? "primary" : "default"}
                          variant="outlined"
                        />
                      </TableCell>
                      <TableCell>{u.timezone ?? "—"}</TableCell>
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
      </Paper>

      {isFetching && !isLoading && (
        <Typography variant="caption" color="text.secondary">
          Updating…
        </Typography>
      )}
    </Box>
  );
}
