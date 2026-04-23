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

import { describe, expect, it } from "vitest";
import { ACTIVE_CASES_CHART_DATA } from "@/features/dashboard/constants/dashboard";
import {
  DASHBOARD_CHART_TOTAL_PLACEHOLDER_DASH,
  DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA,
} from "@/features/dashboard/constants/charts";
import { SeverityLegendKey } from "@features/dashboard/types/dashboard";
import { OperationsChartMode } from "@/features/dashboard/types/charts";
import {
  EMPTY_OUTSTANDING_INCIDENTS_DATA,
  buildEngagementsPieSlices,
  buildOutstandingIncidentsPieSlices,
  formatEngagementsCenterTotal,
  formatOutstandingIncidentsCenterTotal,
  resolveActiveCasesSeriesConfig,
  resolveOutstandingIncidentsChartSource,
} from "@features/dashboard/utils/dashboardCharts";

describe("resolveOutstandingIncidentsChartSource", () => {
  it("includes catastrophic when excludeS0 is false", () => {
    const src = resolveOutstandingIncidentsChartSource(false);
    expect(src.some((r) => r.key === SeverityLegendKey.Catastrophic)).toBe(
      true,
    );
  });

  it("omits catastrophic when excludeS0 is true", () => {
    const src = resolveOutstandingIncidentsChartSource(true);
    expect(src.some((r) => r.key === SeverityLegendKey.Catastrophic)).toBe(
      false,
    );
  });
});

describe("buildOutstandingIncidentsPieSlices", () => {
  const chartSource = resolveOutstandingIncidentsChartSource(false);
  const errorGrey = "#D1D5DB";

  it("returns empty array when loading", () => {
    expect(
      buildOutstandingIncidentsPieSlices(
        chartSource,
        EMPTY_OUTSTANDING_INCIDENTS_DATA,
        true,
        false,
        errorGrey,
      ),
    ).toEqual([]);
  });

  it("returns placeholder slices when error", () => {
    const slices = buildOutstandingIncidentsPieSlices(
      chartSource,
      EMPTY_OUTSTANDING_INCIDENTS_DATA,
      false,
      true,
      errorGrey,
    );
    expect(slices.every((s) => s.value === 1 && s.color === errorGrey)).toBe(
      true,
    );
  });
});

describe("formatOutstandingIncidentsCenterTotal", () => {
  it("returns N/A when API data is absent", () => {
    expect(formatOutstandingIncidentsCenterTotal(false, 0)).toBe(
      DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA,
    );
  });

  it("returns numeric total when API data exists", () => {
    expect(formatOutstandingIncidentsCenterTotal(true, 42)).toBe(42);
  });
});

describe("resolveActiveCasesSeriesConfig", () => {
  it("returns both series for SrAndCr", () => {
    expect(
      resolveActiveCasesSeriesConfig(OperationsChartMode.SrAndCr),
    ).toStrictEqual([...ACTIVE_CASES_CHART_DATA]);
  });

  it("returns SR-only slice for SrOnly", () => {
    const cfg = resolveActiveCasesSeriesConfig(OperationsChartMode.SrOnly);
    expect(cfg).toHaveLength(1);
    expect(cfg[0]?.key).toBe("serviceRequests");
  });
});

describe("buildEngagementsPieSlices", () => {
  const errorGrey = "#D1D5DB";
  const fallbackGrey = "#9CA3AF";

  it("maps categories when healthy", () => {
    const slices = buildEngagementsPieSlices(
      {
        categories: [{ name: "Onboarding", value: 3 }],
        total: 3,
      },
      false,
      false,
      errorGrey,
      fallbackGrey,
    );
    expect(slices.some((s) => s.name === "Onboarding" && s.value === 3)).toBe(
      true,
    );
  });

  it("hides zero-value Services, Follow up, and Improvements only", () => {
    const slices = buildEngagementsPieSlices(
      {
        categories: [
          { name: "Services", value: 0 },
          { name: "Follow up", value: 0 },
          { name: "Improvements", value: 0 },
          { name: "Onboarding", value: 0 },
          { name: "Migration", value: 0 },
        ],
        total: 0,
      },
      false,
      false,
      errorGrey,
      fallbackGrey,
    );
    expect(slices.some((s) => s.name === "Services")).toBe(false);
    expect(slices.some((s) => s.name === "Follow up")).toBe(false);
    expect(slices.some((s) => s.name === "Improvements")).toBe(false);
    expect(slices.some((s) => s.name === "Onboarding")).toBe(true);
    expect(slices.some((s) => s.name === "Migration")).toBe(true);
  });

  it("uses error grey when error and not loading", () => {
    const slices = buildEngagementsPieSlices(
      { categories: [], total: 0 },
      false,
      true,
      errorGrey,
      fallbackGrey,
    );
    expect(slices.every((s) => s.color === errorGrey)).toBe(true);
  });
});

describe("formatEngagementsCenterTotal", () => {
  it("returns dash on error", () => {
    expect(formatEngagementsCenterTotal(true, 0, 0)).toBe(
      DASHBOARD_CHART_TOTAL_PLACEHOLDER_DASH,
    );
  });

  it("returns N/A when no slices", () => {
    expect(formatEngagementsCenterTotal(false, 0, 0)).toBe(
      DASHBOARD_CHART_TOTAL_PLACEHOLDER_NA,
    );
  });

  it("returns total when slices exist", () => {
    expect(formatEngagementsCenterTotal(false, 2, 9)).toBe(9);
  });
});
