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

// Ported from the source app's components/CSBarChart.tsx. The source draws
// this with chart.js + react-chartjs-2; one-wso2 already standardizes on
// recharts (chart.js isn't a dependency here), so this is a recharts
// reimplementation of the same three chart shapes (by product/by priority/
// created-vs-resolved by day), preserving the same colors, axis titles, and
// "no data" fallback — see docs/ported-apps/spl.md §5 on this substitution.
import { Dayjs } from "dayjs";
import {
  Bar,
  BarChart as RechartsBarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useTheme } from "@mui/material/styles";
import { COLOURS } from "../constants";

/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed chart data props */
interface ChartProps {
  data: any;
  distinctProductNames?: string[];
  distinctPriorities?: string[];
  from: Dayjs;
  to: Dayjs;
  chartType: "product" | "priority" | "state";
}

const GREY = "#25AAE1";
const PRODUCT_COLORS: Record<string, string> = {
  "WSO2 API Manager": "#25AAE1",
  "WSO2 API Manager Analytics": "#3EB5E5",
  "WSO2 API Platform for Kubernetes": "#57C0E9",
  Ballerina: "#2299CC",
  "WSO2 Enterprise Integrator": "#1F84A6",
  "WSO2 Identity Server": "#1A6D8B",
  "WSO2 Micro Integrator": "#70CBED",
  Unknown: GREY,
};

export default function CSBarChart({
  data,
  distinctProductNames = [],
  distinctPriorities = [],
  from,
  to,
  chartType,
}: ChartProps) {
  const monthLabels: string[] = [];
  let currentMonth = from.startOf("month");
  while (currentMonth.isBefore(to) || currentMonth.isSame(to, "month")) {
    monthLabels.push(currentMonth.format("MMM"));
    currentMonth = currentMonth.add(1, "month");
  }

  let series: { key: string; color: string }[] = [];
  let rows: Record<string, string | number>[] = [];
  let xKey = "month";
  let xTitle = "Months";
  let title = "";

  if (chartType === "product") {
    title = "Cases Created by Product";
    series = distinctProductNames.map((name, i) => ({
      key: name,
      color: PRODUCT_COLORS[name] || COLOURS[i % COLOURS.length],
    }));
    rows = monthLabels.map((label, i) => {
      const monthKey = from.add(i, "month").month();
      const row: Record<string, string | number> = { month: label };
      distinctProductNames.forEach((name) => {
        row[name] = data[monthKey]?.[name] || 0;
      });
      return row;
    });
  } else if (chartType === "priority") {
    title = "Cases Created by Priority";
    series = distinctPriorities.map((priority, i) => ({
      key: priority,
      color: priority === "Unknown" ? GREY : COLOURS[i % COLOURS.length],
    }));
    rows = monthLabels.map((label, i) => {
      const monthKey = from.add(i, "month").month();
      const row: Record<string, string | number> = { month: label };
      distinctPriorities.forEach((priority) => {
        row[priority] = data[monthKey]?.[priority] || 0;
      });
      return row;
    });
  } else {
    title = "Created vs Resolved Cases";
    xKey = "day";
    xTitle = "Days";
    const hasUnknown = (data as any[]).some((entry) => entry.state === "Unknown");
    series = [
      { key: "Opened", color: "#25AAE1" },
      { key: "Resolved", color: hasUnknown ? GREY : "#1A6D8B" },
    ];
    rows = (data as any[]).map((entry) => ({ day: entry.day, Opened: entry.opened, Resolved: entry.closed }));
  }

  const hasData = rows.some((row) => series.some((s) => Number(row[s.key]) > 0));
  const theme = useTheme();
  // Data-series colors (PRODUCT_COLORS/COLOURS/GREY above) stay as-is in
  // both modes — only the chrome around them (gridlines, axis/label text,
  // tooltip surface) needs to follow the live theme.
  const axisTickStyle = { fill: theme.palette.text.secondary };
  const axisLabelStyle = { fill: theme.palette.text.secondary };

  return (
    <div style={{ width: "100%", height: 400 }}>
      <p style={{ fontWeight: 600, marginBottom: 4, color: theme.palette.text.primary }}>{title}</p>
      {hasData ? (
        <ResponsiveContainer width="100%" height="90%">
          <RechartsBarChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" stroke={theme.palette.divider} />
            <XAxis
              dataKey={xKey}
              tick={axisTickStyle}
              label={{ value: xTitle, position: "insideBottom", offset: -5, style: axisLabelStyle }}
            />
            <YAxis
              allowDecimals={false}
              tick={axisTickStyle}
              label={{ value: "Number of Cases", angle: -90, position: "insideLeft", style: axisLabelStyle }}
            />
            <Tooltip
              contentStyle={{
                background: theme.palette.background.paper,
                border: `1px solid ${theme.palette.divider}`,
                color: theme.palette.text.primary,
              }}
              labelStyle={{ color: theme.palette.text.primary }}
            />
            <Legend verticalAlign="top" />
            {series.map((s) => (
              <Bar key={s.key} dataKey={s.key} fill={s.color} />
            ))}
          </RechartsBarChart>
        </ResponsiveContainer>
      ) : (
        <p style={{ textAlign: "center", color: theme.palette.text.secondary }}>
          No data available for the selected criteria.
        </p>
      )}
    </div>
  );
}
