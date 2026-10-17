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

// Query-hours donut for the time report. Same oxygen donut as the CS report
// charts, with the dashboard composition hues for consumed and remaining.
import { Box, Typography } from "@wso2/oxygen-ui";
import { PieChart as HoursPie } from "@wso2/oxygen-ui-charts-react";
import { paletteColor } from "@features/csm-dashboard/utils/paletteColor";
import { CONSUMED_QUERY_HOURS, REAMAINING_QUERY_HOURS } from "../constants";

type PieChartProps = {
  remainingQueryHours: string;
  totalQueryHours: string;
};

const CHART_HEIGHT = 280;

function parseHours(value: string): number {
  const trimmed = value.trim();
  const clock = trimmed.match(/(\d+)\s*h(?:\s*(\d+)\s*m)?/i);
  if (clock) {
    return parseInt(clock[1], 10) + parseInt(clock[2] || "0", 10) / 60;
  }
  const minutesOnly = trimmed.match(/^(\d+)\s*m$/i);
  if (minutesOnly) return parseInt(minutesOnly[1], 10) / 60;
  const numeric = Number(trimmed);
  return Number.isFinite(numeric) ? numeric : 0;
}

export default function PieChart({ remainingQueryHours, totalQueryHours }: PieChartProps) {
  const total = parseHours(totalQueryHours);
  const remaining = Math.min(parseHours(remainingQueryHours), total);
  const consumed = Math.max(total - remaining, 0);
  const consumedPct = total > 0 ? parseFloat(((consumed * 100) / total).toFixed(1)) : 0;
  const remainingPct = total > 0 ? parseFloat(((remaining * 100) / total).toFixed(1)) : 0;
  const rows = [
    { name: `${CONSUMED_QUERY_HOURS} (${consumedPct}%)`, value: consumed },
    { name: `${REAMAINING_QUERY_HOURS} (${remainingPct}%)`, value: remaining },
  ].filter((row) => row.value > 0);
  const colors = [paletteColor("red", 600, "#dc2626"), paletteColor("blue", 500, "#3b82f6")].slice(0, rows.length);

  return (
    <Box className="report-chart" sx={{ width: "100%", minWidth: 0 }}>
      <Typography component="p" variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>
        Query hours
      </Typography>
      {rows.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
          No query hours for this project.
        </Typography>
      ) : (
        <Box sx={{ width: "100%", height: CHART_HEIGHT }}>
          <HoursPie
            height={CHART_HEIGHT}
            width="100%"
            data={rows}
            colors={colors}
            nameKey="name"
            innerRadius={58}
            outerRadius={86}
            paddingAngle={2}
            startAngle={90}
            endAngle={-270}
            isAnimationActive={false}
            pies={[{ dataKey: "value", nameKey: "name" }]}
            margin={{ top: 4, right: 4, bottom: 0, left: 4 }}
            legend={{ show: true, align: "center", verticalAlign: "bottom" }}
            tooltip={{ show: true }}
          />
        </Box>
      )}
    </Box>
  );
}
