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

import { Box, Paper, Skeleton, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton matching header + two-column layout (detail cards | workflow).
 *
 * @returns {JSX.Element} Loading skeleton.
 */
export default function ChangeRequestDetailsLoadingSkeleton(): JSX.Element {
  const workflowPaper = (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Box sx={{ mb: 2 }}>
        <Skeleton variant="text" width={250} height={32} />
        <Skeleton variant="text" width="60%" height={20} />
      </Box>
      <Stack spacing={0}>
        {[1, 2, 3, 4, 5].map((i) => (
          <Box key={i} sx={{ display: "flex", gap: 2 }}>
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
              }}
            >
              <Skeleton variant="circular" width={40} height={40} />
              {i < 5 && (
                <Box sx={{ width: 2, height: 64, mt: 0.5 }}>
                  <Skeleton variant="rectangular" width={2} height={64} />
                </Box>
              )}
            </Box>
            <Box sx={{ flex: 1, pb: i < 5 ? 2 : 0 }}>
              <Skeleton variant="text" width="30%" height={20} />
              <Skeleton variant="text" width="60%" height={16} />
            </Box>
          </Box>
        ))}
      </Stack>
    </Paper>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 2,
        }}
      >
        <Skeleton variant="rounded" width={200} height={36} />
        <Skeleton variant="rounded" width={140} height={36} />
      </Box>
      <Paper variant="outlined" sx={{ p: 4 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box sx={{ width: "100%" }}>
            <Box
              sx={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "flex-start",
                flexWrap: "wrap",
                gap: 1,
                mb: 1,
              }}
            >
              <Skeleton variant="text" width="60%" height={40} />
              <Stack direction="row" spacing={1}>
                <Skeleton variant="rounded" width={100} height={24} />
                <Skeleton variant="rounded" width={100} height={24} />
              </Stack>
            </Box>
            <Stack direction="row" spacing={1.5} sx={{ alignItems: "center" }}>
              <Skeleton variant="text" width={100} />
              <Skeleton variant="text" width={150} />
              <Skeleton variant="text" width={200} />
            </Stack>
            <Stack
              direction="row"
              spacing={1}
              sx={{ mt: 1.5, justifyContent: "flex-end" }}
            >
              <Skeleton variant="rounded" width={130} height={32} />
              <Skeleton variant="rounded" width={130} height={32} />
              <Skeleton variant="rounded" width={95} height={32} />
            </Stack>
          </Box>
        </Box>
      </Paper>
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", md: "row" },
          gap: 2,
          flex: 1,
        }}
      >
        <Box
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            gap: 2,
          }}
        >
          {[1, 2, 3, 4].map((i) => (
            <Paper key={i} variant="outlined" sx={{ p: 3 }}>
              <Skeleton
                variant="text"
                width={200}
                height={32}
                sx={{ mb: 2 }}
              />
              <Skeleton variant="text" width="100%" />
              <Skeleton variant="text" width="90%" />
              <Skeleton variant="text" width="95%" />
            </Paper>
          ))}
        </Box>
        <Box sx={{ width: { xs: "100%", md: 400 }, flexShrink: 0 }}>
          {workflowPaper}
        </Box>
      </Box>
    </Box>
  );
}
