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
import { ChartLegend } from "./ChartLegend";
import { ACTIVE_CASES_CHART_DATA } from "@/constants/dashboardConstants";

interface ActiveCasesChartProps {
  data: {
    workInProgress: number;
    waitingOnClient: number;
    waitingOnWso2: number;
    total: number;
  };
  isLoading?: boolean;
}

/**
 * Renders the Active Cases chart.
 *
 * @param data - Dataset used to render the chart.
 * @param isLoading - Indicates whether the chart is in a loading state.
 * @returns A JSX element that displays the Active Cases chart.
 */
export const ActiveCasesChart = ({
  data,
  isLoading,
}: ActiveCasesChartProps): JSX.Element => {
  // Map the active cases chart data to the chart data format
  const chartData = ACTIVE_CASES_CHART_DATA.map((item) => ({
    name: item.name,
    value: data[item.key],
    color: item.color,
  }));

  return (
    <Card sx={{ height: "100%", p: 2 }}>
      {/* Title */}
      <Typography variant="h6" component="h3" sx={{ mb: 2 }}>
        Active cases
      </Typography>
      {/* Loading state */}
      {isLoading ? (
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
      ) : (
        <Box sx={{ height: 240, position: "relative" }}>
          <ResponsiveContainer width="100%" height="100%">
            {/* Pie chart */}
            <PieChart legend={{ show: false }} tooltip={{ show: true }}>
              <Pie
                data={chartData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={80}
                paddingAngle={5}
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
                    stroke={colors.common.white}
                  />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          {/* Total cases */}
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
            <Typography variant="h4">{data.total}</Typography>
            <Typography variant="caption">Total</Typography>
          </Box>
        </Box>
      )}
      {/* Loading state */}
      {isLoading ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            gap: 2,
            mt: 2,
          }}
        >
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} variant="rounded" width={80} height={20} />
          ))}
        </Box>
      ) : (
        <ChartLegend data={chartData} />
      )}
    </Card>
  );
};
