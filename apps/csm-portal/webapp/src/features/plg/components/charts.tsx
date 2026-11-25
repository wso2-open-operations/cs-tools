import { Box } from "@wso2/oxygen-ui";
import { BarChart, LineChart, PieChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";

import type { CountByLabel, TimeSeries } from "@features/plg/api/types";
import { EmptyState } from "@features/plg/components/common";
import { useChartColors } from "@features/plg/config/chartPalette";
import { humanizeEnum } from "@features/plg/utils/format";

/**
 * Shared chart primitives. They live here rather than inside a feature because
 * the dashboard, the outreach tab and the funnel tab all render them.
 *
 * Both wrap Oxygen's chart components, which are declarative wrappers over
 * Recharts — the series is configuration rather than child elements, and the
 * axes, grid and tooltip pick up the active theme without being told about it.
 */

/**
 * Labelled proportions. Zero-count slices are dropped so the ring stays readable.
 *
 * The legend sits underneath rather than to the right. A right-aligned legend is
 * laid over the chart area, so the ring stayed centred on the full width and
 * disappeared behind it — which read as the donut being pushed off to the left.
 * A bottom legend has vertical space reserved for it, so the ring centres in
 * what is left. It also suits these narrow cards, where a label like
 * "Identity & Access Management" is wider than the ring itself.
 */
export function DonutChart({
  data,
  humanize,
  keepEmptyLabels,
  solid,
}: {
  data: CountByLabel[];
  humanize?: boolean;
  /** A full pie instead of a ring. */
  solid?: boolean;
  /** Keep zero-count labels in the legend (no slice), for a fixed vocabulary. */
  keepEmptyLabels?: boolean;
}): JSX.Element {
  const colors = useChartColors();
  const rows = keepEmptyLabels ? data : data.filter((d) => d.count > 0);
  if (!rows.some((d) => d.count > 0)) return <EmptyState message="Nothing to show for this period" />;

  return (
    <Box height={280}>
      <PieChart
        height="100%"
        data={rows.map((r) => ({
          name: humanize ? humanizeEnum(r.label) : r.label,
          value: r.count,
        }))}
        colors={colors}
        nameKey="name"
        innerRadius={solid ? 0 : 50}
        outerRadius={78}
        paddingAngle={solid ? 0 : 2}
        // Off, so the ring is there the moment the card paints and does not
        // re-animate every time the period changes.
        isAnimationActive={false}
        pies={[{ dataKey: "value", nameKey: "name" }]}
        margin={{ top: 4, right: 4, bottom: 0, left: 4 }}
        legend={{ show: true, align: "center", verticalAlign: "bottom" }}
        tooltip={{ show: true }}
      />
    </Box>
  );
}

/**
 * A y-axis domain and tick count that can only land on whole numbers.
 *
 * Every series the portal plots is a count of something, and Oxygen's chart
 * wrapper renders its own YAxis without setting Recharts' `allowDecimals`
 * flag — so a chart whose tallest point is 1 gets ticks at 0, 0.25, 0.5, 0.75,
 * 1, and appears to be counting fractions of a registration. Pinning the domain
 * to a round ceiling with a matching tick count avoids that without giving up
 * the wrapper's theme-aware axis styling.
 */
function integerAxis(max: number): { domain: [number, number]; tickCount: number } {
  if (max <= 5) {
    // Small ranges get one tick per value: 0..3 → 0, 1, 2, 3.
    const top = Math.max(1, Math.ceil(max));
    return { domain: [0, top], tickCount: top + 1 };
  }
  // Larger ranges get five intervals, with a step rounded up to something a
  // reader expects to see on an axis rather than 7.4 at a time.
  const rough = max / 5;
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= rough && Number.isInteger(s));
  const chosen = step ?? Math.ceil(rough);
  return { domain: [0, chosen * 5], tickCount: 6 };
}

