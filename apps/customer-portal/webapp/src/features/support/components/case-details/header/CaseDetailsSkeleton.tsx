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

import type {
  CaseDetailsHeaderVariant,
  CaseDetailsSkeletonProps,
} from "@features/support/types/supportComponents";
import { Box, Divider, Paper, Skeleton, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

export type CaseDetailsHeaderSkeletonProps = {
  variant?: CaseDetailsHeaderVariant;
};

/**
 * Header-only skeleton: wso2CaseId | caseNumber | status + optional chips, then title.
 * Used by CaseDetailsHeader when loading and by CaseDetailsSkeleton.
 *
 * @param props - Optional variant matching {@link CaseDetailsHeader}.
 * @returns {JSX.Element} The header skeleton block.
 */
export function CaseDetailsHeaderSkeleton({
  variant = "default",
}: CaseDetailsHeaderSkeletonProps = {}): JSX.Element {
  return (
    <Box>
      {/* Row: wso2CaseId | caseNumber | status [| severity chip] [| assigned engineer] */}
      <Stack
        direction="row"
        spacing={1.5}
        alignItems="center"
        sx={{ mb: 1, flexWrap: "wrap" }}
      >
        {/* wso2CaseId */}
        <Skeleton variant="text" width={80} height={18} />
        <Divider orientation="vertical" flexItem sx={{ height: 14, alignSelf: "center" }} />
        {/* caseNumber */}
        <Skeleton variant="text" width={100} height={20} />
        {/* status dot + label — shown for all variants except engagement */}
        {variant !== "engagement" && (
          <Stack direction="row" spacing={0.75} alignItems="center">
            <Skeleton variant="circular" width={8} height={8} />
            <Skeleton variant="text" width={72} height={16} />
          </Stack>
        )}
        {/* severity chip — default only */}
        {variant === "default" && (
          <Skeleton
            variant="rounded"
            width={56}
            height={20}
            sx={{ borderRadius: "10px" }}
          />
        )}
      </Stack>
      {/* Title */}
      <Skeleton variant="text" width="65%" height={28} sx={{ maxWidth: 420 }} />
    </Box>
  );
}

/**
 * Skeleton placeholder for case details loading: header, action row, and tab bar.
 *
 * @param {CaseDetailsSkeletonProps} props - Optional configuration.
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function CaseDetailsSkeleton({
  hideActionRow = false,
  hideAssignedEngineer = false,
  headerVariant = "default",
}: CaseDetailsSkeletonProps = {}): JSX.Element {
  void hideAssignedEngineer;
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          gap: 2,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <CaseDetailsHeaderSkeleton variant={headerVariant} />
        </Box>

        {/* Action buttons area (right side) */}
        {!hideActionRow && (
          <Stack direction="row" spacing={1} alignItems="center" sx={{ flexShrink: 0 }}>
            <Skeleton variant="rounded" width={88} height={30} sx={{ borderRadius: "4px" }} />
            <Skeleton variant="rounded" width={96} height={30} sx={{ borderRadius: "4px" }} />
          </Stack>
        )}
      </Box>

      {/* Security Report Analysis progress bar — shown instead of action row */}
      {hideActionRow && (
        <Paper
          variant="outlined"
          sx={{
            py: 1.5,
            px: 2,
            bgcolor: "background.default",
          }}
        >
          <Box sx={{ mb: 1 }}>
            <Skeleton variant="text" width="40%" height={20} sx={{ mb: 0.5 }} />
            <Skeleton variant="text" width="20%" height={16} />
          </Box>
          <Skeleton variant="rounded" width="100%" height={8} />
        </Paper>
      )}

      
    </Box>
  );
}
