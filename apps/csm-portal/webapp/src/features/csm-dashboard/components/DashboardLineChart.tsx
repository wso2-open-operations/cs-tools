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

import { Box, Skeleton, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Inbox } from "@wso2/oxygen-ui-icons-react";
import { BarChart, LineChart, XAxis } from "@wso2/oxygen-ui-charts-react";
import type { JSX, SyntheticEvent } from "react";
import type { CaseVolumeTrendChartPoint } from "@features/csm-dashboard/api/useCaseVolumeTrendData";
import type { BeLineBucket, BeLineChart, BeLineSeries } from "@api/backend/types";
import { useDarkMode } from "@utils/useDarkMode";

const CHART_HEIGHT_PX = 220;

interface DashboardLineChartProps {
  points: CaseVolumeTrendChartPoint[];
  isLoading: boolean;
  isError: boolean;
  /** The lines to draw (`BeDashboardWidget.series`); `undefined` or empty
   * draws both. Drawn in the fixed order Opened, Closed whatever the order
   * given. */
  series?: BeLineSeries[];
  /** How the points are grouped (`BeDashboardWidget.bucket`), which decides
   * the x-axis labels; `undefined` is one point per day. */
  bucket?: BeLineBucket;
  /** How the points are drawn (`BeDashboardWidget.chart`): "bar" draws a bar
   * per point and series (Opened and Closed side by side), anything else
   * lines. */
  chart?: BeLineChart;
  /** Makes each point (each bar, with `chart: "bar"`) clickable: called with the point's day (`YYYY-MM-DD`,
   * a UTC calendar day), its line, and for a week or month point its last
   * day. Absent, the points are not clickable, as on a system dashboard. */
  onPointClick?: (date: string, series: BeLineSeries, endDate?: string) => void;
}

/** What recharts hands a function `activeDot` for the hovered point. */
interface ActiveDotProps {
  cx?: number;
  cy?: number;
  index?: number;
}

const ACTIVE_DOT_RADIUS_PX = 5;

const ALL_SERIES: readonly BeLineSeries[] = ["opened", "closed"];

/** Formats a `YYYY-MM-DD` point date for the x-axis — "Sep 1" (no year: a
 * day point's window is capped at 90 days, see `maxWidgetLineDays` on the
 * backend, so a label never needs to disambiguate across years). Pinned to
 * `timeZone: "UTC"` for the same reason `useCaseFeedbackTrendData`'s own
 * `formatBucketLabel` is — the date string is already a UTC calendar day, and
 * reading it back in a negative-UTC-offset timezone without pinning the
 * formatter would roll the label back a day. */
function formatDateLabel(
  date: string,
  options: Intl.DateTimeFormatOptions = { month: "short", day: "numeric" },
): string {
  const parsed = new Date(date);
  if (Number.isNaN(parsed.getTime())) return date;
  return new Intl.DateTimeFormat("en-US", { timeZone: "UTC", ...options }).format(parsed);
}

/** A point's x-axis label: "Sep 1" for a day, the days a week point covers
 * for a week ("Dec 1-7", "Dec 29-Jan 4", "Dec 29-31" when the window cuts
 * it short), and "Jan 2025" for a month (with the year: a month window can
 * span two). */
function formatPointLabel(
  point: CaseVolumeTrendChartPoint,
  bucket: BeLineBucket | undefined,
): string {
  if (bucket === "month") return formatDateLabel(point.date, { month: "short", year: "numeric" });
  if (bucket !== "week" || !point.endDate || point.endDate === point.date) {
    return formatDateLabel(point.date);
  }
  const start = formatDateLabel(point.date);
  const sameMonth = point.endDate.slice(0, 7) === point.date.slice(0, 7);
  const end = sameMonth
    ? formatDateLabel(point.endDate, { day: "numeric" })
    : formatDateLabel(point.endDate);
  return `${start}-${end}`;
}

/**
 * Two-line (Opened/Closed) trend chart for a `shape: "line"` dashboard
 * widget — the one resourceType/shape combination not resolved via a
 * per-slice/per-bucket fan-out (see `useCaseVolumeTrendData`). A point is one
 * day (or week or month, `bucket`) and one line, which the tile turns into that day's cases query
 * (`linePointQuery`) when the page allows it. Colors are this app's own semantic palette (`info` for Opened,
 * `success` for Closed) rather than an arbitrary chart-library rotation —
 * matching how severity/status colors are already assigned by meaning
 * elsewhere in this app, not by picking hues. `series` leaves one of the two
 * lines out (its legend entry too); the empty state then counts only the
 * lines drawn. `onPointClick` makes the hovered point's dot clickable (the
 * dashboard page decides whether to pass it). `chart: "bar"` draws the same
 * points as bars, with the same colors, labels, legend and empty state.
 */
