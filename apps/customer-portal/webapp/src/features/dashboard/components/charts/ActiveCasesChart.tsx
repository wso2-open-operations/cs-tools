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
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { ChartLegend } from "@features/dashboard/components/charts/ChartLegend";
import { ACTIVE_CASES_CHART_DATA } from "@features/dashboard/constants/dashboardConstants";

export type OperationsChartMode = "srAndCr" | "srOnly";

interface ActiveCasesChartProps {
  data: {
    serviceRequests: number;
    changeRequests: number;
    total: number;
  };
  isLoading?: boolean;
  isError?: boolean;
  variant?: OperationsChartMode;
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
  isError,
  variant = "srAndCr",
}: ActiveCasesChartProps): JSX.Element => {
  const safeData = data ?? {
    serviceRequests: 0,
    changeRequests: 0,
    total: 0,
  };

  const seriesConfig =
    variant === "srOnly"
      ? ACTIVE_CASES_CHART_DATA.filter((item) => item.key === "serviceRequests")
      : ACTIVE_CASES_CHART_DATA;

  const chartData = isError
    ? seriesConfig.map((item) => ({
        name: item.name,
        value: 1,
        color: colors.grey?.[300] ?? "#D1D5DB",
      }))
    : isLoading
      ? []
      : seriesConfig.map((item) => ({
          name: item.name,
          value: safeData[item.key as keyof typeof safeData] ?? 0,
          color: item.color,
        }));

  return (
    <Card sx={{ height: "100%", p: 2 }}>
      {/* Title */}
      <Typography variant="h6" component="h3" sx={{ mb: 2 }}>
        Outstanding Operations
      </Typography>
      {/* Chart state */}
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
        <Box
          sx={{
            height: 240,
            position: "relative",
            opacity: isError ? 0.3 : 1,
            filter: isError ? "grayscale(1)" : "none",
            "& *:focus": { outline: "none" },
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
            {/* Center content (Total count or Error indicator) */}
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
                <Box
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                  }}
                >
                  <ErrorIndicator entityName="active cases" />
                  <Typography variant="caption">Total</Typography>
                </Box>
              ) : (
                <>
                  <Typography variant="h4">
                    {data ? safeData.total : "N/A"}
                  </Typography>
                  <Typography variant="caption">Total</Typography>
                </>
              )}
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
          {seriesConfig.map((_, i) => (
            <Skeleton key={i} variant="rounded" width={80} height={20} />
          ))}
        </Box>
      ) : (
        <ChartLegend
          data={seriesConfig.map((item) => ({
            name: item.name,
            value: safeData[item.key as keyof typeof safeData] ?? 0,
            color: item.color,
          }))}
          isError={isError}
          showValues
        />
      )}
    </Card>
  );
};
