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

// Ported from the source app's components/PieChart.tsx (consumed/remaining
// query-hours donut used by the Timelogs report) — recharts instead of
// chart.js/react-chartjs-2 (see CSBarChart.tsx's header comment for why).
import { Cell, Legend, Pie, PieChart as RechartsPieChart, ResponsiveContainer, Tooltip } from "recharts";
import { useTheme } from "@mui/material/styles";
import { CONSUMED_QUERY_HOURS, HOURS, REAMAINING_QUERY_HOURS } from "../constants";

type PieChartProps = {
  remainingQueryHours: string;
  totalQueryHours: string;
};

function parseTime(timeString: string): number {
  const match = timeString.match(/(\d+)h\s*(\d*)m/);
  if (match) {
    const hours = parseInt(match[1], 10);
    const minutes = parseInt(match[2] || "0", 10);
    return hours + minutes / 60;
  }
  return 0;
}

export default function PieChart({ remainingQueryHours, totalQueryHours }: PieChartProps) {
  const remaining = parseTime(remainingQueryHours);
  const total = parseTime(totalQueryHours);
  const consumed = total - remaining;
  const consumedPct = parseFloat(((consumed * 100) / total).toFixed(2));
  const remainingPct = parseFloat(((remaining * 100) / total).toFixed(2));

  const rows = [
    { name: `${CONSUMED_QUERY_HOURS} (${consumedPct}%)`, value: consumed, color: "#FF6384" },
    { name: `${REAMAINING_QUERY_HOURS} (${remainingPct}%)`, value: remaining, color: "#36A2EB" },
  ];
  const theme = useTheme();

  return (
    <div style={{ width: 300, height: 300, margin: "auto", paddingTop: 15 }}>
      <ResponsiveContainer width="100%" height="100%">
        <RechartsPieChart>
          <Pie data={rows} dataKey="value" nameKey="name">
            {rows.map((row) => (
              <Cell key={row.name} fill={row.color} />
            ))}
          </Pie>
          <Tooltip
            formatter={(value) => [`${value} ${HOURS}`, undefined]}
            contentStyle={{
              background: theme.palette.background.paper,
              border: `1px solid ${theme.palette.divider}`,
              color: theme.palette.text.primary,
            }}
          />
          <Legend verticalAlign="bottom" />
        </RechartsPieChart>
      </ResponsiveContainer>
    </div>
  );
}
