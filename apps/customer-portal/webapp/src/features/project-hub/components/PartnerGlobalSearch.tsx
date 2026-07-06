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
  Divider,
  Menu,
  MenuItem,
  Paper,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, Download, FileText, FolderOpen, Search, X } from "@wso2/oxygen-ui-icons-react";
import { type JSX, type MouseEvent, useCallback, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { useGetGlobalSearch } from "@api/useGetGlobalSearch";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { PROJECT_HUB_SEARCH_DEBOUNCE_MS } from "@features/project-hub/constants/projectHubConstants";
import {
  downloadProjectListCsv,
  downloadProjectListPdf,
  fetchAllProjectsForExport,
} from "@features/project-hub/utils/projectsExport";
import {
  downloadCaseListCsv,
  downloadCaseListPdf,
  fetchAllCasesForExport,
} from "@features/project-hub/utils/casesExport";
import { getSeverityLegendColor } from "@features/dashboard/utils/dashboard";
import { formatCasesTableCaseIdentifier, getStatusColor } from "@features/dashboard/utils/casesTable";
import { mapSeverityToDisplay } from "@features/support/utils/support";
import type { GlobalSearchProject, GlobalSearchCase } from "@features/project-hub/types/globalSearch";
import { getCaseNavigationPath, getCaseTypeChipProps } from "@features/project-hub/utils/globalSearchNavigation";

type ExportFormat = "csv" | "pdf";

const GLOBAL_SEARCH_PAGE_SIZE = 5;
const SKELETON_ROW_COUNT = 5;
const DROPDOWN_RESULT_LIMIT = 5;
const DATE_LOCALE = "en-US";
const DATE_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
  day: "numeric",
  month: "short",
  year: "numeric",
};

function formatDate(value: string | null | undefined): string {
  if (!value) return "--";
  const d = new Date(value);
  return isNaN(d.getTime()) ? "--" : d.toLocaleDateString(DATE_LOCALE, DATE_FORMAT_OPTIONS);
}

function highlightMatch(text: string, query: string): JSX.Element {
  if (!query.trim()) return <>{text}</>;
  const lowerText = text.toLowerCase();
  const lowerQuery = query.toLowerCase().trim();
  const idx = lowerText.indexOf(lowerQuery);
  if (idx === -1) return <>{text}</>;
  return (
    <>
      {text.slice(0, idx)}
      <strong>{text.slice(idx, idx + lowerQuery.length)}</strong>
      {text.slice(idx + lowerQuery.length)}
    </>
  );
}

