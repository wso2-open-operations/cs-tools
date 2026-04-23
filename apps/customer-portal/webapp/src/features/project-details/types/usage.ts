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

import type {
  PaginationResponse,
  IdLabelRef,
  SearchRequestBase,
} from "@/types/common";

// Filter type for instance metrics and usage requests.
export type DateRangeFilter = {
  startDate: string;
  endDate: string;
};

// Enum for usage time ranges.
export enum UsageTimeRange {
  THREE_MONTHS = "3m",
  SIX_MONTHS = "6m",
  TWELVE_MONTHS = "12m",
  CUSTOM = "custom",
}

// Response type for GET /projects/:projectId/stats/usage.
export type UsageStatsResponse = {
  deploymentCount: number;
  deployedProductCount: number;
  instanceCount: number;
};

// Model type for deployment metadata within instance metadata.
export type InstanceDeploymentMetadata = {
  os?: string;
  osVersion?: string;
  osArchitecture?: string;
  jdkVersion?: string;
  jdkVendor?: string;
  updateLevel?: string;
  numberOfCores?: string;
};

// Model type for instance metadata nested inside an InstanceItem.
export type InstanceMetadata = {
  id: string;
  coreCount: number | null;
  updates: number | null;
  jdkVersion: string | null;
  deploymentMetadata: InstanceDeploymentMetadata | null;
  createdOn: string;
  updatedOn: string;
  customCreatedOn: string | null;
  customUpdatedOn: string | null;
};

// Item type for a single instance object from POST .../instances/search.
export type InstanceItem = {
  id: string;
  key: string;
  project: IdLabelRef | null;
  deployment: IdLabelRef | null;
  product: IdLabelRef | null;
  deployedProduct: IdLabelRef | null;
  createdOn: string;
  updatedOn: string;
  metadata: InstanceMetadata | null;
};

// Response type for POST .../instances/search.
export type InstancesResponse = PaginationResponse & {
  instances: InstanceItem[];
};

// Item type for a single period summary entry within an instance usage.
export type InstancePeriodSummary = {
  period: string;
  counts: Record<string, number>;
};

// Item type for per-instance entry in an instance usage response.
export type InstanceUsageEntry = {
  instanceId: string;
  instanceKey: string;
  project: IdLabelRef | null;
  deployment: IdLabelRef | null;
  product: IdLabelRef | null;
  deployedProduct: IdLabelRef | null;
  periodSummaries: InstancePeriodSummary[];
};

// Response type for POST .../instances/usages/search.
export type InstanceUsageResponse = {
  usages: InstanceUsageEntry[];
  totalInstances: number;
  startDate: string;
  endDate: string;
};

// Item type for a single data point within an instance metric entry.
export type InstanceMetricDataPoint = {
  date: string;
  createdOn: string;
  coreCount: number | null;
  jdkVersion: string | null;
  updates: number | null;
  deploymentMetadata: InstanceDeploymentMetadata | null;
};

// Item type for per-instance entry in an instance metrics response.
export type InstanceMetricEntry = {
  instanceId: string;
  instanceKey: string;
  project: IdLabelRef | null;
  deployment: IdLabelRef | null;
  product: IdLabelRef | null;
  deployedProduct: IdLabelRef | null;
  dataPoints: InstanceMetricDataPoint[];
};

// Response type for POST .../instances/metrics/search.
export type InstanceMetricsResponse = {
  metrics: InstanceMetricEntry[];
  totalInstances: number;
  startDate: string;
  endDate: string;
};

// Instance Search Filters
export type InstanceSearchFilters = {
  startDate?: string;
  endDate?: string;
};

// Request type for POST .../instances/search.
export type InstanceSearchRequest = SearchRequestBase & {
  filters?: InstanceSearchFilters;
};

// Request type for POST .../instances/usages/search and POST .../instances/metrics/search.
export type InstanceMetricsRequest = {
  filters: DateRangeFilter;
};

// Item type for a single point for Recharts / LineChart x-axis rows.
export type UsageTrendRow = {
  name: string;
  value?: number;
  current?: number;
  average?: number;
};

// Model type for summary counts on the overview tab.
export type UsageOverviewSummary = {
  environments: number;
  products: number;
  instances: number;
};

// Item type for a product row inside an expanded environment on the overview tab.
export type UsageOverviewProductCard = {
  id: string;
  name: string;
  version: string;
  instances: number;
  cores: number;
  transactionsLabel: string;
};

// Item type for a collapsible environment row on the overview tab.
export type UsageEnvironmentBreakdownRow = {
  id: string;
  kind: string;
  title: string;
  subtitle: string;
  totalCores: number;
  transactionsLabel: string;
  products: UsageOverviewProductCard[];
};

export type UsageAggregatedMetricDefinition = {
  id: string;
  title: string;
  caption: string;
  headlineValue: string;
  deltaLabel: string;
  stroke: string;
  data: UsageTrendRow[];
  secondaryStroke?: string;
  secondaryName?: string;
};

// Item type for a mini line chart shown when an instance row is expanded.
export type UsageInstanceChartBlock = {
  title: string;
  caption: string;
  headlineValue: string;
  deltaLabel: string;
  deltaPositive: boolean;
  stroke: string;
  data: UsageTrendRow[];
};

// Item type for chart blocks for a product instance row.
export type UsageInstanceCharts = {
  transactions: UsageInstanceChartBlock;
  cores: UsageInstanceChartBlock;
};

// Item type for per-instance row when a product is expanded.
export type UsageProductInstanceRow = {
  id: string;
  hostName: string;
  javaVersion: string;
  u2Level: string;
  transactionsLabel: string;
  coreCount: number;
  charts: UsageInstanceCharts;
};

// Item type for core metric pill labels on the environment product row.
export type UsageCoreMetricStat = {
  label: string;
  value: string;
};

// Item type for an expandable product within Production / Test / Development tabs.
export type UsageEnvironmentProduct = {
  id: string;
  name: string;
  version: string;
  runningInstances: number;
  transactionsLabel: string;
  thirdMetricLabel: string;
  thirdMetricValue: string;
  coreMetrics: UsageCoreMetricStat[];
  transactionTrend: UsageTrendRow[];
  coreUsageTrend: UsageTrendRow[];
  instances: UsageProductInstanceRow[];
};
