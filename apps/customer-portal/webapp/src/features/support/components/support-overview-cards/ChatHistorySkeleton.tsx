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
import { SUPPORT_OVERVIEW_CHAT_LIMIT } from "@features/support/constants/supportConstants";

/**
 * Renders a list of loading skeletons for the chat history list.
 *
 * @returns {JSX.Element} The skeleton list.
 */
export default function ChatHistorySkeleton(): JSX.Element {
  return (
    <Box
      sx={{ display: "flex", flexDirection: "column", gap: 1.5, width: "100%" }}
    >
      {Array.from({ length: SUPPORT_OVERVIEW_CHAT_LIMIT }).map((_, index) => (
        <Form.CardButton
          key={`skeleton-${index}`}
          sx={{
            p: 2,
            display: "flex",
            flexDirection: "column",
            alignItems: "stretch",
            gap: 1.5,
            pointerEvents: "none",
          }}
        >
          <Form.CardContent
            sx={{
              p: 0,
              display: "flex",
              flexDirection: "column",
              gap: 0.5,
            }}
          >
            <Box sx={{ display: "flex", gap: 1.5, alignItems: "center" }}>
              <Box sx={{ display: "flex", flexShrink: 0 }}>
                <Skeleton variant="circular" width={20} height={20} />
              </Box>
              <Box sx={{ flexGrow: 1 }}>
                <Skeleton variant="text" width="90%" height={24} />
              </Box>
            </Box>
            <Box sx={{ pl: 4 }}>
              <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                <Skeleton variant="text" width={40} height={16} />
                <Skeleton variant="text" width={4} height={16} />
                <Skeleton variant="text" width={60} height={16} />
              </Box>
            </Box>
          </Form.CardContent>
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Skeleton variant="rounded" width={80} height={20} />
            <Skeleton variant="text" width={60} height={16} />
          </Box>
        </Form.CardButton>
      ))}
    </Box>
  );
}
