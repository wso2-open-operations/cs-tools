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

import { describe, expect, it, vi } from "vitest";
import {
  computeCrCardIsCardError,
  computeCrCardIsCardLoading,
  formatCasesTableClearFiltersLabel,
  getAllCoreFailedState,
  getDashboardChartsLoadingState,
  getSeverityFriendlyLabel,
  getSeverityLegendColor,
  isS0SeverityLabel,
  navigateToCreateCase,
} from "@features/dashboard/utils/dashboard";

describe("isS0SeverityLabel", () => {
  it("returns true for catastrophic labels", () => {
    expect(isS0SeverityLabel("Catastrophic (P0)")).toBe(true);
    expect(isS0SeverityLabel("0 - catastrophic")).toBe(true);
  });

  it("returns false for other severities", () => {
    expect(isS0SeverityLabel("1 - Critical")).toBe(false);
    expect(isS0SeverityLabel("")).toBe(false);
  });
});

describe("getSeverityFriendlyLabel", () => {
  it("maps API labels to friendly names", () => {
    expect(getSeverityFriendlyLabel("1 - Critical")).toBe("Critical");
  });

  it("returns placeholder for empty label", () => {
    expect(getSeverityFriendlyLabel("")).toBe("--");
  });
});

describe("getSeverityLegendColor", () => {
  it("returns a hex color for known severity", () => {
    expect(getSeverityLegendColor("2 - High")).toMatch(/^#/);
  });
});

describe("computeCrCardIsCardLoading", () => {
  it("returns true when combined cases are loading without CR", () => {
    expect(
      computeCrCardIsCardLoading(false, undefined, undefined, true, false, false, false),
    ).toBe(true);
  });

  it("returns false when data is loaded", () => {
    expect(
      computeCrCardIsCardLoading(
        false,
        { totalCases: 1 } as never,
        undefined,
        false,
        false,
        false,
        false,
      ),
    ).toBe(false);
  });

  it("returns true when combined cases are loading with includeCrStats", () => {
    expect(
      computeCrCardIsCardLoading(true, undefined, { total: 1 } as never, true, false, false, false),
    ).toBe(true);
  });

  it("returns true when change request stats are loading with includeCrStats", () => {
    expect(
      computeCrCardIsCardLoading(
        true,
        { totalCases: 1 } as never,
        undefined,
        false,
        true,
        false,
        false,
      ),
    ).toBe(true);
  });

  it("returns false when both data sources are loaded with includeCrStats", () => {
    expect(
      computeCrCardIsCardLoading(
        true,
        { totalCases: 1 } as never,
        { total: 1 } as never,
        false,
        false,
        false,
        false,
      ),
    ).toBe(false);
  });

  it("returns false when either source errored with includeCrStats", () => {
    expect(
      computeCrCardIsCardLoading(true, undefined, undefined, true, true, true, false),
    ).toBe(false);
    expect(
      computeCrCardIsCardLoading(true, undefined, undefined, true, true, false, true),
    ).toBe(false);
  });
});

describe("computeCrCardIsCardError", () => {
  it("returns true when combined cases failed", () => {
    expect(
      computeCrCardIsCardError(
        false,
        false,
        undefined,
        undefined,
        true,
        false,
      ),
    ).toBe(true);
  });

  it("returns true when combined cases failed with includeCrStats", () => {
    expect(
      computeCrCardIsCardError(
        true,
        false,
        undefined,
        { total: 1 } as never,
        true,
        false,
      ),
    ).toBe(true);
  });

  it("returns true when change request stats failed with includeCrStats", () => {
    expect(
      computeCrCardIsCardError(
        true,
        false,
        { totalCases: 1 } as never,
        undefined,
        false,
        true,
      ),
    ).toBe(true);
  });

  it("returns true when either stats are missing with includeCrStats", () => {
    expect(
      computeCrCardIsCardError(
        true,
        false,
        undefined,
        { total: 1 } as never,
        false,
        false,
      ),
    ).toBe(true);
    expect(
      computeCrCardIsCardError(
        true,
        false,
        { totalCases: 1 } as never,
        undefined,
        false,
        false,
      ),
    ).toBe(true);
  });

  it("returns false while card is loading with includeCrStats", () => {
    expect(
      computeCrCardIsCardError(
        true,
        true,
        undefined,
        undefined,
        true,
        true,
      ),
    ).toBe(false);
  });

  it("returns false when both data sources loaded with includeCrStats", () => {
    expect(
      computeCrCardIsCardError(
        true,
        false,
        { totalCases: 1 } as never,
        { total: 1 } as never,
        false,
        false,
      ),
    ).toBe(false);
  });
});

describe("getDashboardChartsLoadingState", () => {
  it("returns true when dashboard is loading", () => {
    expect(
      getDashboardChartsLoadingState({
        isDashboardLoading: true,
        isDefaultCaseLoading: false,
        showOpsChart: false,
        isServiceRequestLoading: false,
        isEngagementLoading: false,
        includeCrStats: false,
        isChangeRequestStatsLoading: false,
      }),
    ).toBe(true);
  });
});

describe("getAllCoreFailedState", () => {
  it("returns true when all core charts failed", () => {
    expect(
      getAllCoreFailedState({
        isErrorCombinedCases: true,
        isErrorDefaultCase: true,
        isErrorEngagement: true,
        showOpsChart: false,
        isErrorServiceRequest: false,
        includeCrStats: false,
        isErrorChangeRequestStats: false,
        includeEngagementStats: true,
      }),
    ).toBe(true);
  });

  it("returns false when a core chart succeeded", () => {
    expect(
      getAllCoreFailedState({
        isErrorCombinedCases: true,
        isErrorDefaultCase: false,
        isErrorEngagement: true,
        showOpsChart: false,
        isErrorServiceRequest: false,
        includeCrStats: false,
        isErrorChangeRequestStats: false,
      }),
    ).toBe(false);
  });
});

describe("formatCasesTableClearFiltersLabel", () => {
  it("includes active filter count", () => {
    expect(formatCasesTableClearFiltersLabel(3)).toBe("Clear Filters (3)");
  });
});

describe("navigateToCreateCase", () => {
  it("navigates to describe-issue when agent is enabled", () => {
    const navigate = vi.fn();
    navigateToCreateCase(navigate, "proj-1", true);
    expect(navigate).toHaveBeenCalledWith("/projects/proj-1/support/chat/describe-issue");
  });

  it("navigates to create-case when agent is disabled", () => {
    const navigate = vi.fn();
    navigateToCreateCase(navigate, "proj-1", false);
    expect(navigate).toHaveBeenCalledWith("/projects/proj-1/support/chat/create-case", {
      state: { skipChat: true },
    });
  });
});
