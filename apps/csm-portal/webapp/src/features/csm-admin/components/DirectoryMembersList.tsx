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
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type ChangeEvent, type JSX, type KeyboardEvent } from "react";
import QueryErrorState from "@components/QueryErrorState";
import UserRefLink from "@components/UserRefLink";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { useSearchRoles } from "@features/csm-admin/api/useSearchRoles";
import ResponsiveRoleChips from "@components/ResponsiveRoleChips";
import type { SearchUsersRequest } from "@features/csm-users/types/csmUsers";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useNavTransition } from "@hooks/useNavTransition";
import { displayUserTimezone } from "@utils/userDirectoryDisplay";
import RefreshButton from "@components/RefreshButton";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];
const COLUMN_COUNT = 4;

/** Which `UserSearchFilters` key membership in this entity narrows on. */
export type DirectoryMemberFilterKey = "roleIds" | "groupIds" | "teamIds";

interface DirectoryMembersListProps {
  /** `roleIds` for a role's member page, `groupIds` for a group's, `teamIds`
   * for a team's — must match the entity kind exactly: sending the wrong key
   * (e.g. `groupIds` on a team's page) would silently list the wrong
   * people. */
  filterKey: DirectoryMemberFilterKey;
  /** The role key / group id / team registry key to filter on. */
  entityId: string;
  /** Plural noun used in the empty/error copy, e.g. "role". */
  entityNoun: string;
}

/**
 * A role/group/team's member list, backed by `POST /users/search` with the
 * matching membership filter. Server-side filtered and paginated — never
 * fetch-everything-and-slice, since a membership filter can match anywhere
 * from zero to the whole user base. A filter matching nobody is a normal,
 * legitimate result (an empty state), not an error.
 */
export default function DirectoryMembersList({
  filterKey,
  entityId,
  entityNoun,
}: DirectoryMembersListProps): JSX.Element {
  const navigate = useNavTransition();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const request: SearchUsersRequest = {
    pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
    filters: { [filterKey]: [entityId] },
  };

  const {
    data,
    isLoading,
    isFetching,
    isError,
    error,
    refetch,
    dataUpdatedAt,
  } = useSearchUsers(request);
  const users = data?.users ?? [];
  const total = data?.total ?? 0;

  const { data: rolesData } = useSearchRoles({ pagination: { limit: BE_MAX_PAGE_LIMIT } });
  const roleNameById = useMemo(
    () => new Map((rolesData?.roles ?? []).map((r) => [r.id, r.name])),
    [rolesData],
  );

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label={`Refresh ${entityNoun} members`}
        />
      </Box>
      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
      <TableContainer sx={{ overflow: "visible" }}>
        <Table
          size="small"
          aria-label={`${entityNoun} members`}
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
                  {Array.from({ length: COLUMN_COUNT }).map((__, c) => (
                    <TableCell key={c}>
                      <Skeleton variant="rounded" width="70%" height={18} />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center">
                  <QueryErrorState
                    message={error instanceof Error && error.message.trim() ? error.message : "Failed to load members."}
                    error={error}
                  />
                </TableCell>
              </TableRow>
            ) : users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={COLUMN_COUNT} align="center" sx={{ py: 4 }}>
                  <Typography variant="body2" color="text.secondary">
                    No members found for this {entityNoun}.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              users.map((u) => {
                const profilePath = `/people/${encodeURIComponent(u.id)}`;
                const goToProfile = (): void => navigate(profilePath);
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
                    tabIndex={0}
                    onClick={goToProfile}
                    onKeyDown={handleRowKeyDown}
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