/** One line per series over a shared bucket axis. */
export function MultiLineChart({
  data,
  humanize,
  height = 300,
}: {
  data: TimeSeries;
  humanize?: boolean;
  height?: number;
}): JSX.Element {
  const colors = useChartColors();
  if (!data.buckets.length) return <EmptyState message="Nothing to show for this period" />;

  const label = (raw: string) => (humanize ? humanizeEnum(raw) : raw);

  const peak = Math.max(0, ...data.series.flatMap((s) => s.counts));

  const rows = data.buckets.map((bucket, i) => {
    const row: Record<string, string | number> = { bucket };
    data.series.forEach((s) => {
      row[label(s.label)] = s.counts[i] ?? 0;
    });
    return row;
  });

  return (
    <Box height={height}>
      <LineChart
        height="100%"
        data={rows}
        colors={colors}
        xAxisDataKey="bucket"
        lines={data.series.map((s) => ({
          dataKey: label(s.label),
          type: "monotone",
          strokeWidth: 2,
          dot: false,
        }))}
        // Off for the same reason as the ring below: Recharts draws an
        // animating line by growing its stroke-dasharray from zero, so an
        // animated chart is an *empty* chart until the animation finishes —
        // and it replays on every period change.
        isAnimationActive={false}
        margin={{ left: 0, right: 16, top: 8 }}
        grid={{ show: true, strokeDasharray: "3 3" }}
        yAxis={integerAxis(peak)}
        legend={{ show: true }}
        tooltip={{ show: true }}
      />
    </Box>
  );
}

/** One bar per label, for free-text labels such as UTM sources. Zero rows are dropped. */
export function CountBarChart({
  data,
  humanize,
  height = 280,
}: {
  data: CountByLabel[];
  humanize?: boolean;
  height?: number;
}): JSX.Element {
  const colors = useChartColors();
  const rows = data.filter((d) => d.count > 0);
  if (!rows.length) return <EmptyState message="Nothing to show for this period" />;
  const peak = Math.max(...rows.map((r) => r.count));

  return (
    <Box height={height}>
      <BarChart
        height="100%"
        data={rows.map((r) => ({ name: humanize ? humanizeEnum(r.label) : r.label, Registrations: r.count }))}
        xAxisDataKey="name"
        bars={[{ dataKey: "Registrations", fill: colors[0], radius: [4, 4, 0, 0], isAnimationActive: false }]}
        margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
        grid={{ show: true, strokeDasharray: "3 3" }}
        yAxis={{ show: true, ...integerAxis(peak) }}
        legend={{ show: false }}
        tooltip={{ show: true }}
      />
    </Box>
  );
}

/** One named, coloured stack segment of a StackedBarChart; `key` names a numeric field of the row. */
export interface StackSegment<K extends string = string> {
  key: K;
  label: string;
  color: string;
}

/** A stacked bar per label: the segments add up to the label's total. */
export function StackedBarChart<K extends string, R extends { label: string } & Record<K, number>>({
  rows,
  segments,
  height = 280,
}: {
  rows: R[];
  segments: Array<StackSegment<K>>;
  height?: number;
}): JSX.Element {
  if (!rows.length) return <EmptyState message="Nothing to show for this period" />;
  const peak = Math.max(...rows.map((r) => segments.reduce((sum, seg) => sum + r[seg.key], 0)));

  return (
    <Box height={height}>
      <BarChart
        height="100%"
        data={rows.map((r) => {
          const row: Record<string, string | number> = { name: r.label };
          segments.forEach((seg) => {
            row[seg.label] = r[seg.key];
          });
          return row;
        })}
        xAxisDataKey="name"
        bars={segments.map((seg) => ({
          dataKey: seg.label,
          stackId: "outcome",
          fill: seg.color,
          isAnimationActive: false,
        }))}
        margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
        grid={{ show: true, strokeDasharray: "3 3" }}
        yAxis={{ show: true, ...integerAxis(peak) }}
        legend={{ show: true }}
        tooltip={{ show: true }}
      />
    </Box>
  );
}
