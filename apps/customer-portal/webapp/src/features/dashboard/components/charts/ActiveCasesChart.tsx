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

import { Card, Typography, Box, Skeleton, colors, alpha } from "@wso2/oxygen-ui";
import { Inbox } from "@wso2/oxygen-ui-icons-react";
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
} from "@wso2/oxygen-ui-charts-react";
import { useMemo, type JSX } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { ChartLegend } from "@features/dashboard/components/charts/ChartLegend";
import {
  DASHBOARD_CHART_CAPTION_TOTAL,
  DASHBOARD_CHART_DARK_MODE_OPACITY,
  DASHBOARD_CHART_DARK_MODE_SHADE,
  DASHBOARD_CHART_ERROR_ENTITY_ACTIVE_CASES,
  DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_WIDE_PX,
  DASHBOARD_CHART_PIE_AREA_HEIGHT_PX,
  DASHBOARD_CHART_PIE_SKELETON_SIZE_PX,
  DASHBOARD_CHART_TITLE_OUTSTANDING_OPERATIONS,
} from "@/features/dashboard/constants/charts";
import { useDarkMode } from "@utils/useDarkMode";
import {
  OperationsChartMode,
  type ActiveCasesChartProps,
} from "@/features/dashboard/types/charts";
import {
  EMPTY_ACTIVE_CASES_DATA,
  buildActiveCasesLegendRows,
  buildActiveCasesPieSlices,
  formatActiveCasesCenterTotal,
  resolveActiveCasesSeriesConfig,
} from "@features/dashboard/utils/dashboardCharts";

/**
 * Renders the Active Cases chart.
 *
 * @param data - Dataset used to render the chart.
 * @param isLoading - Indicates whether the chart is in a loading state.
 * @returns {JSX.Element} Active Cases chart.
 */
export const ActiveCasesChart = ({
  data,
  isLoading,
  isError,
  variant = OperationsChartMode.SrAndCr,
  centerContent = false,
  onSliceClick,
}: ActiveCasesChartProps): JSX.Element => {
  const isDarkMode = useDarkMode();
  // safe data
  const safeData = data ?? EMPTY_ACTIVE_CASES_DATA;
  // series config
  const seriesConfig = resolveActiveCasesSeriesConfig(variant);
  // error grey
  const errorGrey = colors.grey?.[300] ?? "#D1D5DB";
  // chart data
  const chartData = buildActiveCasesPieSlices(
    seriesConfig,
    safeData,
    Boolean(isLoading),
    Boolean(isError),
    errorGrey,
  );
  const darkModeColorByName = useMemo(
    () =>
      new Map<string, string>([
        [
          "Service Requests (SR)",
          colors.orange?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.orange?.[300] ??
            "#FDBA74",
        ],
        [
          "Change Requests (CR)",
          colors.blue?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.blue?.[300] ??
            "#93C5FD",
        ],
      ]),
    [],
  );
  const displayChartData = isDarkMode
    ? chartData.map((entry) => ({
        ...entry,
        color: darkModeColorByName.get(entry.name) ?? entry.color,
      }))
    : chartData;
  const displayLegendData = isDarkMode
    ? buildActiveCasesLegendRows(seriesConfig, safeData).map((entry) => ({
        ...entry,
        color: darkModeColorByName.get(entry.name) ?? entry.color,
      }))
    : buildActiveCasesLegendRows(seriesConfig, safeData);

  const isEmpty = !isLoading && !isError && !!data && safeData.total === 0;

  return (
    <Card sx={{ height: "100%", p: 2 }}>
      <Typography
        variant="h6"
        component="h3"
        sx={{ mb: 2, textAlign: centerContent ? "center" : "left" }}
      >
        {DASHBOARD_CHART_TITLE_OUTSTANDING_OPERATIONS}
      </Typography>
      {isLoading ? (
        <>
          <Box sx={{ height: DASHBOARD_CHART_PIE_AREA_HEIGHT_PX, display: "flex", alignItems: "center", justifyContent: "center" }}>
            <Skeleton variant="circular" width={DASHBOARD_CHART_PIE_SKELETON_SIZE_PX} height={DASHBOARD_CHART_PIE_SKELETON_SIZE_PX} />
          </Box>
          <Box sx={{ display: "flex", justifyContent: "center", gap: 2, mt: 2 }}>
            {seriesConfig.map((_, i) => (
              <Skeleton key={i} variant="rounded" width={DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_WIDE_PX} height={20} />
            ))}
          </Box>
        </>
      ) : isEmpty ? (
        <Box sx={{ height: DASHBOARD_CHART_PIE_AREA_HEIGHT_PX + 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 1.5 }}>
          <Box sx={{ width: 52, height: 52, borderRadius: "50%", bgcolor: alpha(colors.grey?.[500] ?? "#6B7280", 0.08), display: "flex", alignItems: "center", justifyContent: "center" }}>
            <Inbox size={24} color={colors.grey?.[400] ?? "#9CA3AF"} />
          </Box>
          <Typography variant="body2" color="text.disabled">No data found</Typography>
        </Box>
      ) : (
        <>
          <Box
            sx={{
              height: DASHBOARD_CHART_PIE_AREA_HEIGHT_PX,
              position: "relative",
              opacity: isError ? 0.3 : 1,
              filter: isError ? "grayscale(1)" : "none",
              "& *:focus": { outline: "none" },
              ...(onSliceClick && !isError && { "& .recharts-pie-sector": { cursor: "pointer" } }),
            }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <PieChart legend={{ show: false }} tooltip={{ show: !isError, wrapperStyle: { zIndex: 1000 } }}>
                <Pie
                  data={displayChartData}
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
                  onClick={
                    onSliceClick && !isError
                      ? (_data, index) => {
                          const key = seriesConfig[index]?.key;
                          if (key) onSliceClick(key);
                        }
                      : undefined
                  }
                >
                  {displayChartData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} stroke="none" opacity={isDarkMode ? DASHBOARD_CHART_DARK_MODE_OPACITY : 1} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <Box sx={{ position: "absolute", top: "50%", left: "50%", transform: "translate(-50%, -50%)", textAlign: "center", pointerEvents: "none" }}>
              {isError ? (
                <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
                  <ErrorIndicator entityName={DASHBOARD_CHART_ERROR_ENTITY_ACTIVE_CASES} />
                  <Typography variant="caption">{DASHBOARD_CHART_CAPTION_TOTAL}</Typography>
                </Box>
              ) : (
                <>
                  <Typography variant="h4">{formatActiveCasesCenterTotal(Boolean(data), safeData.total)}</Typography>
                  <Typography variant="caption">{DASHBOARD_CHART_CAPTION_TOTAL}</Typography>
                </>
              )}
            </Box>
          </Box>
          <Box sx={centerContent ? { maxWidth: 420, width: "100%", mx: "auto" } : undefined}>
            <ChartLegend
              data={displayLegendData}
              isError={isError}
              showValues
              onItemClick={onSliceClick ? (id) => onSliceClick(id) : undefined}
            />
          </Box>
        </>
      )}
    </Card>
  );
};
