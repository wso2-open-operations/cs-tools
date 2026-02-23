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

/**
 * Skeleton loader for the ServiceRequestsList component.
 *
 * @returns {JSX.Element} The rendered skeleton.
 */
export default function ServiceRequestsListSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[1, 2, 3, 4, 5].map((i) => (
        <Box
          key={i}
          sx={{
            p: 3,
            bgcolor: "background.paper",
            display: "flex",
            flexDirection: "column",
            gap: 1.5,
          }}
        >
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Stack direction="row" spacing={1.5} alignItems="center">
              <Skeleton variant="text" width={80} height={24} />
              <Skeleton variant="rectangular" width={90} height={22} />
              <Skeleton variant="rectangular" width={120} height={22} />
            </Stack>
            <Skeleton variant="text" width={60} height={20} />
          </Box>

          <Skeleton variant="text" width="40%" height={28} />
          <Skeleton variant="text" width="100%" height={20} />
          <Skeleton variant="text" width="80%" height={20} />

          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              mt: 1,
            }}
          >
            <Stack direction="row" spacing={3}>
              <Skeleton variant="text" width={100} height={20} />
              <Skeleton variant="text" width={120} height={20} />
              <Skeleton variant="text" width={100} height={20} />
            </Stack>
            <Skeleton variant="text" width={90} height={20} />
          </Box>
        </Box>
      ))}
    </Box>
  );
}
