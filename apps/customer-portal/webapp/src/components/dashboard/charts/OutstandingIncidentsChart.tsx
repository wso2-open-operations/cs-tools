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
import ErrorIndicator from "@components/common/error-indicator/ErrorIndicator";
import { ChartLegend } from "@components/dashboard/charts/ChartLegend";
import { OUTSTANDING_ENGAGEMENTS_CHART_DATA } from "@constants/dashboardConstants";

interface OutstandingIncidentsChartProps {
  data: {
    low: number;
    medium: number;
    high: number;
    critical: number;
    catastrophic: number;
    serviceRequest: number;
    securityReportAnalysis: number;
    total: number;
  };
  isLoading?: boolean;
  isError?: boolean;
  excludeS0?: boolean;
}

/**
 * Displays the Outstanding Incidents chart.
 *
 * @param props - Component props
 * @param props.data - Array of data points for outstanding incidents.
 * @param props.isLoading - Flag indicating if the chart data is still loading.
 * @returns JSX.Element rendering the Outstanding Incidents chart.
 */
export const OutstandingIncidentsChart = ({
  data,
  isLoading,
  isError,
  excludeS0 = false,
}: OutstandingIncidentsChartProps): JSX.Element => {
  const safeData = data ?? {
    low: 0,
    medium: 0,
    high: 0,
    critical: 0,
    catastrophic: 0,
    serviceRequest: 0,
    securityReportAnalysis: 0,
    total: 0,
  };

  const chartSource = excludeS0
    ? OUTSTANDING_ENGAGEMENTS_CHART_DATA.filter(
        (item) => item.key !== "catastrophic",
      )
    : OUTSTANDING_ENGAGEMENTS_CHART_DATA;

  const chartData = isError
    ? chartSource.map((item) => ({
        name: item.displayName,
        value: 1,
        color: colors.grey?.[300] ?? "#D1D5DB",
      }))
    : isLoading
      ? []
      : chartSource.map((item) => ({
          name: item.displayName,
          value: safeData[item.key as keyof typeof safeData] ?? 0,
          color: item.color,
        }));

  return (
    <Card sx={{ height: "100%", p: 2 }}>
      {/* Title */}
      <Typography variant="h6" component="h3" sx={{ mb: 2 }}>
        Outstanding Engagements
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
            <PieChart legend={{ show: false }} tooltip={{ show: !isError }}>
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
                    stroke={colors.common.white}
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
                <ErrorIndicator entityName="outstanding cases" />
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
          {chartSource.map((_, i) => (
            <Skeleton key={i} variant="rounded" width={60} height={20} />
          ))}
        </Box>
      ) : (
        <ChartLegend
          data={chartSource.map((item) => ({
            name: item.displayName,
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
