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

// Outstanding Support Cases (severity donut) dataset from project stats.
export type OutstandingIncidentsChartData = {
  low: number;
  medium: number;
  high: number;
  critical: number;
  catastrophic: number;
  total: number;
};

// Outstanding Engagements donut dataset.
export type CasesTrendChartData = {
  categories: Array<{
    name: string;
    value: number;
  }>;
  total: number;
};

// Outstanding Operations (SR/CR) donut dataset.
export type ActiveCasesChartData = {
  serviceRequests: number;
  changeRequests: number;
  total: number;
};

// Single slice for Oxygen pie charts and legends.
export type ChartPieSlice = {
  name: string;
  value: number;
  color: string;
  id?: string;
};

// Active cases chart: both series or service requests only.
export enum OperationsChartMode {
  SrAndCr = "srAndCr",
  SrOnly = "srOnly",
}

// Outstanding Incidents chart props.
export type OutstandingIncidentsChartProps = {
  data?: OutstandingIncidentsChartData;
  isLoading?: boolean;
  isError?: boolean;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  centerContent?: boolean;
  onSeverityClick?: (severityId: string) => void;
};

// Outstanding Engagements chart props.
export type CasesTrendChartProps = {
  data?: CasesTrendChartData;
  isLoading?: boolean;
  isError?: boolean;
  centerContent?: boolean;
  onSliceClick?: () => void;
};

// Active cases chart props.
export type ActiveCasesChartProps = {
  data?: ActiveCasesChartData;
  isLoading?: boolean;
  isError?: boolean;
  variant?: OperationsChartMode;
  centerContent?: boolean;
  onSliceClick?: (key: string) => void;
};

// Chart legend props.
export type ChartLegendProps = {
  data: ChartPieSlice[];
  isError?: boolean;
  showValues?: boolean;
  onItemClick?: (id: string) => void;
};

// Chart layout props.
export type ChartLayoutProps = {
  outstandingCases: OutstandingIncidentsChartData;
  activeCases: ActiveCasesChartData;
  engagements: CasesTrendChartData;
  isLoading?: boolean;
  isErrorOutstanding?: boolean;
  isErrorActiveCases?: boolean;
  isErrorEngagements?: boolean;
  excludeS0?: boolean;
  restrictSeverityToLow?: boolean;
  showOperationsChart?: boolean;
  operationsChartMode?: OperationsChartMode;
  showEngagementsChart?: boolean;
  onSeverityClick?: (severityId: string) => void;
  onOperationsClick?: (key: string) => void;
  onEngagementsClick?: () => void;
};
