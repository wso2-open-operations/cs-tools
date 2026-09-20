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
import { Cell, Pie, PieChart } from "@wso2/oxygen-ui-charts-react";
import { useState, type JSX, type KeyboardEvent, type SyntheticEvent } from "react";
import type { BeWidgetPaletteColor } from "@api/backend/types";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { useDarkMode } from "@utils/useDarkMode";

const CHART_SIZE_PX = 180;
const INNER_RADIUS_PX = 62;
const OUTER_RADIUS_PX = 88;

/** `inlineLabels` mode's own chart box — bigger than `CHART_SIZE_PX` because
 * labels now live OUTSIDE the ring (leader line + text) instead of in a
 * separate side legend list, so the ring itself has to shrink to leave room
 * for that outer label band on every edge, not just to the right. */
const INLINE_LABELS_CHART_SIZE_PX = 320;
const INLINE_LABELS_INNER_RADIUS_PX = 46;
const INLINE_LABELS_OUTER_RADIUS_PX = 66;
/** How far past `INLINE_LABELS_OUTER_RADIUS_PX` the leader line's own elbow
 * (the point it turns horizontal before reaching the text) sits. */
const LABEL_LEADER_ELBOW_PX = 14;
/** Extra horizontal offset from the elbow to the text itself (a short flat
 * run, same shape as the reference design's own leader lines). */
const LABEL_LEADER_RUN_PX = 10;

/** Used in order when a slice's own config omits `color`. */
const DEFAULT_COLOR_ROTATION: BeWidgetPaletteColor[] = [
  "primary",
  "warning",
  "secondary",
  "info",
  "success",
  "error",
];

