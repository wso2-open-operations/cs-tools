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

// Ported 1:1 from the source app's data/utils/types.ts — just the subset the
// Usage Metrics dashboard and ProductBreakdownRow actually use. Scoped to
// this domain only (no cross-domain sharing), per the port's per-domain
// isolation.

export interface SnRef {
  id: string;
  name: string;
}

export interface SnDeploymentMetadata {
  os?: string;
  osVersion?: string;
  osArchitecture?: string;
  jdkVersion?: string;
  jdkVendor?: string;
  updateLevel?: string;
  numberOfCores?: string;
}

export interface SnInstanceMetadata {
  id: string;
  coreCount: number | null;
  updates: number;
  jdkVersion?: string;
  deploymentMetadata?: SnDeploymentMetadata;
  createdOn: string;
  updatedOn: string;
}

export interface SnInstance {
  id: string;
  key: string;
  project: SnRef | null;
  deployment: SnRef | null;
  product: SnRef | null;
  deployedProduct: SnRef | null;
  environmentType?: string;
  createdOn: string;
  updatedOn: string;
  metadata: SnInstanceMetadata;
}

export interface SnInstancesResponse {
  instances: SnInstance[];
  offset: number;
  limit: number;
  totalRecords: number;
}

export interface DeploymentType {
  id: number;
  label: string;
}

export interface DeploymentItem {
  id: string;
  number: string;
  name: string;
  description: string | null;
  url: string | null;
  version: string | null;
  createdOn: string;
  updatedOn: string;
  project: SnRef;
  type: DeploymentType;
  deployedProductCount: number;
}

export interface DeploymentsSearchResponse {
  deployments: DeploymentItem[];
}

export interface SnDeployedProductUpdate {
  updateLevel: number;
  date: string;
  details?: string | null;
}

export interface SnDeployedProductItem {
  id: string;
  description: string | null;
  cores: number | null;
  tps: number | null;
  updates: SnDeployedProductUpdate[] | null;
  createdOn: string;
  updatedOn: string;
  category: SnRef | null;
  deployment: SnRef | null;
  product: SnRef | null;
  version: SnRef | null;
}

export interface DeployedProductsSearchResponse {
  deployedProducts: SnDeployedProductItem[];
  totalRecords: number;
  offset: number;
  limit: number;
}

export interface DeployedProductMetricsDateRange {
  start: string;
  end: string;
}

export interface DeployedProductMetricsSummary {
  dateRange: DeployedProductMetricsDateRange;
  totalInstances: number;
  minCores: number | null;
  maxCores: number | null;
  avgCores: number | null;
}

export interface DeployedProductMetricsInstance {
  id: string;
  name: string;
  cores: number;
}

export interface DeployedProductMetricsChartPoint {
  date: string;
  instanceCount: number;
  totalCores: number | null;
  minCores: number | null;
  maxCores: number | null;
  avgCores: number | null;
  instances: DeployedProductMetricsInstance[];
}

export interface SnDeployedProductMetricsResponse {
  deployedProduct: SnRef;
  summary: DeployedProductMetricsSummary;
  chartData: DeployedProductMetricsChartPoint[];
}

export interface DeployedProductUsageCountStat {
  aggregation: string;
  min: number;
  max: number;
  avg: number;
  sum?: number;
}

export interface DeployedProductUsageCountsSummary {
  dateRange: DeployedProductMetricsDateRange;
  countTypes: Record<string, DeployedProductUsageCountStat>;
}

export interface DeployedProductUsageCountInstance {
  id: string;
  name: string;
  value: number;
}

export interface DeployedProductUsageCountEntry {
  value: number;
  aggregation: string;
  instances: DeployedProductUsageCountInstance[];
}

export interface DeployedProductUsageCountsChartPoint {
  date: string;
  counts: Record<string, DeployedProductUsageCountEntry>;
}

export interface SnDeployedProductUsageCountsResponse {
  deployedProduct: SnRef;
  summary: DeployedProductUsageCountsSummary;
  chartData: DeployedProductUsageCountsChartPoint[];
}
