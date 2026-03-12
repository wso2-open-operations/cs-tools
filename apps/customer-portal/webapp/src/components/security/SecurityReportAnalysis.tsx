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
  Typography,
  Button,
  Chip,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Paper,
  TextField,
  InputAdornment,
  Divider,
  Form,
  Stack,
  Grid,
  alpha,
  useTheme,
  Pagination,
} from "@wso2/oxygen-ui";
import {
  Plus,
  Search,
  ListFilter,
  ChevronDown,
  ChevronUp,
  X,
  Calendar,
  User,
  FileText,
} from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useNavigate, useParams } from "react-router";
import { CaseType } from "@constants/supportConstants";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetProjectFilters from "@api/useGetProjectFilters";
import type { AllCasesFilterValues, CaseListItem } from "@models/responses";
import SecurityReportAnalysisSkeleton from "@components/security/SecurityReportAnalysisSkeleton";
import TabBar from "@components/common/tab-bar/TabBar";
import {
  resolveColorFromTheme,
  formatDateTime,
  getStatusColor,
  getStatusIcon,
  getSeverityColor,
  mapSeverityToDisplay,
  getAssignedEngineerLabel,
  stripHtml,
} from "@utils/support";

/**
 * SecurityReportAnalysis displays security vulnerability reports uploaded for analysis.
 * @returns {JSX.Element}
 */
