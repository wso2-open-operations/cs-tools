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
import ErrorIndicator from "@/components/common/errorIndicator/ErrorIndicator";

import type { ProjectStatsResponse } from "@/models/responses";
import { statItems } from "@/constants/projectDetailsConstants";

interface ProjectStatisticsCardProps {
  stats?: ProjectStatsResponse["projectStats"];
  isLoading?: boolean;
  isError?: boolean;
  isSidebarOpen?: boolean;
}

const ProjectStatisticsCard = ({
  stats,
  isLoading,
  isError,
  isSidebarOpen = false,
}: ProjectStatisticsCardProps): JSX.Element => {
  const gridSize = isSidebarOpen ? { xs: 12, xl: 4 } : { xs: 12, lg: 4 };
  return (
    <Card sx={{ height: "100%" }}>
      <CardContent sx={{ p: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Activity size={20} />
          <Typography variant="h6">Project Statistics</Typography>
        </Box>
        <Divider sx={{ mb: 3, pb: 2 }} />

        <Grid container spacing={2}>
          {statItems.map((stat) => (
            <Grid size={gridSize} key={stat.label}>
              <StatCard
                label={stat.label}
                value={
                  isLoading
                    ? ((
                        <Skeleton variant="text" width="40%" height={24} />
                      ) as any)
                    : isError
                      ? ((<ErrorIndicator entityName={stat.label} />) as any)
                      : (stats?.[stat.key] ?? "--").toString()
                }
                icon={stat.icon}
                iconColor={stat.iconColor}
              />
            </Grid>
          ))}
        </Grid>
      </CardContent>
    </Card>
  );
};

export default ProjectStatisticsCard;
