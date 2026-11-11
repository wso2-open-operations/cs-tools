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

import { Button, Divider, Tooltip } from "@wso2/oxygen-ui";
import { Bug } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { getReportIssueUrl } from "@config/reportIssueConfig";

/**
 * Carries a visible text label (not just the icon) so the control reads at a
 * glance instead of depending on someone hovering to discover it — reported
 * live that a bare icon tucked among the other header icon buttons went
 * unnoticed. Its own trailing divider sets it apart from the icon-only
 * cluster next to it, rather than reading as one more icon in that group.
 * Renders nothing (divider included) when CSM_PORTAL_REPORT_ISSUE_URL isn't
 * configured, rather than offering a control with nowhere to go.
 */
export default function ReportIssueButton(): JSX.Element | null {
  const reportIssueUrl = getReportIssueUrl();

  if (!reportIssueUrl) return null;

  return (
    <>
      {/* describeChild: adds the title as aria-describedby instead of
          replacing the button's own accessible name ("Report an issue",
          from its visible text) with the tooltip text. */}
      <Tooltip title="Report an internal issue" describeChild>
        <Button
          size="small"
          variant="outlined"
          startIcon={<Bug size={16} />}
          onClick={() =>
            window.open(reportIssueUrl, "_blank", "noopener,noreferrer")
          }
          sx={(theme) => ({
            whiteSpace: "nowrap",
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
          Report an issue
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
