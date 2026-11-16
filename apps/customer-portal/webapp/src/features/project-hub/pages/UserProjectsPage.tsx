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
  Button,
  Chip,
  CircularProgress,
  IconButton,
  InputAdornment,
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
import { ArrowLeft, ChevronDown, Download, FolderOpen, Search, X } from "@wso2/oxygen-ui-icons-react";
import { type ChangeEvent, type JSX, type MouseEvent, useCallback, useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useGetGlobalSearch } from "@api/useGetGlobalSearch";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { PROJECT_HUB_SEARCH_DEBOUNCE_MS } from "@features/project-hub/constants/projectHubConstants";
import {
  downloadProjectListCsv,
  downloadProjectListPdf,
  fetchAllProjectsForExport,
} from "@features/project-hub/utils/projectsExport";
import { formatClosureState } from "@features/project-hub/utils/closureState";

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const DEFAULT_ROWS_PER_PAGE = 10;
const COL_SPAN = 6;

type ExportFormat = "csv" | "pdf";

/**
 * Full-page projects search for users with more than one project.
 * Navigated to via "View More" on the home overview page.
 * Pre-fills the search bar from the `?q=` URL parameter.
 */
export default function UserProjectsPage(): JSX.Element {
  const navigate = useNavigate();
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchQuery, setSearchQuery] = useState(() => searchParams.get("q") ?? "");
  const debouncedSearchQuery = useDebouncedValue(searchQuery, PROJECT_HUB_SEARCH_DEBOUNCE_MS);

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);

  const normalizedQuery = debouncedSearchQuery.trim();

  useEffect(() => {
    setPage(0);
  }, [normalizedQuery]);

  useEffect(() => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (normalizedQuery) {
          next.set("q", normalizedQuery);
        } else {
          next.delete("q");
        }
        return next;
      },
      { replace: true },
    );
  }, [normalizedQuery, setSearchParams]);

  const { data, isLoading, isError } = useGetGlobalSearch({
    filters: {
      types: ["projects"],
      ...(normalizedQuery ? { searchQuery: normalizedQuery } : {}),
    },
    projectsPagination: { offset: page * rowsPerPage, limit: rowsPerPage },
  });

  const projects = data?.projects ?? [];
  const totalRecords = data?.projectsTotal ?? 0;

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
        const allProjects = await fetchAllProjectsForExport(authFetch, normalizedQuery);
        if (allProjects.length === 0) {
          showError("No projects to export.");
          return;
        }
        if (format === "csv") {
          downloadProjectListCsv(allProjects);
        } else {
          downloadProjectListPdf(allProjects);
        }
      } catch {
        showError("Failed to export projects.");
      } finally {
        isExportingRef.current = false;
        setExportingFormat(null);
      }
    },
    [authFetch, normalizedQuery, showError],
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
          aria-label="Back to overview"
          onClick={() => navigate("/")}
          size="small"
        >
          <ArrowLeft size={20} />
        </IconButton>
        <FolderOpen size={22} />
        <Typography sx={{ flex: 1 }} variant="h5">
          Projects
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
          aria-controls="user-projects-page-export-menu"
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
          id="user-projects-page-export-menu"
          onClose={handleExportClose}
          open={Boolean(exportAnchorEl)}
        >
          <MenuItem onClick={() => void handleExport("csv")}>Export to CSV</MenuItem>
          <MenuItem onClick={() => void handleExport("pdf")}>Export to PDF</MenuItem>
        </Menu>
      </Box>

      {/* Filtered-results hint */}
      {normalizedQuery && !isLoading && (
        <Box sx={{ flexShrink: 0, pb: 0.5, px: 2 }}>
          <Typography color="text.secondary" variant="caption">
            Showing results for &ldquo;{normalizedQuery}&rdquo;
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
          placeholder="Search projects..."
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
                <TableCell>Project Key</TableCell>
                <TableCell>Name</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Action Required</TableCell>
                <TableCell align="right">Outstanding</TableCell>
                <TableCell align="right">Active Chats</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
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
                      Failed to load projects. Please try again.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : projects.length === 0 ? (
                <TableRow>
                  <TableCell align="center" colSpan={COL_SPAN}>
                    <Typography color="text.secondary" variant="body2">
                      No projects found.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                projects.map((project) => {
                  const isSuspended =
                    project.closureState?.toLowerCase() === "suspended";
                  return (
                    <TableRow
                      hover
                      key={project.id}
                      onClick={() =>
                        navigate(`/projects/${project.id}/dashboard`)
                      }
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          navigate(`/projects/${project.id}/dashboard`);
                        }
                      }}
                      sx={{ cursor: "pointer" }}
                      tabIndex={0}
                    >
                      <TableCell>
                        <Typography fontWeight="medium" variant="body2">
                          {project.key}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2">{project.name}</Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          color={isSuspended ? "warning" : "success"}
                          label={formatClosureState(project.closureState)}
                          size="small"
                          sx={{ fontWeight: 500 }}
                          variant="outlined"
                        />
                      </TableCell>
                      <TableCell align="right">
                        <Typography variant="body2">
                          {project.actionRequiredCount ?? 0}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <Typography variant="body2">
                          {project.outstandingCount ?? 0}
                        </Typography>
                      </TableCell>
                      <TableCell align="right">
                        <Typography variant="body2">
                          {project.activeChatsCount ?? 0}
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
