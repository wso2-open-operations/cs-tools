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

import { Box, Divider, Skeleton, Stack } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Skeleton placeholder for case details page with divided sections.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function CaseDetailsSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {/* Header section */}
      <Box>
        <Stack direction="row" spacing={1.5} sx={{ mb: 1 }}>
          <Skeleton variant="text" width={120} height={32} />
          <Skeleton variant="rounded" width={80} height={24} />
          <Skeleton variant="rounded" width={50} height={24} />
        </Stack>
        <Skeleton variant="text" width="80%" height={28} />
      </Box>

      <Divider />

      {/* Engineer info card */}
      <Box
        sx={{
          py: 2,
          px: 3,
          bgcolor: "background.default",
          border: 1,
          borderColor: "divider",
          display: "flex",
          alignItems: "center",
          gap: 2,
        }}
      >
        <Skeleton variant="circular" width={28} height={28} />
        <Box sx={{ flex: 1 }}>
          <Skeleton variant="text" width={100} height={20} />
          <Skeleton variant="text" width={80} height={16} />
        </Box>
        <Skeleton variant="text" width={120} height={20} />
      </Box>

      <Divider />

      {/* Details section */}
      <Box sx={{ border: 1, borderColor: "divider", p: 3 }}>
        <Skeleton variant="text" width={100} height={24} sx={{ mb: 2 }} />
        <Divider sx={{ mb: 2 }} />
        <Stack spacing={2}>
          {[1, 2, 3, 4, 5, 6, 7].map((i) => (
            <Box
              key={i}
              sx={{
                display: "flex",
                flexDirection: "row",
                alignItems: "center",
                gap: 2,
              }}
            >
              <Skeleton variant="text" width={120} height={20} />
              <Skeleton variant="text" width={180} height={20} />
            </Box>
          ))}
        </Stack>
      </Box>
    </Box>
  );
}
