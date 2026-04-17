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

import { UsageTimeRange } from "@features/project-details/types/usage";

export const USAGE_LINE_CHART_MARGIN = {
  top: 5,
  right: 40,
  left: 20,
  bottom: 40,
};

/** Prefix for inner TabBar ids for each deployment (`${PREFIX}${deploymentId}`). */
export const USAGE_METRICS_DEPLOYMENT_TAB_PREFIX = "um-dep-";

/** Label for the overview tab in the inner TabBar. */
export const USAGE_METRICS_TAB_OVERVIEW_LABEL = "Overview";

/** Fixed preset buttons (3 / 6 / 12 months) on the time-range toolbar. */
export const USAGE_METRICS_PRESET_TIME_RANGES: readonly UsageTimeRange[] = [
  UsageTimeRange.THREE_MONTHS,
  UsageTimeRange.SIX_MONTHS,
  UsageTimeRange.TWELVE_MONTHS,
];

export const USAGE_TIME_RANGE_LABELS: Record<UsageTimeRange, string> = {
  [UsageTimeRange.THREE_MONTHS]: "Last 3 months",
  [UsageTimeRange.SIX_MONTHS]: "Last 6 months",
  [UsageTimeRange.TWELVE_MONTHS]: "Last 12 months",
  [UsageTimeRange.CUSTOM]: "Custom range",
};

export const USAGE_METRICS_TIME_RANGE_HEADING = "Time Range:";

export const USAGE_METRICS_CUSTOM_RANGE_BUTTON = "Custom";

export const USAGE_METRICS_CUSTOM_RANGE_APPLY = "Apply";

export const USAGE_METRICS_CUSTOM_RANGE_CANCEL = "Cancel";

export const USAGE_METRICS_CUSTOM_RANGE_TO = "to";

export const USAGE_METRICS_CUSTOM_RANGE_PLACEHOLDER = "Select custom range";

export const USAGE_METRICS_OVERVIEW_ERROR =
  "Failed to load usage overview data. Please try again.";

export const USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR =
  "Failed to load environment product data. Please try again.";

export const USAGE_METRICS_NO_PRODUCTS_IN_ENVIRONMENT =
  "No products found for this environment.";

export const USAGE_METRICS_NO_PRODUCTS_IN_DEPLOYMENT =
  "No products found for this deployment.";

export const USAGE_METRICS_NO_INSTANCE_DATA = "No instance data available.";

export const USAGE_METRICS_STAT_ENVIRONMENTS = "Environments";

export const USAGE_METRICS_STAT_PRODUCTS = "Products";

export const USAGE_METRICS_STAT_INSTANCES = "Instances";

export const USAGE_METRICS_SECTION_ENVIRONMENT_BREAKDOWN = "Environment Breakdown";

export const USAGE_METRICS_SECTION_AGGREGATED = "Aggregated Metrics";

export const USAGE_METRICS_ENVIRONMENT_ROW_TRANSACTIONS = "Transactions";

export const USAGE_METRICS_EXPANDED_CARD_INSTANCES = "Instances";

export const USAGE_METRICS_EXPANDED_CARD_CORES = "Cores";

export const USAGE_METRICS_EXPANDED_CARD_TOTAL_TX = "Total Transactions";

export const USAGE_METRICS_INSTANCE_JAVA = "Java Version";

export const USAGE_METRICS_INSTANCE_U2 = "U2 Level";

export const USAGE_METRICS_INSTANCE_TOTAL_TX = "Total Transactions";

export const USAGE_METRICS_INSTANCE_CORE_COUNT = "Core Count";

export const USAGE_METRICS_INSTANCE_CHART_TX_TITLE = "Transactions";

export const USAGE_METRICS_INSTANCE_CHART_TX_CAPTION = "Periodic transaction count";

export const USAGE_METRICS_INSTANCE_CHART_CORE_TITLE = "Core Count";

export const USAGE_METRICS_INSTANCE_CHART_CORE_CAPTION = "Cores over time";

export const USAGE_METRICS_PRODUCT_THIRD_METRIC_LABEL = "API Count";

export const USAGE_METRICS_PRODUCT_PANEL_TOTAL_TRANSACTIONS =
  "Total Transactions";

export const USAGE_METRICS_PRODUCT_TREND_TX_SECTION = "Transaction Trends";

export const USAGE_METRICS_PRODUCT_TREND_CORE_SECTION = "Core Usage Over Time";

export const USAGE_METRICS_PRODUCT_INSTANCES_SECTION = "Instances";

export const USAGE_METRICS_CHART_LINE_CORES_SHORT = "Cores";

export const USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES = "Total Cores";

export const USAGE_METRICS_PRODUCT_CORE_METRIC_INSTANCES = "Instances";

export const USAGE_METRICS_UNKNOWN_LABEL = "Unknown";

export const USAGE_METRICS_VALUE_EM_DASH = "—";

export const USAGE_AGGREGATED_TOTAL_TRANSACTIONS_TITLE = "Total Transactions";

export const USAGE_AGGREGATED_TOTAL_TRANSACTIONS_CAPTION =
  "Across all environments and products";

export const USAGE_AGGREGATED_TOTAL_USERS_TITLE = "Total User Count";

export const USAGE_AGGREGATED_TOTAL_USERS_CAPTION =
  "Active users across all environments";

export const USAGE_AGGREGATED_API_COUNT_TITLE = "Total API Count";

export const USAGE_AGGREGATED_API_COUNT_CAPTION =
  "Unique APIs accessed across all products";

export const USAGE_AGGREGATED_TOTAL_CORES_TITLE = "Total Cores";

export const USAGE_AGGREGATED_TOTAL_CORES_CAPTION =
  "Daily core count allocation across all instances";

export const USAGE_AGGREGATED_ORG_COUNT_TITLE = "Total Organization Count";

export const USAGE_AGGREGATED_ORG_COUNT_CAPTION =
  "External organizations using your APIs";

export const USAGE_METRICS_TREND_LINE_AVERAGE = "Average";

export const USAGE_METRICS_TREND_LINE_CURRENT_FALLBACK = "Current";