function SkeletonRows({ cols }: { cols: number }): JSX.Element {
  return (
    <>
      {Array.from({ length: SKELETON_ROW_COUNT }).map((_, i) => (
        <TableRow key={`sk-${i}`}>
          {Array.from({ length: cols }).map((_, j) => (
            <TableCell key={j}>
              <Skeleton variant="text" width="80%" />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  );
}

/**
 * Salesforce-style global search for partner users.
 * Shows the first 10 projects and first 10 cases in separate table sections,
 * each with a "View More" placeholder button.
 */
export default function PartnerGlobalSearch(): JSX.Element {
  const navigate = useNavigate();
  const authFetch = useAuthApiClient();
  const { showError } = useErrorBanner();
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearchQuery = useDebouncedValue(searchQuery, PROJECT_HUB_SEARCH_DEBOUNCE_MS);

  const [dropdownOpen, setDropdownOpen] = useState(false);

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
        const allProjects = await fetchAllProjectsForExport(authFetch, debouncedSearchQuery);
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
    [authFetch, debouncedSearchQuery, showError],
  );

  const isExporting = exportingFormat !== null;

  const [exportingCasesFormat, setExportingCasesFormat] = useState<ExportFormat | null>(null);
  const isExportingCasesRef = useRef(false);
  const [exportCasesAnchorEl, setExportCasesAnchorEl] = useState<HTMLElement | null>(null);

  const handleCasesExportOpen = (e: MouseEvent<HTMLElement>) => {
    if (!isExportingCasesRef.current) setExportCasesAnchorEl(e.currentTarget);
  };
  const handleCasesExportClose = () => setExportCasesAnchorEl(null);

  const handleCasesExport = useCallback(
    async (format: ExportFormat) => {
      if (isExportingCasesRef.current) return;
      handleCasesExportClose();
      isExportingCasesRef.current = true;
      setExportingCasesFormat(format);
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
        isExportingCasesRef.current = false;
        setExportingCasesFormat(null);
      }
    },
    [authFetch, debouncedSearchQuery, showError],
  );

  const isExportingCases = exportingCasesFormat !== null;

  // Table query — filters by the debounced search query when one is active
  const {
    data: tableData,
    isLoading: isLoadingTable,
    isError: isErrorTable,
  } = useGetGlobalSearch({
    ...(debouncedSearchQuery ? { filters: { searchQuery: debouncedSearchQuery } } : {}),
    projectsPagination: { offset: 0, limit: GLOBAL_SEARCH_PAGE_SIZE },
    casesPagination: { offset: 0, limit: GLOBAL_SEARCH_PAGE_SIZE },
  });

  // Dropdown query — filtered by the live search query
  const {
    data: dropdownData,
    isLoading: isLoadingDropdown,
  } = useGetGlobalSearch(
    {
      filters: debouncedSearchQuery ? { searchQuery: debouncedSearchQuery } : undefined,
      projectsPagination: { offset: 0, limit: DROPDOWN_RESULT_LIMIT },
      casesPagination: { offset: 0, limit: DROPDOWN_RESULT_LIMIT },
    },
    { enabled: Boolean(debouncedSearchQuery) },
  );

  const projects: GlobalSearchProject[] = tableData?.projects ?? [];
  const projectsTotal = tableData?.projectsTotal ?? 0;
  const cases: GlobalSearchCase[] = tableData?.cases ?? [];
  const casesTotal = tableData?.casesTotal ?? 0;

  const isLoadingProjects = isLoadingTable;
  const isLoadingCases = isLoadingTable;
  const isErrorProjects = isErrorTable;
  const isErrorCases = isErrorTable;

  const projectsMoreCount = Math.max(0, projectsTotal - projects.length);
  const casesMoreCount = Math.max(0, casesTotal - cases.length);

  const isDebouncing = searchQuery.trim() !== debouncedSearchQuery.trim();
  const dropdownProjects: GlobalSearchProject[] = isDebouncing || isLoadingDropdown
    ? []
    : (dropdownData?.projects ?? []).slice(0, DROPDOWN_RESULT_LIMIT);
  const dropdownCases: GlobalSearchCase[] = isDebouncing || isLoadingDropdown
    ? []
    : (dropdownData?.cases ?? []).slice(0, DROPDOWN_RESULT_LIMIT);

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
      {/* Page header + search bar */}
      <Box
        sx={{
          alignItems: "center",
          display: "flex",
          flexDirection: "column",
          flexShrink: 0,
          pb: 2,
          pt: 2.5,
          px: 2,
          textAlign: "center",
        }}
      >
        <Box sx={{ maxWidth: 560, position: "relative", width: "100%" }}>
          <TextField
            fullWidth
            inputProps={{ autoComplete: "off" }}
            InputProps={{
              endAdornment: searchQuery ? (
                <InputAdornment position="end">
                  <IconButton
                    aria-label="Clear search"
                    edge="end"
                    onClick={() => {
                      setSearchQuery("");
                      setDropdownOpen(false);
                    }}
                    size="small"
                  >
                    <X size={16} />
                  </IconButton>
                </InputAdornment>
              ) : undefined,
              startAdornment: <Search size={20} style={{ marginRight: 8 }} />,
            }}
            onBlur={() => setDropdownOpen(false)}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setDropdownOpen(Boolean(e.target.value.trim()));
            }}
            onFocus={() => {
              if (searchQuery.trim()) setDropdownOpen(true);
            }}
            onKeyDown={(e) => {
              if (e.key === "Escape") setDropdownOpen(false);
            }}
            placeholder="Search projects and cases..."
            size="small"
            sx={{
              "& input:-webkit-autofill, & input:-webkit-autofill:focus, & input:-webkit-autofill:hover":
                {
                  WebkitBoxShadow: "0 0 0 100px transparent inset",
                  WebkitTextFillColor: "inherit",
                  transition: "background-color 5000s ease-in-out 0s",
                },
            }}
            value={searchQuery}
          />
          {!dropdownOpen && debouncedSearchQuery.trim() && !isLoadingTable && (
            <Typography
              color="text.secondary"
              sx={{ display: "block", mt: 1, textAlign: "left" }}
              variant="caption"
            >
              Showing results for &ldquo;{debouncedSearchQuery.trim()}&rdquo;
            </Typography>
          )}
          {dropdownOpen && (
            <Paper
              elevation={4}
              onMouseDown={(e) => e.preventDefault()}
              sx={{
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                left: 0,
                maxHeight: 400,
                mt: 0.5,
                overflowY: "auto",
                position: "absolute",
                right: 0,
                textAlign: "left",
                top: "100%",
                zIndex: 1300,
              }}
            >
              {/* Projects section */}
              {(isDebouncing || isLoadingDropdown || dropdownProjects.length > 0) && (
                <>
                  {isDebouncing || isLoadingDropdown ? (
                    [1, 2, 3].map((i) => (
                      <Box
                        key={i}
                        sx={{
                          alignItems: "center",
                          display: "flex",
                          gap: 1.5,
                          px: 2,
                          py: 1,
                        }}
                      >
                        <Skeleton height={36} variant="circular" width={36} />
                        <Box sx={{ flex: 1 }}>
                          <Skeleton variant="text" width="70%" />
                          <Skeleton variant="text" width="30%" />
                        </Box>
                      </Box>
                    ))
                  ) : (
                    dropdownProjects.map((project) => (
                      <Box
                        key={project.id}
                        onClick={() => {
                          setDropdownOpen(false);
                          navigate(`/projects/${project.id}/dashboard`);
                        }}
                        sx={{
                          alignItems: "center",
                          cursor: "pointer",
                          display: "flex",
                          gap: 1.5,
                          px: 2,
                          py: 1.25,
                          "&:hover": { bgcolor: "action.hover" },
                        }}
                      >
                        <Box
                          sx={{
                            alignItems: "center",
                            bgcolor: "primary.main",
                            borderRadius: "50%",
                            color: "primary.contrastText",
                            display: "flex",
                            flexShrink: 0,
                            height: 36,
                            justifyContent: "center",
                            width: 36,
                          }}
                        >
                          <FolderOpen size={16} />
                        </Box>
                        <Box sx={{ flex: 1, minWidth: 0 }}>
                          <Typography noWrap variant="body2">
                            {highlightMatch(
                              project.key
                                ? `${project.key} · ${project.name}`
                                : project.name,
                              debouncedSearchQuery,
                            )}
                          </Typography>
                          <Typography color="text.secondary" variant="caption">
                            Project
                          </Typography>
                        </Box>
                      </Box>
                    ))
                  )}
                </>
              )}

              {/* Divider between sections — only when both have results */}
              {!isDebouncing &&
                !isLoadingDropdown &&
                dropdownProjects.length > 0 &&
                dropdownCases.length > 0 && <Divider />}

              {/* Cases section */}
              {(isDebouncing || isLoadingDropdown || dropdownCases.length > 0) && (
                <>
                  {isDebouncing || isLoadingDropdown ? (
                    [1, 2, 3].map((i) => (
                      <Box
                        key={i}
                        sx={{
                          alignItems: "center",
                          display: "flex",
                          gap: 1.5,
                          px: 2,
                          py: 1,
                        }}
                      >
                        <Skeleton height={36} variant="circular" width={36} />
                        <Box sx={{ flex: 1 }}>
                          <Skeleton variant="text" width="70%" />
                          <Skeleton variant="text" width="30%" />
                        </Box>
                      </Box>
                    ))
                  ) : (
                    dropdownCases.map((c) => {
                      const casePath = getCaseNavigationPath(c);
                      const { color: caseTypeColor, displayLabel: caseTypeLabel } = getCaseTypeChipProps(c.caseType?.label);
                      return (
                      <Box
                        key={c.id}
                        onClick={casePath ? () => {
                          setDropdownOpen(false);
                          navigate(casePath, { state: { returnTo: "/" } });
                        } : undefined}
                        sx={{
                          alignItems: "center",
                          cursor: casePath ? "pointer" : "default",
                          display: "flex",
                          gap: 1.5,
                          px: 2,
                          py: 1.25,
                          "&:hover": { bgcolor: "action.hover" },
                        }}
                      >
                        <Box
                          sx={{
                            alignItems: "center",
                            bgcolor: alpha(caseTypeColor, 0.15),
                            borderRadius: "50%",
                            color: caseTypeColor,
                            display: "flex",
                            flexShrink: 0,
                            height: 36,
                            justifyContent: "center",
                            width: 36,
                          }}
                        >
                          <FileText size={16} />
                        </Box>
                        <Box sx={{ flex: 1, minWidth: 0 }}>
                          <Typography noWrap variant="body2">
                            {highlightMatch(c.title ?? "--", debouncedSearchQuery)}
                          </Typography>
                          <Box sx={{ alignItems: "center", display: "flex", gap: 0.75, mt: 0.25 }}>
                            <Typography color="text.secondary" noWrap variant="caption">
                              {formatCasesTableCaseIdentifier(c.number, c.internalId)}
                            </Typography>
                            <Chip
                              label={caseTypeLabel}
                              size="small"
                              variant="outlined"
                              sx={(theme) => ({
                                bgcolor: alpha(caseTypeColor, theme.palette.mode === "dark" ? 0.05 : 0.1),
                                borderColor: alpha(caseTypeColor, theme.palette.mode === "dark" ? 0.18 : 0.3),
                                color: caseTypeColor,
                                flexShrink: 0,
                                fontSize: "0.65rem",
                                fontWeight: 500,
                                height: 16,
                                px: 0,
                                "& .MuiChip-label": { pl: "5px", pr: "5px" },
                              })}
                            />
                          </Box>
                        </Box>
                      </Box>
                      );
                    })
                  )}
                </>
              )}

              {/* No results */}
              {!isDebouncing &&
                !isLoadingDropdown &&
                dropdownProjects.length === 0 &&
                dropdownCases.length === 0 && (
                  <Box sx={{ px: 2, py: 2, textAlign: "center" }}>
                    <Typography color="text.secondary" variant="body2">
                      No results found.
                    </Typography>
                  </Box>
                )}
            </Paper>
          )}
        </Box>
      </Box>

      {/* Scrollable content: projects + cases sections */}
      <Box
        sx={{
          display: "flex",
          flex: "1 1 auto",
          flexDirection: "column",
          gap: 3,
          overflowX: "hidden",
          overflowY: "auto",
          pb: 4,
          px: 2,
        }}
      >
        {/* Projects section */}
        <Box>
          <Box sx={{ alignItems: "center", display: "flex", mb: 1.5 }}>
            <Box sx={{ alignItems: "center", display: "flex", flex: 1, gap: 1 }}>
              <FolderOpen size={20} />
              <Typography variant="h6">
                Projects
                {!isLoadingProjects && (
                  <Typography
                    color="text.secondary"
                    component="span"
                    variant="h6"
                  >
                    {" "}
                    ({projectsTotal})
                  </Typography>
                )}
              </Typography>
            </Box>
            <Button
              aria-controls="partner-export-menu"
              aria-expanded={Boolean(exportAnchorEl)}
              aria-haspopup="menu"
              disabled={isExporting || (!isLoadingProjects && projectsTotal === 0)}
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
              id="partner-export-menu"
              onClose={handleExportClose}
              open={Boolean(exportAnchorEl)}
            >
              <MenuItem onClick={() => void handleExport("csv")}>Export to CSV</MenuItem>
              <MenuItem onClick={() => void handleExport("pdf")}>Export to PDF</MenuItem>
            </Menu>
          </Box>

          <TableContainer component={Paper} sx={{ overflowX: "auto" }}>
            <Table sx={{ minWidth: 680 }}>
              <TableHead>
                <TableRow>
                  <TableCell>Project Key</TableCell>
                  <TableCell>Name</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Start Date</TableCell>
                  <TableCell>End Date</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoadingProjects ? (
                  <SkeletonRows cols={5} />
                ) : isErrorProjects ? (
                  <TableRow>
                    <TableCell align="center" colSpan={5}>
                      <Typography color="text.secondary" variant="body2">
                        Failed to load projects.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : projects.length === 0 ? (
                  <TableRow>
                    <TableCell align="center" colSpan={5}>
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
                        onClick={() => navigate(`/projects/${project.id}/dashboard`)}
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
                            label={project.closureState ?? "Active"}
                            size="small"
                            sx={{ fontWeight: 500 }}
                            variant="outlined"
                          />
                        </TableCell>
                        <TableCell>
                          <Typography color="text.secondary" variant="body2">
                            {formatDate(project.startDate)}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography color="text.secondary" variant="body2">
                            {formatDate(project.endDate)}
                          </Typography>
                        </TableCell>
                      </TableRow>
                    );
                  })
                )}
              </TableBody>
            </Table>
          </TableContainer>

          {projectsMoreCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 1 }}>
              <Button
                onClick={() => {
                  const params = debouncedSearchQuery
                    ? `?q=${encodeURIComponent(debouncedSearchQuery)}`
                    : "";
                  navigate(`/partner/projects${params}`);
                }}
                size="small"
                variant="text"
              >
                {`View More (${projectsMoreCount} more)`}
              </Button>
            </Box>
          )}
        </Box>

        {/* Cases section */}
        <Box>
          <Box sx={{ alignItems: "center", display: "flex", mb: 1.5 }}>
            <Box sx={{ alignItems: "center", display: "flex", flex: 1, gap: 1 }}>
              <FileText size={20} />
              <Typography variant="h6">
                Cases
                {!isLoadingCases && (
                  <Typography
                    color="text.secondary"
                    component="span"
                    variant="h6"
                  >
                    {" "}
                    ({casesTotal})
                  </Typography>
                )}
              </Typography>
            </Box>
            <Button
              aria-controls="partner-cases-export-menu"
              aria-expanded={Boolean(exportCasesAnchorEl)}
              aria-haspopup="menu"
              disabled={isExportingCases || (!isLoadingCases && casesTotal === 0)}
              endIcon={<ChevronDown size={16} />}
              onClick={handleCasesExportOpen}
              size="small"
              startIcon={
                isExportingCases ? (
                  <CircularProgress color="inherit" size={16} />
                ) : (
                  <Download size={16} />
                )
              }
              type="button"
              variant="outlined"
            >
              {isExportingCases ? "Exporting..." : "Export"}
            </Button>
            <Menu
              anchorEl={exportCasesAnchorEl}
              id="partner-cases-export-menu"
              onClose={handleCasesExportClose}
              open={Boolean(exportCasesAnchorEl)}
            >
              <MenuItem onClick={() => void handleCasesExport("csv")}>Export to CSV</MenuItem>
              <MenuItem onClick={() => void handleCasesExport("pdf")}>Export to PDF</MenuItem>
            </Menu>
          </Box>

          <TableContainer component={Paper} sx={{ overflowX: "auto" }}>
            <Table sx={{ minWidth: 720 }}>
              <TableHead>
                <TableRow>
                  <TableCell>Details</TableCell>
                  <TableCell>Case Type</TableCell>
                  <TableCell>Severity</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Created by</TableCell>
                  <TableCell>Created on</TableCell>
                  <TableCell>Project</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {isLoadingCases ? (
                  <SkeletonRows cols={7} />
                ) : isErrorCases ? (
                  <TableRow>
                    <TableCell align="center" colSpan={7}>
                      <Typography color="text.secondary" variant="body2">
                        Failed to load cases.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : cases.length === 0 ? (
                  <TableRow>
                    <TableCell align="center" colSpan={7}>
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
                    const handleNavigate = casePath ? () => navigate(casePath, { state: { returnTo: "/" } }) : undefined;
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
                            {formatDate(c.createdOn)}
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

          {casesMoreCount > 0 && (
            <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 1 }}>
              <Button
                onClick={() => {
                  const params = debouncedSearchQuery
                    ? `?q=${encodeURIComponent(debouncedSearchQuery)}`
                    : "";
                  navigate(`/partner/cases${params}`);
                }}
                size="small"
                variant="text"
              >
                {`View More (${casesMoreCount} more)`}
              </Button>
            </Box>
          )}
        </Box>
      </Box>
    </Box>
  );
}
