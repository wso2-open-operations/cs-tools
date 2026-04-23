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
import { ActiveCasesChart } from "@features/dashboard/components/charts/ActiveCasesChart";
import { CasesTrendChart } from "@features/dashboard/components/charts/CasesTrendChart";
import { OutstandingIncidentsChart } from "@features/dashboard/components/charts/OutstandingIncidentsChart";
import {
  OperationsChartMode,
  type ChartLayoutProps,
} from "@features/dashboard/types/charts";
import { DASHBOARD_CHART_SPAN } from "@/features/dashboard/constants/charts";

/**
 * ChartLayout displays outstanding support cases, optional outstanding operations, and engagements.
 *
 * @param props - Component props
 * @param props.outstandingCases - Severity counts for Outstanding Support Cases chart.
 * @param props.activeCases - Counts for Outstanding Operations chart.
 * @param props.isLoading - Flag indicating if the data is loading.
 * @returns {JSX.Element} Chart grid for the dashboard.
 */
const ChartLayout = ({
  outstandingCases,
  activeCases,
  isLoading,
  isErrorOutstanding,
  isErrorActiveCases,
  isErrorEngagements,
  excludeS0 = false,
  restrictSeverityToLow = false,
  engagements,
  showOperationsChart = true,
  operationsChartMode = OperationsChartMode.SrAndCr,
  showEngagementsChart = true,
  onSeverityClick,
  onOperationsClick,
}: ChartLayoutProps): JSX.Element => {
  const visibleChartsCount =
    1 + (showOperationsChart ? 1 : 0) + (showEngagementsChart ? 1 : 0);
  const singleChartMode = visibleChartsCount === 1;
  const chartSpan =
    singleChartMode
      ? ({ xs: 12 as const, md: 12 as const })
      : showOperationsChart
        ? DASHBOARD_CHART_SPAN
        : ({ xs: 12 as const, md: 6 as const });

  return (
    <Grid container spacing={3} sx={{ mb: 3 }}>
      <Grid size={chartSpan}>
        <OutstandingIncidentsChart
          data={outstandingCases}
          isLoading={isLoading}
          isError={isErrorOutstanding}
          excludeS0={excludeS0}
          restrictSeverityToLow={restrictSeverityToLow}
          centerContent={singleChartMode}
          onSeverityClick={onSeverityClick}
        />
      </Grid>

      {showOperationsChart && (
        <Grid size={chartSpan}>
          <ActiveCasesChart
            data={activeCases}
            isLoading={isLoading}
            isError={isErrorActiveCases}
            variant={operationsChartMode}
            centerContent={singleChartMode}
            onSliceClick={onOperationsClick}
          />
        </Grid>
      )}

      {showEngagementsChart && (
        <Grid size={chartSpan}>
          <CasesTrendChart
            data={engagements}
            isLoading={isLoading}
            isError={isErrorEngagements}
            centerContent={singleChartMode}
          />
        </Grid>
      )}
    </Grid>
  );
};

export default ChartLayout;