export default function DashboardLineChart({
  points,
  isLoading,
  isError,
  series,
  bucket,
  chart,
  onPointClick,
}: DashboardLineChartProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const shown = ALL_SERIES.filter((s) => !series?.length || series.includes(s));
  const lineStyles = {
    opened: { label: "Opened", color: theme.palette.info.main },
    closed: { label: "Closed", color: theme.palette.success.main },
  } as const;

  if (isLoading) {
    return <Skeleton variant="rounded" height={CHART_HEIGHT_PX} />;
  }

  if (isError) {
    return (
      <Typography variant="body2" color="text.secondary">
        Could not load this widget.
      </Typography>
    );
  }

  const total = points.reduce((sum, p) => sum + shown.reduce((s, key) => s + p[key], 0), 0);

  if (points.length === 0 || total === 0) {
    return (
      <Box sx={{ width: "100%" }}>
        <Box
          sx={{
            minHeight: CHART_HEIGHT_PX,
            width: "100%",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: 1.5,
          }}
        >
          <Box
            sx={{
              width: 52,
              height: 52,
              borderRadius: "50%",
              bgcolor: alpha(theme.palette.grey[500], 0.08),
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Inbox size={24} color={isDarkMode ? theme.palette.grey[400] : theme.palette.grey[500]} />
          </Box>
          <Typography
            variant="body2"
            sx={{ color: isDarkMode ? theme.palette.grey[400] : theme.palette.text.disabled }}
          >
            Nothing to show here right now
          </Typography>
        </Box>
      </Box>
    );
  }

  const chartData = points.map((p) => ({
    label: formatPointLabel(p, bucket),
    opened: p.opened,
    closed: p.closed,
  }));

  return (
    <Box sx={{ width: "100%" }}>
      {/* A hand-rolled legend, not the library's own `legend` prop: that
          renders in an opaque, library-positioned block whose height and
          internal alignment aren't exposed through this wrapper's typed
          props, and in practice reserved far more vertical space than its
          one line of content needed while still crowding the chart's own
          top tick label underneath it. Same "don't trust the library's
          legend layout" precedent DashboardPieChart.tsx's own legend list
          already follows in this codebase — this gives this chart tight,
          predictable spacing instead. */}
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 1 }}>
        {shown.map((key) => (
          <Box key={key} sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <Box
              sx={{
                width: 10,
                height: 10,
                borderRadius: "50%",
                bgcolor: lineStyles[key].color,
                flexShrink: 0,
              }}
            />
            <Typography variant="caption" color="text.secondary">
              {lineStyles[key].label}
            </Typography>
          </Box>
        ))}
      </Box>
      <Box
        sx={{
          width: "100%",
          height: CHART_HEIGHT_PX,
          ...(onPointClick && chart === "bar" && { "& .recharts-bar-rectangle": { cursor: "pointer" } }),
        }}
      >
        {chart === "bar" ? (
          <BarChart
            data={chartData}
            xAxisDataKey="label"
            height={CHART_HEIGHT_PX}
            width="100%"
            legend={{ show: false }}
            grid={{ show: true, strokeDasharray: "3 3" }}
            margin={{ top: 8, right: 8, bottom: 8, left: 0 }}
            tooltip={{ show: true, wrapperStyle: { zIndex: 1000 } }}
            // The library's own x-axis has no `interval`; this one thins the
            // labels to what fits, keeping the first and last, as the lines do.
            xAxis={{ show: false }}
            yAxis={{ show: true }}
            bars={shown.map((key) => ({
              dataKey: key,
              name: lineStyles[key].label,
              fill: lineStyles[key].color,
              radius: [4, 4, 0, 0] as [number, number, number, number],
              isAnimationActive: false,
              // A bar is one point of one series, so a click opens its cases
              // the way a line's point does (`onPointClick`).
              ...(onPointClick && {
                onClick: (_data: unknown, index: number, event?: SyntheticEvent) => {
                  event?.stopPropagation();
                  const point = points[index];
                  if (!point) return;
                  if (point.endDate) onPointClick(point.date, key, point.endDate);
                  else onPointClick(point.date, key);
                },
              }),
            }))}
          >
            <XAxis dataKey="label" interval="preserveStartEnd" tickLine={false} />
          </BarChart>
        ) : (
          <LineChart
            data={chartData}
            xAxisDataKey="label"
            height={CHART_HEIGHT_PX}
            width="100%"
            legend={{ show: false }}
            grid={{ show: true, strokeDasharray: "3 3" }}
            margin={{ top: 8, right: 8, bottom: 8, left: 0 }}
            tooltip={{ show: true, wrapperStyle: { zIndex: 1000 } }}
            // `interval: "preserveStartEnd"` thins the x-axis down to as many
            // evenly-spaced labels as actually fit the rendered width (always
            // keeping the first/last), rather than printing every single day
            // in the window -- which crowds into illegibility well before 30
            // days, and far sooner once this tile shares a row with another
            // widget instead of spanning the full 12-column width.
            xAxis={{ show: true, interval: "preserveStartEnd" }}
            yAxis={{ show: true }}
            lines={shown.map((key) => ({
              dataKey: key,
              name: lineStyles[key].label,
              stroke: lineStyles[key].color,
              strokeWidth: 2,
              dot: false,
              isAnimationActive: false,
              // The hovered point's dot, drawn by this function so a click on
              // it knows both the day (recharts' row `index`) and the line
              // (`key`); the library's object form gets the event only.
              ...(onPointClick && {
                activeDot: ({ cx, cy, index }: ActiveDotProps) => {
                  const point = index === undefined ? undefined : points[index];
                  if (cx === undefined || cy === undefined || !point) return <g />;
                  return (
                    <circle
                      cx={cx}
                      cy={cy}
                      r={ACTIVE_DOT_RADIUS_PX}
                      fill={lineStyles[key].color}
                      stroke={theme.palette.background.paper}
                      strokeWidth={2}
                      style={{ cursor: "pointer" }}
                      data-testid={`line-point-${key}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        if (point.endDate) onPointClick(point.date, key, point.endDate);
                        else onPointClick(point.date, key);
                      }}
                    />
                  );
                },
              }),
            }))}
          />
        )}
      </Box>
    </Box>
  );
}
