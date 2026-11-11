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

import { Box, Button, Divider, Tooltip } from "@wso2/oxygen-ui";
import { Bug } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { getReportIssueUrl } from "@config/reportIssueConfig";

/**
 * Carries a visible text label at `sm` and up (not just the icon) so the
 * control reads at a glance instead of depending on someone hovering to
 * discover it — reported live that a bare icon tucked among the other
 * header icon buttons went unnoticed. The label is hidden below `sm` (same
 * breakpoint as QuickNav's own search label) so this fixed-width control
 * doesn't push the profile control out of a narrow header; an explicit
 * `aria-label` keeps the accessible name "Report an issue" at both widths,
 * since a visually hidden label contributes nothing to it. Its own trailing
 * divider sets it apart from the icon-only cluster next to it, rather than
 * reading as one more icon in that group. Renders nothing (divider
 * included) when CSM_PORTAL_REPORT_ISSUE_URL isn't configured, rather than
 * offering a control with nowhere to go.
 */
export default function ReportIssueButton(): JSX.Element | null {
  const reportIssueUrl = getReportIssueUrl();

  if (!reportIssueUrl) return null;

  return (
    <>
      {/* describeChild: adds the title as aria-describedby instead of
          overriding the explicit aria-label below with the tooltip text. */}
      <Tooltip title="Report an internal issue" describeChild>
        <Button
          size="small"
          variant="outlined"
          aria-label="Report an issue"
          startIcon={<Bug size={16} />}
          onClick={() =>
            window.open(reportIssueUrl, "_blank", "noopener,noreferrer")
          }
          sx={(theme) => ({
            whiteSpace: "nowrap",
            // No min-width floor of its own below `sm` — the label (below)
            // disappears there, so there's nothing forcing this wider than
            // the icon-only controls beside it, same as QuickNav's own
            // search label (`display: { xs: "none", sm: "block" }`).
            color: theme.palette.common.white,
            borderColor: "rgba(255, 255, 255, 0.5)",
            "&:hover": {
              // Same as the base state on purpose — hovering should only
              // surface the tooltip above, not change the button's color.
              color: theme.palette.common.white,
              borderColor: "rgba(255, 255, 255, 0.5)",
              backgroundColor: "transparent",
            },
          })}
        >
          {/* Hidden below `sm` so this control stays icon-only at narrow
              widths — the same breakpoint QuickNav's own search label
              already uses — rather than keeping a fixed text width that can
              push the profile control out of the header. */}
          <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>
            Report an issue
          </Box>
        </Button>
      </Tooltip>
      <Divider
        orientation="vertical"
        flexItem
        sx={{ mx: 1, display: { xs: "none", sm: "block" } }}
      />
    </>
  );
}
