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
import { Link as RouterLink } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchAccounts } from "@features/csm-accounts/api/useSearchAccounts";
import type { SearchAccountsRequest } from "@features/csm-accounts/types/csmAccounts";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
// Top option is the backend's max page limit; larger requests are rejected.
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

function formatDate(value?: string | null): string {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleDateString();
}

export default function CsmAccountsPage(): JSX.Element {
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const request = useMemo<SearchAccountsRequest>(
    () => ({
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      searchQuery: debouncedSearch.trim() || undefined,
    }),
    [debouncedSearch, page, rowsPerPage],
  );

  const { data, isLoading, isFetching, isError, error } = useSearchAccounts(request);

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const accounts = data?.accounts ?? [];
  const total = data?.total ?? 0;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box>
        <Typography variant="h5">Accounts</Typography>
        <Typography variant="body2" color="text.secondary">
          Search across account name and Salesforce ID (case-insensitive).
        </Typography>
      </Box>

      <TextField
        size="small"
        label="Search accounts"
        placeholder="Search accounts by name or SF ID"
        value={searchInput}
        onChange={handleSearchChange}
        slotProps={{ htmlInput: { "aria-label": "Search accounts by name or SF ID" } }}
        sx={{ maxWidth: 480 }}
      />

      {isError && (
        <Alert severity="error">
          Failed to load accounts: {error instanceof Error ? error.message : "unknown error"}
        </Alert>
      )}

      <Paper variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>SF ID</TableCell>
                <TableCell>Tier</TableCell>
                <TableCell>Region</TableCell>
                <TableCell>Activated</TableCell>
                <TableCell>Deactivated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={24} />
                  </TableCell>
                </TableRow>
              ) : accounts.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No accounts found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                accounts.map((a) => (
                  <TableRow key={a.id} hover>
                    <TableCell>
                      <Typography
                        component={RouterLink}
                        to={`/accounts/${a.id}`}
                        variant="body2"
                        sx={(t) => ({
                          textDecoration: "none",
                          color: t.palette.primary.dark,
                          ...t.applyStyles("dark", { color: t.palette.primary.main }),
                          "&:hover": { textDecoration: "underline" },
                        })}
                      >
                        {a.name}
                      </Typography>
                    </TableCell>
                    <TableCell>{a.sfId}</TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        label={a.tier}
                        color={a.tier === "enterprise" ? "primary" : "default"}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell>{a.region ?? "—"}</TableCell>
                    <TableCell>{formatDate(a.activationDate)}</TableCell>
                    <TableCell>{formatDate(a.deactivationDate)}</TableCell>
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
