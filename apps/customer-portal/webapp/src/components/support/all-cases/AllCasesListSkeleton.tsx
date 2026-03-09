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

import { Box, Card, Stack, Skeleton } from "@wso2/oxygen-ui";
import type { JSX } from "react";

export interface AllCasesListSkeletonProps {
  /** When true, renders a compact layout (e.g. for announcement cards). */
  compact?: boolean;
}

/**
 * Component to display loading state for the all cases list.
 *
 * @param {AllCasesListSkeletonProps} props - Optional compact variant for announcement-style cards.
 * @returns {JSX.Element} The rendered skeleton list.
 */
export default function AllCasesListSkeleton({
  compact = false,
}: AllCasesListSkeletonProps = {}): JSX.Element {
  if (compact) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {[1, 2, 3].map((i) => (
          <Card key={i} sx={{ p: 3 }}>
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width={60}
              height={20}
              sx={{ mb: 1 }}
            />
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width="65%"
              height={24}
              sx={{ mb: 1 }}
            />
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width="90%"
              height={20}
              sx={{ mb: 2 }}
            />
            <Stack direction="row" spacing={2}>
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={120}
                height={16}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={80}
                height={16}
              />
            </Stack>
          </Card>
        ))}
      </Box>
    );
  }
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {[1, 2, 3].map((i) => (
        <Card key={i} sx={{ p: 3 }}>
          <Box sx={{ mb: 1 }}>
            <Stack direction="row" spacing={1.5} sx={{ mb: 1 }}>
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={80}
                height={20}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="rounded"
                width={56}
                height={20}
                sx={{ borderRadius: "10px" }}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="rounded"
                width={70}
                height={20}
                sx={{ borderRadius: "10px" }}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="rounded"
                width={60}
                height={20}
                sx={{ borderRadius: "10px" }}
              />
            </Stack>
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width="70%"
              height={32}
              sx={{ mb: 1 }}
            />
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width="90%"
              height={20}
            />
            <Skeleton
              data-testid="Skeleton"
              variant="text"
              width="85%"
              height={20}
            />
            <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={120}
                height={16}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={150}
                height={16}
              />
              <Skeleton
                data-testid="Skeleton"
                variant="text"
                width={100}
                height={16}
              />
            </Stack>
          </Box>
        </Card>
      ))}
    </Box>
  );
}
