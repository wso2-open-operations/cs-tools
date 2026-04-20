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

import type { CaseDetailsTabPanelsProps } from "@features/support/types/supportComponents";
import { Box, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import CaseDetailsActivityPanel from "@case-details-activity/CaseDetailsActivityPanel";
import CaseDetailsAttachmentsPanel from "@case-details-attachments/CaseDetailsAttachmentsPanel";
import CaseDetailsDetailsPanel from "@case-details-details/CaseDetailsDetailsPanel";
import CallsPanel from "@case-details-calls/CallsPanel";
import CaseKnowledgeBaseRecommendations from "@features/support/components/knowledge-base/CaseKnowledgeBaseRecommendations";

/**
 * Renders the active tab panel for case details (Activity, Details, Attachments, Calls,
 * Knowledge Base with API-driven recommendations).
 *
 * @param {CaseDetailsTabPanelsProps} props - Active tab index, case data, and error state.
 * @returns {JSX.Element | null} The panel content.
 */
export default function CaseDetailsTabPanels({
  panelIndex,
  caseId,
  data,
  isError = false,
  error,
  projectId = "",
  focusMode = false,
  isEngagement = false,
  isServiceRequest = false,
}: CaseDetailsTabPanelsProps): JSX.Element | null {
  switch (panelIndex) {
    case 0: {
      const resolvedProjectId = data?.project?.id ?? projectId;
      if (!resolvedProjectId) {
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
            projectId={resolvedProjectId}
            caseId={caseId}
            conversationId={data?.conversation?.id ?? null}
            caseCreatedOn={data?.createdOn}
            focusMode={focusMode}
            caseStatus={data?.status?.label}
          />
        </Box>
      );
    }
    case 1:
      return (
        <CaseDetailsDetailsPanel
          data={data}
          isError={isError}
          error={error}
          isEngagement={isEngagement}
          isServiceRequest={isServiceRequest}
        />
      );
    case 2:
      return (
        <CaseDetailsAttachmentsPanel
          caseId={caseId}
          isCaseClosed={!!data?.closedOn || data?.status?.label === "Closed"}
        />
      );
    case 3: {
      const resolvedProjectId = data?.project?.id ?? projectId;
      if (!resolvedProjectId) {
        return (
          <Typography variant="body2" color="text.secondary">
            Call requests will appear here.
          </Typography>
        );
      }
      return (
        <CallsPanel
          projectId={resolvedProjectId}
          caseId={caseId}
          error={error}
          isCaseClosed={!!data?.closedOn || data?.status?.label === "Closed"}
          caseStatusLabel={data?.status?.label}
          caseSeverityId={data?.severity?.id}
        />
      );
    }
    case 4: {
      const resolvedProjectId = data?.project?.id ?? projectId ?? "";
      return (
        <CaseKnowledgeBaseRecommendations
          caseId={caseId}
          projectId={resolvedProjectId}
          data={data}
        />
      );
    }
    default:
      return null;
  }
}
