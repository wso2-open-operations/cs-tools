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

import { Box, Card, Typography } from "@wso2/oxygen-ui";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { USAGE_LINE_CHART_MARGIN } from "@features/usage-metrics/constants/usageMetricsConstants";
import type { UsageMetricTrendCardProps } from "@features/usage-metrics/types/usageMetrics";
import {
  resolveUsageMetricDeltaLabelColor,
  resolveUsageMetricDeltaTone,
} from "@features/usage-metrics/utils/usageMetricDelta";
import { buildUsageMetricTrendLineConfigs } from "@features/usage-metrics/utils/usageMetricTrendChart";
import { UsageChartSurface } from "@features/usage-metrics/components/UsageChartSurface";

/**
 * Overview-style metric card with an Oxygen UI line chart.
 *
 * @param metric - Headline values and series for the chart.
 * @param chartHeight - Chart height in pixels.
 * @returns {JSX.Element} Card with trend visualization.
 */
export default function UsageMetricTrendCard({
  metric,
  chartHeight = 180,
}: UsageMetricTrendCardProps): JSX.Element {
  const hasSecondary =
    metric.secondaryStroke != null &&
    metric.secondaryName != null &&
    metric.data.some((row) => row.current != null && row.average != null);

  const deltaTone = resolveUsageMetricDeltaTone(metric.deltaLabel);
  const deltaColor = resolveUsageMetricDeltaLabelColor(deltaTone);

  return (
    <Card
      sx={{
        p: 2,
        height: "100%",
        display: "flex",
        flexDirection: "column",
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          gap: 1,
          mb: 0.5,
        }}
      >
        <Typography variant="subtitle1" component="h4" sx={{ fontWeight: 600 }}>
          {metric.title}
        </Typography>
        <Box sx={{ textAlign: "right" }}>
          <Typography variant="h6" component="p" sx={{ fontWeight: 600 }}>
            {metric.headlineValue}
          </Typography>
          <Typography
            variant="caption"
            sx={{
              color: deltaColor,
              fontWeight: 600,
            }}
          >
            {metric.deltaLabel}
          </Typography>
        </Box>
      </Box>
      <Typography variant="caption" color="text.secondary" sx={{ mb: 1.5 }}>
        {metric.caption}
      </Typography>
      <UsageChartSurface minHeight={chartHeight}>
        <LineChart
          data={metric.data}
          xAxisDataKey="name"
          height={chartHeight}
          width="100%"
          margin={USAGE_LINE_CHART_MARGIN}
          accessibilityLayer={false}
          legend={
            hasSecondary
              ? { show: true, align: "center", verticalAlign: "bottom" }
              : { show: false }
          }
          grid={{ show: true, strokeDasharray: "3 3" }}
          lines={buildUsageMetricTrendLineConfigs(metric, hasSecondary)}
        />
      </UsageChartSurface>
    </Card>
  );
}
