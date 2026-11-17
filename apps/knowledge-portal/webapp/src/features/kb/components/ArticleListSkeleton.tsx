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
import { VISUALLY_HIDDEN_SX } from "@components/visuallyHidden";

interface ArticleListSkeletonProps {
  rows?: number;
  /** Announced to assistive tech; the placeholders themselves are hidden from it. */
  label?: string;
  compact?: boolean;
}

/** Placeholder rows shaped like article cards, so the page does not jump when results arrive. */
export default function ArticleListSkeleton({
  rows = 5,
  label = "Loading articles",
  compact,
}: ArticleListSkeletonProps): JSX.Element {
  return (
    <Stack role="status" spacing={1.5}>
      <Box component="span" sx={VISUALLY_HIDDEN_SX}>
        {label}
      </Box>
      {Array.from({ length: rows }).map((_, index) => (
        <Skeleton key={index} variant="rounded" height={compact ? 56 : 108} sx={{ borderRadius: 2 }} aria-hidden />
      ))}
    </Stack>
  );
}
