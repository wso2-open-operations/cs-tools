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
import { useParams, useNavigate, useLocation } from "react-router";
import { useCallback, useEffect, useRef, useMemo, type JSX } from "react";
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
import { CaseType, CaseStatus } from "@features/support/constants/supportConstants";
import { ChangeRequestStates } from "@features/operations/constants/operationsConstants";
import {
  calculateProjectStats,
  getProjectPermissions,
  getProjectSeverityPolicy,
} from "@utils/permission";
import { StatCard } from "@features/dashboard/components/stats/StatCard";
import ChartLayout from "@features/dashboard/components/charts/ChartLayout";
import CasesTable from "@features/dashboard/components/cases-table/CasesTable";
import {
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
  const navigate = useNavigate();
  const location = useLocation();

  type ChartNavAction =
    | { chart: "outstanding"; severityId: string }
    | { chart: "operations"; key: string };

  const handleChartNavigation = useCallback(
    (action: ChartNavAction) => {
      switch (action.chart) {
        case "outstanding":
          navigate(
            `/projects/${projectId}/support/cases?severityId=${action.severityId}`,
            { state: { returnTo: location.pathname } },
          );
          break;
        case "operations": {
          const segment =
            action.key === "serviceRequests"
              ? "service-requests"
              : "change-requests";
          navigate(`/projects/${projectId}/operations/${segment}`, {
            state: { returnTo: location.pathname, outstandingOnly: true },
          });
          break;
        }
      }
    },
    [navigate, projectId, location.pathname],
  );

  const handleSeverityClick = useCallback(
    (severityId: string) =>
      handleChartNavigation({ chart: "outstanding", severityId }),
    [handleChartNavigation],
  );

  const handleOperationsClick = useCallback(
    (key: string) => handleChartNavigation({ chart: "operations", key }),
    [handleChartNavigation],
  );

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
  const { data: projectFeatures, isFetching: isProjectFeaturesFetching } =
    useGetProjectFeatures(projectId || "");

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
    (isProjectsLoading ||
      isProjectDetailsFetching ||
      isProjectFeaturesFetching);

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

  // Combined stats scoped to only the case types the user has permission to see.
  // This ensures the Action Required / Outstanding card counts match what is visible on the page.
  // Note: chart-specific queries (defaultCaseStats, serviceRequestStats, engagementStats) remain
  // separate because stateCount here is aggregated and cannot be split back per type.
  const combinedCaseTypes = useMemo(() => {
    const types: string[] = [CaseType.DEFAULT_CASE];
    if (permissions.hasSR) types.push(CaseType.SERVICE_REQUEST);
    if (permissions.hasEngagements) types.push(CaseType.ENGAGEMENT);
    if (permissions.hasSecurityReportAnalysis)
      types.push(CaseType.SECURITY_REPORT_ANALYSIS);
    return types;
  }, [
    permissions.hasSR,
    permissions.hasEngagements,
    permissions.hasSecurityReportAnalysis,
  ]);

  const {
    data: combinedCasesStats,
    isLoading: isCombinedCasesLoading,
    isError: isErrorCombinedCases,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: combinedCaseTypes,
    enabled: !!projectId && !awaitingProjectContext,
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
      ? (serviceRequestStats?.activeCount ??
        serviceRequestStats?.stateCount
          ?.filter((state) => state.label !== "Closed")
          .reduce((sum, state) => sum + state.count, 0) ??
        serviceRequestStats?.outstandingCount ??
        0)
      : 0;
    const changeRequestsCount = hasChangeRequests
      ? (changeRequestStats?.activeCount ??
        changeRequestStats?.stateCount
          ?.filter(
            (state) =>
              state.label !== "Closed" && state.label !== "Canceled",
          )
          .reduce((sum, state) => sum + state.count, 0) ??
        changeRequestStats?.outstandingCount ??
        0)
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
      id: item.id,
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
              // Action Required: Cases/SR/SRA with Awaiting Info or Solution Proposed,
              // plus CRs in Customer Review or Customer Approval.
              isCardError =
                isErrorCombinedCases ||
                (includeCrStats && isErrorChangeRequestStats);
              isCardLoading =
                !isCardError &&
                ((isCombinedCasesLoading && !combinedCasesStats) ||
                  (includeCrStats &&
                    isChangeRequestStatsLoading &&
                    !changeRequestStats));

              if (!isCardError) {
                const casesActionCount =
                  combinedCasesStats?.stateCount
                    ?.filter(
                      (s) =>
                        s.label === CaseStatus.AWAITING_INFO ||
                        s.label === CaseStatus.SOLUTION_PROPOSED,
                    )
                    .reduce((sum, s) => sum + s.count, 0) ?? 0;

                const crActionCount = includeCrStats
                  ? ((changeRequestStats?.stateCount?.find(
                      (s) => s.label === ChangeRequestStates.CUSTOMER_REVIEW,
                    )?.count ?? 0) +
                    (changeRequestStats?.stateCount?.find(
                      (s) => s.label === ChangeRequestStates.CUSTOMER_APPROVAL,
                    )?.count ?? 0))
                  : 0;

                value = casesActionCount + crActionCount;
              }
              break;
            }
            case "openCases": {
              // Outstanding Interactions: Cases/SR/SRA with any non-Closed status,
              // plus CRs excluding Rollback, Closed, and Canceled.
              isCardError =
                isErrorCombinedCases ||
                (includeCrStats && isErrorChangeRequestStats);
              isCardLoading =
                !isCardError &&
                ((isCombinedCasesLoading && !combinedCasesStats) ||
                  (includeCrStats &&
                    isChangeRequestStatsLoading &&
                    !changeRequestStats));

              if (!isCardError) {
                const casesOutstandingCount =
                  combinedCasesStats?.stateCount
                    ?.filter((s) => s.label !== CaseStatus.CLOSED)
                    .reduce((sum, s) => sum + s.count, 0) ?? 0;

                const crOutstandingCount = includeCrStats
                  ? (changeRequestStats?.stateCount
                      ?.filter(
                        (s) =>
                          s.label !== ChangeRequestStates.ROLLBACK &&
                          s.label !== ChangeRequestStates.CLOSED &&
                          s.label !== ChangeRequestStates.CANCELED,
                      )
                      .reduce((sum, s) => sum + s.count, 0) ?? 0)
                  : 0;

                value = casesOutstandingCount + crOutstandingCount;
              }
              break;
            }
            case "resolvedCases": {
              const hasDefault = !!defaultCaseStats && !isErrorDefaultCase;
              const closedTotal =
                hasDefault && defaultCaseStats
                  ? (defaultCaseStats.stateCount?.find(
                      (s) => s.label === CaseStatus.CLOSED,
                    )?.count ?? 0)
                  : 0;

              // Subtract S0 (Catastrophic) closed cases for projects that exclude S0,
              // mirroring the same exclusion AllCasesPage applies client-side.
              // S0 closed ≈ total S0 − outstanding (non-closed) S0.
              const s0ClosedCount = !permissions.includeS0InSupportMetrics
                ? Math.max(
                    0,
                    (defaultCaseStats?.severityCount?.find(
                      (s) => s.label === SEVERITY_API_LABELS[0],
                    )?.count ?? 0) -
                      (defaultCaseStats?.outstandingSeverityCount?.find(
                        (s) => s.label === SEVERITY_API_LABELS[0],
                      )?.count ?? 0),
                  )
                : 0;

              value = Math.max(0, closedTotal - s0ClosedCount);
              isCardError = isErrorDefaultCase;
              isCardLoading =
                !isCardError && isDefaultCaseLoading && !defaultCaseStats;
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
              break;
            }
            default:
              break;
          }

          const statOnClick =
            stat.id === "totalCases"
              ? () => navigate("action-required", { state: { returnTo: location.pathname } })
              : stat.id === "openCases"
                ? () => navigate("outstanding-interactions", { state: { returnTo: location.pathname } })
                : stat.id === "resolvedCases"
                  ? () => navigate("../support/cases?statusFilter=resolved", { state: { returnTo: location.pathname } })
                  : undefined;

          return (
            <Grid key={stat.id} size={{ xs: 12, sm: 6, md: 3 }}>
              <StatCard
                label={stat.label}
                value={value}
                icon={<stat.icon size={20} />}
                iconColor={stat.iconColor}
                tooltipText={stat.tooltipText}
                trend={trend}
                showTrend={false}
                isLoading={isCardLoading}
                isError={isCardError}
                isTrendError={false}
                onClick={statOnClick}
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
        onSeverityClick={handleSeverityClick}
        onOperationsClick={handleOperationsClick}
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
