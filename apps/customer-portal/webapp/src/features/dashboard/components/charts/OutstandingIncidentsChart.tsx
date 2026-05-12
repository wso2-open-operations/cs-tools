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
import { useMemo, useState, type JSX } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { ChartLegend } from "@features/dashboard/components/charts/ChartLegend";
import {
  DASHBOARD_CHART_CAPTION_TOTAL,
  DASHBOARD_CHART_DARK_MODE_OPACITY,
  DASHBOARD_CHART_DARK_MODE_SHADE,
  DASHBOARD_CHART_ERROR_ENTITY_OUTSTANDING_CASES,
  DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_INCIDENTS_PX,
  DASHBOARD_CHART_PIE_AREA_HEIGHT_PX,
  DASHBOARD_CHART_PIE_SKELETON_SIZE_PX,
  DASHBOARD_CHART_TITLE_OUTSTANDING_CASES,
} from "@/features/dashboard/constants/charts";
import { useDarkMode } from "@utils/useDarkMode";
import type { OutstandingIncidentsChartProps } from "@/features/dashboard/types/charts";
import {
  EMPTY_OUTSTANDING_INCIDENTS_DATA,
  buildOutstandingIncidentsLegendRows,
  buildOutstandingIncidentsPieSlices,
  formatOutstandingIncidentsCenterTotal,
  resolveOutstandingIncidentsChartSource,
} from "@features/dashboard/utils/dashboardCharts";
import { SeverityLegendKey } from "@features/dashboard/types/dashboard";

/**
 * Displays the Outstanding Incidents chart.
 *
 * @param props - Component props
 * @param props.data - Array of data points for outstanding incidents.
 * @param props.isLoading - Flag indicating if the chart data is still loading.
 * @returns {JSX.Element} Outstanding Incidents chart.
 */
