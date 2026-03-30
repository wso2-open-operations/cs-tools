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
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import { useGetProjectChangeRequestsStats } from "@api/useGetProjectChangeRequestsStats";
import {
  DASHBOARD_STATS,
  SEVERITY_API_LABELS,
} from "@constants/dashboardConstants";
import { CaseType } from "@constants/supportConstants";
import {
  calculateProjectStats,
  getProjectPermissions,
  shouldExcludeS0,
} from "@utils/subscriptionUtils";
import { StatCard } from "@components/dashboard/stats/StatCard";
import ChartLayout from "@components/dashboard/charts/ChartLayout";
import CasesTable from "@components/dashboard/cases-table/CasesTable";

/**
 * DashboardPage component to display project-specific statistics and overview.
 *
 * @returns {JSX.Element} The rendered Dashboard page.
 */
export default function DashboardPage(): JSX.Element {
  const logger = useLogger();
  const { projectId } = useParams<{ projectId: string }>();
  const { showLoader, hideLoader } = useLoader();

  const { isLoading: isAuthLoading } = useAsgardeo();

  const {
    data: projectsData,
    isLoading: isProjectsLoading,
  } = useInfiniteProjects({ pageSize: 20, enabled: !!projectId });
  const projects = useMemo(() => flattenProjectPages(projectsData), [projectsData]);
  const projectFromList = useMemo(
    () => projects.find((p) => p.id === projectId),
    [projects, projectId],
  );
  const { data: projectDetails, isFetching: isProjectDetailsFetching } =
    useGetProjectDetails(projectId || "");
  const resolvedProject = projectFromList ?? projectDetails ?? undefined;

  const awaitingProjectContext =
    !!projectId &&
    resolvedProject === undefined &&
    (isProjectsLoading || isProjectDetailsFetching);

  const permissions = useMemo(
    () => getProjectPermissions(resolvedProject?.type?.label),
    [resolvedProject?.type?.label],
  );

  const excludeS0 = shouldExcludeS0(resolvedProject?.type?.label);
  const hasAgent = resolvedProject?.hasAgent ?? false;

  const includeCrStats = permissions.includeChangeRequestsInDashboardTotals;
  const showOpsChart = permissions.showOutstandingOpsChart;
  const operationsChartMode =
    permissions.hasSR && !permissions.hasCR ? "srOnly" : "srAndCr";

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

  const {
    data: defaultCaseStats,
    isLoading: isDefaultCaseLoading,
    isError: isErrorDefaultCase,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.DEFAULT_CASE],
    enabled: !!projectId,
  });

  const {
    data: serviceRequestStats,
    isLoading: isServiceRequestLoading,
    isError: isErrorServiceRequest,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.SERVICE_REQUEST],
    enabled: !!projectId && showOpsChart,
  });

  const {
    data: engagementStats,
    isLoading: isEngagementLoading,
    isError: isErrorEngagement,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.ENGAGEMENT],
    enabled: !!projectId,
  });

  const {
    data: changeRequestStats,
    isLoading: isChangeRequestStatsLoading,
    isError: isErrorChangeRequestStats,
  } = useGetProjectChangeRequestsStats(projectId || "", {
    enabled: !!projectId && includeCrStats,
  });

  const isDashboardLoading = isAuthLoading || awaitingProjectContext;

  useEffect(() => {
    if (isDashboardLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isDashboardLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (combinedCasesStats || defaultCaseStats) {
      logger.debug(`Dashboard data loaded for project ID: ${projectId}`);
    }
  }, [combinedCasesStats, defaultCaseStats, logger, projectId]);

  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  useEffect(() => {
    const srFailed = showOpsChart && isErrorServiceRequest;
    const crFailed = includeCrStats && isErrorChangeRequestStats;
    const allCoreFailed =
      isErrorCombinedCases &&
      isErrorDefaultCase &&
      isErrorEngagement &&
      (!showOpsChart || srFailed) &&
      (!includeCrStats || crFailed);

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
    showError,
    logger,
    projectId,
  ]);

  const outstandingCases = useMemo(() => {
    const source = defaultCaseStats;
    const outstanding = source?.outstandingSeverityCount ?? [];

    const catastrophicCount =
      outstanding.find(
        (s) => s.label === SEVERITY_API_LABELS[0],
      )?.count ?? 0;
    const critical =
      outstanding.find(
        (s) => s.label === SEVERITY_API_LABELS[1],
      )?.count ?? 0;
    const high =
      outstanding.find(
        (s) => s.label === SEVERITY_API_LABELS[2],
      )?.count ?? 0;
    const medium =
      outstanding.find(
        (s) => s.label === SEVERITY_API_LABELS[3],
      )?.count ?? 0;
    const low =
      outstanding.find(
        (s) => s.label === SEVERITY_API_LABELS[4],
      )?.count ?? 0;

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

  const outstandingOperations = useMemo(() => {
    const hasServiceRequests =
      !!serviceRequestStats && !isErrorServiceRequest;
    const hasChangeRequests =
      includeCrStats && !!changeRequestStats && !isErrorChangeRequestStats;

    const serviceRequestsCount = hasServiceRequests
      ? serviceRequestStats?.totalCount ??
        serviceRequestStats?.totalCases ??
        0
      : 0;
    const changeRequestsCount = hasChangeRequests
      ? changeRequestStats?.totalCount ?? 0
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

  const outstandingEngagements = useMemo(() => {
    const source = engagementStats;
    const outstanding =
      source?.outstandingEngagementTypeCount ?? [];
    const normalizeEngagementLabel = (label: string): string => {
      if (label === "New Feature / Improvement") return "Improvements";
      if (label === "Consultancy") return "Services";
      return label;
    };

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

  const isChartsLoading =
    isDashboardLoading ||
    isDefaultCaseLoading ||
    (showOpsChart && isServiceRequestLoading) ||
    isEngagementLoading ||
    (includeCrStats && isChangeRequestStatsLoading);

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* Dashboard stats grid */}
      <Grid container spacing={2} sx={{ mb: 3 }}>
        {DASHBOARD_STATS.map((stat) => {
          let value: string | number = 0;
          let trend:
            | {
                value: string;
                direction: "up" | "down";
                color: "success" | "error" | "info" | "warning";
              }
            | undefined;
          let isCardLoading = false;
          let isCardError = false;

          switch (stat.id) {
            case "totalCases": {
              const hasCombined =
                !!combinedCasesStats && !isErrorCombinedCases;
              const hasChange =
                includeCrStats &&
                !!changeRequestStats &&
                !isErrorChangeRequestStats;

              const combinedTotal = hasCombined
                ? combinedCasesStats?.totalCount ??
                  combinedCasesStats?.totalCases ??
                  0
                : 0;
              const changeTotal = hasChange
                ? changeRequestStats?.totalCount ?? 0
                : 0;

              value = combinedTotal + changeTotal;

              isCardError =
                !hasCombined &&
                (!includeCrStats || !hasChange) &&
                (isErrorCombinedCases ||
                  (includeCrStats && isErrorChangeRequestStats));
              isCardLoading =
                !isCardError &&
                ((isCombinedCasesLoading && !combinedCasesStats) ||
                  (includeCrStats &&
                    isChangeRequestStatsLoading &&
                    !changeRequestStats));
              break;
            }
            case "openCases": {
              const hasCombined =
                !!combinedCasesStats && !isErrorCombinedCases;
              const hasChange =
                includeCrStats &&
                !!changeRequestStats &&
                !isErrorChangeRequestStats;

              const combinedActive = hasCombined
                ? combinedCasesStats?.activeCount ??
                  combinedCasesStats?.stateCount
                    ?.filter((state) => state.label !== "Closed")
                    .reduce((sum, state) => sum + state.count, 0) ??
                  0
                : 0;

              const changeActive = hasChange
                ? changeRequestStats?.activeCount ??
                  changeRequestStats?.stateCount
                    ?.filter(
                      (state) =>
                        state.label !== "Closed" &&
                        state.label !== "Canceled",
                    )
                    .reduce((sum, state) => sum + state.count, 0) ??
                  0
                : 0;

              value = combinedActive + changeActive;

              isCardError =
                !hasCombined &&
                (!includeCrStats || !hasChange) &&
                (isErrorCombinedCases ||
                  (includeCrStats && isErrorChangeRequestStats));
              isCardLoading =
                !isCardError &&
                ((isCombinedCasesLoading && !combinedCasesStats) ||
                  (includeCrStats &&
                    isChangeRequestStatsLoading &&
                    !changeRequestStats));
              break;
            }
            case "resolvedCases": {
              const hasDefault =
                !!defaultCaseStats && !isErrorDefaultCase;
              const resolved =
                hasDefault && defaultCaseStats
                  ? defaultCaseStats.resolvedCases.pastThirtyDays ??
                    defaultCaseStats.resolvedCases.currentMonth ??
                    0
                  : 0;

              value = resolved;
              isCardError = isErrorDefaultCase;
              isCardLoading =
                !isCardError &&
                isDefaultCaseLoading &&
                !defaultCaseStats;

              const changeRate = defaultCaseStats?.changeRate;
              if (
                typeof changeRate?.resolvedEngagements === "number"
              ) {
                const rate = changeRate.resolvedEngagements;
                trend = {
                  value: `${rate >= 0 ? "+" : ""}${rate}%`,
                  direction: rate >= 0 ? "up" : "down",
                  color: rate >= 0 ? "success" : "error",
                };
              }
              break;
            }
            case "avgResponseTime": {
              const hasCombined =
                !!combinedCasesStats && !isErrorCombinedCases;
              const avg =
                hasCombined && combinedCasesStats
                  ? combinedCasesStats.averageResponseTime
                  : 0;

              value = `${avg} hrs`;
              isCardError = isErrorCombinedCases;
              isCardLoading =
                !isCardError &&
                isCombinedCasesLoading &&
                !combinedCasesStats;

              const changeRate = combinedCasesStats?.changeRate;
              if (
                typeof changeRate?.averageResponseTime === "number"
              ) {
                const rate = changeRate.averageResponseTime;
                trend = {
                  value: `${rate >= 0 ? "+" : ""}${rate}%`,
                  direction: rate >= 0 ? "up" : "down",
                  color: "success",
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
        showOperationsChart={showOpsChart}
        operationsChartMode={operationsChartMode}
      />
      {/* Cases Table */}
      {projectId && (
        <Box sx={{ mt: 3 }}>
          <CasesTable
            projectId={projectId}
            excludeS0={excludeS0}
            hasAgent={hasAgent}
            includeDeploymentFilter={permissions.hasDeployments}
          />
        </Box>
      )}
    </Box>
  );
}
