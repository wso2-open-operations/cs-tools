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
  BarChart,
  Bar,
  ResponsiveContainer,
} from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import ErrorIndicator from "@/components/common/errorIndicator/ErrorIndicator";
import {
  CASES_TREND_CHART_DATA,
  TREND_CHART_ERROR_PLACEHOLDER_DATA,
} from "@/constants/dashboardConstants";
import { ChartLegend } from "./ChartLegend";

interface CasesTrendChartProps {
  data: Array<{
    name: string;
    TypeA: number;
    TypeB: number;
    TypeC: number;
    TypeD: number;
  }>;
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
  const chartData = isError ? TREND_CHART_ERROR_PLACEHOLDER_DATA : data || [];

  return (
    <Card sx={{ p: 2, height: "100%" }}>
      {/* Title */}
      <Typography variant="h6" component="h3" sx={{ mb: 2 }}>
        Cases trend
      </Typography>
      {isLoading ? (
        <Box sx={{ height: 240 }}>
          <Skeleton variant="rectangular" width="100%" height="100%" />
        </Box>
      ) : (
        <Box sx={{ height: 240, position: "relative" }}>
          {isError && (
            <Box
              sx={{
                position: "absolute",
                top: 0,
                left: 0,
                right: 0,
                bottom: 0,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                zIndex: 1,
                pointerEvents: "none",
              }}
            >
              <ErrorIndicator entityName="cases trend" />
            </Box>
          )}
          <Box
            sx={{
              height: "100%",
              opacity: isError ? 0.3 : 1,
              filter: isError ? "grayscale(1)" : "none",
            }}
          >
            <ResponsiveContainer width="100%" height="100%">
              {/* Bar chart */}
              <BarChart
                data={chartData}
                grid={{ show: true }}
                xAxis={{ show: true }}
                yAxis={{ show: true }}
                tooltip={{ show: !isError }}
                legend={{ show: false }}
                margin={{ top: 10, right: 0, left: -20, bottom: 0 }}
              >
                {CASES_TREND_CHART_DATA.map((item) => (
                  <Bar
                    key={item.key}
                    dataKey={item.key}
                    stackId="a"
                    fill={isError ? colors.grey[300] : item.color}
                    radius={item.radius}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </Box>
        </Box>
      )}
      {/* Custom Trend Legend */}
      {isLoading ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            gap: 2,
            mt: 2,
          }}
        >
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} variant="rounded" width={60} height={20} />
          ))}
        </Box>
      ) : (
        <ChartLegend
          data={CASES_TREND_CHART_DATA.map((item) => ({
            name: item.name,
            value: 0,
            color: item.color,
          }))}
          isError={isError}
        />
      )}
    </Card>
  );
};
