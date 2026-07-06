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
  InputAdornment,
  MenuItem,
  Paper,
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
import { Search } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import { useNavigate } from "react-router";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useSearchProductVulnerabilities } from "@features/csm-security-center/api/useSearchProductVulnerabilities";
import {
  VULNERABILITY_PRIORITIES,
  vulnerabilityPriorityColor,
  vulnerabilityPriorityLabel,
} from "@features/csm-security-center/utils/vulnerabilities";
import type { BeVulnerabilityPriority } from "@api/backend/types";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, 50];

/**
 * Product vulnerabilities listing for the Security Center → Vulnerabilities tab.
 * Searches `POST /products/vulnerabilities/search` with server-side pagination;
 * a row opens the vulnerability detail page.
 */
export default function ProductVulnerabilitiesTab(): JSX.Element {
  const navigate = useNavigate();
  const [searchInput, setSearchInput] = useState("");
  const [priorityFilter, setPriorityFilter] = useState<BeVulnerabilityPriority | "">("");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const debouncedSearch = useDebouncedValue(searchInput.trim(), 300);

  const payload = useMemo(
    () => ({
      filters: {
        ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
        ...(priorityFilter !== "" && { priority: priorityFilter }),
      },
      pagination: { offset: page * rowsPerPage, limit: rowsPerPage },
    }),
    [debouncedSearch, priorityFilter, page, rowsPerPage],
  );

  const { data, isLoading, isError, error, isFetching } =
    useSearchProductVulnerabilities(payload);

  const vulnerabilities = data?.productVulnerabilities ?? [];
  const total = data?.total ?? 0;

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handlePriorityChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setPriorityFilter(e.target.value as BeVulnerabilityPriority | "");
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
        <TextField
          value={searchInput}
          onChange={handleSearchChange}
          placeholder="Search vulnerabilities…"
          size="small"
          sx={{ maxWidth: 400 }}
          slotProps={{
            htmlInput: { "aria-label": "Search vulnerabilities" },
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            },
          }}
        />
        <TextField
          select
          value={priorityFilter}
          onChange={handlePriorityChange}
          size="small"
          label="Priority"
          sx={{ minWidth: 140 }}
          slotProps={{
            htmlInput: { "aria-label": "Filter by priority" },
          }}
        >
          <MenuItem value="">All priorities</MenuItem>
          {VULNERABILITY_PRIORITIES.map((p) => (
            <MenuItem key={p} value={p}>
              {vulnerabilityPriorityLabel(p)}
            </MenuItem>
          ))}
        </TextField>
      </Box>

      {isError && (
        <Alert severity="error">
          Failed to load vulnerabilities:{" "}
          {error instanceof Error ? error.message : "unknown error"}
        </Alert>
      )}

      <Paper variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>CVE / Vulnerability ID</TableCell>
                <TableCell>Component</TableCell>
                <TableCell>Product</TableCell>
                <TableCell>Priority</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Update level</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <TableRow key={i}>
                    {Array.from({ length: 6 }).map((__, j) => (
                      <TableCell key={j}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                // The error Alert above carries the detail; don't fall through
                // to the empty state (which would read as "0 results").
                null
              ) : vulnerabilities.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No vulnerabilities found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                vulnerabilities.map((vuln) => {
                  // Encode the id for the client route; ids are arbitrary
                  // strings and the detail hook encodes its backend path too.
                  const detailPath = `/security-center/vulnerabilities/${encodeURIComponent(
                    vuln.id,
                  )}`;
                  return (
                  <TableRow
                    key={vuln.id}
                    hover
                    onClick={() => navigate(detailPath)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        navigate(detailPath);
                      }
                    }}
                    tabIndex={0}
                    role="button"
                    aria-label={`View vulnerability ${
                      vuln.cveId || vuln.vulnerabilityId || vuln.id
                    }`}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>
                      <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                        {vuln.cveId || vuln.vulnerabilityId || "-"}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" noWrap title={vuln.componentName}>
                        {vuln.componentName || "-"}
                        {vuln.version ? (
                          <Typography
                            component="span"
                            variant="caption"
                            color="text.secondary"
                            sx={{ ml: 0.5 }}
                          >
                            {vuln.version}
                          </Typography>
                        ) : null}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" noWrap>
                        {vuln.productName || "-"}
                        {vuln.productVersion ? (
                          <Typography
                            component="span"
                            variant="caption"
                            color="text.secondary"
                            sx={{ ml: 0.5 }}
                          >
                            {vuln.productVersion}
                          </Typography>
                        ) : null}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      {vuln.priority ? (
                        <Chip
                          size="small"
                          variant="outlined"
                          color={vulnerabilityPriorityColor(vuln.priority)}
                          label={vuln.priority}
                        />
                      ) : (
                        "-"
                      )}
                    </TableCell>
                    <TableCell>{vuln.type || "-"}</TableCell>
                    <TableCell>{vuln.updateLevel || "-"}</TableCell>
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
