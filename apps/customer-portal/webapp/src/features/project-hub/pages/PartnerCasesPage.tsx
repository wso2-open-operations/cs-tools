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
  Chip,
  IconButton,
  InputAdornment,
  LinearProgress,
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
import { ArrowLeft, FileText, Search, X } from "@wso2/oxygen-ui-icons-react";
import { type ChangeEvent, type JSX, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useGetGlobalCasesPage } from "@api/useGetGlobalCasesPage";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { PROJECT_HUB_SEARCH_DEBOUNCE_MS } from "@features/project-hub/constants/projectHubConstants";
import { mapSeverityToDisplay } from "@features/support/utils/support";

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const DEFAULT_ROWS_PER_PAGE = 10;
const SKELETON_ROW_COUNT = 5;
const COL_SPAN = 5;

function getSeverityChipColor(
  label?: string,
): "default" | "error" | "info" | "secondary" | "success" | "warning" {
  const display = mapSeverityToDisplay(label);
  const token = display.replace(/\s*\(.*$/, "").toUpperCase();
  switch (token) {
    case "S0":
      return "error";
    case "S1":
      return "warning";
    case "S2":
      return "info";
    case "S3":
      return "secondary";
    case "S4":
      return "success";
    default:
      return "default";
  }
}

/**
 * Full-page cases search for partner users.
 * Navigated to via "View More" on the partner global search page.
 * Pre-fills the search bar from the `?q=` URL parameter.
 */
export default function PartnerCasesPage(): JSX.Element {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const urlQuery = searchParams.get("q") ?? "";

  const [searchQuery, setSearchQuery] = useState(urlQuery);
  const debouncedSearchQuery = useDebouncedValue(searchQuery, PROJECT_HUB_SEARCH_DEBOUNCE_MS);

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  // Reset to first page when the search query changes.
  useEffect(() => {
    setPage(0);
  }, [debouncedSearchQuery]);

  // Sync debounced value back to URL.
  useEffect(() => {
    const params: Record<string, string> = {};
    if (debouncedSearchQuery.trim()) params.q = debouncedSearchQuery.trim();
    setSearchParams(params, { replace: true });
  }, [debouncedSearchQuery, setSearchParams]);

  const { data, isLoading, isError } = useGetGlobalCasesPage(
    {
      filters: debouncedSearchQuery ? { searchQuery: debouncedSearchQuery } : undefined,
    },
    page * rowsPerPage,
    rowsPerPage,
  );

  const cases = data?.cases ?? [];
  const totalRecords = data?.totalRecords ?? 0;

  const handleChangePage = (_: unknown, newPage: number) => setPage(newPage);
  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box
      sx={{
        display: "flex",
        flex: 1,
        flexDirection: "column",
        minHeight: 0,
        overflow: "hidden",
        width: "100%",
      }}
    >
      {/* Header */}
      <Box
        sx={{
          alignItems: "center",
          display: "flex",
          flexShrink: 0,
          gap: 1.5,
          pb: 1.5,
          pt: 2,
          px: 2,
        }}
      >
        <IconButton
          aria-label="Back to search"
          onClick={() => navigate("/")}
          size="small"
        >
          <ArrowLeft size={20} />
        </IconButton>
        <FileText size={22} />
        <Typography sx={{ flex: 1 }} variant="h5">
          Cases
          {!isLoading && (
            <Typography color="text.secondary" component="span" variant="h5">
              {" "}({totalRecords})
            </Typography>
          )}
          {isLoading && (
            <Skeleton
              sx={{ display: "inline-block", ml: 0.5 }}
              variant="text"
              width={36}
            />
          )}
        </Typography>
      </Box>

      {/* Search bar */}
      <Box sx={{ flexShrink: 0, pb: 1.5, px: 2 }}>
        <TextField
          fullWidth
          inputProps={{ autoComplete: "off" }}
          InputProps={{
            endAdornment: searchQuery ? (
              <InputAdornment position="end">
                <IconButton
                  aria-label="Clear search"
                  edge="end"
                  onClick={() => setSearchQuery("")}
                  size="small"
                >
                  <X size={16} />
                </IconButton>
              </InputAdornment>
            ) : undefined,
            startAdornment: <Search size={20} style={{ marginRight: 8 }} />,
          }}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search cases..."
          size="small"
          sx={{
            maxWidth: 560,
            "& input:-webkit-autofill, & input:-webkit-autofill:focus, & input:-webkit-autofill:hover":
              {
                WebkitBoxShadow: "0 0 0 100px transparent inset",
                WebkitTextFillColor: "inherit",
                transition: "background-color 5000s ease-in-out 0s",
              },
          }}
          value={searchQuery}
        />
      </Box>

      {/* Loading indicator */}
      {isLoading && (
        <LinearProgress
          color="inherit"
          sx={{ flexShrink: 0, height: 2, mx: 2 }}
        />
      )}

      {/* Scrollable table */}
      <Box
        sx={{
          flex: "1 1 auto",
          overflowX: "hidden",
          overflowY: "auto",
          pb: 3,
          px: 2,
        }}
      >
        <TableContainer component={Paper} sx={{ overflowX: "auto" }}>
          <Table sx={{ minWidth: 720 }}>
            <TableHead>
              <TableRow>
                <TableCell>Case #</TableCell>
                <TableCell>Title</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Project</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: SKELETON_ROW_COUNT }).map((_, i) => (
                  <TableRow key={`sk-${i}`}>
                    {Array.from({ length: COL_SPAN }).map((_, j) => (
                      <TableCell key={j}>
                        <Skeleton variant="text" width="80%" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell align="center" colSpan={COL_SPAN}>
                    <Typography color="text.secondary" variant="body2">
                      Failed to load cases. Please try again.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : cases.length === 0 ? (
                <TableRow>
                  <TableCell align="center" colSpan={COL_SPAN}>
                    <Typography color="text.secondary" variant="body2">
                      No cases found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                cases.map((c) => {
                  const severityLabel = c.severity?.label;
                  const severityDisplay = mapSeverityToDisplay(severityLabel);
                  const severityColor = getSeverityChipColor(severityLabel);
                  return (
                    <TableRow
                      hover
                      key={c.id}
                      onClick={() =>
                        navigate(
                          `/projects/${c.project?.id}/support/cases/${c.id}`,
                        )
                      }
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          navigate(
                            `/projects/${c.project?.id}/support/cases/${c.id}`,
                          );
                        }
                      }}
                      sx={{ cursor: "pointer" }}
                      tabIndex={0}
                    >
                      <TableCell>
                        <Typography fontWeight="medium" variant="body2">
                          {c.number}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{c.title}</Typography>
                      </TableCell>
                      <TableCell>
                        {severityLabel ? (
                          <Chip
                            color={severityColor}
                            label={severityDisplay}
                            size="small"
                            sx={{ fontWeight: 500 }}
                            variant="outlined"
                          />
                        ) : (
                          <Typography color="text.secondary" variant="body2">
                            --
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">
                          {c.status?.label ?? "--"}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography color="text.secondary" variant="body2">
                          {c.project?.label ?? "--"}
                        </Typography>
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
          count={totalRecords}
          onPageChange={handleChangePage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          page={page}
          rowsPerPage={rowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        />
      </Box>
    </Box>
  );
}
