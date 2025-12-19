export interface StatsConfig {
  title: string;
  value: number | string;
  icon: React.ReactNode;
  iconBgColor: string;
  iconColor: string;
  trend?: string;
  trendIcon?: React.ReactNode;
}

export interface StatsData {
  stats: {
    totalCases: number;
    totalCasesTrend: string;
    activeCases: number;
    activeCasesTrend: string;
    resolvedThisMonth: number;
    resolvedTrend: string;
    avgResponseTime: string;
    avgResponseTrend: string;
  };
}
