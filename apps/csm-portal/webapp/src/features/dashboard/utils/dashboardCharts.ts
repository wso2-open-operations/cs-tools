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

import {
  ACTIVE_CASES_CHART_DATA,
  OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA,
  OUTSTANDING_INCIDENTS_CHART_DATA,
  SEVERITY_LEGEND_KEY_TO_ID,
} from "@/features/dashboard/constants/dashboard";
import {
  DASHBOARD_CHART_TOTAL_PLACEHOLDER_DASH,
  DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA,
} from "@/features/dashboard/constants/charts";
import {
  SeverityLegendKey,
  type SeverityLegendEntry,
} from "@features/dashboard/types/dashboard";
import {
  OperationsChartMode,
  type ActiveCasesChartData,
  type CasesTrendChartData,
  type ChartPieSlice,
  type OutstandingIncidentsChartData,
} from "@/features/dashboard/types/charts";

export const EMPTY_OUTSTANDING_INCIDENTS_DATA: OutstandingIncidentsChartData = {
  low: 0,
  medium: 0,
  high: 0,
  critical: 0,
  catastrophic: 0,
  total: 0,
};

export const EMPTY_ACTIVE_CASES_DATA: ActiveCasesChartData = {
  serviceRequests: 0,
  changeRequests: 0,
  total: 0,
};

export const EMPTY_CASES_TREND_DATA: CasesTrendChartData = {
  categories: [],
  total: 0,
};

/**
 * Legend + slice source for Outstanding Support Cases, optionally without S0.
 *
 * @param excludeS0 - When true, omits catastrophic slice for non–Managed Cloud projects.
 * @returns {SeverityLegendEntry[]} Rows aligned with `OUTSTANDING_INCIDENTS_CHART_DATA`.
 */
export function resolveOutstandingIncidentsChartSource(
  excludeS0: boolean,
  restrictSeverityToLow: boolean = false,
): SeverityLegendEntry[] {
  if (restrictSeverityToLow) {
    return OUTSTANDING_INCIDENTS_CHART_DATA.filter(
      (item) => item.key === SeverityLegendKey.Low,
    );
  }
  switch (excludeS0) {
    case true:
      return OUTSTANDING_INCIDENTS_CHART_DATA.filter(
        (item) => item.key !== SeverityLegendKey.Catastrophic,
      );
    case false:
    default:
      return OUTSTANDING_INCIDENTS_CHART_DATA;
  }
}

function countForSeverityKey(
  data: OutstandingIncidentsChartData,
  key: SeverityLegendKey,
): number {
  return data[key as keyof OutstandingIncidentsChartData] ?? 0;
}

/**
 * Pie slices for Outstanding Support Cases (loading → empty; error → grey placeholders).
 *
 * @param chartSource - Filtered legend rows (see `resolveOutstandingIncidentsChartSource`).
 * @param safeData - Severity counts; keys match `SeverityLegendKey` string values.
 * @param isLoading - Skeleton state: no slices.
 * @param isError - Error state: uniform placeholder slices.
 * @param errorGrey - Placeholder fill when `isError`.
 * @returns {ChartPieSlice[]} Data passed to Recharts `Pie`.
 */
export function buildOutstandingIncidentsPieSlices(
  chartSource: SeverityLegendEntry[],
  safeData: OutstandingIncidentsChartData,
  isLoading: boolean,
  isError: boolean,
  errorGrey: string,
): ChartPieSlice[] {
  switch (true) {
    case isError:
      return chartSource.map((item) => ({
        name: item.displayName,
        value: 1,
        color: errorGrey,
      }));
    case isLoading:
      return [];
    default:
      return chartSource.map((item) => ({
        name: item.displayName,
        value: countForSeverityKey(safeData, item.key),
        color: item.color,
        id: SEVERITY_LEGEND_KEY_TO_ID[item.key],
      }));
  }
}

/**
 * Legend rows for Outstanding Support Cases (values from `safeData`).
 *
 * @param chartSource - Same source as the pie (`resolveOutstandingIncidentsChartSource`).
 * @param safeData - Severity counts.
 * @returns {ChartPieSlice[]} Entries for `ChartLegend`.
 */
export function buildOutstandingIncidentsLegendRows(
  chartSource: SeverityLegendEntry[],
  safeData: OutstandingIncidentsChartData,
): ChartPieSlice[] {
  return chartSource.map((item) => ({
    name: item.displayName,
    value: countForSeverityKey(safeData, item.key),
    color: item.color,
    id: SEVERITY_LEGEND_KEY_TO_ID[item.key],
  }));
}

/**
 * Center total label for Outstanding Support Cases donut.
 *
 * @param hasApiData - Whether parent passed defined `data`.
 * @param total - `safeData.total`.
 * @returns {number | string} Numeric total or {@link DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA}.
 */
export function formatOutstandingIncidentsCenterTotal(
  hasApiData: boolean,
  total: number,
): number | string {
  switch (hasApiData) {
    case true:
      return total;
    case false:
    default:
      return DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA;
  }
}

type ActiveCasesChartSeriesRow = (typeof ACTIVE_CASES_CHART_DATA)[number];

/**
 * SR/CR series configuration for Outstanding Operations donut.
 *
 * @param variant - Both series or SR-only.
 * @returns Rows from `ACTIVE_CASES_CHART_DATA` (subset when SR-only).
 */
