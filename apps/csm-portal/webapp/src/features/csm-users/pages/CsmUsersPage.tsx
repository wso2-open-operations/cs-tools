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
  Checkbox,
  FormControl,
  IconButton,
  InputAdornment,
  InputLabel,
  ListItemText,
  MenuItem,
  OutlinedInput,
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
  TextField,
  Typography,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent } from "react";
import { useLocation, useSearchParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import UserRefLink from "@components/UserRefLink";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchGroups } from "@api/useSearchGroups";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { useSearchRoles } from "@features/csm-admin/api/useSearchRoles";
import { useSearchTeams } from "@features/csm-admin/api/useSearchTeams";
import ResponsiveRoleChips from "@components/ResponsiveRoleChips";
import RefreshButton from "@components/RefreshButton";
import type { SearchUsersRequest } from "@features/csm-users/types/csmUsers";
import {
  readUsersFiltersFromUrl,
  writeUsersFiltersToUrl,
  type UsersFilters,
} from "@features/csm-users/utils/usersFiltersUrl";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import type { BeGroup } from "@api/backend/types";
import { displayUserTimezone } from "@utils/userDirectoryDisplay";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];
/**
 * The users list, with filters reflected in the URL (`search`, `roles`,
 * `groups`, `teams`, `active`) so a filtered link is shareable and survives a
 * reload — the same `read*FiltersFromUrl` / `write*FiltersToUrl` convention
 * the cases list uses (`casesFiltersUrl.ts`). The free-text key is `search`,
 * not `q`: both this list and the cases list originally wrote it as `?q=`,
 * which collides with the app's QuickNav command palette (it treats `?q=` as
 * a one-shot deep link and pops open pre-filled with whatever's there) — keep
 * it `search` in any future change here, or that collision comes back.
 * Deliberately no project/account filter: "who is on this project" is
 * answered by the project-contacts search instead. Role, group and team
 * filters combine (AND together server-side).
 */
