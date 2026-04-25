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

import { type JSX, useMemo } from "react";
import { Box, Button, Grid, Stack, Typography } from "@wso2/oxygen-ui";
import { useNavigate, useParams } from "react-router";
import { ArrowRight, FileText } from "@wso2/oxygen-ui-icons-react";
import ListStatGrid from "@components/list-view/ListStatGrid";
import SupportOverviewCard from "@features/support/components/support-overview-cards/SupportOverviewCard";
import { SupportOverviewIconVariant } from "@features/support/types/supportOverview";
import OutstandingCasesList from "@features/support/components/support-overview-cards/OutstandingCasesList";
import type { CaseListItem } from "@features/support/types/cases";
import {
  CaseStatus,
  OPERATIONS_STAT_CONFIGS,
  OPERATIONS_OVERVIEW_LIST_LIMIT,
  CaseType,
  type OperationsStatKey,
} from "@features/support/constants/supportConstants";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetChangeRequests from "@features/operations/api/useGetChangeRequests";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import { useGetProjectChangeRequestsStats } from "@features/dashboard/api/useGetProjectChangeRequestsStats";
import { getProjectPermissions, isProjectRestricted } from "@utils/permission";
import { SortOrder } from "@/types/common";
import { resolveCasesTableDefaultStatusIds } from "@features/dashboard/utils/casesTable";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import {
  ChangeRequestStates,
  OPERATIONS_HUB_CARD_TITLE_CR,
  OPERATIONS_HUB_CARD_TITLE_SR,
  OPERATIONS_HUB_FOOTER_VIEW_ALL_CR,
  OPERATIONS_HUB_FOOTER_VIEW_ALL_SR,
  OPERATIONS_HUB_FOOTER_VIEW_MINE,
  OPERATIONS_HUB_HEADER_ACTION_CREATE_SR,
  OPERATIONS_HUB_PROJECT_ERROR_MESSAGE,
  OPERATIONS_HUB_STAT_ENTITY_NAME,
  OUTSTANDING_CHANGE_REQUEST_STATE_IDS,
} from "@features/operations/constants/operationsConstants";
import {
  formatOperationsOverviewChangeRequestsSubtitle,
  formatOperationsOverviewServiceRequestsSubtitle,
} from "@features/operations/utils/operationsPages";

/**
 * OperationsPage component. Displays operations statistics,
 * Service Request / Change Request cards, and overview lists (last 5 SR and CR).
 *
 * @returns {JSX.Element} The rendered Operations page.
 */
