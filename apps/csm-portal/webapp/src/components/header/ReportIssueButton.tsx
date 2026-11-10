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

import { Button, Divider } from "@wso2/oxygen-ui";
import { Bug } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useGetAppConfig } from "@api/useGetAppConfig";

/**
 * Carries a visible text label (not just the icon) so the control reads at a
 * glance instead of depending on someone hovering to discover it — reported
 * live that a bare icon tucked among the other header icon buttons went
 * unnoticed. Its own trailing divider sets it apart from the icon-only
 * cluster next to it, rather than reading as one more icon in that group.
 * Renders nothing (divider included) while the URL hasn't loaded yet or
 * isn't configured (CSM_PORTAL_REPORT_ISSUE_URL unset on the backend),
 * rather than offering a control with nowhere to go.
 */
export default function ReportIssueButton(): JSX.Element | null {
  const { data } = useGetAppConfig();
  const reportIssueUrl = data?.reportIssueUrl;

  if (!reportIssueUrl) return null;

  return (
    <>
      <Button
        size="small"
        variant="outlined"
        color="inherit"
        startIcon={<Bug size={16} />}
        onClick={() =>
          window.open(reportIssueUrl, "_blank", "noopener,noreferrer")
        }
        sx={{ whiteSpace: "nowrap" }}
      >
        Report an issue
      </Button>
      <Divider
        orientation="vertical"
        flexItem
        sx={{ mx: 1, display: { xs: "none", sm: "block" } }}
      />
    </>
  );
}