const SecurityReportAnalysis = (): JSX.Element => {
  const theme = useTheme();
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [viewMode, setViewMode] = useState<"my" | "all">("all");
  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortOrder, setSortOrder] = useState<"desc" | "asc">("desc");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  // Fetch filter metadata
  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  // Build case search request for security report analysis cases
  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
        createdByMe: viewMode === "my" ? true : undefined,
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
        searchQuery: searchTerm.trim() || undefined,
      },
      sortBy: {
        field: "createdOn",
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortOrder, viewMode],
  );

  // Fetch security report analysis cases
  const { data, isLoading, hasNextPage, fetchNextPage } = useGetProjectCases(
    projectId || "",
    caseSearchRequest,
    {
      enabled: !!projectId,
    },
  );

  // Auto-fetch all pages for complete dataset
  useEffect(() => {
    if (!data || !hasNextPage) {
      return;
    }
    void fetchNextPage();
  }, [data, hasNextPage, fetchNextPage]);

  const displayedCases = useMemo(
    () => data?.pages.flatMap((page) => page.cases) ?? [],
    [data],
  );

  const totalItems = displayedCases.length;

  // Pagination logic
  const paginatedCases = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return displayedCases.slice(startIndex, startIndex + pageSize);
  }, [displayedCases, page]);

  const totalPages = Math.ceil(totalItems / pageSize);

  const handleCreateReport = () => {
    // Navigate to create case page with security report analysis type
    navigate(
      `/projects/${projectId}/support/security-report/create`,
    );
  };

  const handleCaseClick = (caseItem: CaseListItem) => {
    navigate(
      `/projects/${projectId}/security-center/security-report-analysis/${caseItem.id}`,
    );
  };

  const handleSearchChange = (event: ChangeEvent<HTMLInputElement>) => {
    setSearchTerm(event.target.value);
    setPage(1);
  };

  const handleFilterChange = (field: string, value: string) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value || undefined,
    }));
    setPage(1);
  };

  const handleClearFilters = () => {
    setFilters({});
    setPage(1);
  };

  const handleSortChange = (value: "desc" | "asc") => {
    setSortOrder(value);
    setPage(1);
  };

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const hasActiveFilters = !!filters.statusId || !!filters.deploymentId;

  const reportViewTabs = useMemo(
    () => [
      { id: "my", label: "My Reports" },
      { id: "all", label: "All Reports" },
    ],
    [],
  );

  return (
    <Paper
      sx={{
        width: "100%",
        mb: 4,
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 3,
      }}
    >
      {/* Header Section */}
      <Box>
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            justifyContent: "space-between",
            alignItems: { xs: "flex-start", sm: "center" },
            gap: 2,
          }}
        >
          {/* Title and Description */}
          <Box>
            <Typography variant="h5" color="text.primary" sx={{ mb: 0.5 }}>
              Security Report Analysis
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Security vulnerability reports uploaded for analysis
            </Typography>
          </Box>

          {/* Action Buttons */}
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              alignItems: "center",
              gap: 1.5,
            }}
          >
            <TabBar
              tabs={reportViewTabs}
              activeTab={viewMode}
              onTabChange={(tabId) => {
                setViewMode(tabId as "my" | "all");
                setPage(1);
              }}
              sx={{ mb: 0, height: 32 }}
            />

            {/* Create Report Button */}
            <Button
              variant="contained"
              color="warning"
              startIcon={<Plus size={16} />}
              onClick={handleCreateReport}
              size="small"
            >
              Create Report
            </Button>
          </Box>
        </Box>
      </Box>

      {/* Search and Filters Section */}
      <Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 2 }}>
          <Box sx={{ position: "relative", flex: 1 }}>
            <TextField
              fullWidth
              size="small"
              placeholder="Search reports by case number, title, or description..."
              value={searchTerm}
              onChange={handleSearchChange}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={16} />
                    </InputAdornment>
                  ),
                },
              }}
            />
          </Box>
          <Button
            variant="outlined"
            size="small"
            onClick={
              hasActiveFilters
                ? handleClearFilters
                : () => setIsFiltersOpen(!isFiltersOpen)
            }
            startIcon={
              hasActiveFilters ? <X size={16} /> : <ListFilter size={16} />
            }
            endIcon={
              !hasActiveFilters &&
              (isFiltersOpen ? (
                <ChevronUp size={16} />
              ) : (
                <ChevronDown size={16} />
              ))
            }
          >
            {hasActiveFilters ? "Clear Filters" : "Filters"}
          </Button>
        </Box>

        {isFiltersOpen && (
          <>
            <Divider sx={{ my: 2 }} />
            <Grid container spacing={2}>
              <Grid size={{ xs: 12, sm: 6, md: 3 }}>
                <FormControl fullWidth size="small">
                  <InputLabel id="status-label">Status</InputLabel>
                  <Select
                    labelId="status-label"
                    value={filters.statusId || ""}
                    label="Status"
                    onChange={(e) =>
                      handleFilterChange("statusId", e.target.value)
                    }
                  >
                    <MenuItem value="">
                      <Typography variant="body2">All Statuses</Typography>
                    </MenuItem>
                    {filterMetadata?.caseStates?.map((status) => (
                      <MenuItem key={status.id} value={status.id}>
                        <Typography variant="body2">{status.label}</Typography>
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </Grid>
              <Grid size={{ xs: 12, sm: 6, md: 3 }}>
                <FormControl fullWidth size="small">
                  <InputLabel id="deployment-label">Deployment</InputLabel>
                  <Select
                    labelId="deployment-label"
                    value={filters.deploymentId || ""}
                    label="Deployment"
                    onChange={(e) =>
                      handleFilterChange("deploymentId", e.target.value)
                    }
                  >
                    <MenuItem value="">
                      <Typography variant="body2">All Deployments</Typography>
                    </MenuItem>
                    {filterMetadata?.deploymentTypes?.map((deployment) => (
                      <MenuItem key={deployment.id} value={deployment.id}>
                        <Typography variant="body2">
                          {deployment.label}
                        </Typography>
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </Grid>
            </Grid>
          </>
        )}
      </Box>

      {/* Sort and Results Count */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedCases.length} of {totalItems} reports
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="sort-label">Sort</InputLabel>
            <Select<"desc" | "asc">
              labelId="sort-label"
              id="sort"
              value={sortOrder}
              label="Sort"
              onChange={(e) =>
                handleSortChange(e.target.value as "desc" | "asc")
              }
            >
              <MenuItem value="desc">
                <Typography variant="body2">Newest First</Typography>
              </MenuItem>
              <MenuItem value="asc">
                <Typography variant="body2">Oldest First</Typography>
              </MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Box>

      {/* Reports List */}
      <Box>
        {isLoading ? (
          <SecurityReportAnalysisSkeleton />
        ) : paginatedCases.length === 0 ? (
          <Box sx={{ textAlign: "center", py: 6 }}>
            <Typography variant="body1" color="text.secondary">
              No reports found.
            </Typography>
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {paginatedCases.map((caseItem) => {
              const StatusIcon = getStatusIcon(caseItem.status?.label);
              const colorPath = getStatusColor(caseItem.status?.label);
              const resolvedColor = resolveColorFromTheme(colorPath, theme);

              return (
                <Form.CardButton
                  key={caseItem.id}
                  onClick={() => handleCaseClick(caseItem)}
                  sx={{
                    p: 3,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "stretch",
                    gap: 1,
                  }}
                >
                  <Form.CardHeader
                    sx={{ p: 0 }}
                    title={
                      <Stack
                        direction="row"
                        spacing={1.5}
                        alignItems="center"
                        sx={{ mb: 1, flexWrap: "wrap" }}
                      >
                        <Typography
                          variant="body2"
                          fontWeight={500}
                          color="text.primary"
                        >
                          {caseItem.number || "--"}
                        </Typography>
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 0.5,
                          }}
                        >
                          <Box
                            sx={{
                              width: 8,
                              height: 8,
                              borderRadius: "50%",
                              bgcolor: getSeverityColor(
                                caseItem.severity?.label,
                              ),
                            }}
                          />
                          <Typography variant="caption" color="text.secondary">
                            {mapSeverityToDisplay(caseItem.severity?.label)}
                          </Typography>
                        </Box>
                        <Chip
                          size="small"
                          variant="outlined"
                          label={caseItem.status?.label || "--"}
                          icon={<StatusIcon size={12} />}
                          sx={{
                            bgcolor: alpha(resolvedColor, 0.1),
                            color: resolvedColor,
                            height: 20,
                            fontSize: "0.75rem",
                            px: 0,
                            "& .MuiChip-icon": {
                              color: "inherit",
                              ml: "6px",
                              mr: "6px",
                            },
                            "& .MuiChip-label": {
                              pl: 0,
                              pr: "6px",
                            },
                          }}
                        />
                        {caseItem.issueType?.label && (
                          <Chip
                            size="small"
                            label={caseItem.issueType.label || "--"}
                            variant="outlined"
                            sx={{
                              height: 20,
                              fontSize: "0.75rem",
                            }}
                          />
                        )}
                      </Stack>
                    }
                  />

                  <Form.CardContent sx={{ p: 0 }}>
                    <Typography
                      variant="h6"
                      color="text.primary"
                      sx={{ mb: 1, fontWeight: 500 }}
                    >
                      {caseItem.title || "--"}
                    </Typography>

                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{
                        mb: 2,
                        display: "-webkit-box",
                        WebkitLineClamp: 2,
                        WebkitBoxOrient: "vertical",
                        overflow: "hidden",
                      }}
                    >
                      {stripHtml(caseItem.description) || "--"}
                    </Typography>
                  </Form.CardContent>

                  <Form.CardActions
                    sx={{
                      p: 0,
                      justifyContent: "flex-start",
                      flexWrap: "wrap",
                      gap: 2,
                    }}
                  >
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 2,
                        flexWrap: "wrap",
                      }}
                    >
                      <Box
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          gap: 0.5,
                          flexShrink: 0,
                        }}
                      >
                        <Calendar size={14} />
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ lineHeight: 1 }}
                        >
                          Created {formatDateTime(caseItem.createdOn) || "--"}
                        </Typography>
                      </Box>
                      {caseItem.createdBy && (
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 0.5,
                            flexShrink: 0,
                          }}
                        >
                          <User size={14} />
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            sx={{ lineHeight: 1 }}
                          >
                            Created by {caseItem.createdBy}
                          </Typography>
                        </Box>
                      )}
                      {(() => {
                        const assignedLabel = getAssignedEngineerLabel(
                          caseItem.assignedEngineer,
                        );
                        return assignedLabel ? (
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "center",
                              gap: 0.5,
                              flexShrink: 0,
                            }}
                          >
                            <User size={14} />
                            <Typography
                              variant="caption"
                              color="text.secondary"
                              sx={{ lineHeight: 1 }}
                            >
                              Assigned to {assignedLabel}
                            </Typography>
                          </Box>
                        ) : null;
                      })()}
                      {caseItem.deployment?.label && (
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 0.5,
                            flexShrink: 0,
                          }}
                        >
                          <FileText size={14} />
                          <Typography
                            variant="caption"
                            color="text.secondary"
                            sx={{ lineHeight: 1 }}
                          >
                            {caseItem.deployment.label}
                          </Typography>
                        </Box>
                      )}
                    </Box>
                  </Form.CardActions>
                </Form.CardButton>
              );
            })}
          </Box>
        )}
      </Box>

      {/* Pagination */}
      {totalPages > 1 && (
        <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
          <Pagination
            count={totalPages}
            page={page}
            onChange={handlePageChange}
            color="primary"
            variant="outlined"
            shape="rounded"
          />
        </Box>
      )}
    </Paper>
  );
};

export default SecurityReportAnalysis;
