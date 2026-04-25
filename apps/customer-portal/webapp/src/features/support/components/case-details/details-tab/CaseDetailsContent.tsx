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

import type {
  CaseDetailsContentProps,
  CaseDetailsHeaderVariant,
} from "@features/support/types/supportComponents";
import { Box, Paper, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useLocation } from "react-router";
import { consumePendingCaseDetailsTab } from "@features/settings/utils/settingsStorage";
import { useGetCaseAttachments } from "@features/support/api/useGetCaseAttachments";
import { useGetCallRequests } from "@features/support/api/useGetCallRequests";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetCaseComments from "@features/support/api/useGetCaseComments";
import { useConversationRecommendationsSearch } from "@features/support/api/useConversationRecommendationsSearch";
import { buildRecommendationRequestFromCase } from "@features/support/utils/recommendations";
import {
  getStatusColor,
  resolveColorFromTheme,
  getStatusIconElement,
  getAssignedEngineerLabel,
  hasSeverityLabelForChip,
  isSecurityReportAnalysisType,
} from "@features/support/utils/support";
import {
  CALL_SCHEDULABLE_CASE_STATUSES,
  type CaseStatus,
} from "@features/support/constants/supportConstants";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import ApiErrorState from "@components/error/ApiErrorState";
import CaseDetailsBackButton from "@case-details/CaseDetailsBackButton";
import CaseDetailsHeader from "@case-details/CaseDetailsHeader";
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";
import CaseDetailsTabs from "@case-details/CaseDetailsTabs";
import CaseDetailsTabPanels from "@case-details/CaseDetailsTabPanels";
import CaseDetailsSkeleton from "@case-details/CaseDetailsSkeleton";

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
  error,
  caseId,
  onBack,
  onOpenRelatedCase,
  projectId = "",
  hideActionRow = false,
  showEngineerOnly = false,
  isServiceRequest = false,
}: CaseDetailsContentProps): JSX.Element {
  const theme = useTheme();
  const location = useLocation();
  const [activeTab, setActiveTab] = useState(() => {
    const pending = consumePendingCaseDetailsTab();
    return pending !== null ? parseInt(pending, 10) : 0;
  });
  const [focusMode, setFocusMode] = useState(false);

  const isEngagementRoute = location.pathname.includes("/engagements/");
  const isSecurityReportAnalysisRoute = location.pathname.includes(
    "security-report-analysis",
  );

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

  // Derive state keys from filters so the queryKey matches CallsPanel exactly —
  // React Query deduplicates the network call when both components are mounted.
  const { data: projectFilters } = useGetProjectFilters(resolvedProjectId);
  const callRequestStateKeys = useMemo<number[] | undefined>(() => {
    if (!projectFilters?.callRequestStates) return undefined;
    return projectFilters.callRequestStates
      .map((s) => Number(s.id))
      .filter((n) => !Number.isNaN(n));
  }, [projectFilters]);

  const callsQuery = useGetCallRequests(
    resolvedProjectId,
    caseId,
    callRequestStateKeys,
  );
  const callCount =
    callsQuery.data?.pages?.[0]?.totalRecords ??
    callsQuery.data?.pages?.flatMap((p) => p.callRequests ?? []).length ??
    undefined;

  const assignedEngineer = data?.assignedEngineer;
  const assignedEngineerLabel = getAssignedEngineerLabel(assignedEngineer);

  const isSecurityReportAnalysis = isSecurityReportAnalysisType(data?.type);

  const headerVariant: CaseDetailsHeaderVariant = useMemo(() => {
    if (isEngagementRoute) return "engagement";
    if (isServiceRequest) return "serviceRequest";
    return "default";
  }, [isEngagementRoute, isServiceRequest]);

  const hideAssignedEngineer =
    isSecurityReportAnalysis ||
    isSecurityReportAnalysisRoute ||
    isServiceRequest;

  const isCallSchedulingAllowed =
    statusLabel != null &&
    CALL_SCHEDULABLE_CASE_STATUSES.includes(statusLabel as CaseStatus);

  const hideCallsTab = isSecurityReportAnalysis || !isCallSchedulingAllowed;
  const hideKnowledgeBaseTab =
    isSecurityReportAnalysis || isEngagementRoute || isServiceRequest;
  const hideRelatedChangeRequestsTab =
    !isServiceRequest || !data?.changeRequests?.length;

  // Eagerly fetch KB recommendations so the tab count is available on page load.
  // React Query deduplicates the network call when the KB tab component mounts later.
  const { data: kbCommentsData, isLoading: isKbCommentsLoading } =
    useGetCaseComments(
      hideKnowledgeBaseTab ? "" : resolvedProjectId,
      hideKnowledgeBaseTab ? "" : caseId,
      { offset: 0 },
    );
  const kbPayload = useMemo(
    () =>
      hideKnowledgeBaseTab
        ? null
        : buildRecommendationRequestFromCase(
            data,
            kbCommentsData?.comments ?? [],
          ),
    [hideKnowledgeBaseTab, data, kbCommentsData],
  );
  const { data: kbRecData, isLoading: isKbRecLoading } =
    useConversationRecommendationsSearch(
      kbPayload,
      !hideKnowledgeBaseTab && !isKbCommentsLoading && !!kbPayload,
    );
  const knowledgeBaseCount = kbRecData
    ? (kbRecData.recommendations?.length ?? 0)
    : undefined;
  const knowledgeBaseCountLoading =
    !hideKnowledgeBaseTab &&
    (isKbCommentsLoading || (!!kbPayload && isKbRecLoading && !kbRecData));

  const visibleTabs = useMemo(
    () => [
      0,
      1,
      2,
      ...(hideCallsTab ? [] : [3]),
      ...(hideKnowledgeBaseTab ? [] : [4]),
      ...(hideRelatedChangeRequestsTab ? [] : [5]),
    ],
    [hideCallsTab, hideKnowledgeBaseTab, hideRelatedChangeRequestsTab],
  );
  const clampedActiveTab = Math.min(
    activeTab,
    Math.max(0, visibleTabs.length - 1),
  );

  const resolvedPanelIndex = useMemo(() => {
    return visibleTabs[clampedActiveTab] ?? visibleTabs[0] ?? 0;
  }, [visibleTabs, clampedActiveTab]);

  if (isError && error) {
    return (
      <ApiErrorState
        error={error}
        fallbackMessage={
          isServiceRequest
            ? "Failed to load service request details."
            : "Failed to load case details."
        }
      />
    );
  }

  if (isLoading) {
    return (
      <Box>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <CaseDetailsBackButton
            onClick={onBack}
            sx={{ mb: 2, ml: -0.5, alignSelf: "flex-start" }}
          />
          <CaseDetailsSkeleton
            hideActionRow={hideActionRow}
            showEngineerOnly={showEngineerOnly}
            hideAssignedEngineer={hideAssignedEngineer}
            headerVariant={headerVariant}
          />
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
        <CaseDetailsBackButton
          onClick={onBack}
          sx={{
            ml: focusMode ? 2 : -0.5,
            mt: focusMode ? 2 : 0,
            alignSelf: "flex-start",
          }}
        />

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
            <ErrorIndicator
              entityName={
                isServiceRequest ? "service request details" : "case details"
              }
              size="medium"
            />
            <Box>
              <Typography variant="body1" color="error" fontWeight={500}>
                {isServiceRequest
                  ? "Failed to load service request details"
                  : "Failed to load case details"}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Something went wrong while fetching the information.
              </Typography>
            </Box>
          </Box>
        ) : (
          !focusMode && (
            <>
              <Box
                sx={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "flex-start",
                  gap: 2,
                  flexWrap: "wrap",
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <CaseDetailsHeader
                    wso2CaseId={data?.internalId}
                    caseNumber={data?.number}
                    title={data?.title}
                    severityLabel={severityLabel ?? undefined}
                    statusLabel={statusLabel}
                    assignedEngineerLabel={
                      hideAssignedEngineer ? null : assignedEngineerLabel
                    }
                    statusChipIcon={statusChipIcon}
                    statusChipSx={statusChipSx}
                    isLoading={isLoading}
                    showSeverityChip={
                      headerVariant === "default" &&
                      !isSecurityReportAnalysis &&
                      hasSeverityLabelForChip(severityLabel)
                    }
                    showStatusChip={headerVariant !== "engagement"}
                    variant={headerVariant}
                  />
                </Box>

                {!isEngagementRoute && (!hideActionRow || showEngineerOnly) && (
                  <CaseDetailsActionRow
                    assignedEngineer={assignedEngineer}
                    engineerInitials=""
                    statusLabel={statusLabel}
                    closedOn={data?.closedOn}
                    onOpenRelatedCase={onOpenRelatedCase}
                    projectId={resolvedProjectId}
                    caseId={caseId}
                    isLoading={isLoading}
                    showOnlyEngineer={showEngineerOnly}
                    hideAssignedEngineer={hideAssignedEngineer}
                  />
                )}
              </Box>
            </>
          )
        )}

        <CaseDetailsTabs
          focusMode={focusMode}
          value={clampedActiveTab}
          onChange={(_e, newValue) => setActiveTab(newValue)}
          onFocusModeToggle={() => setFocusMode((prev) => !prev)}
          attachmentCount={attachmentCount}
          callCount={callCount}
          hideCallsTab={hideCallsTab}
          hideKnowledgeBaseTab={hideKnowledgeBaseTab}
          knowledgeBaseCount={knowledgeBaseCount}
          knowledgeBaseCountLoading={knowledgeBaseCountLoading}
          hideRelatedChangeRequestsTab={hideRelatedChangeRequestsTab}
        />
      </Paper>

      {/* Content: Activity tab has fixed input bar (no scroll); other tabs scroll */}
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          display: "flex",
          flexDirection: "column",
          overflow: resolvedPanelIndex === 0 ? "hidden" : "auto",
          p: resolvedPanelIndex === 0 ? 0 : 3,
          pt: resolvedPanelIndex === 0 ? 0 : 2,
          WebkitOverflowScrolling: "touch",
        }}
      >
        <CaseDetailsTabPanels
          panelIndex={resolvedPanelIndex}
          caseId={caseId}
          data={data}
          isError={isError}
          error={error}
          projectId={projectId}
          focusMode={focusMode}
          isEngagement={isEngagementRoute}
          isServiceRequest={isServiceRequest}
        />
      </Box>
    </Box>
  );
}
