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

import { Box, Paper, Skeleton, Stack, Divider } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton matching the CR details header paper, 2-column layout (detail cards | workflow).
 *
 * @returns {JSX.Element} Loading skeleton.
 */
export default function ChangeRequestDetailsLoadingSkeleton(): JSX.Element {
  const workflowPaper = (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Box sx={{ mb: 2 }}>
        <Skeleton variant="text" width={220} height={28} />
        <Skeleton variant="text" width="70%" height={20} />
      </Box>
      <Stack spacing={0}>
        {[1, 2, 3, 4, 5].map((i) => (
          <Box key={i} sx={{ display: "flex", gap: 2 }}>
            <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
              <Skeleton variant="circular" width={40} height={40} />
              {i < 5 && (
                <Box sx={{ width: 2, height: 64, mt: 0.5 }}>
                  <Skeleton variant="rectangular" width={2} height={64} />
                </Box>
              )}
            </Box>
            <Box sx={{ flex: 1, pb: i < 5 ? 2 : 0 }}>
              <Skeleton variant="text" width="35%" height={20} />
              <Skeleton variant="text" width="60%" height={16} />
            </Box>
          </Box>
        ))}
      </Stack>
    </Paper>
  );

  const detailCard = (bodyLines = 3) => (
    <Paper variant="outlined">
      <Box sx={{ px: 3, pt: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Skeleton variant="circular" width={20} height={20} />
          <Skeleton variant="text" width={180} height={28} />
        </Box>
      </Box>
      <Box sx={{ px: 3, py: 3 }}>
        {Array.from({ length: bodyLines }).map((_, i) => (
          <Skeleton key={i} variant="text" width={i === bodyLines - 1 ? "75%" : "100%"} />
        ))}
      </Box>
    </Paper>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Header Paper: title + chips + info row + action buttons */}
      <Paper variant="outlined" sx={{ p: 4 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box sx={{ width: "100%" }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 1,
                mb: 1,
                flexWrap: "wrap",
              }}
            >
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
                <Skeleton variant="text" width={300} height={40} />
                <Skeleton variant="rounded" width={90} height={24} />
                <Skeleton variant="rounded" width={80} height={24} />
              </Box>
            </Box>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 2,
                flexWrap: "wrap",
              }}
            >
              <Stack direction="row" spacing={1.5} sx={{ alignItems: "center" }}>
                <Skeleton variant="text" width={80} />
                <Skeleton variant="text" width={20} />
                <Skeleton variant="text" width={160} />
                <Skeleton variant="text" width={20} />
                <Skeleton variant="text" width={130} />
              </Stack>
              <Stack direction="row" spacing={1}>
                <Skeleton variant="rounded" width={140} height={32} />
                <Skeleton variant="rounded" width={90} height={32} />
                <Skeleton variant="rounded" width={80} height={32} />
              </Stack>
            </Box>
          </Box>
        </Box>
      </Paper>

      {/* 2-column layout */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", md: "row" },
          gap: 2,
          flex: 1,
        }}
      >
        {/* Left column */}
        <Box sx={{ flex: 1, display: "flex", flexDirection: "column", gap: 2 }}>
          {/* Change Description */}
          {detailCard(4)}

          {/* Scheduled Maintenance Window */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Skeleton variant="circular" width={20} height={20} />
                <Skeleton variant="text" width={220} height={28} />
              </Box>
            </Box>
            <Box sx={{ px: 3, py: 3 }}>
              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                  gap: 3,
                }}
              >
                {[1, 2, 3, 4].map((i) => (
                  <Box key={i}>
                    <Skeleton variant="text" width="40%" height={16} sx={{ mb: 0.5 }} />
                    <Skeleton variant="text" width="70%" height={20} />
                  </Box>
                ))}
              </Box>
            </Box>
          </Paper>

          {/* Deployment & Component */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Skeleton variant="circular" width={20} height={20} />
                <Skeleton variant="text" width={200} height={28} />
              </Box>
            </Box>
            <Box sx={{ px: 3, py: 3 }}>
              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                  gap: 3,
                }}
              >
                {[1, 2].map((i) => (
                  <Box key={i}>
                    <Skeleton variant="text" width="40%" height={16} sx={{ mb: 0.5 }} />
                    <Skeleton variant="text" width="70%" height={20} />
                  </Box>
                ))}
              </Box>
            </Box>
          </Paper>

          {/* Service Outage Details */}
          {detailCard(2)}

          {/* Impact Analysis */}
          {detailCard(3)}

          {/* Communication Plan */}
          {detailCard(2)}

          {/* Rollback Plan */}
          {detailCard(2)}

          {/* Test Plan */}
          {detailCard(2)}

          {/* Approval Information */}
          <Paper variant="outlined">
            <Box sx={{ px: 3, pt: 3, pb: 3 }}>
              <Skeleton variant="text" width={180} height={28} sx={{ mb: 2 }} />
              <Stack spacing={1.5}>
                {[1, 2].map((i) => (
                  <Box key={i} sx={{ display: "flex", justifyContent: "space-between" }}>
                    <Skeleton variant="text" width={80} height={20} />
                    <Skeleton variant="text" width={140} height={20} />
                  </Box>
                ))}
                <Divider />
                {[1, 2].map((i) => (
                  <Box key={i} sx={{ display: "flex", justifyContent: "space-between" }}>
                    <Skeleton variant="text" width={90} height={20} />
                    <Skeleton variant="text" width={120} height={20} />
                  </Box>
                ))}
              </Stack>
            </Box>
          </Paper>
        </Box>

        {/* Right column — Workflow */}
        <Box sx={{ width: { xs: "100%", md: 400 }, flexShrink: 0 }}>
          {workflowPaper}
        </Box>
      </Box>
    </Box>
  );
}
