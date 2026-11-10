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
import { Bar, BarChart, Cell, XAxis } from "@wso2/oxygen-ui-charts-react";
import { useState, type JSX, type SyntheticEvent } from "react";
import type { BeWidgetPaletteColor } from "@api/backend/types";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { useDarkMode } from "@utils/useDarkMode";

// Taller than a plain single-line-label bar chart would need: the extra
// room (over the library's own default margin.bottom) goes to the two-line
// x-axis labels below — see the custom <XAxis> tick rendering further down.
const CHART_HEIGHT_PX = 248;

/** A category label wraps onto at most this many lines, horizontally (never
 * rotated) — a diagonal label was hard to read at a glance and didn't scale
 * to the dashboard's own two-theme/zoom range well; two short horizontal
 * lines read far more naturally under a bar. */
const MAX_X_AXIS_LABEL_LINES = 2;
/** Characters per wrapped line, word-wrapped where possible — a rough,
 * character-count heuristic (not actual text measurement) matching the
 * conventions used elsewhere in this chart (see the old single-line
 * `truncateAxisLabel` this replaces). Whatever still doesn't fit after two
 * lines is truncated with an ellipsis on the second line — the chart's own
 * `tooltip` always shows the full, untruncated name on hover, so nothing is
 * actually lost, just not all shown at once under every bar. */
const MAX_X_AXIS_LABEL_LINE_CHARS = 11;

/**
 * Word-wraps label onto at most `MAX_X_AXIS_LABEL_LINES` lines of at most
 * `MAX_X_AXIS_LABEL_LINE_CHARS` characters each. A single word longer than a
 * whole line is itself cut to fit rather than left to overflow. Anything
 * left over once both lines are full is dropped, with the last line's own
 * tail replaced by an ellipsis so the cut is visible rather than silent.
 */
function wrapAxisLabel(label: string): string[] {
  const words = label.split(/\s+/).filter(Boolean);
  if (words.length === 0) return [""];

  const lines: string[] = [];
  let current = "";
  let truncated = false;
  let i = 0;

  while (i < words.length && lines.length < MAX_X_AXIS_LABEL_LINES) {
    let word = words[i];
    if (word.length > MAX_X_AXIS_LABEL_LINE_CHARS) {
      word = word.slice(0, MAX_X_AXIS_LABEL_LINE_CHARS);
      truncated = true;
    }
    const candidate = current ? `${current} ${word}` : word;
    if (candidate.length <= MAX_X_AXIS_LABEL_LINE_CHARS) {
      current = candidate;
      i++;
    } else if (current) {
      lines.push(current);
      current = "";
    } else {
      // The (already-capped) word alone still doesn't fit next to nothing —
      // place it on its own line rather than looping forever.
      current = word;
      i++;
    }
  }
  if (current && lines.length < MAX_X_AXIS_LABEL_LINES) {
    lines.push(current);
  }
  if (i < words.length) {
    truncated = true;
  }

  if (truncated && lines.length > 0) {
    const lastIdx = lines.length - 1;
    const last = lines[lastIdx];
    const cut =
      last.length >= MAX_X_AXIS_LABEL_LINE_CHARS ? last.slice(0, MAX_X_AXIS_LABEL_LINE_CHARS - 1) : last;
    lines[lastIdx] = `${cut}…`;
  }
  return lines;
}

/** Used in order when a slice's own config omits `color`. */
const DEFAULT_COLOR_ROTATION: BeWidgetPaletteColor[] = [
  "primary",
  "warning",
  "secondary",
  "info",
  "success",
  "error",
];

