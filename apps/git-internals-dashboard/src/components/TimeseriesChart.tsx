"use client";

import { useState } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from "recharts";
import { Box } from "@mui/material";
import { useTimeseries } from "@/lib/api";
import { acrylicSurfaceSx } from "@/lib/surfaces";

const MONO = "var(--font-mono)";

const P_CODE: Record<string, string> = {
  "Critical(P1)": "P1",
  "High(P2)": "P2",
  "Medium(P3)": "P3",
  "Low(P4)": "P4",
};
const CODE_COLOR: Record<string, string> = {
  P1: "var(--sla-p1)",
  P2: "var(--sla-p2)",
  P3: "var(--sla-p3)",
  P4: "var(--sla-p4)",
};
const codeOf = (key: string) => P_CODE[key] ?? key;
const colorOf = (key: string) => CODE_COLOR[codeOf(key)] ?? "var(--sla-fg2)";

type Metric = "violated" | "at_risk" | "total";
const METRICS: { value: Metric; label: string }[] = [
  { value: "violated", label: "Violated" },
  { value: "at_risk", label: "At risk" },
  { value: "total", label: "All" },
];
const WINDOWS = [30, 60, 90] as const;

const TITLE: Record<Metric, string> = {
  violated: "Violated",
  at_risk: "At risk",
  total: "Total cases",
};

interface TimeseriesChartProps {
  repo?: string;
  activePriority?: string;
  onPriorityFilter: (priority: string) => void;
}

function Segmented<T extends string | number>({
  options,
  value,
  onChange,
}: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
}) {
  return (
    <Box sx={{ display: "flex", overflow: "hidden", borderRadius: "9px", border: "1px solid var(--sla-border)" }}>
      {options.map((o, i) => {
        const active = o.value === value;
        return (
          <Box
            key={String(o.value)}
            component="button"
            type="button"
            onClick={() => onChange(o.value)}
            sx={{
              px: 1.5, py: 0.75, fontSize: 12, fontWeight: 600, border: "none", cursor: "pointer",
              borderLeft: i > 0 ? "1px solid var(--sla-border)" : "none",
              bgcolor: active ? "var(--sla-primary)" : "var(--sla-card)",
              color: active ? "var(--sla-contrast-text)" : "var(--sla-fg2)",
            }}
          >
            {o.label}
          </Box>
        );
      })}
    </Box>
  );
}

export function TimeseriesChart({ repo, activePriority, onPriorityFilter }: TimeseriesChartProps) {
  const [metric, setMetric] = useState<Metric>("violated");
  const [days, setDays] = useState<number>(30);

  const { data, isLoading } = useTimeseries({ repo, metric, days, groupBy: "priority" });

  const chartData =
    data?.dates.map((date, i) => {
      const row: Record<string, string | number> = { date: date.slice(5) };
      for (const s of data.series) row[s.key] = s.points[i] ?? 0;
      return row;
    }) ?? [];

  const last = (key: string) => {
    const s = data?.series.find((x) => x.key === key);
    return s ? s.points[s.points.length - 1] ?? 0 : 0;
  };

  return (
    <Box sx={{ ...acrylicSurfaceSx, display: "flex", flexDirection: "column", borderRadius: "16px", border: "1px solid var(--sla-border)", px: "22px", py: 2.5, boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
      <Box sx={{ display: "flex", flexWrap: "wrap", alignItems: "flex-start", justifyContent: "space-between", gap: 1.5 }}>
        <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>
          {TITLE[metric]} over time, by priority
        </Box>
        <Box sx={{ display: "flex", gap: 1 }}>
          <Segmented options={METRICS} value={metric} onChange={(v) => setMetric(v)} />
          <Segmented options={WINDOWS.map((w) => ({ value: w, label: `${w}d` }))} value={days} onChange={(v) => setDays(v)} />
        </Box>
      </Box>

      <Box sx={{ mt: 2, height: 188 }}>
        {isLoading ? (
          <Box sx={{ display: "flex", height: "100%", alignItems: "center", justifyContent: "center", fontSize: 12, color: "var(--sla-fg3)" }}>
            Loading…
          </Box>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={chartData} margin={{ top: 6, right: 6, bottom: 0, left: -22 }}>
              <CartesianGrid horizontal vertical={false} stroke="var(--sla-border-soft)" />
              <ReferenceLine y={0} stroke="var(--sla-border)" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 10, fill: "var(--sla-no-sla)", fontFamily: MONO }}
                axisLine={false}
                tickLine={false}
                minTickGap={40}
              />
              <YAxis
                allowDecimals={false}
                tick={{ fontSize: 10, fill: "var(--sla-no-sla)", fontFamily: MONO }}
                axisLine={false}
                tickLine={false}
                width={48}
              />
              {/* Recharts' Tooltip wrapper ships its own inline default
                  (opaque white background) — contentStyle must explicitly
                  override it or that default wins. Matches Oxygen UI's own
                  MuiPopover/MuiAutocomplete overlay treatment (background.paper
                  + blur.medium), not the page-level acrylic card surface. */}
              <Tooltip
                contentStyle={{
                  fontSize: 12, borderRadius: 10, border: "1px solid var(--sla-border)",
                  backgroundColor: "var(--sla-overlay)",
                  backdropFilter: "var(--sla-overlay-blur)",
                  WebkitBackdropFilter: "var(--sla-overlay-blur)",
                }}
                labelStyle={{ color: "var(--sla-fg2)" }}
              />
              {data?.series.map((s) => {
                const focused = !activePriority || s.key === activePriority;
                return (
                  <Line
                    key={s.key}
                    type="monotone"
                    dataKey={s.key}
                    name={codeOf(s.key)}
                    stroke={colorOf(s.key)}
                    strokeWidth={s.key === activePriority ? 3 : 2}
                    strokeOpacity={focused ? 1 : 0.13}
                    dot={false}
                    isAnimationActive={false}
                  />
                );
              })}
            </LineChart>
          </ResponsiveContainer>
        )}
      </Box>

      {data && data.series.length > 0 && (
        <Box sx={{ mt: 1.75, display: "flex", flexWrap: "wrap", gap: 2, borderTop: "1px solid var(--sla-border-soft)", pt: 1.75 }}>
          {data.series.map((s) => {
            const dim = activePriority && activePriority !== s.key;
            const color = colorOf(s.key);
            return (
              <Box
                key={s.key}
                component="button"
                type="button"
                onClick={() => onPriorityFilter(activePriority === s.key ? "" : s.key)}
                sx={{ display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "var(--sla-fg2)", transition: "opacity 0.15s", opacity: dim ? 0.4 : 1, background: "none", border: "none", cursor: "pointer", p: 0 }}
              >
                <Box component="span" sx={{ height: 3, width: 16, borderRadius: "2px", bgcolor: color }} />
                {codeOf(s.key)}{" "}
                <Box component="b" sx={{ fontWeight: 600, fontFamily: MONO, color }}>
                  {last(s.key)}
                </Box>
              </Box>
            );
          })}
        </Box>
      )}
    </Box>
  );
}