export function resolveActiveCasesSeriesConfig(
  variant: OperationsChartMode,
): ActiveCasesChartSeriesRow[] {
  switch (variant) {
    case OperationsChartMode.SrOnly:
      return ACTIVE_CASES_CHART_DATA.filter(
        (item) => item.key === "serviceRequests",
      );
    case OperationsChartMode.SrAndCr:
    default:
      return [...ACTIVE_CASES_CHART_DATA];
  }
}

function countForActiveCaseKey(
  data: ActiveCasesChartData,
  key: "serviceRequests" | "changeRequests",
): number {
  return data[key] ?? 0;
}

/**
 * Pie slices for Outstanding Operations.
 *
 * @param seriesConfig - From `resolveActiveCasesSeriesConfig`.
 * @param safeData - SR/CR counts.
 * @param isLoading - Empty slices while loading.
 * @param isError - Placeholder slices with `errorGrey`.
 * @returns {ChartPieSlice[]} Data for `Pie`.
 */
export function buildActiveCasesPieSlices(
  seriesConfig: ActiveCasesChartSeriesRow[],
  safeData: ActiveCasesChartData,
  isLoading: boolean,
  isError: boolean,
  errorGrey: string,
): ChartPieSlice[] {
  switch (true) {
    case isError:
      return seriesConfig.map((item) => ({
        name: item.name,
        value: 1,
        color: errorGrey,
      }));
    case isLoading:
      return [];
    default:
      return seriesConfig.map((item) => ({
        name: item.name,
        value: countForActiveCaseKey(
          safeData,
          item.key as "serviceRequests" | "changeRequests",
        ),
        color: item.color,
      }));
  }
}

/**
 * Legend rows for Outstanding Operations (same values as pie when not in error loading state).
 *
 * @param seriesConfig - From `resolveActiveCasesSeriesConfig`.
 * @param safeData - SR/CR counts.
 * @returns {ChartPieSlice[]} Data for `ChartLegend`.
 */
export function buildActiveCasesLegendRows(
  seriesConfig: ActiveCasesChartSeriesRow[],
  safeData: ActiveCasesChartData,
): ChartPieSlice[] {
  return seriesConfig.map((item) => ({
    name: item.name,
    value: countForActiveCaseKey(
      safeData,
      item.key as "serviceRequests" | "changeRequests",
    ),
    color: item.color,
    id: item.key,
  }));
}

/**
 * Center total for Outstanding Operations donut.
 *
 * @param hasApiData - Whether parent passed defined `data`.
 * @param total - `safeData.total`.
 * @returns {number | string} Numeric total or placeholder.
 */
export function formatActiveCasesCenterTotal(
  hasApiData: boolean,
  total: number,
): number | string {
  switch (hasApiData) {
    case true:
      return total;
    case false:
    default:
      return DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA;
  }
}

/**
 * Pie slices for Outstanding Engagements donut.
 *
 * @param safeData - Category breakdown + total.
 * @param isLoading - Loading placeholder slices (zero values, category colors).
 * @param isError - Grey or category colors depending on loading.
 * @param errorGrey - Error fill when `isError && !isLoading`.
 * @param fallbackGrey - When category name is unknown and index fallback exhausted.
 * @returns {ChartPieSlice[]} Data for `Pie` / legend.
 */
export function buildEngagementsPieSlices(
  safeData: CasesTrendChartData,
  isLoading: boolean,
  isError: boolean,
  errorGrey: string,
  fallbackGrey: string,
): ChartPieSlice[] {
  const chartSource = OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA;
  const colorByLabel = new Map(
    chartSource.map((item) => [item.name.toLowerCase(), item.color]),
  );
  const fallbackColors = chartSource.map((item) => item.color);

  switch (true) {
    case isLoading || isError:
      return chartSource.map((item) => ({
        name: item.name,
        value: 0,
        color: isError && !isLoading ? errorGrey : item.color,
      }));
    default:
      return chartSource.map((entry, index) => {
        const category = safeData.categories.find(
          (c) => c.name === entry.name,
        );
        return {
          name: entry.name,
          value: category?.value ?? 0,
          id: category?.id,
          ids: category?.ids,
          color:
            colorByLabel.get(entry.name.toLowerCase()) ??
            fallbackColors[index % fallbackColors.length] ??
            fallbackGrey,
        };
      });
  }
}

/**
 * Total shown in the center of Outstanding Engagements (zero while loading/error path in parent).
 *
 * @param isError - Error state.
 * @param isLoading - Loading state.
 * @param safeTotal - Backing total from API-shaped data.
 * @returns {number} Total for display logic.
 */
export function resolveEngagementsNumericTotal(
  isError: boolean,
  isLoading: boolean,
  safeTotal: number,
): number {
  switch (true) {
    case isError:
    case isLoading:
      return 0;
    default:
      return safeTotal;
  }
}

/**
 * Center primary value for Outstanding Engagements (error → dash; else total or N/A).
 *
 * @param isError - Error state shows {@link DASHBOARD_CHART_TOTAL_PLACEHOLDER_DASH}.
 * @param chartDataLength - Non-empty pie data.
 * @param total - From `resolveEngagementsNumericTotal`.
 * @returns {string | number} Display value in the donut center.
 */
export function formatEngagementsCenterTotal(
  isError: boolean,
  chartDataLength: number,
  total: number,
): string | number {
  switch (true) {
    case isError:
      return DASHBOARD_CHART_TOTAL_PLACEHOLDER_DASH;
    case chartDataLength > 0:
      return total;
    default:
      return DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA;
  }
}
