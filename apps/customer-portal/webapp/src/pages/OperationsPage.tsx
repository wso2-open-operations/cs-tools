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
import { Box, Grid, Stack } from "@wso2/oxygen-ui";
import { useNavigate, useParams } from "react-router";
import { FileText } from "@wso2/oxygen-ui-icons-react";
import SupportStatGrid from "@components/common/stat-grid/SupportStatGrid";
import SupportOverviewCard from "@components/support/support-overview-cards/SupportOverviewCard";
import OutstandingCasesList from "@components/support/support-overview-cards/OutstandingCasesList";
import OutstandingChangeRequestsList from "@components/support/support-overview-cards/OutstandingChangeRequestsList";
import {
  OPERATIONS_STAT_CONFIGS,
  OPERATIONS_OVERVIEW_LIST_LIMIT,
  CaseType,
  type OperationsStatKey,
} from "@constants/supportConstants";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetChangeRequests from "@api/useGetChangeRequests";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import { useGetProjectChangeRequestsStats } from "@api/useGetProjectChangeRequestsStats";
import { getProjectPermissions } from "@utils/subscriptionUtils";

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
  const permissions = getProjectPermissions(projectTypeLabel);

  const isServiceRequestEnabled = permissions.hasSR;
  const isChangeRequestEnabled = permissions.hasCR;

  const {
    data: srData,
    isFetching: isSrLoading,
    isError: isSrError,
  } = useGetProjectCases(
    projectId || "",
    {
      filters: { caseTypes: [CaseType.SERVICE_REQUEST] },
      sortBy: { field: "createdOn", order: "desc" },
    },
    { enabled: !!projectId && isServiceRequestEnabled },
  );
  const serviceRequests =
    srData?.pages?.[0]?.cases?.slice(0, OPERATIONS_OVERVIEW_LIST_LIMIT) ?? [];

  const {
    data: crData,
    isLoading: isCrLoading,
    isError: isCrError,
  } = useGetChangeRequests(
    projectId || "",
    {},
    0,
    OPERATIONS_OVERVIEW_LIST_LIMIT,
    { enabled: !!projectId && isChangeRequestEnabled },
  );
  const changeRequests = crData?.changeRequests ?? [];

  const {
    data: srStats,
    isLoading: isSrStatsLoading,
    isError: isSrStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SERVICE_REQUEST],
    enabled: !!projectId && isServiceRequestEnabled,
  });

  const {
    data: crStats,
    isLoading: isCrStatsLoading,
    isError: isCrStatsError,
  } = useGetProjectChangeRequestsStats(projectId || "", {
    enabled: !!projectId && isChangeRequestEnabled,
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

  // Avoid an empty stats flash before project type is known (enables correct queries).
  const stats: Partial<Record<OperationsStatKey, number>> | undefined =
    isProjectLoading || !project
      ? undefined
      : srReady && crReady
        ? {
            ...(isServiceRequestEnabled &&
              activeServiceRequests !== undefined && {
                activeServiceRequests,
              }),
            ...(isChangeRequestEnabled &&
              activeChangeRequests !== undefined && {
                activeChangeRequests,
              }),
            ...(completedThisMonth > 0 && { completedThisMonth }),
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
    if (!permissionsResolved) {
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
  }, [
    permissionsResolved,
    isServiceRequestEnabled,
    isChangeRequestEnabled,
  ]);

  const overviewGridSize =
    isServiceRequestEnabled && isChangeRequestEnabled
      ? { xs: 12, lg: 6 }
      : { xs: 12, lg: 12 };

  const loadingOverviewGridSize = { xs: 12, lg: 6 };

  return (
    <Stack spacing={3}>
      <Box>
        <SupportStatGrid<OperationsStatKey>
          isLoading={
            !permissionsResolved ||
            (stats === undefined &&
              ((isServiceRequestEnabled && isSrStatsLoading) ||
                (isChangeRequestEnabled && isCrStatsLoading)))
          }
          isError={isError}
          entityName="operations"
          stats={stats}
          configs={operationsStatConfigs}
        />
      </Box>
      {!permissionsResolved ? (
        <Grid container spacing={3} sx={{ alignItems: "stretch" }}>
          <Grid size={loadingOverviewGridSize} sx={{ display: "flex" }}>
            <SupportOverviewCard
              title="Service Requests"
              subtitle={`Latest ${OPERATIONS_OVERVIEW_LIST_LIMIT} service requests`}
              icon={FileText}
              iconVariant="orange"
              footerButtons={[]}
            >
              <OutstandingCasesList
                cases={[]}
                isLoading
              />
            </SupportOverviewCard>
          </Grid>
          <Grid size={loadingOverviewGridSize} sx={{ display: "flex" }}>
            <SupportOverviewCard
              title="Change Requests"
              subtitle={`Latest ${OPERATIONS_OVERVIEW_LIST_LIMIT} change requests`}
              icon={FileText}
              iconVariant="blue"
              footerButtons={[]}
            >
              <OutstandingChangeRequestsList
                changeRequests={[]}
                isLoading
              />
            </SupportOverviewCard>
          </Grid>
        </Grid>
      ) : (isServiceRequestEnabled || isChangeRequestEnabled) ? (
        <>
          <Grid container spacing={3} sx={{ alignItems: "stretch" }}>
            {isServiceRequestEnabled && (
              <Grid size={overviewGridSize} sx={{ display: "flex" }}>
                <SupportOverviewCard
                  title="Service Requests"
                  subtitle={`Latest ${OPERATIONS_OVERVIEW_LIST_LIMIT} service requests`}
                  icon={FileText}
                  iconVariant="orange"
                  headerAction={{
                    label: "Create Service Request",
                    onClick: () =>
                      navigate(
                        `/projects/${projectId}/operations/service-requests/create`,
                      ),
                  }}
                  footerButtons={[
                    {
                      label: "View my requests",
                      onClick: () =>
                        navigate(
                          `/projects/${projectId}/operations/service-requests?createdByMe=true`,
                        ),
                    },
                    {
                      label: "View all requests",
                      onClick: () =>
                        navigate(
                          `/projects/${projectId}/operations/service-requests`,
                        ),
                    },
                  ]}
                  isError={isSrError}
                >
                  <OutstandingCasesList
                    cases={serviceRequests}
                    isLoading={isProjectLoading || isSrLoading}
                    onCaseClick={
                      projectId
                        ? (c) =>
                            navigate(
                              `/projects/${projectId}/operations/service-requests/${c.id}`,
                            )
                        : undefined
                    }
                  />
                </SupportOverviewCard>
              </Grid>
            )}
            {isChangeRequestEnabled && (
              <Grid size={overviewGridSize} sx={{ display: "flex" }}>
                <SupportOverviewCard
                  title="Change Requests"
                  subtitle={`Latest ${OPERATIONS_OVERVIEW_LIST_LIMIT} change requests`}
                  icon={FileText}
                  iconVariant="blue"
                  footerButtons={[
                    {
                      label: "View all change requests",
                      onClick: () =>
                        navigate(
                          `/projects/${projectId}/operations/change-requests`,
                        ),
                    },
                  ]}
                  isError={isCrError}
                >
                  <OutstandingChangeRequestsList
                    changeRequests={changeRequests}
                    isLoading={isProjectLoading || isCrLoading}
                    onItemClick={
                      projectId
                        ? (cr) =>
                            navigate(
                              `/projects/${projectId}/operations/change-requests/${cr.id}`,
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
