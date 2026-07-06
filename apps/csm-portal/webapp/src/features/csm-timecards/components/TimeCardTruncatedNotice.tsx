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

import { Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface TimeCardTruncatedNoticeProps {
  /** How to narrow the scope, shown after the base message. Defaults to a
   * generic hint for views (like a single case) with no filter to narrow. */
  hint?: string;
}

/**
 * Shown when {@link searchTimeCards} hit its page cap — some cards in scope
 * weren't fetched, so the list above/below may be missing older entries.
 * `role="status"` announces it to assistive tech, since it appears after
 * the initial render once data resolves rather than being present upfront.
 */
export default function TimeCardTruncatedNotice({
  hint = "Narrow the search scope to see everything.",
}: TimeCardTruncatedNoticeProps): JSX.Element {
  return (
    <Typography
      variant="caption"
      role="status"
      sx={{
        display: "block",
        px: 1.5,
        py: 1,
        borderRadius: 1,
        bgcolor: "warning.light",
        color: "warning.dark",
      }}
    >
      Showing a partial result — there are more cards in scope than could be
      loaded. {hint}
    </Typography>
  );
}
