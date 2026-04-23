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
import { ChartLegend } from "@features/dashboard/components/charts/ChartLegend";
import { OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA } from "@/features/dashboard/constants/dashboard";
import {
  DASHBOARD_CHART_CAPTION_TOTAL,
  DASHBOARD_CHART_DARK_MODE_OPACITY,
  DASHBOARD_CHART_DARK_MODE_SHADE,
  DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_WIDE_PX,
  DASHBOARD_CHART_PIE_AREA_HEIGHT_PX,
  DASHBOARD_CHART_PIE_SKELETON_SIZE_PX,
  DASHBOARD_CHART_TITLE_OUTSTANDING_ENGAGEMENTS,
} from "@/features/dashboard/constants/charts";
import { useDarkMode } from "@utils/useDarkMode";
import type { CasesTrendChartProps } from "@/features/dashboard/types/charts";
import {
  EMPTY_CASES_TREND_DATA,
  buildEngagementsPieSlices,
  formatEngagementsCenterTotal,
  resolveEngagementsNumericTotal,
} from "@features/dashboard/utils/dashboardCharts";

function normalizeCategory(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s_-]+/g, "-");
}

/**
 * Displays the cases trend chart.
 *
 * @param props - Component props
 * @returns {JSX.Element} Outstanding Engagements chart.
 */
export const CasesTrendChart = ({
  data,
  isLoading,
  isError,
  centerContent = false,
}: CasesTrendChartProps): JSX.Element => {
  const isDarkMode = useDarkMode();
  // safe data
  const safeData = data ?? EMPTY_CASES_TREND_DATA;
  // error grey
  const errorGrey = colors.grey?.[300] ?? "#D1D5DB";
  // fallback grey
  const fallbackGrey = colors.grey?.[500] ?? "#9CA3AF";
  // chart data
  const chartData = buildEngagementsPieSlices(
    safeData,
    Boolean(isLoading),
    Boolean(isError),
    errorGrey,
    fallbackGrey,
  );
  const darkModeColorByCategory = useMemo(
    () =>
      new Map<string, string>([
        [
          normalizeCategory("onboarding"),
          colors.blue?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.blue?.[300] ??
            "#93C5FD",
        ],
        [
          normalizeCategory("migration"),
          colors.orange?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.orange?.[300] ??
            "#FDBA74",
        ],
        [
          normalizeCategory("services"),
          colors.green?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.green?.[300] ??
            "#86EFAC",
        ],
        [
          normalizeCategory("follow-up"),
          colors.purple?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.purple?.[300] ??
            "#D8B4FE",
        ],
        [
          normalizeCategory("improvements"),
          colors.brown?.[DASHBOARD_CHART_DARK_MODE_SHADE] ??
            colors.brown?.[300] ??
            "#D6BFA8",
        ],
      ]),
    [],
  );
  const displayChartData = isDarkMode
    ? chartData.map((entry) => ({
        ...entry,
        color:
          darkModeColorByCategory.get(normalizeCategory(entry.name)) ??
          entry.color,
      }))
    : chartData;

  // total
  const total = resolveEngagementsNumericTotal(
    Boolean(isError),
    Boolean(isLoading),
    safeData.total,
  );

  // center value
  const centerValue = formatEngagementsCenterTotal(
    Boolean(isError),
    chartData.length,
    total,
  );

  const isEmpty = !isLoading && !isError && !!data && safeData.total === 0;

  return (
    <Card sx={{ p: 2, height: "100%" }}>
      <Typography
        variant="h6"
        component="h3"
        sx={{ mb: 2, textAlign: centerContent ? "center" : "left" }}
      >
        {DASHBOARD_CHART_TITLE_OUTSTANDING_ENGAGEMENTS}
      </Typography>
      {isLoading ? (
        <>
          <Box
            sx={{
              height: DASHBOARD_CHART_PIE_AREA_HEIGHT_PX,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Skeleton
              variant="circular"
              width={DASHBOARD_CHART_PIE_SKELETON_SIZE_PX}
              height={DASHBOARD_CHART_PIE_SKELETON_SIZE_PX}
            />
          </Box>
          <Box
            sx={{
              display: "flex",
              justifyContent: "center",
              gap: 2,
              mt: 2,
            }}
          >
            {OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA.map((_, index) => (
              <Skeleton
                key={index}
                variant="rounded"
                width={DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_WIDE_PX}
                height={20}
              />
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
            }}
          >
            <ResponsiveContainer width="100%" height="100%">
              <PieChart
                legend={{ show: false }}
                tooltip={{ show: !isError, wrapperStyle: { zIndex: 1000 } }}
              >
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
                >
                  {displayChartData.map((entry, index) => (
                    <Cell
                      key={`cell-${index}`}
                      fill={entry.color}
                      stroke="none"
                      opacity={isDarkMode ? DASHBOARD_CHART_DARK_MODE_OPACITY : 1}
                    />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
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
                  <Typography variant="h4" color="text.disabled">{centerValue}</Typography>
                  <Typography variant="caption" color="text.disabled">{DASHBOARD_CHART_CAPTION_TOTAL}</Typography>
                </>
              ) : (
                <>
                  <Typography variant="h4">{centerValue}</Typography>
                  <Typography variant="caption">{DASHBOARD_CHART_CAPTION_TOTAL}</Typography>
                </>
              )}
            </Box>
          </Box>
          <Box
            sx={
              centerContent
                ? { maxWidth: 420, width: "100%", mx: "auto" }
                : undefined
            }
          >
            <ChartLegend
              data={displayChartData.map((item) => ({
                name: item.name,
                value: item.value,
                color: item.color,
                id: item.id ?? item.name,
              }))}
              isError={isError}
              showValues
            />
          </Box>
        </>
      )}
    </Card>
  );
};
