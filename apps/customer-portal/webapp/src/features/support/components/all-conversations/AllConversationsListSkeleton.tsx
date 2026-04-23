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

import { Box, Form, Stack, Skeleton } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Component to display loading state for the all conversations list.
 *
 * @returns {JSX.Element} The rendered skeleton list.
 */
export default function AllConversationsListSkeleton(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[1, 2, 3].map((i) => (
        <Form.CardButton
          key={i}
          sx={{
            p: 3,
            display: "flex",
            flexDirection: "column",
            alignItems: "stretch",
            gap: 1,
          }}
        >
          <Box sx={{ mb: 1 }}>
            <Stack direction="row" spacing={1.5} sx={{ mb: 1 }}>
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={120}
                height={20}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="rounded"
                width={80}
                height={20}
                sx={{ borderRadius: "10px" }}
              />
            </Stack>
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width="80%"
              height={24}
              sx={{ mb: 1 }}
            />
            <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={100}
                height={16}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={80}
                height={16}
              />
            </Stack>
          </Box>
          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
            <Skeleton variant="text" width={80} height={20} />
          </Box>
        </Form.CardButton>
      ))}
    </Box>
  );
}
