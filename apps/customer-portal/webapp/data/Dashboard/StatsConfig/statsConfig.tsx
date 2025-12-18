import CircleAlertIcon from "../../../src/assets/icons/dashboard/circle-alert-icon";
import ClockIcon from "../../../src/assets/icons/dashboard/clock-icon";
import CircleCheckIcon from "../../../src/assets/icons/dashboard/circle-check-icon";
import TrendingDownIcon from "../../../src/assets/icons/dashboard/trending-down-icon";
import type { StatsConfig } from "./types";
import type { ProjectMetadataResponse } from "../../../src/types/project-metadata.types";

export function getStatsConfig(data: ProjectMetadataResponse): StatsConfig[] {
  return [
    {
      title: "Total Cases",
      value: data.projectStatistics.totalCasesCount,
      icon: <CircleAlertIcon width={20} height={20} />,
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
      icon: <CircleCheckIcon width={20} height={20} />,
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
