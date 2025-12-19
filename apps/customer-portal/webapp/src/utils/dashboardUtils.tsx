import type { ProjectMetadataResponse } from "../types/project-metadata.types";
import type { StatsConfig } from "../types/dashboard.types";
import {
  AlertCircleIcon,
  CheckCircleIcon,
  ClockIcon,
} from "@/assets/icons/common-icons";
import { TrendingDownIcon } from "@/assets/icons/common/trending-down-icon";

export const getOutstandingIncidentsData = (
  projectMetadata: ProjectMetadataResponse | undefined
) => {
  return projectMetadata
    ? [
        {
          name: "Critical (S0)",
          value: projectMetadata.projectStatistics.incidentCount.s0,
          color: "#ef4444",
        },
        {
          name: "High (S1)",
          value: projectMetadata.projectStatistics.incidentCount.s1,
          color: "#f97316",
        },
        {
          name: "Medium (S2)",
          value: projectMetadata.projectStatistics.incidentCount.s2,
          color: "#6366f1",
        },
        {
          name: "Low (S3)",
          value: projectMetadata.projectStatistics.incidentCount.s3,
          color: "#22d3ee",
        },
        {
          name: "Minimal (S4)",
          value: projectMetadata.projectStatistics.incidentCount.s4,
          color: "#9ca3af",
        },
      ]
    : [];
};

export const getActiveCasesData = (
  projectMetadata: ProjectMetadataResponse | undefined
) => {
  return projectMetadata
    ? [
        {
          name: "Awaiting",
          value:
            projectMetadata.projectStatistics.activeCaseCount.awaitingCount,
          color: "#6366f1",
        },
        {
          name: "Work in Progress",
          value:
            projectMetadata.projectStatistics.activeCaseCount
              .workInProgressCount,
          color: "#22d3ee",
        },
        {
          name: "Waiting on WSO2",
          value:
            projectMetadata.projectStatistics.activeCaseCount
              .waitingOnWSO2Count,
          color: "#fb923c",
        },
      ]
    : [];
};

export function getDashboardStatsConfig(
  data: ProjectMetadataResponse
): StatsConfig[] {
  return [
    {
      title: "Total Cases",
      value: data.projectStatistics.totalCasesCount,
      icon: <AlertCircleIcon width={20} height={20} />,
      iconBgColor: "#fff7ed",
      iconColor: "#ea580c",
    },
    {
      title: "Active Cases",
      value: data.projectStatistics.openCasesCount,
      icon: <ClockIcon width={20} height={20} />,
      iconBgColor: "#eff6ff",
      iconColor: "#2563eb",
    },
    {
      title: "Resolved This Month",
      value: data.projectStatistics.currentMonthResolvedCasesCount,
      icon: <CheckCircleIcon width={20} height={20} />,
      iconBgColor: "#f0fdf4",
      iconColor: "#16a34a",
    },
    {
      title: "Avg. Response Time",
      value: data.projectStatistics.avgResponseTime,
      icon: <TrendingDownIcon width={20} height={20} color="#9333ea" />,
      iconBgColor: "#faf5ff",
      iconColor: "#9333ea",
    },
  ];
}
