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

// Card title: severity distribution donut.
export const DASHBOARD_CHART_TITLE_OUTSTANDING_CASES = "Outstanding Support Cases";

// Card title: engagements distribution donut.
export const DASHBOARD_CHART_TITLE_OUTSTANDING_ENGAGEMENTS =
  "Outstanding Engagements";

// Card title: operations (SR/CR) donut.
export const DASHBOARD_CHART_TITLE_OUTSTANDING_OPERATIONS =
  "Outstanding Operations";

// Center label under the total in donut charts.
export const DASHBOARD_CHART_CAPTION_TOTAL = "Total";

// Shown when total cannot be derived.
export const DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA = "N/A";

// Shown in engagements chart center on error.
export const DASHBOARD_CHART_TOTAL_PLACEHOLDER_DASH = "--";

// `ErrorIndicator` entity label for outstanding severity chart.
export const DASHBOARD_CHART_ERROR_ENTITY_OUTSTANDING_CASES = "outstanding cases";

// `ErrorIndicator` entity label for operations chart.
export const DASHBOARD_CHART_ERROR_ENTITY_ACTIVE_CASES = "active cases";

// Pie chart plot height (px).
export const DASHBOARD_CHART_PIE_AREA_HEIGHT_PX = 240;

// Circular skeleton size for pie loading state.
export const DASHBOARD_CHART_PIE_SKELETON_SIZE_PX = 160;

// Legend skeleton width when loading (Outstanding Incidents).
export const DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_INCIDENTS_PX = 60;

// Legend skeleton width when loading (Engagements / Operations).
export const DASHBOARD_CHART_LEGEND_SKELETON_WIDTH_WIDE_PX = 80;

// chart span
export const DASHBOARD_CHART_SPAN = { xs: 12 as const, md: 4 as const };

// Dark-mode shade used for dashboard chart slices and legends.
export const DASHBOARD_CHART_DARK_MODE_SHADE = 500;

// Opacity applied to chart slice areas in dark mode only.
export const DASHBOARD_CHART_DARK_MODE_OPACITY = 0.6;
