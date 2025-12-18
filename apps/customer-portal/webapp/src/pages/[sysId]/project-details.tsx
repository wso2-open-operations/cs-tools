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

import { Box, Typography, Tabs, Tab, Chip } from "@mui/material";
import { useState, useEffect } from "react";
import {
  FolderIcon,
  BuildingIcon,
  ActivityIcon,
  ZapIcon,
} from "../../assets/icons/project-details/project-details-icons";
import UsersIcon from "../../assets/icons/common/users-icon";
import CrownIcon from "../../assets/icons/common/crown-icon";
import {
  InfoIcon,
  ServerIcon,
  ClockIcon,
} from "../../assets/icons/support/support-icons";
import projectDetailsData from "../../data/project-details-mock.json";

import { useProject } from "../../context/ProjectContext";

export default function ProjectDetails() {
  const { currentProject } = useProject();
  const [activeTab, setActiveTab] = useState(0);

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setActiveTab(newValue);
  };

  // Scroll to top when component mounts
  useEffect(() => {
    window.scrollTo(0, 0);
  }, []);

  return (
    <>
      {/* Header Section */}
      <Box
        sx={{
          bgcolor: "white",
          borderBottom: 1,
          borderColor: "grey.200",
          px: 4, // Matches px-8 (32px)
          py: 3, // Matches py-6 (24px)
        }}
      >
        <Box display="flex" alignItems="center" gap={1.5} mb={1}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              p: 1,
              bgcolor: "#ffedd5",
              borderRadius: "8px",
              color: "#ea580c",
            }}
          >
            <FolderIcon width="24px" height="24px" color="currentColor" />
          </Box>
          <Box>
            <Typography
              variant="h6"
              component="h1"
              sx={{
                color: "grey.900",
                fontWeight: 600,
                lineHeight: 1.2,
                fontSize: "1.25rem",
              }}
            >
              Project Details
            </Typography>
            <Typography variant="body2" color="grey.500" fontSize="0.875rem">
              Project information for{" "}
              {currentProject?.name || "Unknown Project"}
            </Typography>
          </Box>
        </Box>
      </Box>

      {/* Content Body */}
      <Box sx={{ flex: 1, px: 4, py: 3 }}>
        {/* Custom "Pill" Tabs */}
        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          sx={{
            height: 36, // h-9
            minHeight: 36,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            width: "fit-content",
            bgcolor: "#f4f4f5", // bg-muted
            borderRadius: "12px", // rounded-xl
            p: "3px", // p-[3px]
            mb: 3,

            "& .MuiTabs-indicator": {
              display: "none",
            },
            "& .MuiTabs-flexContainer": {
              gap: 0, // Tailwind has no gap here
              height: "100%",
            },
          }}
        >
          {/* Overview */}
          <Tab
            disableRipple
            icon={<InfoIcon width={16} height={16} />}
            iconPosition="start"
            label="Overview"
            sx={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "8px", // gap-2
              padding: "4px 8px", // px-2 py-1
              height: "calc(100% - 1px)",
              minHeight: "unset",
              minWidth: "unset",
              borderRadius: "12px",
              textTransform: "none",
              fontSize: "0.875rem", // text-sm
              fontWeight: 500,
              border: "1px solid transparent",
              color: "#71717a", // text-muted-foreground
              transition: "color 0.2s, box-shadow 0.2s",

              "&.Mui-selected": {
                bgcolor: "white", // bg-card
                color: "#09090b", // text-foreground
                boxShadow: "0 1px 2px rgba(0,0,0,0.05)", // shadow-sm
              },

              "& .MuiTab-iconWrapper": {
                marginBottom: "0 !important",
              },
            }}
          />

          {/* Deployments */}
          <Tab
            disableRipple
            icon={<ServerIcon width={16} height={16} />}
            iconPosition="start"
            label="Deployments"
            sx={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "8px",
              padding: "4px 8px",
              height: "calc(100% - 1px)",
              minHeight: "unset",
              minWidth: "unset",
              borderRadius: "12px",
              textTransform: "none",
              fontSize: "0.875rem",
              fontWeight: 500,
              border: "1px solid transparent",
              color: "#71717a",
              transition: "color 0.2s, box-shadow 0.2s",

              "&.Mui-selected": {
                bgcolor: "white",
                color: "#09090b",
                boxShadow: "0 1px 2px rgba(0,0,0,0.05)",
              },

              "& .MuiTab-iconWrapper": {
                marginBottom: "0 !important",
              },
            }}
          />

          {/* Time Tracking */}
          <Tab
            disableRipple
            icon={<ClockIcon width={16} height={16} />}
            iconPosition="start"
            label="Time Tracking"
            sx={{
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "8px",
              padding: "4px 8px",
              height: "calc(100% - 1px)",
              minHeight: "unset",
              minWidth: "unset",
              borderRadius: "12px",
              textTransform: "none",
              fontSize: "0.875rem",
              fontWeight: 500,
              border: "1px solid transparent",
              color: "#71717a",
              transition: "color 0.2s, box-shadow 0.2s",

              "&.Mui-selected": {
                bgcolor: "white",
                color: "#09090b",
                boxShadow: "0 1px 2px rgba(0,0,0,0.05)",
              },

              "& .MuiTab-iconWrapper": {
                marginBottom: "0 !important",
              },
            }}
          />
        </Tabs>
        {/* Overview Tab Content */}
        {activeTab === 0 && (
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "repeat(2, 1fr)" },
              gap: 3, // gap-6 (24px)
            }}
          >
            {/* Project Information Card */}
            <Box
              sx={{
                bgcolor: "white",
                borderRadius: "12px", // rounded-xl
                border: 1,
                borderColor: "grey.200",
                p: 3, // p-6 (24px)
              }}
            >
              <Box display="flex" alignItems="center" gap={1} mb={2}>
                <BuildingIcon
                  width="20px"
                  height="20px"
                  color="#4b5563" // text-gray-600
                />
                <Typography
                  variant="h6"
                  color="grey.900"
                  fontSize="1.125rem"
                  fontWeight={600}
                >
                  Project Information
                </Typography>
              </Box>

              <Box
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 1.5,
                }}
              >
                <Box>
                  <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                    Project Name
                  </Typography>
                  <Typography fontSize="0.875rem" color="grey.900">
                    {currentProject?.name || "N/A"}
                  </Typography>
                </Box>

                <Box>
                  <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                    Project Key
                  </Typography>
                  <Typography
                    fontSize="0.875rem"
                    color="grey.900"
                    fontFamily="monospace"
                  >
                    {currentProject?.projectKey || "N/A"}
                  </Typography>
                </Box>

                <Box>
                  <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                    Description
                  </Typography>
                  <Typography fontSize="0.875rem" color="grey.900">
                    {currentProject?.description || "N/A"}
                  </Typography>
                </Box>

                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: "repeat(2, 1fr)",
                    gap: 1.5,
                  }}
                >
                  <Box>
                    <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                      Project Type
                    </Typography>
                    <Chip
                      label={projectDetailsData.projectType}
                      size="small"
                      sx={{
                        bgcolor: "#dbeafe", // blue-100
                        color: "#1d4ed8", // blue-700
                        fontSize: "0.75rem",
                        height: "20px",
                        borderRadius: "6px",
                        fontWeight: 500,
                      }}
                    />
                  </Box>
                  <Box>
                    <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                      Support Tier
                    </Typography>
                    <Chip
                      icon={<CrownIcon width="12px" height="12px" />}
                      label={projectDetailsData.supportTier}
                      size="small"
                      sx={{
                        bgcolor: "#f3e8ff", // purple-100
                        color: "#7c3aed", // purple-700
                        fontSize: "0.75rem",
                        height: "20px",
                        borderRadius: "6px",
                        fontWeight: 500,
                        "& .MuiChip-icon": {
                          color: "inherit",
                          marginLeft: "4px",
                        },
                      }}
                    />
                  </Box>
                </Box>

                <Box>
                  <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                    Created Date
                  </Typography>
                  <Typography fontSize="0.875rem" color="grey.900">
                    {currentProject?.createdOn || "N/A"}
                  </Typography>
                </Box>

                <Box>
                  <Typography fontSize="0.75rem" color="grey.500" mb={0.5}>
                    SLA Status
                  </Typography>
                  <Chip
                    label={projectDetailsData.slaStatus}
                    size="small"
                    sx={{
                      bgcolor: "#dcfce7", // green-100
                      color: "#15803d", // green-700
                      fontSize: "0.75rem",
                      height: "20px",
                      borderRadius: "6px",
                      fontWeight: 500,
                    }}
                  />
                </Box>

                <Box sx={{ pt: 1.5, borderTop: 1, borderColor: "grey.200" }}>
                  <Box display="flex" justifyContent="space-between" mb={1}>
                    <Typography fontSize="0.75rem" color="grey.500">
                      Subscription Period
                    </Typography>
                    <Chip
                      label={projectDetailsData.subscriptionPeriod.status}
                      size="small"
                      sx={{
                        bgcolor: "#fee2e2", // red-100
                        color: "#991b1b", // red-700
                        fontSize: "0.75rem",
                        height: "20px",
                        borderRadius: "6px",
                        fontWeight: 500,
                      }}
                    />
                  </Box>

                  <Box sx={{ mb: 1 }}>
                    <Box
                      sx={{
                        height: 8,
                        bgcolor: "grey.200",
                        borderRadius: "99px",
                        overflow: "hidden",
                      }}
                    >
                      <Box
                        sx={{
                          height: "100%",
                          bgcolor: "#ef4444",
                          width: `${projectDetailsData.subscriptionPeriod.progress}%`,
                          transition: "width 0.3s",
                        }}
                      />
                    </Box>
                  </Box>

                  <Box
                    display="flex"
                    justifyContent="space-between"
                    fontSize="0.75rem"
                  >
                    <Box>
                      <Typography fontSize="0.75rem" color="grey.400" mb={0.25}>
                        Start
                      </Typography>
                      <Typography fontSize="0.75rem" color="grey.900">
                        {projectDetailsData.subscriptionPeriod.start}
                      </Typography>
                    </Box>
                    <Box textAlign="center">
                      <Typography fontSize="0.75rem" color="grey.400" mb={0.25}>
                        Remaining
                      </Typography>
                      <Typography
                        fontSize="0.75rem"
                        fontWeight={500}
                        color="#b91c1c"
                      >
                        {projectDetailsData.subscriptionPeriod.remaining}
                      </Typography>
                    </Box>
                    <Box textAlign="right">
                      <Typography fontSize="0.75rem" color="grey.400" mb={0.25}>
                        End
                      </Typography>
                      <Typography fontSize="0.75rem" color="grey.900">
                        {projectDetailsData.subscriptionPeriod.end}
                      </Typography>
                    </Box>
                  </Box>
                </Box>
              </Box>
            </Box>

            {/* Project Statistics Card */}
            <Box
              sx={{
                bgcolor: "white",
                borderRadius: "12px",
                border: 1,
                borderColor: "grey.200",
                p: 3,
              }}
            >
              <Box display="flex" alignItems="center" gap={1} mb={2}>
                <ActivityIcon width="20px" height="20px" color="#4b5563" />
                <Typography
                  variant="h6"
                  color="grey.900"
                  fontSize="1.125rem"
                  fontWeight={600}
                >
                  Project Statistics
                </Typography>
              </Box>

              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: "repeat(3, 1fr)",
                  gap: 2,
                }}
              >
                <Box sx={{ bgcolor: "#eff6ff", p: 2, borderRadius: "8px" }}>
                  <Typography fontSize="0.75rem" color="grey.600" mb={0.5}>
                    Open Cases
                  </Typography>
                  <Typography variant="h4" color="grey.900" fontSize="1.5rem">
                    {projectDetailsData.statistics.openCases}
                  </Typography>
                </Box>
                <Box sx={{ bgcolor: "#f0fdf4", p: 2, borderRadius: "8px" }}>
                  <Typography fontSize="0.75rem" color="grey.600" mb={0.5}>
                    Active Chats
                  </Typography>
                  <Typography variant="h4" color="grey.900" fontSize="1.5rem">
                    {projectDetailsData.statistics.activeChats}
                  </Typography>
                </Box>
                <Box sx={{ bgcolor: "#fff7ed", p: 2, borderRadius: "8px" }}>
                  <Typography fontSize="0.75rem" color="grey.600" mb={0.5}>
                    Deployments
                  </Typography>
                  <Typography variant="h4" color="grey.900" fontSize="1.5rem">
                    {projectDetailsData.statistics.deployments}
                  </Typography>
                </Box>
              </Box>
            </Box>

            {/* Contact Information Card */}
            <Box
              sx={{
                bgcolor: "white",
                borderRadius: "12px",
                border: 1,
                borderColor: "grey.200",
                p: 3,
              }}
            >
              <Box display="flex" alignItems="center" gap={1} mb={2}>
                <UsersIcon width="20px" height="20px" color="#4b5563" />
                <Typography
                  variant="h6"
                  color="grey.900"
                  fontSize="1.125rem"
                  fontWeight={600}
                >
                  Contact Information
                </Typography>
              </Box>

              <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
                <Box>
                  <Typography fontSize="0.75rem" color="grey.500" mb={1}>
                    Account Manager
                  </Typography>
                  <Box display="flex" alignItems="center" gap={1.5}>
                    <Box
                      sx={{
                        width: 40,
                        height: 40,
                        borderRadius: "50%",
                        background:
                          "linear-gradient(to bottom right, #3b82f6, #2563eb)",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        color: "white",
                      }}
                    >
                      <UsersIcon width="20px" height="20px" color="white" />
                    </Box>
                    <Typography
                      component="a"
                      href={`mailto:${projectDetailsData.contacts.accountManager}`}
                      fontSize="0.875rem"
                      color="grey.900"
                      sx={{
                        textDecoration: "none",
                        "&:hover": { color: "primary.main" },
                      }}
                    >
                      {projectDetailsData.contacts.accountManager}
                    </Typography>
                  </Box>
                </Box>

                <Box sx={{ pt: 1.5, borderTop: 1, borderColor: "grey.200" }}>
                  <Typography fontSize="0.75rem" color="grey.500" mb={1}>
                    Technical Owner
                  </Typography>
                  <Box display="flex" alignItems="center" gap={1.5}>
                    <Box
                      sx={{
                        width: 40,
                        height: 40,
                        borderRadius: "50%",
                        background:
                          "linear-gradient(to bottom right, #a855f7, #9333ea)",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        color: "white",
                      }}
                    >
                      {/* Using placeholder icon for shield as it wasn't in imports */}
                      <UsersIcon width="20px" height="20px" color="white" />
                    </Box>
                    <Typography
                      component="a"
                      href={`mailto:${projectDetailsData.contacts.technicalOwner}`}
                      fontSize="0.875rem"
                      color="grey.900"
                      sx={{
                        textDecoration: "none",
                        "&:hover": { color: "primary.main" },
                      }}
                    >
                      {projectDetailsData.contacts.technicalOwner}
                    </Typography>
                  </Box>
                </Box>
              </Box>
            </Box>

            {/* Recent Activity Card */}
            <Box
              sx={{
                bgcolor: "white",
                borderRadius: "12px",
                border: 1,
                borderColor: "grey.200",
                p: 3,
              }}
            >
              <Box display="flex" alignItems="center" gap={1} mb={2}>
                <ZapIcon width="20px" height="20px" color="#4b5563" />
                <Typography
                  variant="h6"
                  color="grey.900"
                  fontSize="1.125rem"
                  fontWeight={600}
                >
                  Recent Activity
                </Typography>
              </Box>

              <Box
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 1.5,
                }}
              >
                <Box display="flex" justifyContent="space-between">
                  <Typography fontSize="0.875rem" color="grey.600">
                    Total Time Logged
                  </Typography>
                  <Typography fontSize="0.875rem" color="grey.900">
                    {projectDetailsData.activity.totalTimeLogged}
                  </Typography>
                </Box>
                <Box display="flex" justifyContent="space-between">
                  <Typography fontSize="0.875rem" color="grey.600">
                    Billable Hours
                  </Typography>
                  <Typography fontSize="0.875rem" color="grey.900">
                    {projectDetailsData.activity.billableHours}
                  </Typography>
                </Box>
                <Box display="flex" justifyContent="space-between">
                  <Typography fontSize="0.875rem" color="grey.600">
                    Last Deployment
                  </Typography>
                  <Typography fontSize="0.875rem" color="grey.900">
                    {projectDetailsData.activity.lastDeployment}
                  </Typography>
                </Box>
                <Box display="flex" justifyContent="space-between">
                  <Typography fontSize="0.875rem" color="grey.600">
                    System Health
                  </Typography>
                  <Chip
                    label={projectDetailsData.activity.systemHealth}
                    size="small"
                    sx={{
                      bgcolor: "#dcfce7",
                      color: "#15803d",
                      fontSize: "0.75rem",
                      height: "20px",
                      borderRadius: "6px",
                      fontWeight: 500,
                    }}
                  />
                </Box>
              </Box>
            </Box>
          </Box>
        )}
      </Box>
    </>
  );
}
