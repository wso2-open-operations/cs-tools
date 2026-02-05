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

import { Grid } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { ActiveCasesChart } from "./ActiveCasesChart";
import { CasesTrendChart } from "./CasesTrendChart";
import { OutstandingIncidentsChart } from "./OutstandingIncidentsChart";

interface ChartLayoutProps {
  outstandingCases: {
    medium: number;
    high: number;
    critical: number;
    total: number;
  };
  activeCases: {
    workInProgress: number;
    waitingOnClient: number;
    waitingOnWso2: number;
    total: number;
  };
  casesTrend: Array<{
    name: string;
    TypeA: number;
    TypeB: number;
    TypeC: number;
    TypeD: number;
  }>;
  isLoading?: boolean;
  isErrorOutstanding?: boolean;
  isErrorActiveCases?: boolean;
  isErrorTrend?: boolean;
}

/**
 * ChartLayout component displays multiple chart sections including
 * outstanding incidents, active cases, and cases trend.
 *
 * @param {Object} props - Component props
 * @param {number} props.outstandingIncidents - Number of outstanding incidents.
 * @param {number} props.activeCases - Number of active cases.
 * @param {CasesTrendData[]} props.casesTrend - Array of trend data for cases.
 * @param {boolean} props.isLoading - Flag indicating if the data is loading.
 * @returns {JSX.Element} The chart layout element.
 */
const ChartLayout = ({
  outstandingCases,
  activeCases,
  casesTrend,
  isLoading,
  isErrorOutstanding,
  isErrorActiveCases,
  isErrorTrend,
}: ChartLayoutProps): JSX.Element => {
  return (
    <Grid container spacing={3} sx={{ mb: 3 }}>
      {/* Outstanding Incidents */}
      <Grid size={{ xs: 12, md: 4 }}>
        <OutstandingIncidentsChart
          data={outstandingCases}
          isLoading={isLoading}
          isError={isErrorOutstanding}
        />
      </Grid>

      {/* Active Cases */}
      <Grid size={{ xs: 12, md: 4 }}>
        <ActiveCasesChart
          data={activeCases}
          isLoading={isLoading}
          isError={isErrorActiveCases}
        />
      </Grid>

      {/* Cases Trend */}
      <Grid size={{ xs: 12, md: 4 }}>
        <CasesTrendChart
          data={casesTrend}
          isLoading={isLoading}
          isError={isErrorTrend}
        />
      </Grid>
    </Grid>
  );
};

export default ChartLayout;
