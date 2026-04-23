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

import { Grid, colors, pxToRem } from "@wso2/oxygen-ui";
import { CircleCheck, Clock4, OctagonAlert } from "@wso2/oxygen-ui-icons-react";
import { MetricWidget, PieChartWidget, BarChartWidget } from "@components/features/dashboard";

import {
  MOCK_BAR_CHART_DATA_CASES_TREND,
  MOCK_BAR_CHART_SERIES_CASES_TREND,
  MOCK_PIE_DATA_ACTIVE_CASES,
  MOCK_PIE_DATA_OUTSTANDING_INCIDENTS,
} from "@src/mocks/data/home";

export default function HomePage() {
  return (
    <>
      <Grid spacing={1.5} container>
        <Grid size={6}>
          <MetricWidget
            label="Active Cases"
            value={10}
            trend={{ direction: "up", value: "+10%" }}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.orange[500]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="All Cases"
            value={25}
            trend={{ direction: "up", value: "+3%" }}
            icon={<OctagonAlert size={pxToRem(18)} color={colors.yellow[700]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Resolved This Month"
            value={47}
            trend={{ direction: "up", value: "+18%" }}
            icon={<CircleCheck size={pxToRem(18)} color={colors.green[500]} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricWidget
            label="Average Response Time"
            value="2.4H"
            trend={{ direction: "down", value: "-15%" }}
            icon={<Clock4 size={pxToRem(18)} color={colors.purple[500]} />}
          />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Outstanding Incidents" data={MOCK_PIE_DATA_OUTSTANDING_INCIDENTS} />
        </Grid>
        <Grid size={6}>
          <PieChartWidget title="Active Cases" data={MOCK_PIE_DATA_ACTIVE_CASES} />
        </Grid>
        <Grid size={12}>
          <BarChartWidget
            xAxisKey="year"
            title="Cases Trend"
            series={MOCK_BAR_CHART_SERIES_CASES_TREND}
            data={MOCK_BAR_CHART_DATA_CASES_TREND}
          />
        </Grid>
      </Grid>
    </>
  );
}