interface DashboardPieChartProps {
  slices: PieSliceResult[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  onSliceClick: (slice: PieSliceResult) => void;
  /** Opt-in alternate rendering: each slice's own "{label} {value}" drawn
   * outside the ring with a leader line back to its wedge, instead of the
   * default donut + separate legend list below it. Absent/`false` renders
   * byte-for-byte as before this prop existed — every existing pie widget
   * across every other dashboard is unaffected. See
   * `BeDashboardWidget.inlineLabels`. */
  inlineLabels?: boolean;
}

/** One entry of the custom outer-label render function's own return value —
 * recharts calls `label` once per slice with its own geometry (`cx`/`cy`/
 * `midAngle`/`innerRadius`/`outerRadius`/...), not with this app's own
 * `PieSliceResult` — that's threaded in separately via closure below. */
interface RechartsPieLabelGeometry {
  // recharts' own `PieLabelRenderProps` types every one of these as
  // `string | number | undefined` (a `Coordinate`/SVG-attribute shape it
  // shares with other chart primitives, not pie-label-specific) even though
  // a real pie-label render call always supplies plain numbers — matching
  // that wider, optional shape here (rather than requiring `number`) is what
  // makes this function assignable to the library's `label` prop type at
  // all; see the destructure below for the runtime `Number(...)` coercion
  // and fallback.
  cx?: string | number;
  cy?: string | number;
  midAngle?: string | number;
  innerRadius?: string | number;
  outerRadius?: string | number;
  index?: number;
}

/**
 * Donut chart + legend for a `shape: "pie"` dashboard widget — a fixed
 * center total, one wedge (and one legend row) per slice, both clickable
 * through to that slice's own filtered case list (see
 * `DashboardWidgetTile`'s pie branch for how the destination href is
 * built). A total of 0 renders an empty state (same treatment as the
 * customer-portal app's own dashboard charts) instead of an all-grey ring.
 */
export default function DashboardPieChart({
  slices,
  total,
  isLoading,
  isError,
  onSliceClick,
  inlineLabels,
}: DashboardPieChartProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const [activeIndex, setActiveIndex] = useState<number | undefined>(undefined);
  const chartSize = inlineLabels ? INLINE_LABELS_CHART_SIZE_PX : CHART_SIZE_PX;
  const innerRadius = inlineLabels ? INLINE_LABELS_INNER_RADIUS_PX : INNER_RADIUS_PX;
  const outerRadius = inlineLabels ? INLINE_LABELS_OUTER_RADIUS_PX : OUTER_RADIUS_PX;

  const colorFor = (slice: PieSliceResult, i: number): string => {
    const key = slice.color ?? DEFAULT_COLOR_ROTATION[i % DEFAULT_COLOR_ROTATION.length];
    return theme.palette[key].main;
  };

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", alignItems: "center", gap: 3 }}>
        <Skeleton variant="circular" width={CHART_SIZE_PX} height={CHART_SIZE_PX} sx={{ flexShrink: 0 }} />
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1 }}>
          {slices.map((slice) => (
            <Skeleton key={slice.label} variant="rounded" height={20} />
          ))}
        </Box>
      </Box>
    );
  }

  if (isError) {
    return (
      <Typography variant="body2" color="text.secondary">
        Could not load this widget.
      </Typography>
    );
  }

  if (total === 0) {
    return (
      <Box
        sx={{
          minHeight: CHART_SIZE_PX,
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
          {/* text.disabled/text.secondary render too close to the
              background's own color in this app's dark theme (same issue
              fixed for the bar chart's value labels) — grey[400] is legible
              against a dark background without looking harsh in light mode. */}
          <Inbox size={24} color={isDarkMode ? theme.palette.grey[400] : theme.palette.grey[500]} />
        </Box>
        <Typography variant="body2" sx={{ color: isDarkMode ? theme.palette.grey[400] : theme.palette.text.disabled }}>
          Nothing to show here right now
        </Typography>
      </Box>
    );
  }

  const chartData = slices.map((slice) => ({ name: slice.label, value: slice.value }));

  // `inlineLabels` mode's own custom outer-label renderer — standard
  // recharts "customized pie label" recipe: compute the label's own point
  // from the wedge's midAngle just past outerRadius, an elbowed leader line
  // back to the ring, text-anchor flipped between "start"/"end" depending on
  // which half of the circle the point falls on. No percentage in the
  // visible text (just "{label} {value}") — that's this widget's own
  // reference design, unlike the legend rows' "{value} ({pct}%)".
  //
  // This label is ALSO this slice's sole accessible/keyboard-operable
  // target in this mode (role="button", tabIndex, Enter/Space) — there is
  // no separate legend row to carry that in inlineLabels mode, and a
  // recharts wedge's own <path> is not natively focusable. The leader
  // line itself carries no semantics of its own (aria-hidden) so a screen
  // reader announces the slice exactly once, via this label, rather than
  // once per decorative line segment.
  const renderOuterLabel = (props: RechartsPieLabelGeometry): JSX.Element | null => {
    const { index = -1 } = props;
    const slice = slices[index];
    if (!slice) return null;
    const cx = Number(props.cx ?? 0);
    const cy = Number(props.cy ?? 0);
    const midAngle = Number(props.midAngle ?? 0);
    const sliceOuterRadius = Number(props.outerRadius ?? outerRadius);
    const RADIAN = Math.PI / 180;
    const cos = Math.cos(-midAngle * RADIAN);
    const sin = Math.sin(-midAngle * RADIAN);
    const startX = cx + sliceOuterRadius * cos;
    const startY = cy + sliceOuterRadius * sin;
    const elbowX = cx + (sliceOuterRadius + LABEL_LEADER_ELBOW_PX) * cos;
    const elbowY = cy + (sliceOuterRadius + LABEL_LEADER_ELBOW_PX) * sin;
    const isRightHalf = cos >= 0;
    const endX = elbowX + (isRightHalf ? LABEL_LEADER_RUN_PX : -LABEL_LEADER_RUN_PX);
    const pct = total > 0 ? Math.round((slice.value / total) * 100) : 0;
    const color = colorFor(slice, index);
    return (
      <g key={slice.label}>
        <path
          aria-hidden="true"
          d={`M${startX},${startY}L${elbowX},${elbowY}L${endX},${elbowY}`}
          stroke={color}
          fill="none"
        />
        <text
          x={endX + (isRightHalf ? 4 : -4)}
          y={elbowY}
          textAnchor={isRightHalf ? "start" : "end"}
          dominantBaseline="central"
          fontSize={11}
          fill={theme.palette.text.primary}
          role="button"
          tabIndex={0}
          aria-label={`${slice.label}: ${slice.value} cases (${pct}%)`}
          style={{ cursor: "pointer", outline: "none" }}
          onMouseEnter={() => setActiveIndex(index)}
          onMouseLeave={() => setActiveIndex(undefined)}
          onFocus={() => setActiveIndex(index)}
          onBlur={() => setActiveIndex(undefined)}
          // Same stopPropagation rationale as the wedge's own onClick below
          // — this label sits inside the tile-level click-through
          // `DashboardWidgetTile` attaches for shape "pie"/"bar".
          onClick={(event: SyntheticEvent) => {
            event.stopPropagation();
            onSliceClick(slice);
          }}
          onKeyDown={(event: KeyboardEvent) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              event.stopPropagation();
              onSliceClick(slice);
            }
          }}
        >
          {`${slice.label} ${slice.value}`}
        </text>
      </g>
    );
  };

  return (
    <Box
      sx={
        inlineLabels
          ? { display: "flex", justifyContent: "center" }
          : { display: "flex", alignItems: "center", gap: 3, flexWrap: "wrap" }
      }
    >
      <Box
        sx={{
          position: "relative",
          width: chartSize,
          height: chartSize,
          flexShrink: 0,
          "& .recharts-pie-sector": { cursor: "pointer" },
        }}
      >
        {/* wrapperStyle's zIndex matches the same fix the customer-portal
            app's own dashboard charts use (see OutstandingIncidentsChart.tsx)
            — without it the tooltip can render underneath/get clipped by
            this tile's own header, looking cut off / see-through.
            margin: PieChart's own default is {top:12,right:24,left:24,
            bottom:40} — the large bottom value reserves room for a legend,
            which is disabled here. Left uncleared, that asymmetric margin
            shifts the plot's (and so the ring's) center downward within the
            180x180 box, making the top of the donut look pushed toward /
            cut off by this tile's own header above it. */}
        <PieChart
          // Explicit pixel size, not "100%": the parent Box just above is
          // already a fixed chartSize square, not a responsive/percentage
          // container, so there's nothing for a percentage width/height to
          // measure against except an internal ResizeObserver callback --
          // that callback fires one tick after first paint, so the chart's
          // very first render sees width/height as -1 (a real, harmless but
          // noisy console warning: "The width(-1) and height(-1) of chart
          // should be greater than 0"). Passing the known constant directly
          // sizes it synchronously on the first render, no observer race.
          width={chartSize}
          height={chartSize}
          legend={{ show: false }}
          margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
          tooltip={{ show: true, wrapperStyle: { zIndex: 1000 } }}
        >
          <Pie
            data={chartData}
            cx="50%"
            cy="50%"
            innerRadius={innerRadius}
            outerRadius={outerRadius}
            paddingAngle={0}
            dataKey="value"
            nameKey="name"
            startAngle={90}
            endAngle={-270}
            label={inlineLabels ? renderOuterLabel : false}
            labelLine={false}
            onMouseEnter={(_data: unknown, i: number) => setActiveIndex(i)}
            onMouseLeave={() => setActiveIndex(undefined)}
            // Stops the click from also bubbling up to the tile-level
            // click-through `DashboardWidgetTile` attaches to the whole
            // card for `shape: "pie"`/`"bar"` — without this, clicking a
            // wedge would navigate to the slice's own filtered list AND
            // then (via bubbling) immediately re-navigate to the tile's
            // base-filtered list.
            onClick={(_data: unknown, i: number, event?: SyntheticEvent) => {
              event?.stopPropagation();
              const slice = slices[i];
              if (slice) onSliceClick(slice);
            }}
          >
            {slices.map((slice, i) => (
              <Cell
                key={slice.label}
                fill={colorFor(slice, i)}
                stroke={activeIndex === i ? colorFor(slice, i) : "none"}
                strokeWidth={activeIndex === i ? 3 : 0}
              />
            ))}
          </Pie>
        </PieChart>
        <Box
          sx={{
            position: "absolute",
            inset: 0,
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            pointerEvents: "none",
          }}
        >
          <Typography variant="h5">{total}</Typography>
          <Typography variant="caption" color="text.secondary">
            Total
          </Typography>
        </Box>
      </Box>

      {!inlineLabels && (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75, flex: 1, minWidth: 180 }}>
        {slices.map((slice, i) => {
          const pct = total > 0 ? Math.round((slice.value / total) * 100) : 0;
          return (
            <Box
              key={slice.label}
              role="button"
              tabIndex={0}
              aria-label={`${slice.label}: ${slice.value} cases (${pct}%)`}
              // stopPropagation for the same reason as the wedge's own
              // onClick above — this row sits inside the tile-level
              // click-through `DashboardWidgetTile` attaches for
              // shape "pie"/"bar".
              onClick={(event) => {
                event.stopPropagation();
                onSliceClick(slice);
              }}
              onKeyDown={(event: KeyboardEvent) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  event.stopPropagation();
                  onSliceClick(slice);
                }
              }}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                px: 0.5,
                py: 0.25,
                borderRadius: 1,
                cursor: "pointer",
                "&:hover": { bgcolor: "action.hover" },
                "&:focus-visible": {
                  outline: `2px solid ${theme.palette.primary.main}`,
                  outlineOffset: -2,
                },
              }}
            >
              <Box
                sx={{
                  width: 10,
                  height: 10,
                  borderRadius: "50%",
                  bgcolor: colorFor(slice, i),
                  flexShrink: 0,
                }}
              />
              <Typography variant="body2" sx={{ flex: 1 }} noWrap>
                {slice.label}
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {slice.value} ({pct}%)
              </Typography>
            </Box>
          );
        })}
      </Box>
      )}
    </Box>
  );
}
