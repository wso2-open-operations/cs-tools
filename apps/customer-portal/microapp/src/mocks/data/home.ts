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

import { colors } from "@wso2/oxygen-ui";
import type { BarSeriesConfig, PieDataItem } from "@components/features/dashboard";

export const MOCK_PIE_DATA_OUTSTANDING_INCIDENTS: PieDataItem[] = [
  { label: "Critical (P1)", value: 1, color: colors.red[500] },
  { label: "High (P2)", value: 4, color: colors.orange[500] },
  { label: "Medium (P3)", value: 7, color: colors.blue[500] },
];

export const MOCK_PIE_DATA_ACTIVE_CASES: PieDataItem[] = [
  { label: "Awaiting", value: 9, color: colors.blue[500] },
  { label: "Work in Progress", value: 3, color: colors.cyan[500] },
  { label: "Waiting on WSO2", value: 1, color: colors.orange[500] },
];

export const MOCK_BAR_CHART_SERIES_CASES_TREND: BarSeriesConfig[] = [
  { dataKey: "acme", name: "Acme", stackId: "total", color: colors.blue[500] },
  { dataKey: "bites", name: "Bites", stackId: "total", color: colors.cyan[500] },
  { dataKey: "cupertino", name: "CupertinoHQ", stackId: "total", color: colors.red[500] },
  { dataKey: "dunlop", name: "Dunlop", stackId: "total", color: colors.orange[500] },
];

export const MOCK_BAR_CHART_DATA_CASES_TREND = [
  { year: "2020", acme: 40, bites: 35, cupertino: 25, dunlop: 20 },
  { year: "2021", acme: 55, bites: 40, cupertino: 35, dunlop: 30 },
  { year: "2022", acme: 65, bites: 50, cupertino: 40, dunlop: 35 },
  { year: "2023", acme: 80, bites: 60, cupertino: 50, dunlop: 45 },
  { year: "2024", acme: 95, bites: 75, cupertino: 65, dunlop: 55 },
  { year: "2025", acme: 105, bites: 85, cupertino: 75, dunlop: 65 },
];
