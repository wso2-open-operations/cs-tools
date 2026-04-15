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

import { Card, Typography, Box, Skeleton, colors } from "@wso2/oxygen-ui";
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
} from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA } from "@features/dashboard/constants/dashboardConstants";
import { ChartLegend } from "@features/dashboard/components/charts/ChartLegend";

interface CasesTrendChartProps {
  data: {
    categories: Array<{
      name: string;
      value: number;
    }>;
    total: number;
  };
  isLoading?: boolean;
  isError?: boolean;
}

/**
 * Displays the cases trend chart.
 *
 * `@param` props - Component props
 */
export const CasesTrendChart = ({
  data,
  isLoading,
  isError,
}: CasesTrendChartProps): JSX.Element => {
  const chartSource = OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA;
  const colorByLabel = new Map(
    chartSource.map((item) => [item.name.toLowerCase(), item.color]),
  );
  const fallbackColors = chartSource.map((item) => item.color);

  const safeData =
    data ??
    ({
      categories: [],
      total: 0,
    } as const);

  const chartData =
    isLoading || isError
      ? chartSource.map((item) => ({
          name: item.name,
          value: 0,
          color:
            isError && !isLoading
              ? (colors.grey?.[300] ?? "#D1D5DB")
              : item.color,
        }))
      : [
          ...safeData.categories.map((category, index) => ({
            name: category.name,
            value: category.value,
            color:
              colorByLabel.get(category.name.toLowerCase()) ??
              fallbackColors[index % fallbackColors.length] ??
              (colors.grey?.[500] ?? "#9CA3AF"),
          })),
        ];

  const total = !isError && !isLoading ? safeData.total : 0;

  return (
    <Card sx={{ p: 2, height: "100%" }}>
      {/* Title */}
      <Typography variant="h6" component="h3" sx={{ mb: 2 }}>
        Outstanding Engagements
      </Typography>
      {isLoading ? (
        <>
          <Box
            sx={{
              height: 240,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Skeleton variant="circular" width={160} height={160} />
          </Box>
          <Box
            sx={{
              display: "flex",
              justifyContent: "center",
              gap: 2,
              mt: 2,
            }}
          >
            {chartSource.map((_, index) => (
              <Skeleton key={index} variant="rounded" width={80} height={20} />
            ))}
          </Box>
        </>
      ) : (
        <>
          <Box
            sx={{
              height: 240,
              position: "relative",
            }}
          >
            <Box
              sx={{
                height: "100%",
                opacity: isError ? 0.3 : 1,
                filter: isError ? "grayscale(1)" : "none",
                "& *:focus": { outline: "none" },
                position: "relative",
              }}
            >
              <ResponsiveContainer width="100%" height="100%">
                <PieChart
                  legend={{ show: false }}
                  tooltip={{ show: !isError, wrapperStyle: { zIndex: 1000 } }}
                >
                  <Pie
                    data={chartData}
                    cx="50%"
                    cy="50%"
                    innerRadius={60}
                    outerRadius={80}
                    paddingAngle={0}
                    minAngle={15}
                    dataKey="value"
                    nameKey="name"
                    startAngle={90}
                    endAngle={-270}
                    label={false}
                    labelLine={false}
                  >
                    {chartData.map((entry, index) => (
                      <Cell
                        key={`cell-${index}`}
                        fill={entry.color}
                        stroke="none"
                      />
                    ))}
                  </Pie>
                </PieChart>
              </ResponsiveContainer>
            </Box>
            {/* Center content: total value or error indicator */}
            <Box
              sx={{
                position: "absolute",
                top: "50%",
                left: "50%",
                transform: "translate(-50%, -50%)",
                textAlign: "center",
                pointerEvents: "none",
              }}
            >
              {isError ? (
                <>
                  <Typography variant="h4" color="text.disabled">
                    --
                  </Typography>
                  <Typography variant="caption" color="text.disabled">
                    Total
                  </Typography>
                </>
              ) : (
                <>
                  <Typography variant="h4">
                    {chartData.length > 0 ? total : "N/A"}
                  </Typography>
                  <Typography variant="caption">Total</Typography>
                </>
              )}
            </Box>
          </Box>
          {/* Legend */}
          <ChartLegend
            data={chartData.map((item) => ({
              name: item.name,
              value: item.value,
              color: item.color,
            }))}
            isError={isError}
            showValues
          />
        </>
      )}
    </Card>
  );
};
