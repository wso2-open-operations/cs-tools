// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { useNavigate } from "react-router-dom";
import { useEffect, useState } from "react";
import {
  Box,
  Typography,
  Card,
  IconButton,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Avatar,
  Chip,
} from "@mui/material";
import { Info, ExternalLink, ArrowUpDown, Plus } from "lucide-react";

import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
} from "recharts";

import { Close } from "@mui/icons-material";
import {

  getActiveCasesData,
  getOutstandingIncidentsData,
} from "@/utils/dashboardUtils";
import StatCard from "@/components/Dashboard/StatCard";
import { Endpoints } from "@/services/endpoints";
import { useGet } from "@/services/useApi";
import type { ProjectMetadataResponse } from "@/types/project-metadata.types";
import type { CaseResponse } from "@/types/support.types";
import { useProject } from "@/context/ProjectContext";
import { useParams } from "@/router";
import { getSeverityColor } from "@/utils/color";
import { formatDate } from "@/utils/dateUtils";

import { EllipsisVerticalIcon, FunnelIcon } from "@/assets/icons/common-icons";
import type { StatsConfig } from "@/types/dashboard.types";
import { getDashboardStatsConfig } from "@/utils/dashboardUtils";

export default function Dashboard() {
  const { sysId } = useParams("/:sysId/dashboard");
  const navigate = useNavigate();

  const { currentProject } = useProject();

  // Pagination state
  const [page, setPage] = useState(1);
  const rowsPerPage = 5;

  // Fetch project metadata
  const {
    data: projectMetadata,
    isLoading,
    isError,
  } = useGet<ProjectMetadataResponse>(
    ["project-metadata", sysId],
    Endpoints.getProjectMetaData(sysId || "")
  );

  // Fetch outstanding cases
  const {
    data: casesData,
    isLoading: casesLoading,
    isError: casesError,
  } = useGet<CaseResponse>(
    ["dashboard-cases", sysId, page],
    Endpoints.getAllCases(sysId || "", (page - 1) * rowsPerPage, rowsPerPage),
    {
      enabled: !!sysId,
    }
  );

  // Scroll to top when component mounts
  useEffect(() => {
    window.scrollTo(0, 0);
  }, []);

  // Transform API data for stats cards
  const statsConfig = projectMetadata
    ? getDashboardStatsConfig(projectMetadata)
    : [];

  // Transform API data for Outstanding Incidents pie chart
  const outstandingIncidentsData = getOutstandingIncidentsData(projectMetadata);

  const totalIncidents = outstandingIncidentsData.reduce(
    (sum, item) => sum + item.value,
    0
  );

  // Transform API data for Active Cases pie chart
  const activeCasesData = getActiveCasesData(projectMetadata);

  const totalActiveCases = activeCasesData.reduce(
    (sum, item) => sum + item.value,
    0
  );

  // Cases trend data - sample values for different case types
  const casesTrendData = [
    { year: "2020", Issue: 12, Feature: 8, Feedback: 5, Outage: 3 },
    { year: "2021", Issue: 15, Feature: 10, Feedback: 7, Outage: 4 },
    { year: "2022", Issue: 18, Feature: 12, Feedback: 9, Outage: 5 },
    { year: "2023", Issue: 22, Feature: 15, Feedback: 11, Outage: 6 },
    { year: "2024", Issue: 28, Feature: 18, Feedback: 14, Outage: 8 },
    { year: "2025", Issue: 32, Feature: 21, Feedback: 16, Outage: 9 },
  ];

  // Get outstanding cases from API
  const outstandingCases = casesData?.cases || [];

  return (
    <Box sx={{ p: 3, maxWidth: "1600px", mx: "auto" }}>
      {/* Header */}
      <Box sx={{ mb: 3 }}>
        <Typography variant="subtitle1" sx={{ color: "#111827", mb: "4px" }}>
          Dashboard
        </Typography>
        <Typography sx={{ fontSize: "0.875rem", color: "grey.600" }}>
          Viewing data for{" "}
          <Box component="span" sx={{ color: "grey.900" }}>
            {currentProject?.name || "Unknown Project"}
          </Box>
        </Typography>
      </Box>

      {/* Welcome Banner */}
      <Card
        sx={{
          background: "linear-gradient(to right, #f97316, #ea580c)",
          borderRadius: "12px",
          color: "white",
          p: "24px",
          mb: 3,
          position: "relative",
          overflow: "hidden",
          boxShadow: 0,
        }}
      >
        <Box sx={{ position: "absolute", inset: 0, opacity: 0.1 }}>
          <Box
            sx={{
              position: "absolute",
              top: 0,
              right: 0,
              width: 256,
              height: 256,
              bgcolor: "white",
              borderRadius: "50%",
              transform: "translate(50%, -50%)",
            }}
          />
          <Box
            sx={{
              position: "absolute",
              bottom: 0,
              right: "33.333%",
              width: 192,
              height: 192,
              bgcolor: "white",
              borderRadius: "50%",
              transform: "translateY(50%)",
            }}
          />
        </Box>
        <Box
          sx={{
            position: "relative",
            display: "flex",
            alignItems: "start",
            justifyContent: "space-between",
            gap: "16px",
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: "8px",
                mb: "8px",
              }}
            >
              <Info size={20} />
              <Typography sx={{ fontSize: "1.25rem", fontWeight: 600 }}>
                Welcome to your Customer Portal
              </Typography>
            </Box>
            <Typography
              sx={{
                color: "rgba(255,255,255,0.9)",
                fontSize: "0.875rem",
                maxWidth: "672px",
                fontWeight: 400,
              }}
            >
              Track and manage your support cases, view system status, and
              access important resources all in one place.
            </Typography>
          </Box>
          <IconButton
            sx={{
              width: 36,
              height: 36,
              borderRadius: "6px",
              color: "white",
              flexShrink: 0,
              "&:hover": { bgcolor: "rgba(255,255,255,0.2)" },
            }}
          >
            <Close sx={{ fontSize: 16 }} />
          </IconButton>
        </Box>
      </Card>

      {/* Stats Cards */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: {
            xs: "1fr",
            md: "repeat(2, 1fr)",
            lg: "repeat(4, 1fr)",
          },
          gap: "16px",
          mb: 3,
        }}
      >
        {statsConfig.map((stat: StatsConfig, index: number) => (
          <StatCard key={index} {...stat} />
        ))}
      </Box>

      {/* Charts Row */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", lg: "repeat(3, 1fr)" },
          gap: "24px",
          mb: 3,
        }}
      >
        {/* Outstanding Incidents */}
        <Card
          sx={{
            borderRadius: "12px",
            border: "1px solid #e5e7eb",
            p: "24px",
            boxShadow: 0,
          }}
        >
          {isLoading ? (
            <Typography sx={{ mb: "16px", color: "#111827" }}>
              Loading...
            </Typography>
          ) : isError ? (
            <Typography sx={{ mb: "16px", color: "#ef4444" }}>
              Error loading data
            </Typography>
          ) : (
            <Typography
              sx={{
                mb: "16px",
                color: "#111827",
                fontSize: "1rem",
                fontWeight: 500,
              }}
            >
              Outstanding incidents
            </Typography>
          )}
          <Box sx={{ position: "relative", height: 240 }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={outstandingIncidentsData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  dataKey="value"
                  stroke="#fff"
                  strokeWidth={2}
                >
                  {outstandingIncidentsData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <Box
              sx={{
                position: "absolute",
                top: "50%",
                left: "50%",
                transform: "translate(-50%, -50%)",
                textAlign: "center",
                pointerEvents: "none",
              }}
            >
              <Typography variant="h4" sx={{ color: "grey.900" }}>
                {totalIncidents}
              </Typography>
              <Typography variant="caption" sx={{ color: "grey.500" }}>
                Total
              </Typography>
            </Box>
          </Box>
          <Box
            sx={{
              mt: "16px",
              display: "flex",
              flexDirection: "column",
              gap: "8px",
            }}
          >
            {outstandingIncidentsData.map((item, index) => (
              <Box
                key={index}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  fontSize: "0.875rem",
                }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Box
                    sx={{
                      width: 12,
                      height: 12,
                      borderRadius: "50%",
                      bgcolor: item.color,
                    }}
                  />
                  <Typography sx={{ color: "grey.600", fontSize: "0.875rem" }}>
                    {item.name}
                  </Typography>
                </Box>
                <Typography sx={{ color: "grey.900", fontSize: "0.875rem" }}>
                  {item.value}
                </Typography>
              </Box>
            ))}
          </Box>
        </Card>

        {/* Active Cases */}
        <Card
          sx={{
            borderRadius: "12px",
            border: "1px solid #e5e7eb",
            p: "24px",
            boxShadow: 0,
          }}
        >
          <Typography
            sx={{
              mb: "16px",
              color: "#111827",
              fontSize: "1rem",
              fontWeight: 500,
            }}
          >
            Active cases
          </Typography>
          <Box sx={{ position: "relative", height: 240 }}>
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={activeCasesData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={80}
                  dataKey="value"
                  stroke="#fff"
                  strokeWidth={2}
                >
                  {activeCasesData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <Box
              sx={{
                position: "absolute",
                top: "50%",
                left: "50%",
                transform: "translate(-50%, -50%)",
                textAlign: "center",
                pointerEvents: "none",
              }}
            >
              <Typography variant="h4" sx={{ color: "grey.900" }}>
                {totalActiveCases}
              </Typography>
              <Typography variant="caption" sx={{ color: "grey.500" }}>
                Total
              </Typography>
            </Box>
          </Box>
          <Box
            sx={{
              mt: "16px",
              display: "flex",
              flexDirection: "column",
              gap: "8px",
            }}
          >
            {activeCasesData.map((item, index) => (
              <Box
                key={index}
                sx={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  fontSize: "0.875rem",
                }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <Box
                    sx={{
                      width: 12,
                      height: 12,
                      borderRadius: "50%",
                      bgcolor: item.color,
                    }}
                  />
                  <Typography sx={{ color: "grey.600", fontSize: "0.875rem" }}>
                    {item.name}
                  </Typography>
                </Box>
                <Typography sx={{ color: "grey.900", fontSize: "0.875rem" }}>
                  {item.value}
                </Typography>
              </Box>
            ))}
          </Box>
        </Card>

        {/* Cases Trend */}
        <Card
          sx={{
            borderRadius: "12px",
            border: "1px solid #e5e7eb",
            p: "24px",
            boxShadow: 0,
          }}
        >
          <Typography
            sx={{
              mb: "16px",
              color: "#111827",
              fontSize: "1rem",
              fontWeight: 500,
            }}
          >
            Cases trend
          </Typography>
          <ResponsiveContainer
            width="100%"
            height={240}
            style={{ marginTop: "48px" }}
          >
            <BarChart data={casesTrendData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
              <XAxis dataKey="year" stroke="#9ca3af" fontSize={12} />
              <YAxis stroke="#9ca3af" fontSize={12} />
              <Bar
                dataKey="Issue"
                stackId="a"
                fill="#ef4444"
                radius={[0, 0, 0, 0]}
              />
              <Bar
                dataKey="Feature"
                stackId="a"
                fill="#6366f1"
                radius={[0, 0, 0, 0]}
              />
              <Bar
                dataKey="Feedback"
                stackId="a"
                fill="#22d3ee"
                radius={[0, 0, 0, 0]}
              />
              <Bar
                dataKey="Outage"
                stackId="a"
                fill="#fb923c"
                radius={[4, 4, 0, 0]}
              />
            </BarChart>
          </ResponsiveContainer>
          <Box
            sx={{
              display: "flex",
              flexWrap: "wrap",
              gap: "12px",
              mt: "16px",
              justifyContent: "center",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: "6px",
                fontSize: "0.75rem",
              }}
            >
              <Box
                sx={{
                  width: 12,
                  height: 12,
                  borderRadius: "2px",
                  bgcolor: "#ef4444",
                }}
              />
              <Typography sx={{ color: "grey.600", fontSize: "0.75rem" }}>
                Issue
              </Typography>
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: "6px",
                fontSize: "0.75rem",
              }}
            >
              <Box
                sx={{
                  width: 12,
                  height: 12,
                  borderRadius: "2px",
                  bgcolor: "#6366f1",
                }}
              />
              <Typography sx={{ color: "grey.600", fontSize: "0.75rem" }}>
                Feature
              </Typography>
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: "6px",
                fontSize: "0.75rem",
              }}
            >
              <Box
                sx={{
                  width: 12,
                  height: 12,
                  borderRadius: "2px",
                  bgcolor: "#22d3ee",
                }}
              />
              <Typography sx={{ color: "grey.600", fontSize: "0.75rem" }}>
                Feedback
              </Typography>
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: "6px",
                fontSize: "0.75rem",
              }}
            >
              <Box
                sx={{
                  width: 12,
                  height: 12,
                  borderRadius: "2px",
                  bgcolor: "#fb923c",
                }}
              />
              <Typography sx={{ color: "grey.600", fontSize: "0.75rem" }}>
                Outage
              </Typography>
            </Box>
          </Box>
        </Card>
      </Box>

      {/* Outstanding Cases Table */}
      <Card
        sx={{
          borderRadius: "12px",
          border: "1px solid #e5e7eb",
          p: "24px",
          boxShadow: 0,
          display: "flex",
          flexDirection: "column",
          gap: "24px",
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Box>
            <Typography
              sx={{
                color: "#111827",
                mb: "4px",
                fontSize: "1rem",
                fontWeight: 500,
              }}
            >
              Outstanding cases
            </Typography>
            <Typography sx={{ fontSize: "0.875rem", color: "#6b7280" }}>
              Manage and track all your open support cases
            </Typography>
          </Box>
          <Box sx={{ display: "flex", gap: "8px" }}>
            <Button
              variant="outlined"
              startIcon={<FunnelIcon width={16} height={16} />}
              sx={{
                height: 32,
                fontSize: "0.875rem",
                borderColor: "#e5e7eb",
                color: "#374151",
                borderRadius: "8px",
                textTransform: "none",
                "&:hover": {
                  bgcolor: "#f9fafb",
                  borderColor: "#e5e7eb",
                },
              }}
            >
              Filter
            </Button>
            <Button
              variant="outlined"
              onClick={() => navigate(`/${sysId}/support/cases`)}
              sx={{
                height: 32,
                fontSize: "0.875rem",
                borderColor: "#e5e7eb",
                color: "#374151",
                borderRadius: "8px",
                textTransform: "none",
                "&:hover": {
                  bgcolor: "#f9fafb",
                  borderColor: "#e5e7eb",
                },
              }}
            >
              All cases
            </Button>
            <Button
              variant="contained"
              startIcon={<Plus size={16} />}
              onClick={() => navigate(`/${sysId}/support/cases/create-case`)}
              sx={{
                height: 32,
                fontSize: "0.875rem",
                borderRadius: "8px",
                bgcolor: "#ea580c",
                textTransform: "none",
                "&:hover": { bgcolor: "#c2410c" },
                boxShadow: 0,
              }}
            >
              Create case
            </Button>
          </Box>
        </Box>

        <TableContainer
          sx={{
            border: "1px solid #e5e7eb",
            borderRadius: "8px",
            overflow: "hidden",
          }}
        >
          <Table>
            <TableHead sx={{ bgcolor: "#f9fafb" }}>
              <TableRow sx={{ "&:hover": { bgcolor: "#f9fafb" } }}>
                <TableCell
                  sx={{
                    width: 140,
                    p: "8px",
                    whiteSpace: "nowrap",
                  }}
                >
                  <Box
                    component="button"
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: "4px",
                      cursor: "pointer",
                      border: "none",
                      bgcolor: "transparent",
                      fontWeight: 500,
                      fontSize: "0.875rem",
                      p: 0,
                      //"&:hover": { color: "#111827" },
                    }}
                  >
                    Created <ArrowUpDown size={12} />
                  </Box>
                </TableCell>
                <TableCell
                  sx={{
                    p: "8px",
                    whiteSpace: "nowrap",
                  }}
                >
                  <Box
                    component="button"
                    sx={{
                      display: "flex",
                      fontWeight: 500,
                      fontSize: "0.875rem",
                      alignItems: "center",
                      gap: "4px",
                      cursor: "pointer",
                      border: "none",
                      bgcolor: "transparent",
                      p: 0,
                      //"&:hover": { color: "#111827" },
                    }}
                  >
                    Case <ArrowUpDown size={12} />
                  </Box>
                </TableCell>
                <TableCell
                  sx={{
                    fontWeight: 500,
                    p: "8px",
                    fontSize: "0.875rem",
                    color: "#111827",
                    whiteSpace: "nowrap",
                  }}
                >
                  Contact
                </TableCell>
                <TableCell
                  sx={{
                    fontWeight: 500,
                    p: "8px",
                    fontSize: "0.875rem",
                    color: "#111827",
                    whiteSpace: "nowrap",
                  }}
                >
                  Assigned to
                </TableCell>
                <TableCell
                  sx={{
                    p: "8px",

                    whiteSpace: "nowrap",
                  }}
                >
                  <Box
                    component="button"
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: "4px",
                      cursor: "pointer",
                      border: "none",
                      bgcolor: "transparent",
                      fontWeight: 500,
                      fontSize: "0.875rem",
                      p: 0,
                      //"&:hover": { color: "#111827" },
                    }}
                  >
                    Priority <ArrowUpDown size={12} />
                  </Box>
                </TableCell>
                <TableCell
                  sx={{
                    fontWeight: 500,
                    p: "8px",
                    fontSize: "0.875rem",
                    color: "#111827",
                    whiteSpace: "nowrap",
                  }}
                >
                  Status
                </TableCell>
                <TableCell
                  sx={{
                    width: 50,
                    p: "8px",
                    fontSize: "0.875rem",
                    whiteSpace: "nowrap",
                  }}
                ></TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {casesLoading ? (
                <TableRow>
                  <TableCell colSpan={7} sx={{ textAlign: "center", p: 3 }}>
                    <Typography sx={{ color: "#6b7280" }}>
                      Loading cases...
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : casesError ? (
                <TableRow>
                  <TableCell colSpan={7} sx={{ textAlign: "center", p: 3 }}>
                    <Typography sx={{ color: "#ef4444" }}>
                      Error loading cases
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : outstandingCases.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} sx={{ textAlign: "center", p: 3 }}>
                    <Typography sx={{ color: "#6b7280" }}>
                      No outstanding cases
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                outstandingCases.map((caseItem, index) => {
                  const severityColors = getSeverityColor(caseItem.severity);
                  const formattedDate = formatDate(caseItem.createdDate);

                  return (
                    <TableRow
                      key={caseItem.sysId || index}
                      sx={{
                        borderBottom: "1px solid #e5e7eb",
                        "&:hover": { bgcolor: "#f9fafb" },
                        "&:last-child": { borderBottom: 0 },
                      }}
                    >
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <Box>
                          <Typography
                            sx={{ fontSize: "0.875rem", color: "#111827" }}
                          >
                            {formattedDate}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <Box
                          sx={{
                            display: "flex",
                            flexDirection: "column",
                            gap: "4px",
                          }}
                        >
                          <Box
                            component="button"
                            onClick={() =>
                              navigate(
                                `/${sysId}/support/cases/${caseItem.sysId}`
                              )
                            }
                            sx={{
                              fontSize: "0.875rem",
                              color: "#111827",
                              display: "flex",
                              alignItems: "center",
                              gap: "8px",
                              cursor: "pointer",
                              border: "none",
                              bgcolor: "transparent",
                              p: 0,
                              textAlign: "left",
                              transition: "color 0.2s",
                              "&:hover": { color: "#ea580c" },
                              "&:hover .external-link-icon": { opacity: 1 },
                            }}
                          >
                            {caseItem.title}
                            <ExternalLink
                              size={12}
                              className="external-link-icon"
                              style={{
                                color: "#9ca3af",
                                opacity: 0,
                                transition: "opacity 0.2s",
                              }}
                            />
                          </Box>
                          <Typography
                            sx={{ fontSize: "0.75rem", color: "#6b7280" }}
                          >
                            ID: {caseItem.number}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <Typography
                          sx={{ fontSize: "0.875rem", color: "#4b5563" }}
                        >
                          -
                        </Typography>
                      </TableCell>
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: "8px",
                          }}
                        >
                          {caseItem.assignee ? (
                            <>
                              <Avatar
                                sx={{
                                  width: 28,
                                  height: 28,
                                  fontSize: "0.75rem",
                                  bgcolor: "#f3f4f6",
                                  color: "#374151",
                                }}
                              >
                                {caseItem.assignee
                                  .replace(/ⓦ/g, "")
                                  .trim()
                                  .split(" ")
                                  .map((n) => n[0])
                                  .join("")
                                  .toUpperCase()}
                              </Avatar>
                              <Typography
                                sx={{ fontSize: "0.875rem", color: "#111827" }}
                              >
                                {caseItem.assignee}
                              </Typography>
                            </>
                          ) : (
                            <Typography
                              sx={{ fontSize: "0.875rem", color: "#6b7280" }}
                            >
                              Unassigned
                            </Typography>
                          )}
                        </Box>
                      </TableCell>
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <Chip
                          label={caseItem.severity}
                          size="small"
                          sx={{
                            fontSize: "0.75rem",
                            height: 20,
                            fontWeight: 500,
                            bgcolor: severityColors.bg,
                            color: severityColors.text,
                            border: "1px solid",
                            borderColor: severityColors.border,
                          }}
                        />
                      </TableCell>
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: "8px",
                          }}
                        >
                          <Box
                            sx={{
                              width: 8,
                              height: 8,
                              borderRadius: "50%",
                              bgcolor:
                                caseItem.status.toLowerCase() === "resolved"
                                  ? "#10b981"
                                  : caseItem.status.toLowerCase() ===
                                    "in progress"
                                  ? "#3b82f6"
                                  : "#f97316",
                            }}
                          />
                          <Typography
                            sx={{ fontSize: "0.875rem", color: "#374151" }}
                          >
                            {caseItem.status}
                          </Typography>
                        </Box>
                      </TableCell>
                      <TableCell sx={{ p: "8px", whiteSpace: "nowrap" }}>
                        <IconButton
                          size="small"
                          sx={{
                            width: 32,
                            height: 32,
                            "&:hover": { bgcolor: "#f9fafb" },
                          }}
                        >
                          <EllipsisVerticalIcon width={16} height={16} />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>

        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <Typography sx={{ fontSize: "0.875rem", color: "#4b5563" }}>
            Showing {(page - 1) * rowsPerPage + 1}-
            {Math.min(
              page * rowsPerPage,
              casesData?.pagination?.totalRecords || 0
            )}{" "}
            of {casesData?.pagination?.totalRecords || 0} cases
          </Typography>
          <Box sx={{ display: "flex", gap: "4px" }}>
            <Button
              variant="outlined"
              size="small"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1 || casesLoading}
              sx={{
                height: 32,
                minWidth: "auto",
                fontSize: "0.875rem",
                textTransform: "none",
                borderRadius: "8px",
                borderColor: "#e5e7eb",
                color: "#374151",
              }}
            >
              Previous
            </Button>

            {[
              ...Array(
                Math.ceil(
                  (casesData?.pagination?.totalRecords || 0) / rowsPerPage
                )
              ),
            ].map((_, i) => {
              const pageNum = i + 1;
              // Show limited page numbers to avoid overcrowding (simple implementation for now)
              if (
                pageNum === 1 ||
                pageNum ===
                  Math.ceil(
                    (casesData?.pagination?.totalRecords || 0) / rowsPerPage
                  ) ||
                (pageNum >= page - 1 && pageNum <= page + 1)
              ) {
                return (
                  <Button
                    key={pageNum}
                    variant="outlined"
                    size="small"
                    onClick={() => setPage(pageNum)}
                    sx={{
                      height: 32,
                      minWidth: "auto",
                      fontSize: "0.875rem",
                      textTransform: "none",
                      borderRadius: "8px",
                      bgcolor: page === pageNum ? "#fff7ed" : "transparent",
                      color: page === pageNum ? "#ea580c" : "#374151",
                      borderColor: page === pageNum ? "#fed7aa" : "#e5e7eb",
                      "&:hover": {
                        bgcolor: page === pageNum ? "#ffedd5" : "#f9fafb",
                        borderColor: page === pageNum ? "#fed7aa" : "#e5e7eb",
                      },
                    }}
                  >
                    {pageNum}
                  </Button>
                );
              }
              if (pageNum === page - 2 || pageNum === page + 2) {
                return (
                  <Typography
                    key={pageNum}
                    sx={{ px: 1, py: 0.5, color: "#9ca3af" }}
                  >
                    ...
                  </Typography>
                );
              }
              return null;
            })}

            <Button
              variant="outlined"
              size="small"
              onClick={() => setPage((p) => p + 1)}
              disabled={
                page >=
                  Math.ceil(
                    (casesData?.pagination?.totalRecords || 0) / rowsPerPage
                  ) || casesLoading
              }
              sx={{
                height: 32,
                minWidth: "auto",
                fontSize: "0.875rem",
                textTransform: "none",
                borderRadius: "8px",
                borderColor: "#e5e7eb",
                color: "#374151",
                "&:hover": {
                  bgcolor: "#f9fafb",
                  borderColor: "#e5e7eb",
                },
              }}
            >
              Next
            </Button>
          </Box>
        </Box>
      </Card>
    </Box>
  );
}