export const OutstandingIncidentsChart = ({
  data,
  isLoading,
  isError,
  excludeS0 = false,
  restrictSeverityToLow = false,
  centerContent = false,
  onSeverityClick,
}: OutstandingIncidentsChartProps): JSX.Element => {
  const isDarkMode = useDarkMode();
  const [activePieIndex, setActivePieIndex] = useState<number | undefined>(undefined);
  // safe data
  const safeData = data ?? EMPTY_OUTSTANDING_INCIDENTS_DATA;
  const displayedData = restrictSeverityToLow
    ? {
        ...safeData,
        critical: 0,
        high: 0,
        medium: 0,
        catastrophic: 0,
        total: safeData.low,
      }
    : safeData;
  // chart source
  const chartSource = resolveOutstandingIncidentsChartSource(
    excludeS0,
    restrictSeverityToLow,
  );
  const darkModeSeverityColorByKey = useMemo(
    () =>
      new Map<SeverityLegendKey, string>([
        [
          SeverityLegendKey.Catastrophic,
          colors.red?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.red?.[300] ??
            "#e57373",
        ],
        [
          SeverityLegendKey.Critical,
          colors.orange?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.orange?.[300] ??
            "#FDBA74",
        ],
        [
          SeverityLegendKey.High,
          colors.yellow?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.yellow?.[300] ??
            "#FDE047",
        ],
        [
          SeverityLegendKey.Medium,
          colors.blue?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.blue?.[300] ??
            "#93C5FD",
        ],
        [
          SeverityLegendKey.Low,
          colors.green?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.green?.[300] ??
            "#86EFAC",
        ],
      ]),
    [],
  );
  const darkModeChartSource = chartSource.map((item) => ({
    ...item,
    color: darkModeSeverityColorByKey.get(item.key) ?? item.color,
  }));
  const displayChartSource = isDarkMode ? darkModeChartSource : chartSource;
  // error grey
  const errorGrey = colors.grey?.[300] ?? "#D1D5DB";

  // chart data
  const chartData = buildOutstandingIncidentsPieSlices(
    displayChartSource,
    displayedData,
    Boolean(isLoading),
    Boolean(isError),
    errorGrey,
  );
  const displayLegendData = buildOutstandingIncidentsLegendRows(
    displayChartSource,
    displayedData,
  );

  const isEmpty = !isLoading && !isError && !!data && displayedData.total === 0;

  return (
    <Card sx={{ height: "100%", p: 2 }}>
      <Typography
        variant="h6"
        component="h3"
        sx={{ mb: 2, textAlign: centerContent ? "center" : "left" }}
      >
        {DASHBOARD_CHART_TITLE_OUTSTANDING_CASES}
      </Typography>
      {isLoading ? (
        <>
          <Box sx={{ height: DASHBOARD_CHART_PIE_AREA_HEIGHT_PX, display: "flex", alignItems: "center", justifyContent: "center" }}>
            <Skeleton variant="circular" width={DASHBOARD_CHART_PIE_SKELETON_SIZE_PX} height={DASHBOARD_CHART_PIE_SKELETON_SIZE_PX} />
          </Box>
          <Box sx={{ display: "flex", justifyContent: "center", gap: 2, mt: 2 }}>
            {chartSource.map((_, i) => (
              <Skeleton key={i} variant="rounded" width={DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_INCIDENTS_PX} height={20} />
            ))}
          </Box>
        </>
      ) : isEmpty ? (
        <Box sx={{ height: DASHBOARD_CHART_PIE_AREA_HEIGHT_PX + 64, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 1.5 }}>
          <Box sx={{ width: 52, height: 52, borderRadius: "50%", bgcolor: alpha(colors.grey?.[500] ?? "#6B7280", 0.08), display: "flex", alignItems: "center", justifyContent: "center" }}>
            <Inbox size={24} color={colors.grey?.[400] ?? "#9CA3AF"} />
          </Box>
          <Typography variant="body2" color="text.disabled">No {DASHBOARD_CHART_TITLE_OUTSTANDING_CASES} found</Typography>
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
              ...(onSeverityClick && !isError && {
                "& .recharts-pie-sector": { cursor: "pointer" },
              }),
            }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <PieChart legend={{ show: false }} tooltip={{ show: !isError, wrapperStyle: { zIndex: 1000 } }}>
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
                  onMouseEnter={
                    onSeverityClick && !isError
                      ? (_data: unknown, index: number) => setActivePieIndex(index)
                      : undefined
                  }
                  onMouseLeave={onSeverityClick && !isError ? () => setActivePieIndex(undefined) : undefined}
                  onClick={
                    onSeverityClick && !isError
                      ? (_data, index) => {
                          const entry = chartData[index];
                          if (entry?.id) onSeverityClick(entry.id);
                        }
                      : undefined
                  }
                >
                  {chartData.map((entry, index) => (
                    <Cell
                      key={`cell-${index}`}
                      fill={entry.color}
                      stroke={activePieIndex === index ? entry.color : "none"}
                      strokeWidth={activePieIndex === index ? 3 : 0}
                      opacity={isDarkMode ? DASHBOARD_CHART_DARK_MODE_OPACITY : 1}
                    />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
            <Box sx={{ position: "absolute", top: "50%", left: "50%", transform: "translate(-50%, -50%)", textAlign: "center", pointerEvents: "none" }}>
              {isError ? (
                <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
                  <ErrorIndicator entityName={DASHBOARD_CHART_ERROR_ENTITY_OUTSTANDING_CASES} />
                  <Typography variant="caption">{DASHBOARD_CHART_CAPTION_TOTAL}</Typography>
                </Box>
              ) : (
                <>
                  <Typography variant="h4">{formatOutstandingIncidentsCenterTotal(Boolean(data), displayedData.total)}</Typography>
                  <Typography variant="caption">{DASHBOARD_CHART_CAPTION_TOTAL}</Typography>
                </>
              )}
            </Box>
          </Box>
          <Box sx={centerContent ? { maxWidth: 420, width: "100%", mx: "auto" } : undefined}>
            <ChartLegend data={displayLegendData} isError={isError} showValues onItemClick={onSeverityClick} />
          </Box>
        </>
      )}
    </Card>
  );
};
