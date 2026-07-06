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
  alpha,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  InputAdornment,
  LinearProgress,
  Menu,
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
import { ArrowLeft, ChevronDown, Download, FileText, Search, X } from "@wso2/oxygen-ui-icons-react";
import { type ChangeEvent, type JSX, type MouseEvent, useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useGetGlobalSearch } from "@api/useGetGlobalSearch";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { PROJECT_HUB_SEARCH_DEBOUNCE_MS } from "@features/project-hub/constants/projectHubConstants";
import { getSeverityLegendColor } from "@features/dashboard/utils/dashboard";
import { formatCasesTableCaseIdentifier, getStatusColor } from "@features/dashboard/utils/casesTable";
import { mapSeverityToDisplay } from "@features/support/utils/support";
import { getCaseNavigationPath, getCaseTypeChipProps } from "@features/project-hub/utils/globalSearchNavigation";
import {
  downloadCaseListCsv,
  downloadCaseListPdf,
  fetchAllCasesForExport,
} from "@features/project-hub/utils/casesExport";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const DEFAULT_ROWS_PER_PAGE = 10;
const SKELETON_ROW_COUNT = 5;
const COL_SPAN = 6;


/**
 * Full-page cases search for partner users.
 * Navigated to via "View More" on the partner global search page.
 * Pre-fills the search bar from the `?q=` URL parameter.
 */
type ExportFormat = "csv" | "pdf";

