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

import { Box, Grid } from "@wso2/oxygen-ui";
import { useParams } from "react-router";
import { useEffect, useRef, useMemo, type JSX } from "react";
import { useAsgardeo } from "@asgardeo/react";
import { useLogger } from "@hooks/useLogger";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import useInfiniteProjects, { flattenProjectPages } from "@api/useGetProjects";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { isForbiddenError, getForbiddenMessage } from "@utils/ApiError";
import Error403Page from "@components/error/Error403Page";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import { useGetProjectChangeRequestsStats } from "@features/dashboard/api/useGetProjectChangeRequestsStats";
import {
  DASHBOARD_STATS,
  SEVERITY_API_LABELS,
} from "@/features/dashboard/constants/dashboard";
import { OperationsChartMode } from "@/features/dashboard/types/charts";
import { TrendDirection, TrendColor } from "@features/dashboard/types/stats";
import { CaseType } from "@features/support/constants/supportConstants";
import {
  calculateProjectStats,
  getProjectPermissions,
  getProjectSeverityPolicy,
} from "@utils/permission";
import { StatCard } from "@features/dashboard/components/stats/StatCard";
import ChartLayout from "@features/dashboard/components/charts/ChartLayout";
import CasesTable from "@features/dashboard/components/cases-table/CasesTable";
import {
  computeCrCardIsCardError,
  computeCrCardIsCardLoading,
  getAllCoreFailedState,
  getDashboardChartsLoadingState,
  normalizeEngagementLabel,
} from "@features/dashboard/utils/dashboard";

/**
 * DashboardPage component to display project-specific statistics and overview.
 *
 * @returns {JSX.Element} The rendered Dashboard page.
 */
