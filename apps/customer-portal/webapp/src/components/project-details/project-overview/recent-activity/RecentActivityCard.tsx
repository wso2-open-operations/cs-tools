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
  Card,
  CardContent,
  Chip,
  Typography,
  Divider,
  Skeleton,
} from "@wso2/oxygen-ui";
import { Zap } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import ErrorIndicator from "@/components/common/error-indicator/ErrorIndicator";

import {
  getRecentActivityItems,
  type ActivityItem,
} from "@/constants/projectDetailsConstants";
import type { ProjectStatsResponse } from "@/models/responses";

interface RecentActivityCardProps {
  activity?: ProjectStatsResponse["recentActivity"];
  isLoading?: boolean;
  isError?: boolean;
}

const RecentActivityCard = ({
  activity,
  isLoading,
  isError,
}: RecentActivityCardProps): JSX.Element => {
  const activities: ActivityItem[] = getRecentActivityItems(activity);
  return (
    <Card sx={{ height: "100%" }}>
      <CardContent
        sx={{
          p: 3,
          height: "100%",
          display: "flex",
          flexDirection: "column",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Zap size={20} />
          <Typography variant="h6">Recent Activity</Typography>
        </Box>
        <Divider sx={{ mb: 3, pb: 2 }} />

        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            gap: 1.5,
            flex: 1,
            justifyContent: "center",
          }}
        >
          {activities.map((activityItem, index) => (
            <Box
              key={activityItem.label}
              sx={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
              }}
            >
              <Typography variant="body2" color="text.secondary">
                {activityItem.label}
              </Typography>

              {isLoading ? (
                index === 3 ? (
                  <Skeleton variant="rounded" width={80} height={24} />
                ) : (
                  <Skeleton variant="text" width={100} height={24} />
                )
              ) : isError ? (
                <ErrorIndicator entityName={activityItem.label} />
              ) : activityItem.type === "chip" ? (
                <Chip
                  label={activityItem.value}
                  size="small"
                  variant="outlined"
                  color={activityItem.chipColor}
                />
              ) : (
                <Typography variant="body2" color="text.primary">
                  {activityItem.value}
                </Typography>
              )}
            </Box>
          ))}
        </Box>
      </CardContent>
    </Card>
  );
};

export default RecentActivityCard;
