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

import { Box, Skeleton, Card } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton loading component for Change Requests list.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function ChangeRequestsListSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[...Array(5)].map((_, index) => (
        <Card
          key={index}
          sx={{
            p: 3,
            display: "flex",
            flexDirection: "row",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 3,
          }}
        >
          {/* Right Section - Status badges */}
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              gap: 1,
              flexShrink: 0,
              order: 2,
            }}
          >
            <Skeleton variant="rounded" width={60} height={22} />
            <Skeleton variant="rounded" width={80} height={22} />
          </Box>

          {/* Left Section */}
          <Box sx={{ flex: 1, minWidth: 0, order: 1 }}>
            {/* Title */}
            <Box sx={{ mb: 1.5 }}>
              <Skeleton variant="text" width="60%" height={24} />
            </Box>

            {/* Details row */}
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1.5,
                mb: 1.5,
              }}
            >
              <Skeleton variant="text" width={100} height={20} />
              <Skeleton variant="text" width={8} height={20} />
              <Skeleton variant="text" width={90} height={20} />
              <Skeleton variant="text" width={8} height={20} />
              <Skeleton variant="text" width={120} height={20} />
            </Box>

            {/* Date/Time row */}
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1.5,
              }}
            >
              <Skeleton variant="text" width={150} height={20} />
              <Skeleton variant="text" width={8} height={20} />
              <Skeleton variant="text" width={150} height={20} />
              <Skeleton variant="rounded" width={50} height={20} />
            </Box>
          </Box>
        </Card>
      ))}
    </Box>
  );
}
