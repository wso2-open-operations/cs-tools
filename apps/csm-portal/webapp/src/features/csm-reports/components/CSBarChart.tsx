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

import { Dayjs } from "dayjs";
import { Box, Typography, useTheme } from "@wso2/oxygen-ui";
import { BarChart, XAxis, YAxis } from "@wso2/oxygen-ui-charts-react";
import { priorityChartColor, useReportChartColors } from "./useReportChartColors";

/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed chart data props */
interface ChartProps {
  data: any;
  distinctProductNames?: string[];
  distinctPriorities?: string[];
  from: Dayjs;
  to: Dayjs;
  chartType: "product" | "priority" | "state";
}

const CHART_HEIGHT = 300;

export default function CSBarChart({
  data,
  distinctProductNames = [],
  distinctPriorities = [],
  from,
  to,
  chartType,
}: ChartProps) {
  const theme = useTheme();
  const colors = useReportChartColors();
  const unknown = theme.palette.grey[500];

  const monthLabels: string[] = [];
  let currentMonth = from.startOf("month");
  while (currentMonth.isBefore(to) || currentMonth.isSame(to, "month")) {
    monthLabels.push(currentMonth.format("MMM"));
    currentMonth = currentMonth.add(1, "month");
  }

  let series: { key: string; color: string }[] = [];
  let rows: Record<string, string | number>[] = [];
  let xKey = "month";
  let title = "";

  if (chartType === "product") {
    title = "Cases created by product";
    series = distinctProductNames.map((name, i) => ({
      key: name,
      color: name === "Unknown" ? unknown : colors[i % colors.length],
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
    title = "Cases created by priority";
    series = distinctPriorities.map((priority, i) => ({
      key: priority,
      color: priorityChartColor(priority, i, colors, unknown),
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
    title = "Created vs resolved";
    xKey = "day";
    series = [
      { key: "Opened", color: theme.palette.info.main },
      { key: "Resolved", color: theme.palette.success.main },
    ];
    rows = (data as any[]).map((entry) => ({ day: entry.day, Opened: entry.opened, Resolved: entry.closed }));
  }

  const hasData = rows.some((row) => series.some((s) => Number(row[s.key]) > 0));
  const crowded = chartType === "state" && rows.length > 8;
  const axis = theme.palette.text.secondary;

  return (
    <Box className="report-chart" sx={{ width: "100%", minWidth: 0 }}>
      <Typography component="p" variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>
        {title}
      </Typography>
      {hasData ? (
        <Box sx={{ width: "100%", height: CHART_HEIGHT }}>
          <BarChart
            data={rows}
            xAxisDataKey={xKey}
            height={CHART_HEIGHT}
            width="100%"
            colors={series.map((s) => s.color)}
            bars={series.map((s) => ({
              dataKey: s.key,
              fill: s.color,
              radius: [4, 4, 0, 0],
              isAnimationActive: false,
            }))}
            isAnimationActive={false}
            maxBarSize={28}
            legend={{ show: true, align: "center", verticalAlign: "top" }}
            grid={{ show: true, strokeDasharray: "3 3" }}
            tooltip={{ show: true }}
            xAxis={{ show: !crowded }}
            yAxis={{ show: false }}
            margin={{ top: 4, right: 8, bottom: crowded ? 28 : 8, left: 8 }}
          >
            <YAxis
              allowDecimals={false}
              width={36}
              stroke={axis}
              tick={{ fill: axis, fontSize: 12 }}
              tickMargin={8}
              axisLine={{ stroke: axis }}
              tickLine={{ stroke: axis }}
            />
            {crowded ? (
              <XAxis
                dataKey={xKey}
                interval={Math.ceil(rows.length / 6) - 1}
                angle={-35}
                textAnchor="end"
                height={56}
                tickFormatter={(value) => (typeof value === "string" && value.length >= 10 ? value.slice(5) : String(value))}
                stroke={axis}
                tick={{ fill: axis, fontSize: 12 }}
                tickMargin={8}
                axisLine={{ stroke: axis }}
                tickLine={{ stroke: axis }}
              />
            ) : null}
          </BarChart>
        </Box>
      ) : (
        <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center", py: 4 }}>
          No data available for the selected criteria.
        </Typography>
      )}
    </Box>
  );
}
