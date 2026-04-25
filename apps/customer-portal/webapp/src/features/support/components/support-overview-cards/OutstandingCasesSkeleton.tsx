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

import { SUPPORT_OVERVIEW_CASES_LIMIT } from "@features/support/constants/supportConstants";
import { Box, Form, Skeleton, Stack } from "@wso2/oxygen-ui";
import { type JSX } from "react";

/**
 * Renders a list of loading skeletons for the outstanding cases list.
 *
 * @returns {JSX.Element} The skeleton list.
 */
export default function OutstandingCasesSkeleton(): JSX.Element {
  return (
    <Box
      sx={{ display: "flex", flexDirection: "column", gap: 1.5, width: "100%" }}
    >
      {Array.from({ length: SUPPORT_OVERVIEW_CASES_LIMIT }).map((_, index) => (
        <Form.CardButton
          key={`skeleton-${index}`}
          sx={{
            p: 2,
            display: "flex",
            flexDirection: "column",
            alignItems: "stretch",
            gap: 1,
          }}
        >
          {/* Header: internalId | number | status dot + label */}
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
            <Skeleton variant="text" width={72} height={18} />
            <Skeleton variant="text" width={8} height={18} />
            <Skeleton variant="text" width={90} height={18} />
            <Stack direction="row" spacing={0.75} alignItems="center">
              <Skeleton variant="circular" width={8} height={8} />
              <Skeleton variant="text" width={64} height={16} />
            </Stack>
          </Stack>

          {/* Content: title + description */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
            <Skeleton variant="text" width="85%" height={20} />
            <Skeleton variant="text" width="95%" height={16} />
            <Skeleton variant="text" width="70%" height={16} />
          </Box>

          {/* Footer: assigned engineer + relative time */}
          <Stack direction="row" justifyContent="flex-end" alignItems="center" spacing={1.5}>
            <Skeleton variant="text" width={110} height={14} />
            <Skeleton variant="text" width={70} height={14} />
          </Stack>
        </Form.CardButton>
      ))}
    </Box>
  );
}