export default function DashboardPage(): JSX.Element {
  // logger
  const logger = useLogger();
  // project id
  const { projectId } = useParams<{ projectId: string }>();

  // loader
  const { showLoader, hideLoader } = useLoader();

  // auth
  const { isLoading: isAuthLoading } = useAsgardeo();

  // project list
  const {
    data: projectsData,
    isLoading: isProjectsLoading,
    error: projectsError,
  } = useInfiniteProjects({ pageSize: 20, enabled: !!projectId });

  // project list
  const projects = useMemo(
    () => flattenProjectPages(projectsData),
    [projectsData],
  );

  // project from list
  const projectFromList = useMemo(
    () => projects.find((p) => p.id === projectId),
    [projects, projectId],
  );

  // project details
  const {
    data: projectDetails,
    isFetching: isProjectDetailsFetching,
    error: projectDetailsError,
  } = useGetProjectDetails(projectId || "");
  const {
    data: projectFeatures,
    isFetching: isProjectFeaturesFetching,
  } = useGetProjectFeatures(projectId || "");

  // forbidden
  const isForbidden =
    isForbiddenError(projectsError) || isForbiddenError(projectDetailsError);

  // forbidden message
  const forbiddenMessage =
    getForbiddenMessage(projectsError) ??
    getForbiddenMessage(projectDetailsError);

  // resolved project
  const resolvedProject = projectFromList ?? projectDetails ?? undefined;

  // awaiting project context
  const awaitingProjectContext =
    !!projectId &&
    (resolvedProject === undefined ||
      (resolvedProject !== undefined && isProjectFeaturesFetching)) &&
    (isProjectsLoading || isProjectDetailsFetching || isProjectFeaturesFetching);

  // permissions
  const permissions = useMemo(
    () =>
      getProjectPermissions(resolvedProject?.type?.label, {
        projectFeatures,
      }),
    [resolvedProject?.type?.label, projectFeatures],
  );

  // severity policy
  const { excludeS0, restrictSeverityToLow } = getProjectSeverityPolicy(
    resolvedProject?.type?.label,
    { projectFeatures },
  );

  // has agent
  const hasAgent = resolvedProject?.hasAgent ?? false;

  // include CR stats
  const includeCrStats = permissions.includeChangeRequestsInDashboardTotals;

  // show operations chart
  const showOpsChart = permissions.showOutstandingOpsChart;

  // operations chart mode
  const operationsChartMode =
    permissions.hasSR && !permissions.hasCR
      ? OperationsChartMode.SrOnly
      : OperationsChartMode.SrAndCr;

  // combined cases stats
  const {
    data: combinedCasesStats,
    isLoading: isCombinedCasesLoading,
    isError: isErrorCombinedCases,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [
      CaseType.DEFAULT_CASE,
      CaseType.SERVICE_REQUEST,
      CaseType.ENGAGEMENT,
    ],
    enabled: !!projectId,
  });

  // default case stats
  const {
    data: defaultCaseStats,
    isLoading: isDefaultCaseLoading,
    isError: isErrorDefaultCase,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.DEFAULT_CASE],
    enabled: !!projectId,
  });

  // service request stats
  const {
    data: serviceRequestStats,
    isLoading: isServiceRequestLoading,
    isError: isErrorServiceRequest,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SERVICE_REQUEST],
    enabled: !!projectId && showOpsChart,
  });

  // engagement stats
  const {
    data: engagementStats,
    isLoading: isEngagementLoading,
    isError: isErrorEngagement,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.ENGAGEMENT],
    enabled: !!projectId && permissions.hasEngagements,
  });

  // change request stats
  const {
    data: changeRequestStats,
    isLoading: isChangeRequestStatsLoading,
    isError: isErrorChangeRequestStats,
  } = useGetProjectChangeRequestsStats(projectId || "", {
    enabled: !!projectId && includeCrStats,
  });

  // is dashboard loading
  const isDashboardLoading = isAuthLoading || awaitingProjectContext;

  // use effect to show loader
  useEffect(() => {
    if (isForbidden) {
      hideLoader();
      return;
    }
    if (isDashboardLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isDashboardLoading, isForbidden, showLoader, hideLoader]);

  // use effect to log dashboard data loaded
  useEffect(() => {
    if (combinedCasesStats || defaultCaseStats) {
      logger.debug(`Dashboard data loaded for project ID: ${projectId}`);
    }
  }, [combinedCasesStats, defaultCaseStats, logger, projectId]);

  // show error
  const { showError } = useErrorBanner();

  // has shown error ref
  const hasShownErrorRef = useRef(false);

  // use effect to show error
  useEffect(() => {
    if (isForbidden) return;
    const allCoreFailed = getAllCoreFailedState({
      isErrorCombinedCases,
      isErrorDefaultCase,
      isErrorEngagement,
      showOpsChart,
      isErrorServiceRequest,
      includeCrStats,
      isErrorChangeRequestStats,
      includeEngagementStats: permissions.hasEngagements,
    });

    if (allCoreFailed && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load dashboard statistics.");
      logger.error(
        `Failed to load dashboard stats for project ID: ${projectId}`,
      );
    }
    if (!allCoreFailed) {
      hasShownErrorRef.current = false;
    }
  }, [
    isErrorCombinedCases,
    isErrorDefaultCase,
    isErrorServiceRequest,
    isErrorEngagement,
    isErrorChangeRequestStats,
    showOpsChart,
    includeCrStats,
    permissions.hasEngagements,
    showError,
    logger,
    projectId,
    isForbidden,
  ]);

  // outstanding cases
  const outstandingCases = useMemo(() => {
    const source = defaultCaseStats;
    const outstanding = source?.outstandingSeverityCount ?? [];

    const catastrophicCount =
      outstanding.find((s) => s.label === SEVERITY_API_LABELS[0])?.count ?? 0;
    const critical =
      outstanding.find((s) => s.label === SEVERITY_API_LABELS[1])?.count ?? 0;
    const high =
      outstanding.find((s) => s.label === SEVERITY_API_LABELS[2])?.count ?? 0;
    const medium =
      outstanding.find((s) => s.label === SEVERITY_API_LABELS[3])?.count ?? 0;
    const low =
      outstanding.find((s) => s.label === SEVERITY_API_LABELS[4])?.count ?? 0;

    const catastrophic = permissions.includeS0InSupportMetrics
      ? catastrophicCount
      : 0;
    const total = catastrophic + critical + high + medium + low;

    return {
      catastrophic,
      critical,
      high,
      medium,
      low,
      total,
    };
  }, [defaultCaseStats, permissions.includeS0InSupportMetrics]);

  // outstanding operations
  const outstandingOperations = useMemo(() => {
    const hasServiceRequests = !!serviceRequestStats && !isErrorServiceRequest;
    const hasChangeRequests =
      includeCrStats && !!changeRequestStats && !isErrorChangeRequestStats;

    const serviceRequestsCount = hasServiceRequests
      ? (serviceRequestStats?.totalCount ??
        serviceRequestStats?.totalCases ??
        0)
      : 0;
    const changeRequestsCount = hasChangeRequests
      ? (changeRequestStats?.totalCount ?? 0)
      : 0;

    return calculateProjectStats(
      permissions,
      serviceRequestsCount,
      changeRequestsCount,
    );
  }, [
    serviceRequestStats,
    changeRequestStats,
    isErrorServiceRequest,
    isErrorChangeRequestStats,
    includeCrStats,
    permissions,
  ]);

  // outstanding engagements
  const outstandingEngagements = useMemo(() => {
    const source = engagementStats;
    const outstanding = source?.outstandingEngagementTypeCount ?? [];

    const categories = outstanding.map((item) => ({
      name: normalizeEngagementLabel(item.label),
      value: item.count ?? 0,
    }));
    const total = categories.reduce((sum, item) => sum + item.value, 0);

    return {
      categories,
      total,
    };
  }, [engagementStats]);

  // is charts loading
  const isChartsLoading = getDashboardChartsLoadingState({
    isDashboardLoading,
    isDefaultCaseLoading,
    showOpsChart,
    isServiceRequestLoading,
    isEngagementLoading,
    includeCrStats,
    isChangeRequestStatsLoading,
    includeEngagementStats: permissions.hasEngagements,
  });

  // cr branch state
  const crBranchState = useMemo(() => {
    const hasCombined = !!combinedCasesStats && !isErrorCombinedCases;
    const hasChange = !!changeRequestStats && !isErrorChangeRequestStats;
    const isCardLoading = computeCrCardIsCardLoading(
      includeCrStats,
      combinedCasesStats,
      changeRequestStats,
      isCombinedCasesLoading,
      isChangeRequestStatsLoading,
      isErrorCombinedCases,
      isErrorChangeRequestStats,
    );
    const isCardError = computeCrCardIsCardError(
      includeCrStats,
      isCardLoading,
      combinedCasesStats,
      changeRequestStats,
      isErrorCombinedCases,
      isErrorChangeRequestStats,
    );
    return { hasCombined, hasChange, isCardLoading, isCardError };
  }, [
    changeRequestStats,
    combinedCasesStats,
    includeCrStats,
    isChangeRequestStatsLoading,
    isCombinedCasesLoading,
    isErrorChangeRequestStats,
    isErrorCombinedCases,
  ]);

  // render
  if (isForbidden) {
    return <Error403Page message={forbiddenMessage} />;
  }

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* Dashboard stats grid */}
      <Grid container spacing={2} sx={{ mb: 3 }}>
        {DASHBOARD_STATS.map((stat) => {
          let value: string | number = 0;
          let trend:
            | {
                value: string;
                direction: TrendDirection;
                color: TrendColor;
              }
            | undefined;
          let isCardLoading = false;
          let isCardError = false;

          switch (stat.id) {
            case "totalCases": {
              isCardLoading = crBranchState.isCardLoading;
              isCardError = crBranchState.isCardError;

              const combinedTotal = crBranchState.hasCombined
                ? (combinedCasesStats?.totalCount ??
                  combinedCasesStats?.totalCases ??
                  0)
                : 0;
              const changeTotal = crBranchState.hasChange
                ? (changeRequestStats?.totalCount ?? 0)
                : 0;

              value = includeCrStats
                ? !isCardError &&
                  crBranchState.hasCombined &&
                  crBranchState.hasChange
                  ? combinedTotal + changeTotal
                  : 0
                : combinedTotal;
              break;
            }
            case "openCases": {
              isCardLoading = crBranchState.isCardLoading;
              isCardError = crBranchState.isCardError;

              const combinedActive = crBranchState.hasCombined
                ? (combinedCasesStats?.activeCount ??
                  combinedCasesStats?.stateCount
                    ?.filter((state) => state.label !== "Closed")
                    .reduce((sum, state) => sum + state.count, 0) ??
                  0)
                : 0;

              const changeActive = crBranchState.hasChange
                ? (changeRequestStats?.activeCount ??
                  changeRequestStats?.stateCount
                    ?.filter(
                      (state) =>
                        state.label !== "Closed" && state.label !== "Canceled",
                    )
                    .reduce((sum, state) => sum + state.count, 0) ??
                  0)
                : 0;

              value = includeCrStats
                ? !isCardError &&
                  crBranchState.hasCombined &&
                  crBranchState.hasChange
                  ? combinedActive + changeActive
                  : 0
                : combinedActive;
              break;
            }
            case "resolvedCases": {
              const hasDefault = !!defaultCaseStats && !isErrorDefaultCase;
              const resolved =
                hasDefault && defaultCaseStats
                  ? (defaultCaseStats.resolvedCases.pastThirtyDays ??
                    defaultCaseStats.resolvedCases.currentMonth ??
                    0)
                  : 0;

              value = resolved;
              isCardError = isErrorDefaultCase;
              isCardLoading =
                !isCardError && isDefaultCaseLoading && !defaultCaseStats;

              const changeRate = defaultCaseStats?.changeRate;
              if (typeof changeRate?.resolvedEngagements === "number") {
                const rate = changeRate.resolvedEngagements;
                trend = {
                  value: `${rate >= 0 ? "+" : ""}${rate}%`,
                  direction:
                    rate >= 0 ? TrendDirection.UP : TrendDirection.DOWN,
                  color: rate >= 0 ? TrendColor.SUCCESS : TrendColor.ERROR,
                };
              }
              break;
            }
            case "avgResponseTime": {
              const hasCombined = !!combinedCasesStats && !isErrorCombinedCases;
              const avg =
                hasCombined && combinedCasesStats
                  ? combinedCasesStats.averageResponseTime
                  : 0;

              value = `${avg} hrs`;
              isCardError = isErrorCombinedCases;
              isCardLoading =
                !isCardError && isCombinedCasesLoading && !combinedCasesStats;

              const changeRate = combinedCasesStats?.changeRate;
              if (typeof changeRate?.averageResponseTime === "number") {
                const rate = changeRate.averageResponseTime;
                trend = {
                  value: `${rate >= 0 ? "+" : ""}${rate}%`,
                  direction:
                    rate >= 0 ? TrendDirection.UP : TrendDirection.DOWN,
                  color: TrendColor.SUCCESS,
                };
              }

              break;
            }
            default:
              break;
          }

          const showTrend = stat.id !== "totalCases" && stat.id !== "openCases";

          return (
            <Grid key={stat.id} size={{ xs: 12, sm: 6, md: 3 }}>
              <StatCard
                label={stat.label}
                value={value}
                icon={<stat.icon size={20} />}
                iconColor={stat.iconColor}
                tooltipText={stat.tooltipText}
                trend={trend}
                showTrend={showTrend}
                isLoading={isCardLoading}
                isError={isCardError}
                isTrendError={false}
              />
            </Grid>
          );
        })}
      </Grid>
      {/* Charts row */}
      <ChartLayout
        outstandingCases={outstandingCases}
        activeCases={outstandingOperations}
        engagements={outstandingEngagements}
        isLoading={isChartsLoading}
        isErrorOutstanding={isErrorDefaultCase}
        isErrorActiveCases={
          showOpsChart
            ? includeCrStats
              ? isErrorServiceRequest || isErrorChangeRequestStats
              : isErrorServiceRequest
            : false
        }
        isErrorEngagements={isErrorEngagement}
        excludeS0={excludeS0}
        restrictSeverityToLow={restrictSeverityToLow}
        showOperationsChart={showOpsChart}
        operationsChartMode={operationsChartMode}
        showEngagementsChart={permissions.hasEngagements}
      />
      {/* Cases Table */}
      {projectId && (
        <Box sx={{ mt: 3 }}>
          <CasesTable
            projectId={projectId}
            excludeS0={excludeS0}
            restrictSeverityToLow={restrictSeverityToLow}
            hasAgent={hasAgent}
            includeDeploymentFilter={permissions.hasDeployments}
          />
        </Box>
      )}
    </Box>
  );
}
