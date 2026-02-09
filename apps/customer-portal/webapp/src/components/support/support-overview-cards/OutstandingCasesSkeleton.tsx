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

import { Box, Form, Skeleton } from "@wso2/oxygen-ui";
import { type JSX } from "react";

/**
 * Renders a list of loading skeletons for the outstanding cases list.
 *
 * @returns {JSX.Element} The skeleton list.
 */
export default function OutstandingCasesSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      {Array.from({ length: 5 }).map((_, index) => (
        <Form.CardButton
          key={`skeleton-${index}`}
          sx={{
            p: 2,
            display: "flex",
            flexDirection: "column",
            alignItems: "stretch",
            gap: 1.5,
            cursor: "default",
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Skeleton variant="text" width={60} height={20} />
            <Skeleton variant="circular" width={8} height={8} />
            <Skeleton variant="text" width={40} height={16} />
          </Box>
          <Skeleton variant="text" width="90%" height={24} />
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Skeleton variant="rounded" width={80} height={20} />
            <Skeleton variant="text" width={100} height={16} />
          </Box>
        </Form.CardButton>
      ))}
    </Box>
  );
}
