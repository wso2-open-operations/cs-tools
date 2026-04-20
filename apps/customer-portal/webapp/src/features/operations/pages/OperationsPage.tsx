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
import { Box, Grid, Stack, Typography } from "@wso2/oxygen-ui";
import { useNavigate, useParams } from "react-router";
import { FileText } from "@wso2/oxygen-ui-icons-react";
import ListStatGrid from "@components/list-view/ListStatGrid";
import SupportOverviewCard from "@features/support/components/support-overview-cards/SupportOverviewCard";
import { SupportOverviewIconVariant } from "@features/support/types/supportOverview";
import OutstandingCasesList from "@features/support/components/support-overview-cards/OutstandingCasesList";
import OutstandingChangeRequestsList from "@features/operations/components/change-requests/OutstandingChangeRequestsList";
import {
  OPERATIONS_STAT_CONFIGS,
  OPERATIONS_OVERVIEW_LIST_LIMIT,
  CaseType,
  type OperationsStatKey,
} from "@features/support/constants/supportConstants";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetChangeRequests from "@features/operations/api/useGetChangeRequests";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import { useGetProjectChangeRequestsStats } from "@features/dashboard/api/useGetProjectChangeRequestsStats";
import { getProjectPermissions } from "@utils/permission";
import { SortOrder } from "@/types/common";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import {
  OPERATIONS_HUB_CARD_TITLE_CR,
  OPERATIONS_HUB_CARD_TITLE_SR,
  OPERATIONS_HUB_FOOTER_VIEW_ALL_CR,
  OPERATIONS_HUB_FOOTER_VIEW_ALL_SR,
  OPERATIONS_HUB_FOOTER_VIEW_MINE,
  OPERATIONS_HUB_HEADER_ACTION_CREATE_SR,
  OPERATIONS_HUB_PROJECT_ERROR_MESSAGE,
  OPERATIONS_HUB_STAT_ENTITY_NAME,
  ALLOWED_CHANGE_REQUEST_STATE_IDS,
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

  const projectFetchSettled = !isProjectLoading;
  const projectLoadFailed =
    !!projectId &&
    projectFetchSettled &&
    (isProjectDetailsError || project === undefined);
  const permissionsReady =
    projectFetchSettled && !!project && !isProjectDetailsError;

  const projectTypeLabel = permissionsReady ? project?.type?.label : undefined;
  const permissions = getProjectPermissions(projectTypeLabel, {
    hasPdpSubscription: project?.hasPdpSubscription,
  });

  const isServiceRequestEnabled = permissions.hasSR;
  const isChangeRequestEnabled = permissions.hasCR;
  const operationsPath = `/projects/${projectId}/operations`;

  const {
    data: srData,
    isLoading: isSrLoading,
    isError: isSrError,
  } = useGetProjectCases(
    projectId || "",
    {
      filters: { caseTypes: [CaseType.SERVICE_REQUEST] },
      sortBy: { field: "createdOn", order: SortOrder.DESC },
    },
    { enabled: !!projectId && permissionsReady && isServiceRequestEnabled },
  );
  const serviceRequests =
    srData?.pages?.[0]?.cases?.slice(0, OPERATIONS_OVERVIEW_LIST_LIMIT) ?? [];

  const {
    data: crData,
    isLoading: isCrLoading,
    isError: isCrError,
  } = useGetChangeRequests(
    projectId || "",
    {
      filters: {
        stateKeys: [...ALLOWED_CHANGE_REQUEST_STATE_IDS],
      },
    },
    0,
    OPERATIONS_OVERVIEW_LIST_LIMIT,
    { enabled: !!projectId && permissionsReady && isChangeRequestEnabled },
  );
  const changeRequests = crData?.changeRequests ?? [];

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

  const activeServiceRequests = srStats?.activeCount;
  const activeChangeRequests = crStats?.activeCount;

  const scheduledCrCount =
    crStats?.stateCount?.find((s) => s.label === "Scheduled")?.count ?? 0;

  const closedSrCount =
    srStats?.stateCount?.find((s) => s.label === "Closed")?.count ?? 0;
  const closedCrCount =
    crStats?.stateCount?.find((s) => s.label === "Closed")?.count ?? 0;

  const completedThisMonth =
    (isServiceRequestEnabled ? closedSrCount : 0) +
    (isChangeRequestEnabled ? closedCrCount : 0);

  const srReady = !isServiceRequestEnabled || srStats !== undefined;
  const crReady = !isChangeRequestEnabled || crStats !== undefined;

  const stats: Partial<Record<OperationsStatKey, number>> | undefined =
    !permissionsReady
      ? undefined
      : srReady && crReady
        ? {
            ...(isServiceRequestEnabled && {
              activeServiceRequests: activeServiceRequests ?? 0,
            }),
            ...(isChangeRequestEnabled && {
              activeChangeRequests: activeChangeRequests ?? 0,
            }),
            ...((isServiceRequestEnabled || isChangeRequestEnabled) && {
              completedThisMonth,
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
        (isServiceRequestEnabled || c.key !== "activeServiceRequests") &&
        (isChangeRequestEnabled ||
          (c.key !== "activeChangeRequests" && c.key !== "upcomingChanges")),
    );
  }, [permissionsReady, isServiceRequestEnabled, isChangeRequestEnabled]);

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
        />
      </Box>
      {!permissionsReady ? (
        <Grid container spacing={3}>
          <Grid size={loadingOverviewGridSize}>
            <SupportOverviewCard
              title={OPERATIONS_HUB_CARD_TITLE_SR}
              subtitle={srOverviewSubtitle}
              icon={FileText}
              iconVariant={SupportOverviewIconVariant.Orange}
              footerButtons={[]}
            >
              <OutstandingCasesList cases={[]} isLoading />
            </SupportOverviewCard>
          </Grid>
          <Grid size={loadingOverviewGridSize}>
            <SupportOverviewCard
              title={OPERATIONS_HUB_CARD_TITLE_CR}
              subtitle={crOverviewSubtitle}
              icon={FileText}
              iconVariant={SupportOverviewIconVariant.Blue}
              footerButtons={[]}
            >
              <OutstandingChangeRequestsList changeRequests={[]} isLoading />
            </SupportOverviewCard>
          </Grid>
        </Grid>
      ) : isServiceRequestEnabled || isChangeRequestEnabled ? (
        <>
          <Grid container spacing={3}>
            {isServiceRequestEnabled && (
              <Grid size={overviewGridSize}>
                <SupportOverviewCard
                  title={OPERATIONS_HUB_CARD_TITLE_SR}
                  subtitle={srOverviewSubtitle}
                  icon={FileText}
                  iconVariant={SupportOverviewIconVariant.Orange}
                  headerAction={{
                    label: OPERATIONS_HUB_HEADER_ACTION_CREATE_SR,
                    onClick: () =>
                      navigate(
                        `/projects/${projectId}/operations/service-requests/create`,
                      ),
                  }}
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
                  isError={isSrError}
                >
                  <OutstandingCasesList
                    cases={serviceRequests}
                    isLoading={isSrLoading}
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
              <Grid size={overviewGridSize}>
                <SupportOverviewCard
                  title={OPERATIONS_HUB_CARD_TITLE_CR}
                  subtitle={crOverviewSubtitle}
                  icon={FileText}
                  iconVariant={SupportOverviewIconVariant.Blue}
                  footerButtons={[
                    {
                      label: OPERATIONS_HUB_FOOTER_VIEW_ALL_CR,
                      onClick: () =>
                        navigate(
                          `/projects/${projectId}/operations/change-requests`,
                          { state: { returnTo: operationsPath } },
                        ),
                    },
                  ]}
                  isError={isCrError}
                >
                  <OutstandingChangeRequestsList
                    changeRequests={changeRequests}
                    isLoading={isCrLoading}
                    onItemClick={
                      projectId
                        ? (cr) =>
                            navigate(
                              `/projects/${projectId}/operations/change-requests/${cr.id}`,
                              { state: { returnTo: operationsPath } },
                            )
                        : undefined
                    }
                  />
                </SupportOverviewCard>
              </Grid>
            )}
          </Grid>
        </>
      ) : null}
    </Stack>
  );
}
