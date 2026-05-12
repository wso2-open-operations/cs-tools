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

import { Box } from "@wso2/oxygen-ui";
import { useParams, useLocation } from "react-router";
import { useCallback, useEffect, useRef, useMemo, type JSX } from "react";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
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
  OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA,
  SEVERITY_API_LABELS,
} from "@/features/dashboard/constants/dashboard";
import { OperationsChartMode } from "@/features/dashboard/types/charts";
import { CaseType } from "@features/support/constants/supportConstants";
import {
  calculateProjectStats,
  getProjectPermissions,
  getProjectSeverityPolicy,
} from "@utils/permission";
import ChartLayout from "@features/dashboard/components/charts/ChartLayout";
import CasesTable from "@features/dashboard/components/cases-table/CasesTable";
import SupportStatGrid from "@components/stat-grid/SupportStatGrid";
import type { SupportStatConfig } from "@features/support/constants/supportConstants";
import {
  getAllCoreFailedState,
  getDashboardChartsLoadingState,
} from "@features/dashboard/utils/dashboard";

/**
 * DashboardPage component to display project-specific statistics and overview.
 *
 * @returns {JSX.Element} The rendered Dashboard page.
 */
export default function DashboardPage(): JSX.Element {
  type DashboardStatKey =
    | "totalCases"
    | "openCases"
    | "resolvedCases"
    | "avgResponseTime";

  // logger
  const logger = useLogger();
  // project id
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useModifierAwareNavigate();
  const location = useLocation();

  type ChartNavAction =
    | { chart: "outstanding"; severityId: string }
    | { chart: "operations"; key: string }
    | { chart: "engagements"; id: string; label?: string };

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
        case "engagements":
          navigate(`/projects/${projectId}/engagements`, {
            state: {
              returnTo: location.pathname,
              engagementTypeId: action.id,
              engagementTypeLabel: action.label,
            },
          });
          break;
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
  const hasAgent =
    projectDetails?.hasAgent ?? projectDetails?.account?.hasAgent ?? false;

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
    enabled: !!projectId && !awaitingProjectContext && permissions.hasSR,
  });

  // engagement stats
  const {
    data: engagementStats,
    isLoading: isEngagementLoading,
    isError: isErrorEngagement,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.ENGAGEMENT],
    enabled:
      !!projectId && !awaitingProjectContext && permissions.hasEngagements,
  });

  // change request stats
  const {
    data: changeRequestStats,
    isLoading: isChangeRequestStatsLoading,
    isError: isErrorChangeRequestStats,
  } = useGetProjectChangeRequestsStats(projectId || "", {
    enabled: !!projectId && !awaitingProjectContext && includeCrStats,
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
      ? (serviceRequestStats?.outstandingCount ?? 0)
      : 0;
    const changeRequestsCount = hasChangeRequests
      ? (changeRequestStats?.outstandingCount ?? 0)
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

  // outstanding engagements — grouped into 4 display categories, Services = Follow Up + Consultancy
  const outstandingEngagements = useMemo(() => {
    const outstanding = engagementStats?.outstandingEngagementTypeCount ?? [];

    const categories = OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA.map(
      (chartEntry) => {
        const matching = outstanding.filter(
          (item) => item.label.toLowerCase() === chartEntry.name.toLowerCase(),
        );
        const value = matching.reduce(
          (sum, item) => sum + (item.count ?? 0),
          0,
        );
        const matchingIds = matching.map((item) => item.id);
        return {
          name: chartEntry.name,
          value,
          ...(matchingIds.length === 1 ? { id: matchingIds[0] } : {}),
          ...(matchingIds.length > 1 ? { ids: matchingIds } : {}),
        };
      },
    );

    const total = categories.reduce((sum, item) => sum + item.value, 0);
    return { categories, total };
  }, [engagementStats]);

  const handleEngagementsClick = useCallback(
    (id: string) => {
      const category = outstandingEngagements.categories.find(
        (c) => (c.ids?.join(",") ?? c.id ?? c.name) === id,
      );
      handleChartNavigation({
        chart: "engagements",
        id,
        label: category?.name,
      });
    },
    [handleChartNavigation, outstandingEngagements],
  );

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

  const dashboardStatConfigs = useMemo<SupportStatConfig<DashboardStatKey>[]>(
    () =>
      DASHBOARD_STATS.map((stat) => ({
        key: stat.id as DashboardStatKey,
        label: stat.label,
        icon: stat.icon,
        iconColor: stat.iconColor,
        tooltipText: stat.tooltipText,
      })),
    [],
  );

  const dashboardStatValues = useMemo<
    Partial<Record<DashboardStatKey, number | string>>
  >(() => {
    // Action Required: sum actionRequiredCount from cases + CR (based on permissions)
    const casesActionRequired = combinedCasesStats?.actionRequiredCount ?? 0;
    const crActionRequired = includeCrStats
      ? (changeRequestStats?.actionRequiredCount ?? 0)
      : 0;

    // Outstanding: sum outstandingCount from cases + CR (based on permissions)
    const casesOutstanding = combinedCasesStats?.outstandingCount ?? 0;
    const crOutstanding = includeCrStats
      ? (changeRequestStats?.outstandingCount ?? 0)
      : 0;

    // Closed Last 30 days: sum resolvedCases.pastThirtyDays from cases + resolvedCount.pastThirtyDays from CR
    const casesResolved =
      combinedCasesStats?.resolvedCases?.pastThirtyDays ?? 0;
    const crResolved = includeCrStats
      ? (changeRequestStats?.resolvedCount?.pastThirtyDays ?? 0)
      : 0;

    // Average Response Time: cases only (CR does not have averageResponseTime)
    const avgHours = (combinedCasesStats?.averageResponseTime ?? 0) * 60;
    const totalMinutes = Math.round(avgHours);
    const hrs = Math.floor(totalMinutes / 60);
    const mins = totalMinutes % 60;
    const avgResponseTime =
      hrs > 0 && mins > 0
        ? `${hrs} hr${hrs !== 1 ? "s" : ""} ${mins} min`
        : hrs > 0
          ? `${hrs} hr${hrs !== 1 ? "s" : ""}`
          : `${mins} min`;

    return {
      totalCases: casesActionRequired + crActionRequired,
      openCases: casesOutstanding + crOutstanding,
      resolvedCases: casesResolved + crResolved,
      avgResponseTime,
    };
  }, [combinedCasesStats, changeRequestStats, includeCrStats]);

  const isDashboardStatsLoading =
    isCombinedCasesLoading || (includeCrStats && isChangeRequestStatsLoading);
  const isDashboardStatsError =
    isErrorCombinedCases || (includeCrStats && isErrorChangeRequestStats);

  // render
  if (isForbidden) {
    return <Error403Page message={forbiddenMessage} />;
  }

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      <Box sx={{ mb: 3 }}>
        <SupportStatGrid<DashboardStatKey>
          isLoading={isDashboardStatsLoading}
          isError={isDashboardStatsError}
          entityName="dashboard statistics"
          configs={dashboardStatConfigs}
          stats={
            dashboardStatValues as Partial<Record<DashboardStatKey, number>>
          }
          valueFormatter={(value) => String(value)}
          nonClickableKeys={["avgResponseTime"]}
          onStatClick={(key) => {
            if (key === "totalCases") {
              navigate("action-required", {
                state: { returnTo: location.pathname },
              });
              return;
            }
            if (key === "openCases") {
              navigate("outstanding-interactions", {
                state: { returnTo: location.pathname },
              });
              return;
            }
            if (key === "resolvedCases") {
              navigate("closed-last-30d", {
                state: { returnTo: location.pathname },
              });
            }
          }}
        />
      </Box>
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
        onEngagementsClick={handleEngagementsClick}
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
