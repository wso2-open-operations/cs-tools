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
  InputAdornment,
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
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavigate } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { useSearchChangeRequests } from "@features/csm-operations/api/useSearchChangeRequests";
import {
  changeRequestImpactColor,
  changeRequestImpactLabel,
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

function formatDate(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      year: "numeric",
      month: "numeric",
      day: "numeric",
    }) ?? "—"
  );
}

/**
 * Change-requests listing for the Operations → Change requests tab. Searches
 * `POST /change-requests/search` (subject + number) with server-side
 * pagination; a row opens the change-request detail page.
 */
export default function ChangeRequestsTab(): JSX.Element {
  const navigate = useNavigate();
  const [searchInput, setSearchInput] = useState("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(searchInput.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: {
        ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
      },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [debouncedSearch, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching } =
    useSearchChangeRequests(payload);

  const changeRequests = data?.changeRequests ?? [];
  const total = data?.total ?? 0;

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <TextField
        value={searchInput}
        onChange={handleSearchChange}
        placeholder="Search change requests by number or subject…"
        size="small"
        sx={{ maxWidth: 480 }}
        slotProps={{
          htmlInput: { "aria-label": "Search change requests by number or subject" },
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={16} />
              </InputAdornment>
            ),
          },
        }}
      />

      {isError && (
        <Alert severity="error">
          Failed to load change requests:{" "}
          {error instanceof Error ? error.message : "unknown error"}
        </Alert>
      )}

      <Paper variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
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
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={24} />
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
