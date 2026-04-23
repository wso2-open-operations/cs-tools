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

import {
  Box,
  Card,
  CardContent,
  Divider,
  Skeleton,
  Stack,
} from "@wso2/oxygen-ui";
import { type JSX } from "react";

/**
 * Skeleton placeholder for call requests list loading state.
 * Renders two cards mirroring the structure of call request cards.
 *
 * @returns {JSX.Element} The calls list skeleton.
 */
export default function CallsListSkeleton(): JSX.Element {
  return (
    <Stack spacing={2} data-testid="calls-list-skeleton">
      {[1, 2].map((i) => (
        <Card key={i} variant="outlined">
          <CardContent sx={{ p: 2, "&:last-child": { pb: 2 } }}>
            {/* Header section */}
            <Stack
              direction="row"
              spacing={2}
              alignItems="flex-start"
              sx={{ mb: 2 }}
            >
              <Skeleton variant="rectangular" width={40} height={40} />
              <Box sx={{ flex: 1 }}>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  sx={{ mb: 0.5 }}
                >
                  <Skeleton variant="text" width={80} height={20} />
                  <Skeleton variant="rectangular" width={60} height={20} />
                </Stack>
                <Skeleton variant="text" width={150} height={16} />
              </Box>
            </Stack>

            {/* Details Grid section */}
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr 1fr", sm: "1fr 1fr 1fr" },
                gap: 2,
                mb: 2,
              }}
            >
              {[1, 2, 3].map((j) => (
                <Box key={j}>
                  <Skeleton
                    variant="text"
                    width={60}
                    height={14}
                    sx={{ mb: 0.5 }}
                  />
                  <Skeleton variant="text" width={100} height={20} />
                </Box>
              ))}
            </Box>

            <Divider sx={{ mb: 2 }} />

            {/* Notes section */}
            <Box>
              <Skeleton
                variant="text"
                width={40}
                height={14}
                sx={{ mb: 0.5 }}
              />
              <Skeleton variant="text" width="90%" height={20} />
            </Box>
          </CardContent>
        </Card>
      ))}
    </Stack>
  );
}
