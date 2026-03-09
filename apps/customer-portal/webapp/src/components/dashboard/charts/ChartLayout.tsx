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
import { ActiveCasesChart } from "@components/dashboard/charts/ActiveCasesChart";
import { CasesTrendChart } from "@components/dashboard/charts/CasesTrendChart";
import { OutstandingIncidentsChart } from "@components/dashboard/charts/OutstandingIncidentsChart";

interface ChartLayoutProps {
  outstandingCases: {
    low: number;
    medium: number;
    high: number;
    critical: number;
    catastrophic: number;
    serviceRequest: number;
    securityReportAnalysis: number;
    total: number;
  };
  activeCases: {
    open: number;
    workInProgress: number;
    awaitingInfo: number;
    waitingOnWso2: number;
    solutionProposed: number;
    reopened: number;
    total: number;
  };
  casesTrend: Array<{
    period: string;
    critical: number;
    high: number;
    medium: number;
    low: number;
    catastrophic: number;
  }>;
  isLoading?: boolean;
  isErrorOutstanding?: boolean;
  isErrorActiveCases?: boolean;
  isErrorTrend?: boolean;
  excludeS0?: boolean;
}

/**
 * ChartLayout component displays multiple chart sections including
 * outstanding engagements (severities + case types), active cases, and cases trend.
 *
 * @param {ChartLayoutProps} props - Component props
 * @param {Object} props.outstandingCases - Severity and case type counts for Outstanding Engagements chart.
 * @param {Object} props.activeCases - State counts for Active Engagements chart.
 * @param {Array} props.casesTrend - Array of trend data for Cases Trend chart.
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
  excludeS0 = false,
}: ChartLayoutProps): JSX.Element => {
  return (
    <Grid container spacing={3} sx={{ mb: 3 }}>
      {/* Outstanding Incidents */}
      <Grid size={{ xs: 12, md: 4 }}>
        <OutstandingIncidentsChart
          data={outstandingCases}
          isLoading={isLoading}
          isError={isErrorOutstanding}
          excludeS0={excludeS0}
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
          excludeS0={excludeS0}
        />
      </Grid>
    </Grid>
  );
};

export default ChartLayout;
