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

import { Card, Box, Skeleton } from "@wso2/oxygen-ui";
import { type JSX } from "react";

/**
 * TimeTrackingCardSkeleton component displays a skeletal loading state for a TimeTrackingCard.
 *
 * @returns {JSX.Element} The rendered skeleton card.
 */
export default function TimeTrackingCardSkeleton(): JSX.Element {
  return (
    <Card
      sx={{
        p: "20px",
        display: "flex",
        flexDirection: "column",
        gap: "24px",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          mb: "12px",
        }}
      >
        <Box sx={{ flex: 1 }}>
          {/* Badges Skeleton */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: "8px",
              mb: "8px",
            }}
          >
            <Skeleton variant="rounded" width={60} height={20} />
            <Skeleton variant="rounded" width={60} height={20} />
            <Skeleton variant="rounded" width={80} height={20} />
          </Box>

          {/* Description Skeleton */}
          <Skeleton variant="text" width="90%" height={20} sx={{ mb: "8px" }} />

          {/* User/Role/Date Skeleton */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: "16px",
            }}
          >
            <Skeleton variant="text" width={100} height={16} />
            <Skeleton variant="text" width={120} height={16} />
            <Skeleton variant="text" width={80} height={16} />
          </Box>
        </Box>

        {/* Hours Skeleton */}
        <Box sx={{ textAlign: "right" }}>
          <Skeleton variant="text" width={50} height={40} />
        </Box>
      </Box>
    </Card>
  );
}
