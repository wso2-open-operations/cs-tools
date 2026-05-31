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
  Divider,
  Paper,
  Skeleton,
  alpha,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton loader for the DeploymentCard accordion component.
 *
 * @returns {JSX.Element} The DeploymentCardSkeleton component.
 */
export default function DeploymentCardSkeleton(): JSX.Element {
  return (
    <Paper
      variant="outlined"
      elevation={0}
      sx={{ borderRadius: 1, overflow: "hidden" }}
    >
      {/* Accordion summary skeleton */}
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          px: 3,
          py: 1.5,
          gap: 2,
        }}
      >
        <Box sx={{ flex: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 0.5 }}>
            <Skeleton variant="text" width={160} height={28} />
            <Skeleton variant="rounded" width={72} height={20} />
          </Box>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Skeleton variant="text" width={80} height={18} />
            <Skeleton variant="text" width={200} height={18} />
          </Box>
        </Box>
        <Skeleton variant="circular" width={20} height={20} sx={{ mt: 0.5 }} />
      </Box>

      {/* Accordion details skeleton */}
      <Box sx={{ px: 3, pb: 3, display: "flex", flexDirection: "column", gap: 3 }}>
        <Divider />
        <Box>
          <Skeleton variant="text" width="80%" height={20} />
          <Skeleton variant="text" width="60%" height={20} />
        </Box>
        <Divider />

        {/* Products section */}
        <Box>
          <Box sx={{ display: "flex", justifyContent: "space-between", mb: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Skeleton variant="circular" width={16} height={16} />
              <Skeleton variant="text" width={120} height={24} />
            </Box>
            <Skeleton variant="rounded" width={100} height={32} />
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {[1, 2].map((i) => (
              <Box
                key={i}
                sx={{
                  p: 2,
                  bgcolor: (theme) => alpha(theme.palette.grey[500], 0.05),
                  borderRadius: "8px",
                }}
              >
                <Box sx={{ display: "flex", gap: 1.5, mb: 1.5 }}>
                  <Skeleton variant="rounded" width={20} height={20} />
                  <Skeleton variant="text" width="35%" height={20} />
                  <Skeleton variant="rounded" width={50} height={20} />
                </Box>
                <Skeleton variant="text" width="70%" height={16} sx={{ mb: 1.5 }} />
                <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 1.5 }}>
                  <Skeleton variant="text" width={70} height={16} />
                  <Skeleton variant="text" width={60} height={16} />
                  <Skeleton variant="text" width={100} height={16} />
                  <Skeleton variant="text" width={50} height={16} />
                </Box>
              </Box>
            ))}
          </Box>
        </Box>

        <Divider />

        {/* Documents section */}
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          {[1, 2].map((i) => (
            <Box
              key={i}
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                p: 2,
                bgcolor: (theme) => alpha(theme.palette.grey[500], 0.05),
              }}
            >
              <Box sx={{ display: "flex", gap: 2, flex: 1 }}>
                <Skeleton variant="rounded" width={20} height={20} sx={{ mt: 0.5 }} />
                <Box sx={{ flex: 1 }}>
                  <Box sx={{ display: "flex", gap: 1, mb: 0.5 }}>
                    <Skeleton variant="text" width="40%" height={20} />
                    <Skeleton variant="rounded" width={70} height={20} />
                  </Box>
                  <Box sx={{ display: "flex", gap: 1 }}>
                    <Skeleton variant="text" width={50} height={16} />
                    <Skeleton variant="text" width={90} height={16} />
                  </Box>
                </Box>
              </Box>
              <Box sx={{ display: "flex", gap: 0.25 }}>
                <Skeleton variant="rounded" width={32} height={32} />
                <Skeleton variant="rounded" width={32} height={32} />
              </Box>
            </Box>
          ))}
        </Box>

        <Divider />
        <Box sx={{ display: "flex", gap: 3 }}>
          <Skeleton variant="text" width={140} height={20} />
          <Skeleton variant="text" width={100} height={20} />
        </Box>
      </Box>
    </Paper>
  );
}