export default function PartnerCasesPage(): JSX.Element {
  const navigate = useNavigate();
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();
  const [searchParams, setSearchParams] = useSearchParams();
  const urlQuery = searchParams.get("q") ?? "";

  const [searchQuery, setSearchQuery] = useState(urlQuery);
  const debouncedSearchQuery = useDebouncedValue(searchQuery, PROJECT_HUB_SEARCH_DEBOUNCE_MS);

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  // Resync input when the URL ?q= changes externally (back/forward navigation).
  useEffect(() => {
    setSearchQuery(urlQuery);
  }, [urlQuery]);

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

  const { data, isLoading, isError } = useGetGlobalSearch({
    filters: {
      types: ["cases"],
      ...(debouncedSearchQuery ? { searchQuery: debouncedSearchQuery } : {}),
    },
    casesPagination: { offset: page * rowsPerPage, limit: rowsPerPage },
  });

  const cases = data?.cases ?? [];
  const totalRecords = data?.casesTotal ?? 0;

  const [exportingFormat, setExportingFormat] = useState<ExportFormat | null>(null);
  const isExportingRef = useRef(false);
  const [exportAnchorEl, setExportAnchorEl] = useState<HTMLElement | null>(null);

  const handleExportOpen = (e: MouseEvent<HTMLElement>) => {
    if (!isExportingRef.current) setExportAnchorEl(e.currentTarget);
  };
  const handleExportClose = () => setExportAnchorEl(null);

  const handleExport = useCallback(
    async (format: ExportFormat) => {
      if (isExportingRef.current) return;
      handleExportClose();
      isExportingRef.current = true;
      setExportingFormat(format);
      try {
        const allCases = await fetchAllCasesForExport(authFetch, debouncedSearchQuery);
        if (allCases.length === 0) {
          showError("No cases to export.");
          return;
        }
        if (format === "csv") {
          downloadCaseListCsv(allCases);
        } else {
          downloadCaseListPdf(allCases);
        }
      } catch {
        showError("Failed to export cases.");
      } finally {
        isExportingRef.current = false;
        setExportingFormat(null);
      }
    },
    [authFetch, debouncedSearchQuery, showError],
  );

  const isExporting = exportingFormat !== null;

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
        <Button
          aria-controls="cases-page-export-menu"
          aria-expanded={Boolean(exportAnchorEl)}
          aria-haspopup="menu"
          disabled={isExporting || (!isLoading && totalRecords === 0)}
          endIcon={<ChevronDown size={16} />}
          onClick={handleExportOpen}
          size="small"
          startIcon={
            isExporting ? (
              <CircularProgress color="inherit" size={16} />
            ) : (
              <Download size={16} />
            )
          }
          type="button"
          variant="outlined"
        >
          {isExporting ? "Exporting..." : "Export"}
        </Button>
        <Menu
          anchorEl={exportAnchorEl}
          id="cases-page-export-menu"
          onClose={handleExportClose}
          open={Boolean(exportAnchorEl)}
        >
          <MenuItem onClick={() => void handleExport("csv")}>Export to CSV</MenuItem>
          <MenuItem onClick={() => void handleExport("pdf")}>Export to PDF</MenuItem>
        </Menu>
      </Box>

      {/* Filtered-results hint */}
      {debouncedSearchQuery.trim() && !isLoading && (
        <Box sx={{ flexShrink: 0, pb: 0.5, px: 2 }}>
          <Typography color="text.secondary" variant="caption">
            Showing results for &ldquo;{debouncedSearchQuery.trim()}&rdquo;
          </Typography>
        </Box>
      )}

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
                <TableCell>Details</TableCell>
                <TableCell>Case Type</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Created by</TableCell>
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
                  const severityColor = getSeverityLegendColor(severityLabel);
                  const { color: caseTypeColor, displayLabel: caseTypeLabel } = getCaseTypeChipProps(c.caseType?.label);
                  const casePath = getCaseNavigationPath(c);
                  const returnTo = `/partner/cases${searchParams.toString() ? `?${searchParams.toString()}` : ""}`;
                  const handleNavigate = casePath ? () => navigate(casePath, { state: { returnTo } }) : undefined;
                  return (
                    <TableRow
                      hover={Boolean(casePath)}
                      key={c.id}
                      onClick={handleNavigate}
                      onKeyDown={(e) => {
                        if (handleNavigate && (e.key === "Enter" || e.key === " ")) {
                          e.preventDefault();
                          handleNavigate();
                        }
                      }}
                      sx={{ cursor: casePath ? "pointer" : "default" }}
                      tabIndex={casePath ? 0 : undefined}
                    >
                      <TableCell sx={{ maxWidth: 320 }}>
                        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
                          <Typography
                            noWrap
                            variant="body2"
                            sx={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                          >
                            {c.title ?? "--"}
                          </Typography>
                          <Typography color="text.secondary" variant="caption">
                            {formatCasesTableCaseIdentifier(c.number, c.internalId)}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Chip
                          label={caseTypeLabel}
                          size="small"
                          variant="outlined"
                          sx={(theme) => ({
                            bgcolor: alpha(caseTypeColor, theme.palette.mode === "dark" ? 0.05 : 0.1),
                            borderColor: alpha(caseTypeColor, theme.palette.mode === "dark" ? 0.18 : 0.3),
                            color: caseTypeColor,
                            fontSize: "0.75rem",
                            fontWeight: 500,
                            height: 20,
                            px: 0,
                            "& .MuiChip-label": { pl: "6px", pr: "6px" },
                          })}
                        />
                      </TableCell>
                      <TableCell>
                        {severityLabel ? (
                          <Chip
                            label={severityDisplay}
                            size="small"
                            variant="outlined"
                            sx={{
                              bgcolor: alpha(severityColor, 0.1),
                              borderColor: alpha(severityColor, 0.3),
                              color: severityColor,
                              fontSize: "0.75rem",
                              fontWeight: 500,
                              height: 20,
                              px: 0,
                              "& .MuiChip-label": { pl: "6px", pr: "6px" },
                            }}
                          />
                        ) : (
                          <Typography color="text.secondary" variant="body2">
                            --
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>
                        <Box sx={{ alignItems: "center", display: "flex", gap: 1 }}>
                          <Box
                            sx={{
                              backgroundColor: getStatusColor(c.state?.label),
                              borderRadius: "50%",
                              flexShrink: 0,
                              height: 8,
                              width: 8,
                            }}
                          />
                          <Typography variant="body2">
                            {c.state?.label ?? "--"}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Typography color="text.secondary" variant="body2">
                          {c.createdBy ?? "--"}
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
