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

import type { ReactNode } from "react";
import type { UsageAggregatedMetricDefinition } from "@features/project-details/types/usage";
import type {
  UsageEnvironmentProduct,
  UsageInstanceChartBlock,
  UsageProductInstanceRow,
  UsageTimeRange,
} from "@features/project-details/types/usage";

/** ISO date strings (YYYY-MM-DD) for usage API filters. */
export type UsageDateRangeIso = {
  startDate: string;
  endDate: string;
};

/** Fixed inner tab id for the Usage & Metrics overview. */
export enum UsageMetricsInnerTabId {
  OVERVIEW = "um-overview",
}

/** Keys for the five aggregated metric cards on the overview tab. */
export enum UsageAggregatedMetricCardId {
  TOTAL_TRANSACTIONS = "total-transactions",
  TOTAL_USERS = "total-users",
  API_COUNT = "api-count",
  TOTAL_CORES = "total-cores",
  TOTAL_ORGANIZATION_COUNT = "total-organization-count",
}

/** Semantic tone for delta / change labels on trend UI. */
export enum UsageMetricDeltaTone {
  NEGATIVE = "negative",
  POSITIVE = "positive",
  NEUTRAL = "neutral",
}

export type UsageChartSurfaceProps = {
  children: ReactNode;
  minHeight?: number;
};

export type UsageMetricTrendCardProps = {
  metric: UsageAggregatedMetricDefinition;
  chartHeight?: number;
};

export type UsageMetricsTimeRangeSelectorProps = {
  innerTab: string;
  timeRange: UsageTimeRange;
  onTimeRangeChange: (range: UsageTimeRange) => void;
  onClearCustomApplied: () => void;
  customStart: string;
  customEnd: string;
  onCustomStartChange: (value: string) => void;
  onCustomEndChange: (value: string) => void;
  onApplyCustom: () => void;
  onCancelCustom: () => void;
  appliedCustomStart: string;
  appliedCustomEnd: string;
};

export type UsageOverviewPanelProps = {
  projectId: string | undefined;
  dateRange: UsageDateRangeIso;
  expandedEnvironmentIds: Set<string>;
  onToggleEnvironment: (id: string) => void;
  timeRangeSelector?: ReactNode;
};

export type UsageEnvironmentProductsPanelProps = {
  deploymentId: string;
  projectId: string | undefined;
  dateRange: UsageDateRangeIso;
  expandedProductIds: Set<string>;
  onToggleProduct: (productId: string) => void;
};

export type ExpandedProductSummary = {
  id: string;
  label: string;
  instanceCount: number;
  coreCount: number;
  totalTx: number;
};

export type UsageOverviewEnvironmentAccent = {
  main: string;
  title: string;
  border: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
  statTileBg: string;
};

export type ExpandedProductCardProps = {
  product: ExpandedProductSummary;
  accent: UsageOverviewEnvironmentAccent;
};

export type DeploymentExpandedViewProps = {
  deploymentId: string;
  typeId: string;
  dateRange: UsageDateRangeIso;
};

export type EnvironmentBreakdownRow = {
  id: string;
  deploymentId: string;
  kind: string;
  title: string;
  productCount: number;
  instanceCount: number;
  totalCores: number;
  transactionsLabel: string;
};

export type EnvironmentBreakdownAccordionProps = {
  row: EnvironmentBreakdownRow;
  expanded: boolean;
  onToggle: () => void;
  dateRange: UsageDateRangeIso;
};

export type ProductExpandedViewProps = {
  product: UsageEnvironmentProduct;
  expandedInstanceKeys: Set<string>;
  onToggleInstance: (key: string) => void;
};

export type InstanceMiniTrendCardProps = {
  block: UsageInstanceChartBlock;
};

export type InstanceAccordionRowProps = {
  instance: UsageProductInstanceRow;
  expanded: boolean;
  onToggle: () => void;
};

export type ProductAccordionRowProps = {
  product: UsageEnvironmentProduct;
  deploymentId: string;
  expanded: boolean;
  onToggle: () => void;
  expandedInstanceKeys: Set<string>;
  onToggleInstance: (key: string) => void;
};

export type MetricPillProps = {
  icon: ReactNode;
  label: string;
  value: string;
};

export type UseSearchProjectTimeCardsParams = {
  projectId: string;
  startDate?: string;
  endDate?: string;
  states?: string[];
  enabled?: boolean;
};

export type UseGetTimeCardsStatsParams = {
  projectId: string;
  startDate: string;
  endDate: string;
};

export type UsageEnvironmentPanelAccent = {
  main: string;
  stroke: string;
  borderDefault: string;
  borderHover: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
};
