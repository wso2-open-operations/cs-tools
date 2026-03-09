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
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import {
  DASHBOARD_STATS,
  OUTSTANDING_ENGAGEMENTS_CHART_DATA,
  SEVERITY_API_LABELS,
} from "@constants/dashboardConstants";
import { PROJECT_TYPE_LABELS } from "@constants/projectDetailsConstants";
import { getIncidentAndQueryIds } from "@utils/support";
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
    data: project,
    isLoading: isProjectLoading,
  } = useGetProjectDetails(projectId || "");
  const projectReady = !isProjectLoading && project !== undefined;

  const {
    data: filters,
    isLoading: isFiltersLoading,
    isError: isErrorFilters,
  } = useGetProjectFilters(projectId || "");

  const isManagedCloudSubscription =
    projectReady &&
    project?.type?.label === PROJECT_TYPE_LABELS.MANAGED_CLOUD_SUBSCRIPTION;
  const excludeS0 = projectReady ? !isManagedCloudSubscription : false;

  const { incidentId, queryId } = useMemo(
    () => getIncidentAndQueryIds(filters?.caseTypes),
    [filters?.caseTypes],
  );

  const {
    data: casesStats,
    isLoading: isCasesLoading,
    isError: isErrorCases,
  } = useGetProjectCasesStats(projectId || "", {
    incidentId,
    queryId,
    enabled: !!projectId && !isFiltersLoading && !!incidentId && !!queryId,
  });

  const isDashboardLoading =
    isAuthLoading ||
    isProjectLoading ||
    isFiltersLoading ||
    isCasesLoading ||
    (!filters && !isErrorFilters) ||
    (!casesStats && !isErrorCases);

  useEffect(() => {
    if (isDashboardLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isDashboardLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (filters && casesStats) {
      logger.debug(`Dashboard data loaded for project ID: ${projectId}`);
    }
  }, [filters, casesStats, logger, projectId]);

  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  useEffect(() => {
    if (
      (isErrorCases || isErrorFilters) &&
      !hasShownErrorRef.current
    ) {
      hasShownErrorRef.current = true;
      showError("Could not load dashboard statistics.");

      if (isErrorCases) {
        logger.error(`Failed to load cases stats for project ID: ${projectId}`);
      }
      if (isErrorFilters) {
        logger.error(`Failed to load case filters for project ID: ${projectId}`);
      }
    }
    if (!isErrorCases && !isErrorFilters) {
      hasShownErrorRef.current = false;
    }
  }, [isErrorCases, isErrorFilters, showError, logger, projectId]);

  const activeCases = useMemo(() => {
    const open = casesStats?.stateCount.find((s) => s.label === "Open")?.count ?? 0;
    const workInProgress = casesStats?.stateCount.find((s) => s.label === "Work In Progress")?.count ?? 0;
    const awaitingInfo = casesStats?.stateCount.find((s) => s.label === "Awaiting Info")?.count ?? 0;
    const waitingOnWso2 = casesStats?.stateCount.find((s) => s.label === "Waiting On WSO2")?.count ?? 0;
    const solutionProposed = casesStats?.stateCount.find((s) => s.label === "Solution Proposed")?.count ?? 0;
    const reopened = casesStats?.stateCount.find((s) => s.label === "Reopened")?.count ?? 0;
    const total = open + workInProgress + awaitingInfo + waitingOnWso2 + solutionProposed + reopened;

    return {
      open,
      workInProgress,
      awaitingInfo,
      waitingOnWso2,
      solutionProposed,
      reopened,
      total,
    };
  }, [casesStats]);

  const outstandingCases = useMemo(() => {
    const severityByKey: Record<string, number> = {};
    for (const item of OUTSTANDING_ENGAGEMENTS_CHART_DATA) {
      if (item.key === "serviceRequest" || item.key === "securityReportAnalysis") break;
      severityByKey[item.key] =
        casesStats?.outstandingSeverityCount.find((s) => s.label === item.label)
          ?.count ?? 0;
    }
    const serviceRequest =
      casesStats?.caseTypeCount.find(
        (c) => /service\s*request/i.test(c.label),
      )?.count ?? 0;
    const securityReportAnalysis =
      casesStats?.caseTypeCount.find(
        (c) => /security\s*report\s*analysis/i.test(c.label),
      )?.count ?? 0;

    let catastrophic = severityByKey.catastrophic ?? 0;
    const critical = severityByKey.critical ?? 0;
    const high = severityByKey.high ?? 0;
    const medium = severityByKey.medium ?? 0;
    const low = severityByKey.low ?? 0;

    if (!isManagedCloudSubscription) {
      catastrophic = 0;
    }

    const total =
      catastrophic +
      critical +
      high +
      medium +
      low +
      serviceRequest +
      securityReportAnalysis;

    return {
      catastrophic,
      critical,
      high,
      medium,
      low,
      serviceRequest,
      securityReportAnalysis,
      total,
    };
  }, [casesStats, isManagedCloudSubscription]);

  const casesTrend = useMemo(() => {
    const catastrophicCount = isManagedCloudSubscription
      ? (s: { label: string; count?: number }[]) =>
          s.find((x) => x.label === SEVERITY_API_LABELS[0])?.count ?? 0
      : () => 0;
    const mapped = (casesStats?.casesTrend ?? []).map(({ period, severities }) => ({
      period,
      catastrophic: catastrophicCount(severities),
      critical: severities.find((s) => s.label === SEVERITY_API_LABELS[1])?.count ?? 0,
      high: severities.find((s) => s.label === SEVERITY_API_LABELS[2])?.count ?? 0,
      medium: severities.find((s) => s.label === SEVERITY_API_LABELS[3])?.count ?? 0,
      low: severities.find((s) => s.label === SEVERITY_API_LABELS[4])?.count ?? 0,
    }));
    return mapped.sort((a, b) => {
      const parse = (p: string) => {
        const m = p.match(/(\d{4})\D*[Qq](\d)/);
        return m ? Number(m[1]) * 4 + Number(m[2]) : 0;
      };
      return parse(a.period) - parse(b.period);
    });
  }, [casesStats, isManagedCloudSubscription]);

  return (
    <Box sx={{ width: "100%", pt: 0, position: "relative" }}>
      {/* Dashboard stats grid */}
      <Grid container spacing={2} sx={{ mb: 3 }}>
        {DASHBOARD_STATS.map((stat) => {
          const changeRate = casesStats?.changeRate;
          let trend: { value: string; direction: "up" | "down"; color: "success" | "error" | "info" | "warning" } | undefined;
          if (stat.id === "resolvedCases" && typeof changeRate?.resolvedEngagements === "number") {
            const rate = changeRate.resolvedEngagements;
            trend = {
              value: `${rate >= 0 ? "+" : ""}${rate}%`,
              direction: rate >= 0 ? "up" : "down",
              color: rate >= 0 ? "success" : "error",
            };
          } else if (stat.id === "avgResponseTime" && typeof changeRate?.averageResponseTime === "number") {
            const rate = changeRate.averageResponseTime;
            trend = {
              value: `${rate >= 0 ? "+" : ""}${rate}%`,
              direction: rate >= 0 ? "up" : "down",
              color: "success",
            };
          }

          let value: string | number = 0;
          if (casesStats) {
            switch (stat.id) {
              case "totalCases":
                value = casesStats.totalCases;
                break;
              case "openCases":
                value = casesStats.stateCount
                  .filter((state) => state.label !== "Closed")
                  .reduce((sum, state) => sum + state.count, 0);
                break;
              case "resolvedCases":
                value = casesStats.resolvedCases.currentMonth;
                break;
              case "avgResponseTime":
                value = `${casesStats.averageResponseTime} hrs`;
                break;
              default:
                break;
            }
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
                isLoading={
                  (isDashboardLoading || !casesStats) &&
                  !isErrorCases
                }
                isError={isErrorCases}
                isTrendError={false}
              />
            </Grid>
          );
        })}
      </Grid>
      {/* Charts row */}
      <ChartLayout
        outstandingCases={outstandingCases}
        activeCases={activeCases}
        casesTrend={casesTrend || []}
        isLoading={
          (isDashboardLoading || !casesStats) && !isErrorCases
        }
        isErrorOutstanding={isErrorCases}
        isErrorActiveCases={isErrorCases}
        isErrorTrend={isErrorCases}
        excludeS0={excludeS0}
      />
      {/* Cases Table */}
      {projectId && (
        <Box sx={{ mt: 3 }}>
          <CasesTable
            projectId={projectId}
            excludeS0={excludeS0}
          />
        </Box>
      )}
    </Box>
  );
}
