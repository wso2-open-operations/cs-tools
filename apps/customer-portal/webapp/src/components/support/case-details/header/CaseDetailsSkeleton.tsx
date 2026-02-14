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

import { Box, Divider, Paper, Skeleton, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Header-only skeleton: case ID, severity, status chip, and title.
 * Used by CaseDetailsHeader when loading and by CaseDetailsSkeleton.
 *
 * @returns {JSX.Element} The header skeleton block.
 */
export function CaseDetailsHeaderSkeleton(): JSX.Element {
  return (
    <Box>
      <Stack
        direction="row"
        spacing={1.5}
        alignItems="center"
        sx={{ mb: 0.5, flexWrap: "wrap" }}
      >
        <Skeleton variant="text" width={100} height={20} />
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
          <Skeleton variant="circular" width={8} height={8} />
          <Skeleton variant="text" width={28} height={16} />
        </Box>
        <Skeleton variant="rounded" width={72} height={20} />
      </Stack>
      <Skeleton
        variant="text"
        width="70%"
        height={28}
        sx={{ maxWidth: 400 }}
      />
    </Box>
  );
}

/**
 * Skeleton placeholder for case details loading: header and action row only.
 * Sub nav tab items (tabs and tab panel content) are not shown as skeleton.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function CaseDetailsSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <CaseDetailsHeaderSkeleton />

      {/* Action row: avatar, engineer, manage status, buttons */}
      <Paper
        variant="outlined"
        sx={{
          mt: 2,
          mb: 1,
          py: 0.5,
          px: 2,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          flexWrap: "wrap",
          gap: 1,
          bgcolor: "background.default",
          minHeight: 0,
        }}
      >
        <Stack direction="row" spacing={1.5} alignItems="center">
          <Skeleton variant="circular" width={18} height={18} />
          <Box>
            <Skeleton variant="text" width={90} height={14} sx={{ mb: 0.25 }} />
            <Box sx={{ fontSize: "0.7rem", color: "text.secondary" }}>
              Support Engineer
            </Box>
          </Box>
          <Divider orientation="vertical" flexItem />
          <Stack direction="row" spacing={1.5} alignItems="center">
            <Skeleton variant="circular" width={12} height={12} />
            <Skeleton variant="text" width={100} height={14} />
          </Stack>
        </Stack>
      </Paper>
    </Box>
  );
}
