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

import { colors } from "@wso2/oxygen-ui";
import type { UsageAggregatedMetricDefinition } from "@features/project-details/types/usage";
import {
  USAGE_METRICS_TREND_LINE_AVERAGE,
  USAGE_METRICS_TREND_LINE_CURRENT_FALLBACK,
} from "@features/usage-metrics/constants/usageMetricsConstants";

type LineChartLineConfig = {
  dataKey: string;
  name: string;
  stroke: string;
  strokeWidth?: number;
  strokeDasharray?: string;
  dot?: boolean;
};

/**
 * Line series config for aggregated usage metric `LineChart` (single or dual series).
 *
 * @param metric - Metric definition from overview derivation.
 * @param hasSecondary - Whether current vs average series are shown.
 * @returns Line definitions for the chart.
 */
export function buildUsageMetricTrendLineConfigs(
  metric: UsageAggregatedMetricDefinition,
  hasSecondary: boolean,
): LineChartLineConfig[] {
  switch (hasSecondary) {
    case true:
      return [
        {
          dataKey: "current",
          name: metric.secondaryName ?? USAGE_METRICS_TREND_LINE_CURRENT_FALLBACK,
          stroke: metric.stroke,
          strokeWidth: 2.5,
        },
        {
          dataKey: "average",
          name: USAGE_METRICS_TREND_LINE_AVERAGE,
          stroke: metric.secondaryStroke ?? colors.grey?.[400],
          strokeWidth: 2,
          strokeDasharray: "5 5",
        },
      ];
    case false:
      return [
        {
          dataKey: "value",
          name: metric.title,
          stroke: metric.stroke,
          strokeWidth: 2.5,
          dot: false,
        },
      ];
  }
}