export default function CsmUsersPage(): JSX.Element {
  const navigate = useNavTransition();
  const location = useLocation();
  // Set by a dashboard widget's click-through, since this page has no
  // dashboard context of its own. No Back button of its own reads this
  // directly any more -- `CsmAdminLayout` (the parent route shell) renders
  // the single Back button for every page it wraps and reads this exact
  // same `location.state`, since a layout component sees the same route
  // match's state as its child. Still needed here to forward `parentState`
  // onward below, so a dashboard → here → profile → Back → Back chain
  // restores correctly instead of landing back on the tile grid partway
  // through.
  const backState = location.state as { from?: string } | undefined;
  const [searchParams, setSearchParams] = useSearchParams();
  const filters = useMemo(() => readUsersFiltersFromUrl(searchParams), [searchParams]);

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const setFilters = (next: UsersFilters): void => {
    setPage(0);
    setSearchParams(writeUsersFiltersToUrl(next), { replace: true });
  };

  const debouncedSearch = useDebouncedValue(filters.search, 300);

  // Role/team catalogues are small and curated, so one full-catalogue page is
  // enough to populate the picker (unlike groups, a live, potentially large
  // query against the backing data source — see the async group picker
  // below).
  const { data: rolesData } = useSearchRoles({ pagination: { limit: BE_MAX_PAGE_LIMIT } });
  const { data: teamsData } = useSearchTeams({ pagination: { limit: BE_MAX_PAGE_LIMIT } });
  const roles = useMemo(() => rolesData?.roles ?? [], [rolesData]);
  const teams = useMemo(() => teamsData?.teams ?? [], [teamsData]);
  const roleNameById = useMemo(
    () => new Map(roles.map((r) => [r.id, r.name])),
    [roles],
  );
  const teamNameById = useMemo(
    () => new Map(teams.map((t) => [t.id, t.name])),
    [teams],
  );

  const request = useMemo<SearchUsersRequest>(
    () => ({
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      filters: {
        ...(debouncedSearch.trim() && { searchQuery: debouncedSearch.trim() }),
        ...(filters.roleIds.length > 0 && { roleIds: filters.roleIds }),
        ...(filters.groupIds.length > 0 && { groupIds: filters.groupIds }),
        ...(filters.teamIds.length > 0 && { teamIds: filters.teamIds }),
        ...(filters.active !== "all" && { active: filters.active === "active" }),
      },
      sortBy: { field: "name", order: "asc" },
    }),
    [debouncedSearch, page, rowsPerPage, filters],
  );

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchUsers(request);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    setFilters({ ...filters, search: e.target.value });
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const handleRoleChange = (e: SelectChangeEvent<string[]>) => {
    const value = e.target.value;
    setFilters({ ...filters, roleIds: typeof value === "string" ? value.split(",") : value });
  };

  const handleTeamChange = (e: SelectChangeEvent<string[]>) => {
    const value = e.target.value;
    setFilters({ ...filters, teamIds: typeof value === "string" ? value.split(",") : value });
  };

  const handleActiveChange = (e: SelectChangeEvent) => {
    setFilters({ ...filters, active: e.target.value as UsersFilters["active"] });
  };

  const users = data?.users ?? [];
  const total = data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Search across username and email (case-insensitive). Filter by role, group, team and
          status.
        </Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh users"
        />
      </Box>

      <Stack direction={{ xs: "column", sm: "row" }} spacing={2} sx={{ flexWrap: "wrap" }}>
        <TextField
          size="small"
          label="Search users"
          placeholder="Search users by username or email"
          value={filters.search}
          onChange={handleSearchChange}
          slotProps={{ htmlInput: { "aria-label": "Search users by username or email" } }}
          sx={{ minWidth: 280, flex: 1 }}
        />

        <FormControl size="small" sx={{ width: 200, flexShrink: 0 }}>
          <InputLabel
            id="user-roles-label"
            shrink={filters.roleIds.length > 0}
            sx={{ top: "0px !important" }}
          >
            Roles
          </InputLabel>
          <Select
            labelId="user-roles-label"
            multiple
            notched={filters.roleIds.length > 0}
            value={filters.roleIds}
            onChange={handleRoleChange}
            input={
              <OutlinedInput
                label="Roles"
                endAdornment={
                  filters.roleIds.length > 0 ? (
                    <InputAdornment position="end" sx={{ mr: 1.5 }}>
                      <IconButton
                        size="small"
                        aria-label="Clear roles filter"
                        onMouseDown={(e) => e.stopPropagation()}
                        onClick={(e) => {
                          e.stopPropagation();
                          setFilters({ ...filters, roleIds: [] });
                        }}
                      >
                        <X size={14} />
                      </IconButton>
                    </InputAdornment>
                  ) : undefined
                }
              />
            }
            MenuProps={{
              anchorOrigin: { vertical: "bottom", horizontal: "left" },
              transformOrigin: { vertical: "top", horizontal: "left" },
              slotProps: { paper: { sx: { maxHeight: 320 } } },
            }}
            renderValue={(selected) => {
              const label = (selected as string[])
                .map((id) => roleNameById.get(id) ?? id)
                .join(", ");
              return <Box component="span" title={label}>{label}</Box>;
            }}
            sx={{
              "& .MuiSelect-select": {
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              },
            }}
          >
            {roles.map((role) => (
              <MenuItem key={role.id} value={role.id}>
                <Checkbox checked={filters.roleIds.includes(role.id)} />
                <ListItemText primary={role.name} />
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <Box sx={{ minWidth: 240, flex: 1 }}>
          <AsyncEntityMultiSelect<BeGroup>
            id="user-groups-filter"
            label="Groups"
            placeholder="Search groups…"
            values={filters.groupIds}
            onChange={(next) => setFilters({ ...filters, groupIds: next })}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
            compactSelectedValues
          />
        </Box>

        <FormControl size="small" sx={{ width: 200, flexShrink: 0 }}>
          <InputLabel
            id="user-teams-label"
            shrink={filters.teamIds.length > 0}
            sx={{ top: "0px !important" }}
          >
            Teams
          </InputLabel>
          <Select
            labelId="user-teams-label"
            multiple
            notched={filters.teamIds.length > 0}
            value={filters.teamIds}
            onChange={handleTeamChange}
            input={
              <OutlinedInput
                label="Teams"
                endAdornment={
                  filters.teamIds.length > 0 ? (
                    <InputAdornment position="end" sx={{ mr: 1.5 }}>
                      <IconButton
                        size="small"
                        aria-label="Clear teams filter"
                        onMouseDown={(e) => e.stopPropagation()}
                        onClick={(e) => {
                          e.stopPropagation();
                          setFilters({ ...filters, teamIds: [] });
                        }}
                      >
                        <X size={14} />
                      </IconButton>
                    </InputAdornment>
                  ) : undefined
                }
              />
            }
            MenuProps={{
              anchorOrigin: { vertical: "bottom", horizontal: "left" },
              transformOrigin: { vertical: "top", horizontal: "left" },
              slotProps: { paper: { sx: { maxHeight: 320 } } },
            }}
            renderValue={(selected) => {
              const label = (selected as string[])
                .map((id) => teamNameById.get(id) ?? id)
                .join(", ");
              return <Box component="span" title={label}>{label}</Box>;
            }}
            sx={{
              "& .MuiSelect-select": {
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              },
            }}
          >
            {teams.map((team) => (
              <MenuItem key={team.id} value={team.id}>
                <Checkbox checked={filters.teamIds.includes(team.id)} />
                <ListItemText primary={team.name} secondary={team.family} />
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl size="small" sx={{ minWidth: 140 }}>
          <InputLabel
            id="user-active-label"
            shrink={filters.active !== "all"}
            sx={{ top: "0px !important" }}
          >
            Status
          </InputLabel>
          <Select
            labelId="user-active-label"
            notched={filters.active !== "all"}
            value={filters.active}
            onChange={handleActiveChange}
            input={<OutlinedInput label="Status" />}
          >
            <MenuItem value="all">All</MenuItem>
            <MenuItem value="active">Active</MenuItem>
            <MenuItem value="inactive">Inactive</MenuItem>
          </Select>
        </FormControl>
      </Stack>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer sx={{ overflow: "visible" }}>
          <Table
            size="small"
            aria-label="Users search results"
            sx={{
              width: "100%",
              tableLayout: "fixed",
              "& .MuiTableCell-root": { borderColor: "divider" },
            }}
          >
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell sx={{ width: "32%" }}>User</TableCell>
                <TableCell sx={{ width: "44%" }}>Roles</TableCell>
                <TableCell sx={{ width: "12%" }}>Status</TableCell>
                <TableCell sx={{ width: "12%" }}>Timezone</TableCell>
              </TableRow>
            </TableHead>
            <TableBody sx={isFetching && !isLoading ? { opacity: 0.6 } : undefined}>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell>
                      <Skeleton variant="rounded" width="65%" height={18} />
                      <Skeleton variant="rounded" width="85%" height={14} sx={{ mt: 0.75 }} />
                    </TableCell>
                    <TableCell><Skeleton variant="rounded" width={64} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={64} height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="55%" height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={4} align="center">
                    <QueryErrorState
                      message={error instanceof Error && error.message.trim() ? error.message : "Failed to load users."}
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No users found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                users.map((u) => {
                  const profilePath = `/people/${encodeURIComponent(u.id)}`;
                  // `parentState` nests this page's OWN inherited state
                  // (e.g. `{ from: "/dashboard" }`, when this page was
                  // itself reached from a dashboard widget) so the profile
                  // page can restore it on its own way back — otherwise a
                  // dashboard → here → profile → Back round trip would land
                  // back on this page with no `from`, silently dropping its
                  // own Back button.
                  const goToProfile = (): void =>
                    navigate(profilePath, {
                      state: {
                        from: `${location.pathname}${location.search}`,
                        parentState: backState ?? null,
                      },
                    });
                  const handleRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>): void => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      goToProfile();
                    }
                  };
                  const displayName = u.name?.trim();
                  const primaryIdentity = displayName || u.userName;
                  const secondaryIdentity = [
                    u.userName !== primaryIdentity ? u.userName : undefined,
                    u.email !== primaryIdentity && u.email !== u.userName ? u.email : undefined,
                  ].filter((value): value is string => Boolean(value));

                  return (
                    <TableRow
                      key={u.id}
                      hover
                      onClick={goToProfile}
                      onKeyDown={handleRowKeyDown}
                      tabIndex={0}
                      aria-label={`View profile for ${u.name || u.userName}`}
                      sx={{
                        cursor: "pointer",
                        "&:focus-visible": {
                          outline: "2px solid",
                          outlineColor: "primary.main",
                          outlineOffset: -2,
                        },
                      }}
                    >
                      <TableCell sx={{ minWidth: 0, maxWidth: 320 }}>
                        <UserRefLink
                          name={primaryIdentity}
                          email={u.email}
                          userId={u.id}
                          underlineOnHover={false}
                        />
                        {secondaryIdentity.length > 0 && (
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            noWrap
                            title={secondaryIdentity.join(" · ")}
                            sx={{ display: "block" }}
                          >
                            {secondaryIdentity.join(" · ")}
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        {u.roles && u.roles.length > 0 ? (
                          <ResponsiveRoleChips
                            roleIds={u.roles}
                            roleNameById={roleNameById}
                            userLabel={u.name || u.userName}
                            onViewAll={goToProfile}
                          />
                        ) : (
                          "—"
                        )}
                      </TableCell>
                      <TableCell>
                        {u.lockedOut ? (
                          <Stack direction="row" spacing={0.75} alignItems="center">
                            <Box
                              aria-hidden
                              sx={{
                                width: 7,
                                height: 7,
                                flexShrink: 0,
                                borderRadius: "50%",
                                bgcolor: "error.main",
                              }}
                            />
                            <Typography variant="body2">Locked out</Typography>
                          </Stack>
                        ) : u.active === undefined ? (
                          "—"
                        ) : (
                          <Stack direction="row" spacing={0.75} alignItems="center">
                            <Box
                              aria-hidden
                              sx={{
                                width: 7,
                                height: 7,
                                flexShrink: 0,
                                borderRadius: "50%",
                                bgcolor: u.active ? "success.main" : "text.disabled",
                              }}
                            />
                            <Typography variant="body2">
                              {u.active ? "Active" : "Inactive"}
                            </Typography>
                          </Stack>
                        )}
                      </TableCell>
                      <TableCell>{displayUserTimezone(u.timezone)}</TableCell>
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
