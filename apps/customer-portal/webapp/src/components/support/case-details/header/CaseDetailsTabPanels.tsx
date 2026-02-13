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
import { type JSX } from "react";
import type { CaseDetails } from "@models/responses";
import CaseDetailsActivityPanel from "@case-details-activity/CaseDetailsActivityPanel";
import CaseDetailsAttachmentsPanel from "@case-details-attachments/CaseDetailsAttachmentsPanel";
import CaseDetailsDetailsPanel from "@case-details-details/CaseDetailsDetailsPanel";

export interface CaseDetailsTabPanelsProps {
  activeTab: number;
  caseId: string;
  data?: CaseDetails;
  isError?: boolean;
}

/**
 * Renders the active tab panel for case details (Activity, Details, Attachments, Calls, Knowledge Base).
 *
 * @param {CaseDetailsTabPanelsProps} props - Active tab index, case data, and error state.
 * @returns {JSX.Element | null} The panel content.
 */
export default function CaseDetailsTabPanels({
  activeTab,
  caseId,
  data,
  isError = false,
}: CaseDetailsTabPanelsProps): JSX.Element | null {
  if (activeTab === 0) {
    const projectId = data?.project?.id ?? "";
    if (!projectId) {
      return (
        <Typography variant="body2" color="text.secondary">
          Activity timeline will appear here.
        </Typography>
      );
    }
    return (
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        <CaseDetailsActivityPanel
          projectId={projectId}
          caseId={caseId}
          caseCreatedOn={data?.createdOn}
        />
      </Box>
    );
  }

  if (activeTab === 1) {
    return <CaseDetailsDetailsPanel data={data} isError={isError} />;
  }

  if (activeTab === 2) {
    return <CaseDetailsAttachmentsPanel caseId={caseId} />;
  }

  if (activeTab === 3) {
    return (
      <Typography variant="body2" color="text.secondary">
        Calls will appear here.
      </Typography>
    );
  }

  if (activeTab === 4) {
    return (
      <Typography variant="body2" color="text.secondary">
        Knowledge Base articles will appear here.
      </Typography>
    );
  }

  return null;
}
