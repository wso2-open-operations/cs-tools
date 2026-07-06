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

import { type JSX } from "react";
import { Card, Skeleton, Stack } from "@wso2/oxygen-ui";

export interface SearchResultSkeletonProps {
  count?: number;
}

/**
 * Skeleton placeholder that matches the shape of a search result card.
 *
 * @param {SearchResultSkeletonProps} props - Number of skeleton cards to render.
 * @returns {JSX.Element} The rendered skeleton cards.
 */
export default function SearchResultSkeleton({
  count = 10,
}: SearchResultSkeletonProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {Array.from({ length: count }, (_, i) => (
        <Card key={i} sx={{ p: 3 }}>
          <Stack direction="row" spacing={1} sx={{ mb: 1 }}>
            <Skeleton variant="text" width={80} height={20} />
            <Skeleton variant="rounded" width={56} height={20} sx={{ borderRadius: "10px" }} />
            <Skeleton variant="rounded" width={70} height={20} sx={{ borderRadius: "10px" }} />
          </Stack>
          <Skeleton variant="text" width="70%" height={28} sx={{ mb: 0.5 }} />
          <Skeleton variant="text" width="90%" height={18} />
          <Skeleton variant="text" width="80%" height={18} sx={{ mb: 1.5 }} />
          <Stack direction="row" spacing={2}>
            <Skeleton variant="text" width={100} height={16} />
            <Skeleton variant="text" width={120} height={16} />
          </Stack>
        </Card>
      ))}
    </Stack>
  );
}