export default function OperationsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const {
    data: project,
    isLoading: isProjectLoading,
    isError: isProjectDetailsError,
  } = useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");

  const projectFetchSettled = !isProjectLoading;
  const projectLoadFailed =
    !!projectId &&
    projectFetchSettled &&
    (isProjectDetailsError || project === undefined);
  const permissionsReady =
    projectFetchSettled &&
    !isProjectFeaturesLoading &&
    projectFeatures !== undefined &&
    !!project &&
    !isProjectDetailsError;

  const projectTypeLabel = permissionsReady ? project?.type?.label : undefined;
  const permissions = getProjectPermissions(projectTypeLabel, {
    projectFeatures,
  });

  const isServiceRequestEnabled = permissions.hasSR;
  const isChangeRequestEnabled = permissions.hasCR;
  const isRestricted = isProjectRestricted(project?.closureState);
  const operationsPath = `/projects/${projectId}/operations`;

  const { data: filterMetadata, isLoading: isFilterMetadataLoading, isError: isFilterMetadataError } =
    useGetProjectFilters(projectId || "");

  const nonClosedStatusIds = useMemo(
    () => resolveCasesTableDefaultStatusIds(filterMetadata?.caseStates),
    [filterMetadata?.caseStates],
  );

  const {
    data: srData,
    isLoading: isSrDataLoading,
    isError: isSrError,
  } = useGetProjectCasesPage(
    projectId || "",
    {
      filters: {
        caseTypes: [CaseType.SERVICE_REQUEST],
        statusIds: nonClosedStatusIds,
      },
      sortBy: { field: "createdOn", order: SortOrder.DESC },
    },
    0,
    OPERATIONS_OVERVIEW_LIST_LIMIT,
    {
      enabled:
        !!projectId &&
        permissionsReady &&
        isServiceRequestEnabled &&
        filterMetadata !== undefined,
    },
  );
  const isSrLoading = isSrDataLoading || isFilterMetadataLoading;
  const combinedIsSrError = isSrError || isFilterMetadataError;
  const serviceRequests = srData?.cases ?? [];

  const {
    data: crData,
    isLoading: isCrLoading,
    isError: isCrError,
  } = useGetChangeRequests(
    projectId || "",
    {
      filters: {
        stateKeys: [...OUTSTANDING_CHANGE_REQUEST_STATE_IDS],
      },
    },
    0,
    OPERATIONS_OVERVIEW_LIST_LIMIT,
    { enabled: !!projectId && permissionsReady && isChangeRequestEnabled },
  );
  const changeRequests = crData?.changeRequests ?? [];

  const changeRequestsAsCases = useMemo<CaseListItem[]>(
    () =>
      changeRequests.map((cr) => ({
        id: cr.id,
        internalId: cr.case?.internalId ?? cr.internalId ?? undefined,
        number: cr.number,
        title: cr.title,
        description: cr.description ?? "",
        assignedEngineer: cr.assignedEngineer,
        project: cr.project ?? { id: "", label: "" },
        issueType: null,
        deployedProduct: cr.deployedProduct,
        deployment: cr.deployment,
        severity: null,
        status: cr.state,
        type: cr.type,
        caseTypes: null,
        createdOn: cr.createdOn,
        updatedOn: cr.updatedOn,
        createdBy: cr.createdBy,
        updatedBy: cr.updatedBy,
      })),
    [changeRequests],
  );

  const {
    data: srStats,
    isLoading: isSrStatsLoading,
    isError: isSrStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SERVICE_REQUEST],
    enabled: !!projectId && permissionsReady && isServiceRequestEnabled,
  });

  const {
    data: crStats,
    isLoading: isCrStatsLoading,
    isError: isCrStatsError,
  } = useGetProjectChangeRequestsStats(projectId || "", {
    enabled: !!projectId && permissionsReady && isChangeRequestEnabled,
  });

  // Action Required SRs = Awaiting Info + Solution Proposed
  const awaitingInfoSrCount =
    srStats?.stateCount?.find((s) => s.label === CaseStatus.AWAITING_INFO)?.count ?? 0;
  const solutionProposedSrCount =
    srStats?.stateCount?.find((s) => s.label === CaseStatus.SOLUTION_PROPOSED)?.count ?? 0;
  const actionRequiredSrCount = awaitingInfoSrCount + solutionProposedSrCount;

  // Outstanding SRs = all SR states except Closed
  const closedSrCount =
    srStats?.stateCount?.find((s) => s.label === CaseStatus.CLOSED)?.count ?? 0;
  const totalSrCount =
    srStats?.stateCount?.reduce((sum, s) => sum + (s.count ?? 0), 0) ?? 0;
  const outstandingSrCount = totalSrCount - closedSrCount;

  // Action Required CRs = Customer Approval + Customer Review
  const customerApprovalCrCount =
    crStats?.stateCount?.find((s) => s.label === ChangeRequestStates.CUSTOMER_APPROVAL)?.count ?? 0;
  const customerReviewCrCount =
    crStats?.stateCount?.find((s) => s.label === ChangeRequestStates.CUSTOMER_REVIEW)?.count ?? 0;
  const actionRequiredCrCount = customerApprovalCrCount + customerReviewCrCount;

  const scheduledCrCount =
    crStats?.stateCount?.find((s) => s.label === ChangeRequestStates.SCHEDULED)?.count ?? 0;

  const srReady = !isServiceRequestEnabled || srStats !== undefined;
  const crReady = !isChangeRequestEnabled || crStats !== undefined;

  const stats: Partial<Record<OperationsStatKey, number>> | undefined =
    !permissionsReady
      ? undefined
      : srReady && crReady
        ? {
            ...(isServiceRequestEnabled && {
              actionRequiredServiceRequests: actionRequiredSrCount,
              outstandingServiceRequests: outstandingSrCount,
            }),
            ...(isChangeRequestEnabled && {
              actionRequiredChangeRequests: actionRequiredCrCount,
            }),
            ...(isChangeRequestEnabled &&
              scheduledCrCount > 0 && {
                upcomingChanges: scheduledCrCount,
              }),
          }
        : undefined;

  const isError =
    (isServiceRequestEnabled && isSrStatsError) ||
    (isChangeRequestEnabled && isCrStatsError);

  const operationsStatConfigs = useMemo(() => {
    if (!permissionsReady) {
      return OPERATIONS_STAT_CONFIGS;
    }
    if (!isServiceRequestEnabled && !isChangeRequestEnabled) {
      return [];
    }

    return OPERATIONS_STAT_CONFIGS.filter(
      (c) =>
        (isServiceRequestEnabled ||
          (c.key !== "actionRequiredServiceRequests" && c.key !== "outstandingServiceRequests")) &&
        (isChangeRequestEnabled ||
          (c.key !== "actionRequiredChangeRequests" && c.key !== "upcomingChanges")),
    );
  }, [permissionsReady, isServiceRequestEnabled, isChangeRequestEnabled]);

  const handleStatClick = (key: OperationsStatKey) => {
    switch (key) {
      case "actionRequiredServiceRequests":
        navigate(`/projects/${projectId}/operations/service-requests`, {
          state: { returnTo: operationsPath, actionRequired: true },
        });
        break;
      case "outstandingServiceRequests":
        navigate(`/projects/${projectId}/operations/service-requests`, {
          state: { returnTo: operationsPath, outstandingOnly: true },
        });
        break;
      case "actionRequiredChangeRequests":
        navigate(`/projects/${projectId}/operations/change-requests`, {
          state: { returnTo: operationsPath, actionRequired: true },
        });
        break;
      case "upcomingChanges":
        navigate(`/projects/${projectId}/operations/change-requests`, {
          state: { returnTo: operationsPath, scheduledOnly: true },
        });
        break;
    }
  };

  const overviewGridSize =
    isServiceRequestEnabled && isChangeRequestEnabled
      ? { xs: 12, lg: 6 }
      : { xs: 12, lg: 12 };

  const loadingOverviewGridSize = { xs: 12, lg: 6 };

  const srOverviewSubtitle = formatOperationsOverviewServiceRequestsSubtitle(
    OPERATIONS_OVERVIEW_LIST_LIMIT,
  );
  const crOverviewSubtitle = formatOperationsOverviewChangeRequestsSubtitle(
    OPERATIONS_OVERVIEW_LIST_LIMIT,
  );

  if (projectLoadFailed) {
    return (
      <Box sx={{ textAlign: "center", py: 6 }}>
        <ErrorIndicator entityName="project" size="medium" />
        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          {OPERATIONS_HUB_PROJECT_ERROR_MESSAGE}
        </Typography>
      </Box>
    );
  }

  return (
    <Stack spacing={3}>
      <Box>
        <ListStatGrid<OperationsStatKey>
          isLoading={
            !permissionsReady ||
            (stats === undefined &&
              ((isServiceRequestEnabled && isSrStatsLoading) ||
                (isChangeRequestEnabled && isCrStatsLoading)))
          }
          isError={isError}
          entityName={OPERATIONS_HUB_STAT_ENTITY_NAME}
          stats={stats}
          configs={operationsStatConfigs}
          onStatClick={handleStatClick}
        />
      </Box>
      {!permissionsReady ? (
        <Grid container spacing={3}>
          <Grid size={loadingOverviewGridSize} sx={{ display: "flex", minWidth: 0 }}>
            <SupportOverviewCard
              sx={{ flex: 1, width: "100%", minWidth: 0 }}
              title={OPERATIONS_HUB_CARD_TITLE_SR}
              subtitle={srOverviewSubtitle}
              icon={FileText}
              iconVariant={SupportOverviewIconVariant.Orange}
              footerButtons={[]}
            >
              <OutstandingCasesList cases={[]} isLoading />
            </SupportOverviewCard>
          </Grid>
          <Grid size={loadingOverviewGridSize} sx={{ display: "flex", minWidth: 0 }}>
            <SupportOverviewCard
              sx={{ flex: 1, width: "100%", minWidth: 0 }}
              title={OPERATIONS_HUB_CARD_TITLE_CR}
              subtitle={crOverviewSubtitle}
              icon={FileText}
              iconVariant={SupportOverviewIconVariant.Blue}
              footerButtons={[]}
            >
              <Box
                sx={{
                  display: "flex",
                  flexDirection: "column",
                  width: "100%",
                  flex: 1,
                  minHeight: 0,
                }}
              >
                <Box sx={{ flex: 1, minHeight: 0 }}>
                  <OutstandingCasesList cases={[]} isLoading />
                </Box>
                {projectId && (
                  <>
                    <Box sx={{ borderTop: 1, borderColor: "divider", mt: 1.5 }} />
                    <Button
                      fullWidth
                      variant="text"
                      color="primary"
                      onClick={() =>
                        navigate(
                          `/projects/${projectId}/operations/change-requests`,
                          { state: { returnTo: operationsPath } },
                        )
                      }
                      endIcon={<ArrowRight size={16} />}
                      sx={{
                        justifyContent: "flex-start",
                        textTransform: "none",
                        fontWeight: 500,
                      }}
                    >
                      {OPERATIONS_HUB_FOOTER_VIEW_ALL_CR}
                    </Button>
                  </>
                )}
              </Box>
            </SupportOverviewCard>
          </Grid>
        </Grid>
      ) : isServiceRequestEnabled || isChangeRequestEnabled ? (
        <>
          <Grid container spacing={3}>
            {isServiceRequestEnabled && (
              <Grid size={overviewGridSize} sx={{ display: "flex", minWidth: 0 }}>
                <SupportOverviewCard
                  sx={{ flex: 1, width: "100%", minWidth: 0 }}
                  title={OPERATIONS_HUB_CARD_TITLE_SR}
                  subtitle={srOverviewSubtitle}
                  icon={FileText}
                  iconVariant={SupportOverviewIconVariant.Orange}
                  headerAction={
                    isRestricted
                      ? undefined
                      : {
                          label: OPERATIONS_HUB_HEADER_ACTION_CREATE_SR,
                          onClick: () =>
                            navigate(
                              `/projects/${projectId}/operations/service-requests/create`,
                            ),
                        }
                  }
                  footerButtons={[
                    {
                      label: OPERATIONS_HUB_FOOTER_VIEW_MINE,
                      onClick: () =>
                        navigate(
                          `/projects/${projectId}/operations/service-requests?createdByMe=true`,
                          { state: { returnTo: operationsPath } },
                        ),
                    },
                    {
                      label: OPERATIONS_HUB_FOOTER_VIEW_ALL_SR,
                      onClick: () =>
                        navigate(
                          `/projects/${projectId}/operations/service-requests`,
                          { state: { returnTo: operationsPath } },
                        ),
                    },
                  ]}
                  isError={combinedIsSrError}
                >
                  <OutstandingCasesList
                    cases={serviceRequests}
                    isLoading={isSrLoading}
                    showInternalId
                    onCaseClick={
                      projectId
                        ? (c) =>
                            navigate(
                              `/projects/${projectId}/operations/service-requests/${c.id}`,
                              { state: { returnTo: operationsPath } },
                            )
                        : undefined
                    }
                  />
                </SupportOverviewCard>
              </Grid>
            )}
            {isChangeRequestEnabled && (
              <Grid size={overviewGridSize} sx={{ display: "flex", minWidth: 0 }}>
                <SupportOverviewCard
                  sx={{ flex: 1, width: "100%", minWidth: 0 }}
                  title={OPERATIONS_HUB_CARD_TITLE_CR}
                  subtitle={crOverviewSubtitle}
                  icon={FileText}
                  iconVariant={SupportOverviewIconVariant.Blue}
                  footerButtons={[]}
                  isError={isCrError}
                >
                  <Box
                    sx={{
                      display: "flex",
                      flexDirection: "column",
                      width: "100%",
                      flex: 1,
                      minHeight: 0,
                    }}
                  >
                    <Box sx={{ flex: 1, minHeight: 0 }}>
                      <OutstandingCasesList
                        cases={changeRequestsAsCases}
                        isLoading={isCrLoading}
                        isError={isCrError}
                        useChangeRequestColors
                        showInternalId
                        onCaseClick={
                          projectId
                            ? (c) =>
                                navigate(
                                  `/projects/${projectId}/operations/change-requests/${c.id}`,
                                  { state: { returnTo: operationsPath } },
                                )
                            : undefined
                        }
                      />
                    </Box>
                    {projectId && !isCrError && (
                      <>
                        <Box sx={{ borderTop: 1, borderColor: "divider", mt: 1.5 }} />
                        <Button
                          fullWidth
                          variant="text"
                          color="primary"
                          onClick={() =>
                            navigate(
                              `/projects/${projectId}/operations/change-requests`,
                              { state: { returnTo: operationsPath } },
                            )
                          }
                          endIcon={<ArrowRight size={16} />}
                          sx={{
                            justifyContent: "flex-start",
                            textTransform: "none",
                            fontWeight: 500,
                          }}
                        >
                          {OPERATIONS_HUB_FOOTER_VIEW_ALL_CR}
                        </Button>
                      </>
                    )}
                  </Box>
                </SupportOverviewCard>
              </Grid>
            )}
          </Grid>
        </>
      ) : null}
    </Stack>
  );
}
