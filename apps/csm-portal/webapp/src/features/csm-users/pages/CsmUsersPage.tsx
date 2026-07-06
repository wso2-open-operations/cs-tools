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
  Checkbox,
  Chip,
  FormControl,
  InputLabel,
  ListItemText,
  MenuItem,
  OutlinedInput,
  Paper,
  Select,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TableSortLabel,
  TextField,
  Typography,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import {
  INTERNAL_USER_ROLES,
  type SearchUsersRequest,
  type SnUserRole,
  type UserSortField,
  type UserSortOrder,
} from "@features/csm-users/types/csmUsers";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

const ALL_ROLES: SnUserRole[] = [
  ...INTERNAL_USER_ROLES,
  "commenter",
  "external",
  "customer",
  "customer_admin",
  "partner",
  "partner_admin",
];

type ActiveFilter = "all" | "active" | "inactive";

export default function CsmUsersPage(): JSX.Element {
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [roleFilter, setRoleFilter] = useState<SnUserRole[]>([]);
  const [activeFilter, setActiveFilter] = useState<ActiveFilter>("all");
  const [sortField, setSortField] = useState<UserSortField>("name");
  const [sortOrder, setSortOrder] = useState<UserSortOrder>("asc");

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const request = useMemo<SearchUsersRequest>(
    () => ({
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      filters: {
        ...(debouncedSearch.trim() && { searchQuery: debouncedSearch.trim() }),
        ...(roleFilter.length > 0 && { roles: roleFilter }),
        ...(activeFilter !== "all" && { active: activeFilter === "active" }),
      },
      sortBy: { field: sortField, order: sortOrder },
    }),
    [debouncedSearch, page, rowsPerPage, roleFilter, activeFilter, sortField, sortOrder],
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

  const handleRoleChange = (e: SelectChangeEvent<SnUserRole[]>) => {
    const value = e.target.value;
    setRoleFilter(typeof value === "string" ? (value.split(",") as SnUserRole[]) : value);
    setPage(0);
  };

  const handleActiveChange = (e: SelectChangeEvent) => {
    setActiveFilter(e.target.value as ActiveFilter);
    setPage(0);
  };

  const handleSort = (field: UserSortField) => {
    if (sortField === field) {
      setSortOrder((o) => (o === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortOrder("asc");
    }
    setPage(0);
  };

  const users = data?.users ?? [];
  const total = data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="body2" color="text.secondary">
        Search across username and email (case-insensitive). Filter by role and status.
      </Typography>

      <Stack direction={{ xs: "column", sm: "row" }} spacing={2} sx={{ flexWrap: "wrap" }}>
        <TextField
          size="small"
          label="Search users"
          placeholder="Search users by username or email"
          value={searchInput}
          onChange={handleSearchChange}
          slotProps={{ htmlInput: { "aria-label": "Search users by username or email" } }}
          sx={{ minWidth: 280, flex: 1 }}
        />

        <FormControl size="small" sx={{ minWidth: 200 }}>
          <InputLabel id="user-roles-label">Roles</InputLabel>
          <Select
            labelId="user-roles-label"
            multiple
            value={roleFilter}
            onChange={handleRoleChange}
            input={<OutlinedInput label="Roles" />}
            renderValue={(selected) => (selected as SnUserRole[]).join(", ")}
          >
            {ALL_ROLES.map((role) => (
              <MenuItem key={role} value={role}>
                <Checkbox checked={roleFilter.includes(role)} />
                <ListItemText primary={role} />
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 140 }}>
          <InputLabel id="user-active-label">Status</InputLabel>
          <Select
            labelId="user-active-label"
            value={activeFilter}
            onChange={handleActiveChange}
            input={<OutlinedInput label="Status" />}
          >
            <MenuItem value="all">All</MenuItem>
            <MenuItem value="active">Active</MenuItem>
            <MenuItem value="inactive">Inactive</MenuItem>
          </Select>
        </FormControl>
      </Stack>

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
                <TableCell sortDirection={sortField === "name" ? sortOrder : false}>
                  <TableSortLabel
                    active={sortField === "name"}
                    direction={sortField === "name" ? sortOrder : "asc"}
                    onClick={() => handleSort("name")}
                  >
                    Name
                  </TableSortLabel>
                </TableCell>
                <TableCell>Email</TableCell>
                <TableCell>Roles</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Timezone</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                [0, 1, 2, 3, 4, 5].map((i) => (
                  <TableRow key={i}>
                    {[0, 1, 2, 3, 4, 5].map((c) => (
                      <TableCell key={c}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No users found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                users.map((u) => (
                  <TableRow key={u.id} hover>
                    <TableCell>{u.userName}</TableCell>
                    <TableCell>{u.name || "—"}</TableCell>
                    <TableCell>{u.email}</TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5} sx={{ flexWrap: "wrap", gap: 0.5 }}>
                        {u.roles && u.roles.length > 0
                          ? u.roles.map((r) => (
                              <Chip
                                key={r}
                                size="small"
                                label={r}
                                color={(INTERNAL_USER_ROLES as string[]).includes(r) ? "primary" : "default"}
                                variant="outlined"
                              />
                            ))
                          : u.userType
                            ? <Chip
                                size="small"
                                label={u.userType}
                                color={u.userType === "internal" ? "primary" : "default"}
                                variant="outlined"
                              />
                            : "—"}
                      </Stack>
                    </TableCell>
                    <TableCell>
                      {u.active === undefined ? (
                        "—"
                      ) : (
                        <Chip
                          size="small"
                          label={u.active ? "Active" : "Inactive"}
                          color={u.active ? "success" : "default"}
                          variant="outlined"
                        />
                      )}
                    </TableCell>
                    <TableCell>{u.timezone ?? "—"}</TableCell>
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
      </Paper>

      {isFetching && !isLoading && (
        <Typography variant="caption" color="text.secondary">
          Updating…
        </Typography>
      )}
    </Box>
  );
}
