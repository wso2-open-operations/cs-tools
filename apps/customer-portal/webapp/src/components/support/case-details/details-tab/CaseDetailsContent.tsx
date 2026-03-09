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

import { Box, Paper, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useLocation } from "react-router";
import type { CaseDetails } from "@models/responses";
import { useGetCaseAttachments } from "@api/useGetCaseAttachments";
import { useGetCallRequests } from "@api/useGetCallRequests";
import {
  getStatusColor,
  resolveColorFromTheme,
  getStatusIconElement,
  getInitials,
  isSecurityReportAnalysisType,
} from "@utils/support";
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import CaseDetailsBackButton from "@case-details/CaseDetailsBackButton";
import CaseDetailsHeader from "@case-details/CaseDetailsHeader";
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";
import SecurityReportAnalysisHeader from "@case-details/SecurityReportAnalysisHeader";
import CaseDetailsTabs from "@case-details/CaseDetailsTabs";
import CaseDetailsTabPanels from "@case-details/CaseDetailsTabPanels";
import CaseDetailsSkeleton from "@case-details/CaseDetailsSkeleton";

export interface CaseDetailsContentProps {
  data: CaseDetails | undefined;
  isLoading: boolean;
  isError: boolean;
  caseId: string;
  onBack: () => void;
  onOpenRelatedCase?: () => void;
  projectId?: string;
}

/**
 * CaseDetailsContent displays case header and details in a layout similar to the template.
 *
 * @param {CaseDetailsContentProps} props - Data, loading/error state, and callbacks.
 * @returns {JSX.Element} The rendered case details content.
 */
export default function CaseDetailsContent({
  data,
  isLoading,
  isError,
  caseId,
  onBack,
  onOpenRelatedCase,
  projectId = "",
}: CaseDetailsContentProps): JSX.Element {
  const theme = useTheme();
  const location = useLocation();
  const [activeTab, setActiveTab] = useState(0);
  const [focusMode, setFocusMode] = useState(false);

  const isSecurityReportAnalysisUrl =
    /(^|\/)security-report-analysis(\/|$)/.test(location.pathname);

  const statusLabel = data?.status?.label;
  const severityLabel = data?.severity?.label;
  const statusColorPath = getStatusColor(statusLabel ?? undefined);
  const resolvedStatusColor = resolveColorFromTheme(statusColorPath, theme);
  const statusChipIcon = getStatusIconElement(statusLabel, 12);
  const statusChipSx = useMemo(
    () => ({
      bgcolor: alpha(resolvedStatusColor, 0.1),
      color: resolvedStatusColor,
      height: 20,
      fontSize: "0.75rem",
      px: 0,
      "& .MuiChip-icon": {
        color: "inherit",
        ml: "6px",
        mr: "6px",
      },
      "& .MuiChip-label": {
        pl: 0,
        pr: "6px",
      },
    }),
    [resolvedStatusColor],
  );

  const attachmentsQuery = useGetCaseAttachments(caseId);
  const attachmentCount = attachmentsQuery.data?.pages?.[0]?.totalRecords;

  const resolvedProjectId = data?.project?.id ?? projectId;
  const callsQuery = useGetCallRequests(resolvedProjectId, caseId);
  const callCount =
    callsQuery.data?.pages?.[0]?.totalRecords ??
    callsQuery.data?.pages?.flatMap((p) => p.callRequests ?? []).length ??
    0;

  const assignedEngineer = data?.assignedEngineer;
  const engineerInitials = getInitials(assignedEngineer);

  const isSecurityReportAnalysis = isSecurityReportAnalysisType(data?.type);

  if (isLoading) {
    return (
      <Box>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <CaseDetailsBackButton
            onClick={onBack}
            sx={{ mb: 2, ml: -0.5, alignSelf: "flex-start" }}
          />
          <CaseDetailsSkeleton hideActionRow={isSecurityReportAnalysisUrl} />
        </Paper>
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        flex: 1,
        minHeight: 0,
        overflow: "hidden",
      }}
    >
      {/* Page header: static, not scrollable (case-details header) */}
      <Paper
        variant="outlined"
        sx={{
          p: focusMode ? 0 : 2,
          flexShrink: 0,
          zIndex: 10,
          borderRadius: 0,
        }}
      >
        {!focusMode && <CaseDetailsBackButton onClick={onBack} />}

        {isError ? (
          <Box
            sx={{
              mt: 2,
              mb: 1,
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              py: 2,
            }}
          >
            <ErrorIndicator entityName="case details" size="medium" />
            <Box>
              <Typography variant="body1" color="error" fontWeight={500}>
                Failed to load case details
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Something went wrong while fetching the information.
              </Typography>
            </Box>
          </Box>
        ) : (
          !focusMode && (
            <>
              <CaseDetailsHeader
                caseNumber={data?.number}
                title={data?.title}
                severityLabel={severityLabel ?? undefined}
                statusLabel={statusLabel}
                statusChipIcon={statusChipIcon}
                statusChipSx={statusChipSx}
                isLoading={isLoading}
              />

              {isSecurityReportAnalysis ? (
                <SecurityReportAnalysisHeader data={data} />
              ) : (
                <CaseDetailsActionRow
                  assignedEngineer={assignedEngineer}
                  engineerInitials={engineerInitials}
                  statusLabel={statusLabel}
                  closedOn={data?.closedOn}
                  onOpenRelatedCase={onOpenRelatedCase}
                  projectId={resolvedProjectId}
                  caseId={caseId}
                  isLoading={isLoading}
                />
              )}
            </>
          )
        )}

        <CaseDetailsTabs
          focusMode={focusMode}
          value={activeTab}
          onChange={(_e, newValue) => setActiveTab(newValue)}
          onFocusModeToggle={() => setFocusMode((prev) => !prev)}
          attachmentCount={attachmentCount}
          callCount={callCount}
        />
      </Paper>

      {/* Content: Activity tab has fixed input bar (no scroll); other tabs scroll */}
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          mt: 2,
          display: "flex",
          flexDirection: "column",
          overflow: activeTab === 0 ? "hidden" : "auto",
          p: activeTab === 0 ? 0 : 3,
          pt: 0,
          WebkitOverflowScrolling: "touch",
        }}
      >
        <CaseDetailsTabPanels
          activeTab={activeTab}
          caseId={caseId}
          data={data}
          isError={isError}
          projectId={projectId}
          focusMode={focusMode}
        />
      </Box>
    </Box>
  );
}
