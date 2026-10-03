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

// Ported from the source app's components/CSLineChart.tsx — recharts instead
// of chart.js/react-chartjs-2 (see CSBarChart.tsx's header comment for why).
import type { Dayjs } from "dayjs";
import { CartesianGrid, Legend, Line, LineChart as RechartsLineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { useTheme } from "@mui/material/styles";

/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed chart data prop */
interface ChartProps {
  data: any;
  // Accepted for call-site parity with the source component (which used
  // them to seed month labels); recharts' XAxis derives its domain from
  // `data` directly, so these aren't needed here.
  from?: Dayjs;
  to?: Dayjs;
}

export default function CSLineChart({ data }: ChartProps) {
  const rows = (data as any[]).map((entry) => ({
    yearAndMonth: entry.yearAndMonth,
    "Incident Cases": entry.incidentCount,
    "Query Cases": entry.queryCount,
  }));

  const hasData = rows.some(
    (row) => Number(row["Incident Cases"]) > 0 || Number(row["Query Cases"]) > 0,
  );
  const theme = useTheme();
  // Line colors ("#ff7300"/"#25AAE1") stay as-is in both modes — only the
  // chrome around them needs to follow the live theme.
  const axisTickStyle = { fill: theme.palette.text.secondary };
  const axisLabelStyle = { fill: theme.palette.text.secondary };

  return (
    <div style={{ width: "100%", height: 400 }}>
      {hasData ? (
        <ResponsiveContainer width="100%" height="100%">
          <RechartsLineChart data={rows}>
            <CartesianGrid strokeDasharray="3 3" stroke={theme.palette.divider} />
            <XAxis
              dataKey="yearAndMonth"
              tick={axisTickStyle}
              label={{ value: "Months", position: "insideBottom", offset: -5, style: axisLabelStyle }}
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
            <Line type="linear" dataKey="Incident Cases" stroke="#ff7300" strokeWidth={1} dot />
            <Line type="linear" dataKey="Query Cases" stroke="#25AAE1" strokeWidth={1} dot />
          </RechartsLineChart>
        </ResponsiveContainer>
      ) : (
        <p style={{ textAlign: "center", color: theme.palette.text.secondary }}>
          No data available for case volume by months.
        </p>
      )}
    </div>
  );
}
