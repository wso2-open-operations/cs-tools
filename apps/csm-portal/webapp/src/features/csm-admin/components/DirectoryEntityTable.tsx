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
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { type ChangeEvent, type JSX } from "react";
import { Link as RouterLink } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import RefreshButton from "@components/RefreshButton";

const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

/** One row this table can render: a role, group, or team. */
export interface DirectoryEntityRow {
  id: string;
  name: string;
  /** Team-only: the CRE/SRE family, shown alongside the name when present. */
  family?: string;
}

interface DirectoryEntityTableProps {
  /** Plural noun for headings/empty states, e.g. "roles". */
  entityNounPlural: string;
  /** Route each row links to, e.g. "/admin/roles" (the id is appended). */
  memberBasePath: string;
  rows: DirectoryEntityRow[];
  total: number;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  error: unknown;
  search: string;
  onSearchChange: (value: string) => void;
  page: number;
  onPageChange: (page: number) => void;
  rowsPerPage: number;
  onRowsPerPageChange: (rowsPerPage: number) => void;
  onRefresh: () => void;
  refreshedAt?: number;
}

/**
 * Shared list table for the Roles / Groups / Teams directory pages: a search
 * box, a server-paginated table, and a click-through link from every row to
 * its member list (`/admin/<kind>/:id`). The entity's display name travels
 * with the link as router state so the member page can show it immediately,
 * without a second catalogue lookup.
 */
export default function DirectoryEntityTable({
  entityNounPlural,
  memberBasePath,
  rows,
  total,
  isLoading,
  isFetching,
  isError,
  error,
  search,
  onSearchChange,
  page,
  onPageChange,
  rowsPerPage,
  onRowsPerPageChange,
  onRefresh,
  refreshedAt,
}: DirectoryEntityTableProps): JSX.Element {
  const entityLabel =
    entityNounPlural.charAt(0).toUpperCase() +
    entityNounPlural.slice(1, entityNounPlural.endsWith("s") ? -1 : undefined);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onSearchChange(e.target.value);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    onRowsPerPageChange(parseInt(e.target.value, 10));
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <TextField
          size="small"
          label={`Search ${entityNounPlural}`}
          value={search}
          onChange={handleSearchChange}
          sx={{ maxWidth: 360, flex: 1 }}
          slotProps={{ htmlInput: { "aria-label": `Search ${entityNounPlural} by name` } }}
        />
        <RefreshButton
          onRefresh={onRefresh}
          isFetching={isFetching}
          updatedAt={refreshedAt}
          label={`Refresh ${entityNounPlural}`}
        />
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table
            size="small"
            aria-label={`${entityNounPlural} search results`}
            sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}
          >
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>{entityLabel}</TableCell>
              </TableRow>
            </TableHead>
            <TableBody sx={isFetching && !isLoading ? { opacity: 0.6 } : undefined}>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell>
                      <Skeleton variant="rounded" width="60%" height={18} />
                    </TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell align="center">
                    <QueryErrorState
                      message={
                        error instanceof Error
                          ? error.message
                          : `Failed to load ${entityNounPlural}.`
                      }
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : rows.length === 0 ? (
                <TableRow>
                  <TableCell align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No {entityNounPlural} found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                rows.map((row) => {
                  const destination = `${memberBasePath}/${encodeURIComponent(row.id)}`;

                  return (
                    <TableRow
                      key={row.id}
                      hover
                      sx={{ position: "relative", cursor: "pointer" }}
                    >
                      <TableCell sx={{ minWidth: 0, maxWidth: 420, position: "relative" }}>
                        <Box
                          component={RouterLink}
                          to={destination}
                          state={{ name: row.name }}
                          aria-label={`View members of ${row.name}`}
                          sx={{
                            position: "absolute",
                            inset: 0,
                            color: "inherit",
                            textDecoration: "none",
                            "&:focus-visible": {
                              outline: "2px solid",
                              outlineColor: "primary.main",
                              outlineOffset: -2,
                            },
                          }}
                        />
                        <Typography
                          variant="body2"
                          noWrap
                          title={row.name}
                          sx={{ position: "relative", pointerEvents: "none" }}
                        >
                          {row.name}
                        </Typography>
                        {row.family && (
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            noWrap
                            sx={{ display: "block", position: "relative", pointerEvents: "none" }}
                          >
                            {row.family.toUpperCase()} team
                          </Typography>
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
          onPageChange={(_, newPage) => onPageChange(newPage)}
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
