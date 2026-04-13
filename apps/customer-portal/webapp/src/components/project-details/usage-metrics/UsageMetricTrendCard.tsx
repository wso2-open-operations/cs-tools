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

import { Box, Card, Typography, colors } from "@wso2/oxygen-ui";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import type { UsageAggregatedMetricDefinition } from "@/types/usage";
import { USAGE_LINE_CHART_MARGIN } from "@constants/usageMetricsConstants";
import { UsageChartSurface } from "@components/project-details/usage-metrics/UsageChartSurface";

export interface UsageMetricTrendCardProps {
  metric: UsageAggregatedMetricDefinition;
  chartHeight?: number;
}

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
              color: metric.deltaLabel.startsWith("-")
                ? (colors.red?.[600] ?? "#DC2626")
                : (colors.green?.[600] ?? "#16A34A"),
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
          lines={
            hasSecondary
              ? [
                  {
                    dataKey: "current",
                    name: metric.secondaryName ?? "Current",
                    stroke: metric.stroke,
                    strokeWidth: 2.5,
                  },
                  {
                    dataKey: "average",
                    name: "Average",
                    stroke: metric.secondaryStroke ?? colors.grey?.[400],
                    strokeWidth: 2,
                    strokeDasharray: "5 5",
                  },
                ]
              : [
                  {
                    dataKey: "value",
                    name: metric.title,
                    stroke: metric.stroke,
                    strokeWidth: 2.5,
                    dot: false,
                  },
                ]
          }
        />
      </UsageChartSurface>
    </Card>
  );
}
