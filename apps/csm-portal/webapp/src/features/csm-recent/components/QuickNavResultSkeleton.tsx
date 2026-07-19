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

import { Box, Skeleton, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface QuickNavResultSkeletonProps {
  count?: number;
}

/**
 * Placeholder shown while the case search is in flight — matches
 * {@link QuickNavCaseCard}'s shape so the palette doesn't jump around once
 * real results replace it.
 */
export default function QuickNavResultSkeleton({
  count = 3,
}: QuickNavResultSkeletonProps): JSX.Element {
  return (
    <Stack spacing={1.5}>
      {Array.from({ length: count }, (_, i) => (
        <Box
          key={i}
          sx={{ p: 1.5, borderRadius: 1, border: 1, borderColor: "divider" }}
        >
          <Stack direction="row" spacing={1} sx={{ mb: 1 }}>
            <Skeleton variant="text" width={70} height={20} />
            <Skeleton variant="rounded" width={48} height={20} />
            <Skeleton variant="rounded" width={64} height={20} />
          </Stack>
          <Skeleton variant="text" width="60%" height={20} sx={{ mb: 1 }} />
          <Skeleton variant="text" width={120} height={16} />
        </Box>
      ))}
    </Stack>
  );
}