interface DashboardBarChartProps {
  slices: PieSliceResult[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  onSliceClick: (slice: PieSliceResult) => void;
  /** How a value reads on its bar and in the tooltip (e.g. `formatHours`
   * for a time card totals widget). Absent shows the plain number, as
   * before this prop existed. */
  formatValue?: (value: number) => string;
}

/**
 * Bar chart for a `shape: "bar"` dashboard widget — one bar per slice
 * (`useWidgetPieData` is shape-agnostic: it resolves N labeled slice totals
 * the same way regardless of whether they render as wedges or bars), each
 * clickable through to that slice's own filtered case list, same as
 * `DashboardPieChart`'s wedges/legend rows. A total of 0 renders an empty
 * state instead of an all-grey chart, same treatment as `DashboardPieChart`.
 */
export default function DashboardBarChart({
  slices,
  total,
  isLoading,
  isError,
  onSliceClick,
  formatValue,
}: DashboardBarChartProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const [activeIndex, setActiveIndex] = useState<number | undefined>(undefined);

  const colorFor = (slice: PieSliceResult, i: number): string => {
    const key = slice.color ?? DEFAULT_COLOR_ROTATION[i % DEFAULT_COLOR_ROTATION.length];
    return theme.palette[key].main;
  };

  const axisTickColor = isDarkMode ? theme.palette.grey[400] : theme.palette.text.secondary;

  // Custom tick so the category label always renders, word-wrapped onto up
  // to two horizontal lines, rather than relying on the library's default
  // single-line horizontal tick — which recharts silently drops (no label
  // at all, not even on hover) once several real account names don't all
  // fit on one line at the tile's rendered width. `interval={0}` on the
  // <XAxis> below forces every tick to call this, instead of recharts
  // deciding some don't fit and skipping them. Signature matches recharts'
  // own tick-renderer contract: called with that tick's `x`/`y` anchor point
  // and `payload.value` (the category label), everything else about the
  // axis passed through (unused here).
  function renderAxisTick(props: { x?: number; y?: number; payload?: { value?: unknown } }): JSX.Element {
    const { x = 0, y = 0, payload } = props;
    const label = typeof payload?.value === "string" ? payload.value : "";
    const lines = wrapAxisLabel(label);
    return (
      <text x={x} y={y} textAnchor="middle" fontSize={11} fill={axisTickColor}>
        {lines.map((line, i) => (
          <tspan key={i} x={x} dy={i === 0 ? 14 : 13}>
            {line}
          </tspan>
        ))}
      </text>
    );
  }

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

  // The widget's overall total renders next to its title in
  // `DashboardWidgetTile`'s own header row, not here — bars already label
  // their own individual values (see the `label` prop below), so there's
  // nothing else this chart needs to show for it.
  if (total === 0) {
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
          <Typography variant="body2" sx={{ color: isDarkMode ? theme.palette.grey[400] : theme.palette.text.disabled }}>
            Nothing to show here right now
          </Typography>
        </Box>
      </Box>
    );
  }

  const chartData = slices.map((slice) => ({ name: slice.label, value: slice.value }));

  return (
    <Box sx={{ width: "100%" }}>
      <Box sx={{ width: "100%", height: CHART_HEIGHT_PX, "& .recharts-bar-rectangle": { cursor: "pointer" } }}>
        <BarChart
          data={chartData}
          xAxisDataKey="name"
          // height is a fixed constant (the parent Box's own height, CHART_HEIGHT_PX),
          // not a percentage -- passing it directly avoids the same first-render
          // ResizeObserver race as DashboardPieChart's width/height (see that
          // component's comment). width genuinely must stay "100%": this tile's
          // rendered width varies with its grid column count, so it still depends
          // on a ResizeObserver measurement, which is the expected/correct
          // behavior for a responsive dimension -- only the height half of the
          // warning is avoidable here.
          height={CHART_HEIGHT_PX}
          width="100%"
          legend={{ show: false }}
          // The library's own <XAxis> only ever renders a plain horizontal
          // tick, with no interval/wrap/truncation control exposed through
          // this prop, and its recharts default (`interval="preserveEnd"`)
          // auto-drops whichever category labels it calculates would
          // overlap — reported live as account names silently missing
          // (name only visible on hover) once a neighboring label was long
          // enough to crowd them. Suppressed here so the custom,
          // always-visible two-line <XAxis> below (rendered via
          // `children`, same composition the library itself supports for
          // `Bar`/`Cell`) is the only one drawn.
          xAxis={{ show: false }}
          yAxis={{ show: false }}
          grid={{ show: false }}
          // Library default top margin (12px) is too tight once each bar has
          // its own always-visible value label sitting above it; bottom is
          // widened past the library's own default to fit up to two lines
          // of horizontal category label below (less than a rotated label
          // needed, since there's no diagonal sweep to clear any more).
          margin={{ top: 24, right: 8, bottom: 40, left: 0 }}
          // cursor: false drops the library's default full-height highlight
          // box behind the hovered bar — the bar's own hover color/border is
          // enough of a highlight on its own.
          tooltip={{
            show: true,
            wrapperStyle: { zIndex: 1000 },
            cursor: false,
            ...(formatValue ? { formatter: (value: unknown) => formatValue(Number(value)) } : {}),
          }}
        >
          <XAxis dataKey="name" interval={0} tickLine={false} axisLine={{ stroke: axisTickColor }} tick={renderAxisTick} />
          <Bar
            dataKey="value"
            radius={[4, 4, 0, 0]}
            isAnimationActive={false}
            label={{
              position: "top",
              fill: isDarkMode ? theme.palette.common.white : theme.palette.text.primary,
              fontSize: 13,
              fontWeight: 600,
              ...(formatValue ? { formatter: (value: unknown) => formatValue(Number(value)) } : {}),
            }}
            onMouseEnter={(_data: unknown, i: number) => setActiveIndex(i)}
            onMouseLeave={() => setActiveIndex(undefined)}
            // Stops the click from also bubbling up to the tile-level
            // click-through `DashboardWidgetTile` attaches to the whole card
            // for shape "pie"/"bar" — see `DashboardPieChart`'s wedge onClick
            // for the same fix and full rationale.
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
                strokeWidth={activeIndex === i ? 2 : 0}
              />
            ))}
          </Bar>
        </BarChart>
      </Box>
    </Box>
  );
}
