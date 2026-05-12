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
  Card,
  CardContent,
  Typography,
  Grid,
  Box,
  StatCard,
  Divider,
  Skeleton,
} from "@wso2/oxygen-ui";
import { Activity } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

import { statItems } from "@features/project-details/constants/projectDetailsConstants";
import type { ProjectStatisticsCardProps } from "@features/project-details/types/projectDetailsComponents";


const StatCardSkeleton = ({ gridSize }: { gridSize: object }): JSX.Element => (
  <Grid size={gridSize} sx={{ display: "flex" }}>
    {/* Mirrors StatCard: outlined Card > CardContent > flex row (icon left, value+label right) */}
    <Box
      sx={{
        display: "flex",
        width: "100%",
        border: 1,
        borderColor: "divider",
        borderRadius: 1,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 2,
          p: 2,
          width: "100%",
        }}
      >
        <Skeleton variant="circular" width={24} height={24} sx={{ flexShrink: 0 }} />
        <Box>
          <Skeleton variant="text" width={40} height={32} sx={{ mb: 0.5 }} />
          <Skeleton variant="text" width={90} height={16} />
        </Box>
      </Box>
    </Box>
  </Grid>
);

const ProjectStatisticsCard = ({
  stats,
  isLoading,
  isError,
  isSidebarOpen = false,
  showDeploymentsStat = true,
  showServiceRequestStat = true,
  showChangeRequestStat = true,
  showSecurityReportStat = true,
}: ProjectStatisticsCardProps): JSX.Element => {
  const gridSize = isSidebarOpen ? { xs: 12, xl: 4 } : { xs: 12, sm: 6, lg: 4 };
  const visibleStats = statItems.filter((s) => {
    if (s.key === "deployments" && !showDeploymentsStat) return false;
    if (s.key === "outstandingServiceRequestCount" && !showServiceRequestStat) return false;
    if (s.key === "outstandingChangeRequestCount" && !showChangeRequestStat) return false;
    if (s.key === "outstandingSraCount" && !showSecurityReportStat) return false;
    return true;
  });
  const isStatLoading = isLoading || (!isError && !stats);
  return (
    <Card sx={{ height: "100%" }}>
      <CardContent sx={{ p: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Activity size={20} />
          <Typography variant="h6">Project Statistics</Typography>
        </Box>
        <Divider sx={{ mb: 3, pb: 2 }} />

        <Grid container spacing={2} sx={{ alignItems: "stretch" }}>
          {isStatLoading
            ? Array.from({ length: visibleStats.length }).map((_, i) => (
                <StatCardSkeleton key={i} gridSize={gridSize} />
              ))
            : visibleStats.map((stat) => {
                const StatIcon = stat.icon;
                return (
                  <Grid size={gridSize} key={stat.label} sx={{ display: "flex" }}>
                    <StatCard
                      label={stat.label}
                      value={isError ? "Error" : (stats?.[stat.key] ?? "--").toString()}
                      icon={<StatIcon size={24} />}
                      iconColor={stat.iconColor}
                      sx={{ width: "100%" }}
                    />
                  </Grid>
                );
              })}
        </Grid>
      </CardContent>
    </Card>
  );
};

export default ProjectStatisticsCard;
