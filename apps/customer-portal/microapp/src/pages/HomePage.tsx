// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import {
  MetricWidget,
  PieChartWidget,
  BarChartWidget,
  type PieDataItem,
  type BarSeriesConfig,
} from "@components/features/dashboard";
import { Grid } from "@mui/material";
import { CheckCircle, Report, WatchLater } from "@mui/icons-material";

const PIE_DATA_OUTSTANDING_INCIDENTS: PieDataItem[] = [
  { label: "Critical (P1)", value: 1, color: "#FF4522" },
  { label: "High (P2)", value: 4, color: "#FF8C00" },
  { label: "Medium (P3)", value: 7, color: "#4D53E8" },
];

const PIE_DATA_ACTIVE_CASES: PieDataItem[] = [
  { label: "Critical (P1)", value: 1, color: "#FF4522" },
  { label: "High (P2)", value: 4, color: "#FF8C00" },
  { label: "Medium (P3)", value: 7, color: "#4D53E8" },
];

const BAR_CHART_SERIES_CASES_TREND: BarSeriesConfig[] = [
  { dataKey: "acme", label: "Acme", stack: "total", color: "#4D53E8" },
  { dataKey: "bites", label: "Bites", stack: "total", color: "#14A9C1" },
  { dataKey: "cupertino", label: "CupertinoHQ", stack: "total", color: "#E50051" },
  { dataKey: "dunlop", label: "Dunlop", stack: "total", color: "#FF8C00" },
];

const BAR_CHART_DATA_CASES_TREND = [
  { year: "2020", acme: 40, bites: 35, cupertino: 25, dunlop: 20 },
  { year: "2021", acme: 55, bites: 40, cupertino: 35, dunlop: 30 },
  { year: "2022", acme: 65, bites: 50, cupertino: 40, dunlop: 35 },
  { year: "2023", acme: 80, bites: 60, cupertino: 50, dunlop: 45 },
  { year: "2024", acme: 95, bites: 75, cupertino: 65, dunlop: 55 },
  { year: "2025", acme: 105, bites: 85, cupertino: 75, dunlop: 65 },
];

export default function HomePage() {
  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={6}>
          <MetricWidget
            label="Active Cases"
            value={10}
            trend={{ direction: "up", value: "+10%" }}
            icon={<Report sx={{ color: "semantic.portal.accent.orange" }} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="All Cases"
            value={25}
            trend={{ direction: "up", value: "+3%" }}
            icon={<Report sx={{ color: "semantic.portal.accent.yellow" }} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Resolved This Month"
            value={47}
            trend={{ direction: "up", value: "+18%" }}
            icon={<CheckCircle sx={{ color: "semantic.portal.accent.green" }} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Average Response Time"
            value="2.4H"
            trend={{ direction: "down", value: "-15%" }}
            icon={<WatchLater sx={{ color: "semantic.portal.accent.purple" }} />}
          />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Outstanding Incidents" data={PIE_DATA_OUTSTANDING_INCIDENTS} />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Active Cases" data={PIE_DATA_ACTIVE_CASES} />
        </Grid>
        <Grid size={12}>
          <BarChartWidget
            xAxisKey="year"
            title="Cases Trend"
            series={BAR_CHART_SERIES_CASES_TREND}
            data={BAR_CHART_DATA_CASES_TREND}
          />
        </Grid>
      </Grid>
    </>
  );
}
