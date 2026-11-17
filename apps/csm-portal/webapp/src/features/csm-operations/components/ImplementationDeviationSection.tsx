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

import { Box, Typography } from "@wso2/oxygen-ui";
import { GitCompare } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { PLAN_RICH_TEXT_SX } from "@features/csm-operations/utils/planRichText";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";

/** The section's heading id, which names the section for assistive tech. */
const TITLE_ID = "cr-plan-implementation-deviation-title";

/**
 * The "Deviation from the implementation plan" on a change request's Plan tab:
 * what the implementation or the review actually did differently from the
 * approved plan. It sits right under the implementation plan, and stands apart
 * from the plain plan sections by a warning-coloured edge and icon, so nobody
 * reading the plan misses it, without turning it into an alert that takes over
 * the page. Renders nothing when there is no deviation.
 */
export default function ImplementationDeviationSection({ html }: { html?: string | null }): JSX.Element | null {
  if (!html || isBlankHtml(html)) return null;
  const safeHtml = sanitizeRichTextHtml(html);
  return (
    <Box
      component="section"
      aria-labelledby={TITLE_ID}
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 0.75,
        border: 1,
        borderColor: "warning.main",
        borderLeftWidth: 4,
        borderRadius: 1,
        px: 2,
        py: 1.5,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Box component="span" aria-hidden sx={{ display: "inline-flex", color: "warning.main" }}>
          <GitCompare size={16} />
        </Box>
        <Typography id={TITLE_ID} variant="subtitle2">
          Deviation from the implementation plan
        </Typography>
      </Box>
      <Box sx={PLAN_RICH_TEXT_SX} dangerouslySetInnerHTML={{ __html: safeHtml }} />
    </Box>
  );
}
