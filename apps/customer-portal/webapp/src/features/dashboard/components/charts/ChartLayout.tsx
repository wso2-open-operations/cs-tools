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
import {
  ActiveCasesChart,
  type OperationsChartMode,
} from "@features/dashboard/components/charts/ActiveCasesChart";
import { CasesTrendChart } from "@features/dashboard/components/charts/CasesTrendChart";
import { OutstandingIncidentsChart } from "@features/dashboard/components/charts/OutstandingIncidentsChart";

interface ChartLayoutProps {
  outstandingCases: {
    low: number;
    medium: number;
    high: number;
    critical: number;
    catastrophic: number;
    total: number;
  };
  activeCases: {
    serviceRequests: number;
    changeRequests: number;
    total: number;
  };
  engagements: {
    categories: Array<{
      name: string;
      value: number;
    }>;
    total: number;
  };
  isLoading?: boolean;
  isErrorOutstanding?: boolean;
  isErrorActiveCases?: boolean;
  isErrorEngagements?: boolean;
  excludeS0?: boolean;
  showOperationsChart?: boolean;
  operationsChartMode?: OperationsChartMode;
}

/**
 * ChartLayout displays outstanding support cases, optional outstanding operations, and engagements.
 *
 * @param {ChartLayoutProps} props - Component props
 * @param {Object} props.outstandingCases - Severity counts for Outstanding Support Cases chart.
 * @param {Object} props.activeCases - Counts for Outstanding Operations chart.
 * @param {boolean} props.isLoading - Flag indicating if the data is loading.
 * @returns {JSX.Element} The chart layout element.
 */
const ChartLayout = ({
  outstandingCases,
  activeCases,
  isLoading,
  isErrorOutstanding,
  isErrorActiveCases,
  isErrorEngagements,
  excludeS0 = false,
  engagements,
  showOperationsChart = true,
  operationsChartMode = "srAndCr",
}: ChartLayoutProps): JSX.Element => {
  const chartSpan = showOperationsChart
    ? { xs: 12 as const, md: 4 as const }
    : { xs: 12 as const, md: 6 as const };

  return (
    <Grid container spacing={3} sx={{ mb: 3 }}>
      {/* Outstanding Incidents */}
      <Grid size={chartSpan}>
        <OutstandingIncidentsChart
          data={outstandingCases}
          isLoading={isLoading}
          isError={isErrorOutstanding}
          excludeS0={excludeS0}
        />
      </Grid>

      {showOperationsChart && (
        <Grid size={{ xs: 12, md: 4 }}>
          <ActiveCasesChart
            data={activeCases}
            isLoading={isLoading}
            isError={isErrorActiveCases}
            variant={operationsChartMode}
          />
        </Grid>
      )}

      {/* Cases Trend */}
      <Grid size={chartSpan}>
        <CasesTrendChart
          data={engagements}
          isLoading={isLoading}
          isError={isErrorEngagements}
        />
      </Grid>
    </Grid>
  );
};

export default ChartLayout;
