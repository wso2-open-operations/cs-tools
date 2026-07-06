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
  Menu,
  MenuItem,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, Download } from "@wso2/oxygen-ui-icons-react";
import { type JSX, useState, useMemo, useRef, useCallback, type MouseEvent } from "react";
import { useNavigate } from "react-router";
import type { ProjectListItem } from "@features/project-hub/types/projects";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import {
  downloadProjectListCsv,
  downloadProjectListPdf,
} from "@features/project-hub/utils/projectsExport";

const DATE_LOCALE = "en-US";
const DATE_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "short",
  day: "numeric",
};
const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const DEFAULT_ROWS_PER_PAGE = 10;
const COL_SPAN = 7;

function formatDate(value: string | null | undefined): string {
  if (!value) return "--";
  const d = new Date(value);
  if (isNaN(d.getTime())) return "--";
  return d.toLocaleDateString(DATE_LOCALE, DATE_FORMAT_OPTIONS);
}

type ProjectListTableProps = {
  projects: ProjectListItem[];
  isFetchingNextPage?: boolean;
};

/**
 * List-view table for partner users with more than 4 projects.
 * Columns: Project Key, Name, Status, Start Date, End Date, Action Required Items, Outstanding Items.
 * Clicking a row navigates to the project dashboard.
 */
type ExportFormat = "csv" | "pdf";

const ProjectListTable = ({
  projects,
  isFetchingNextPage,
}: ProjectListTableProps): JSX.Element => {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [exportingFormat, setExportingFormat] = useState<ExportFormat | null>(null);
  const isExportingRef = useRef(false);
  const [exportAnchorEl, setExportAnchorEl] = useState<HTMLElement | null>(null);

  const handleExportOpen = (e: MouseEvent<HTMLElement>) => {
    if (!isExportingRef.current) setExportAnchorEl(e.currentTarget);
  };
  const handleExportClose = () => setExportAnchorEl(null);

  const handleExport = useCallback(
    (format: ExportFormat) => {
      if (isExportingRef.current) return;
      handleExportClose();
      if (projects.length === 0) {
        showError("No projects to export.");
        return;
      }
      isExportingRef.current = true;
      setExportingFormat(format);
      try {
        if (format === "csv") {
          downloadProjectListCsv(projects);
        } else {
          downloadProjectListPdf(projects);
        }
      } catch {
        showError("Failed to export projects.");
      } finally {
        isExportingRef.current = false;
        setExportingFormat(null);
      }
    },
    [projects, showError],
  );

  const maxPage = Math.max(0, Math.floor((projects.length - 1) / rowsPerPage));
  if (page > maxPage) setPage(maxPage);

  const paginatedProjects = useMemo(
    () => projects.slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage),
    [projects, page, rowsPerPage],
  );

  const handleChangePage = (_: unknown, newPage: number) => setPage(newPage);

  const handleChangeRowsPerPage = (e: React.ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  const isExporting = exportingFormat !== null;

  return (
    <Box sx={{ width: "100%", mt: 1 }}>
      <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 1 }}>
        <Button
          type="button"
          variant="outlined"
          size="small"
          onClick={handleExportOpen}
          disabled={isExporting || projects.length === 0}
          aria-haspopup="menu"
          aria-expanded={Boolean(exportAnchorEl)}
          aria-controls="project-list-export-menu"
          startIcon={
            isExporting ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <Download size={16} />
            )
          }
          endIcon={<ChevronDown size={16} />}
        >
          {isExporting ? "Exporting..." : "Export"}
        </Button>
        <Menu
          id="project-list-export-menu"
          anchorEl={exportAnchorEl}
          open={Boolean(exportAnchorEl)}
          onClose={handleExportClose}
        >
          <MenuItem onClick={() => handleExport("csv")}>Export to CSV</MenuItem>
          <MenuItem onClick={() => handleExport("pdf")}>Export to PDF</MenuItem>
        </Menu>
      </Box>
      <TableContainer component={Paper} sx={{ overflowX: "auto" }}>
        <Table sx={{ minWidth: 800 }}>
          <TableHead>
            <TableRow>
              <TableCell>Project Key</TableCell>
              <TableCell>Name</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Start Date</TableCell>
              <TableCell>End Date</TableCell>
              <TableCell>Action Required Items</TableCell>
              <TableCell>Outstanding Items</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {projects.length === 0 && !isFetchingNextPage ? (
              <TableRow>
                <TableCell colSpan={COL_SPAN} align="center">
                  <Typography variant="body2" color="text.secondary">
                    No projects found.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              paginatedProjects.map((project) => {
                const isSuspended =
                  project.closureState?.toLowerCase() === "suspended";
                return (
                  <TableRow
                    key={project.id}
                    hover
                    tabIndex={0}
                    sx={{ cursor: "pointer" }}
                    onClick={() => navigate(`/projects/${project.id}/dashboard`)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        navigate(`/projects/${project.id}/dashboard`);
                      }
                    }}
                  >
                    <TableCell>
                      <Typography variant="body2" fontWeight="medium">
                        {project.key}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{project.name}</Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={project.closureState ?? "Active"}
                        size="small"
                        color={isSuspended ? "warning" : "success"}
                        variant="outlined"
                        sx={{ fontWeight: 500 }}
                      />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {formatDate(project.startDate)}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" color="text.secondary">
                        {formatDate(project.endDate)}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {project.actionRequiredCount ?? 0}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {project.outstandingCount ?? 0}
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
        count={projects.length}
        page={page}
        onPageChange={handleChangePage}
        rowsPerPage={rowsPerPage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
      />
    </Box>
  );
};

export default ProjectListTable;
