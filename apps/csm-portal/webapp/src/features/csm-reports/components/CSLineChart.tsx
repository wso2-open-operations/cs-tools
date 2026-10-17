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

import type { Dayjs } from "dayjs";
import { Box, Typography, useTheme } from "@wso2/oxygen-ui";
import { LineChart } from "@wso2/oxygen-ui-charts-react";

/* eslint-disable @typescript-eslint/no-explicit-any -- mirrors the source's own loosely-typed chart data prop */
interface ChartProps {
  data: any;
  from?: Dayjs;
  to?: Dayjs;
}

const CHART_HEIGHT = 280;

/** Whole-number y axis, same rule the portal's own line charts use. */
function integerAxis(max: number): { domain: [number, number]; tickCount: number } {
  if (max <= 5) {
    const top = Math.max(1, Math.ceil(max));
    return { domain: [0, top], tickCount: top + 1 };
  }
  const rough = max / 5;
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * magnitude).find((s) => s >= rough && Number.isInteger(s));
  const chosen = step ?? Math.ceil(rough);
  return { domain: [0, chosen * 5], tickCount: 6 };
}

export default function CSLineChart({ data }: ChartProps) {
  const theme = useTheme();
  const rows = (data as any[]).map((entry) => ({
    yearAndMonth: entry.yearAndMonth,
    "Incident Cases": entry.incidentCount,
    "Query Cases": entry.queryCount,
  }));

  const hasData = rows.some((row) => Number(row["Incident Cases"]) > 0 || Number(row["Query Cases"]) > 0);
  const peak = Math.max(0, ...rows.flatMap((row) => [Number(row["Incident Cases"]), Number(row["Query Cases"])]));

  return (
    <Box className="report-chart" sx={{ width: "100%", minWidth: 0 }}>
      {hasData ? (
        <Box sx={{ width: "100%", height: CHART_HEIGHT }}>
          <LineChart
            data={rows}
            xAxisDataKey="yearAndMonth"
            height={CHART_HEIGHT}
            width="100%"
            lines={[
              {
                dataKey: "Incident Cases",
                stroke: theme.palette.error.main,
                strokeWidth: 2,
                dot: false,
                type: "monotone",
                isAnimationActive: false,
              },
              {
                dataKey: "Query Cases",
                stroke: theme.palette.info.main,
                strokeWidth: 2,
                dot: false,
                type: "monotone",
                isAnimationActive: false,
              },
            ]}
            isAnimationActive={false}
            margin={{ left: 0, right: 16, top: 8, bottom: 8 }}
            grid={{ show: true, strokeDasharray: "3 3" }}
            xAxis={{ show: true, interval: rows.length > 8 ? "preserveStartEnd" : 0 }}
            yAxis={integerAxis(peak)}
            legend={{ show: true, align: "center", verticalAlign: "top" }}
            tooltip={{ show: true }}
          />
        </Box>
      ) : (
        <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center", py: 4 }}>
          No data available for case volume by months.
        </Typography>
      )}
    </Box>
  );
}
