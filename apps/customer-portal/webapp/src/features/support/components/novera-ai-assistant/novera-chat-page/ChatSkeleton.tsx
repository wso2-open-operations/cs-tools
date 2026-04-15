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

import { Box, Skeleton } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * ChatSkeleton component displays loading skeletons for chat messages.
 *
 * @returns {JSX.Element} The rendered ChatSkeleton.
 */
export default function ChatSkeleton(): JSX.Element {
  return (
    <Box
      sx={{
        flex: 1,
        overflowY: "auto",
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 3,
      }}
    >
      {/* Bot message skeleton */}
      <Box sx={{ display: "flex", justifyContent: "flex-start" }}>
        <Box sx={{ maxWidth: "75%" }}>
          <Skeleton variant="rounded" width={300} height={80} />
          <Skeleton variant="text" width={100} sx={{ mt: 0.5 }} />
        </Box>
      </Box>

      {/* User message skeleton */}
      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <Box sx={{ maxWidth: "75%" }}>
          <Skeleton variant="rounded" width={250} height={60} />
          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
            <Skeleton variant="text" width={100} sx={{ mt: 0.5 }} />
          </Box>
        </Box>
      </Box>

      {/* Bot message skeleton */}
      <Box sx={{ display: "flex", justifyContent: "flex-start" }}>
        <Box sx={{ maxWidth: "75%" }}>
          <Skeleton variant="rounded" width={320} height={100} />
          <Skeleton variant="text" width={100} sx={{ mt: 0.5 }} />
        </Box>
      </Box>

      {/* User message skeleton */}
      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <Box sx={{ maxWidth: "75%" }}>
          <Skeleton variant="rounded" width={280} height={70} />
          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
            <Skeleton variant="text" width={100} sx={{ mt: 0.5 }} />
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
