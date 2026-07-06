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
import {
  DASHBOARD_CHART_CAPTION_TOTAL,
  DASHBOARD_CHART_TITLE_OUTSTANDING_CASES,
  DASHBOARD_CHART_TITLE_OUTSTANDING_OPERATIONS,
} from "@features/dashboard/constants/charts";
import {
  CASES_TABLE_HEADER_TITLE,
  CASES_TABLE_CLEAR_FILTERS_LABEL,
  DASHBOARD_CASES_VIEW_TABS,
} from "@features/dashboard/constants/casesTable";
import { DASHBOARD_STATS } from "@features/dashboard/constants/dashboard";
import { DashboardCasesViewMode } from "@features/dashboard/types/casesTable";

describe("charts constants", () => {
  it("exports chart titles and captions", () => {
    expect(DASHBOARD_CHART_TITLE_OUTSTANDING_CASES).toBe("Outstanding Support Cases");
    expect(DASHBOARD_CHART_TITLE_OUTSTANDING_OPERATIONS).toBe("Outstanding Operations");
    expect(DASHBOARD_CHART_CAPTION_TOTAL).toBe("Total");
  });
});

describe("casesTable constants", () => {
  it("exports table header copy and view tabs", () => {
    expect(CASES_TABLE_HEADER_TITLE).toBe("Outstanding Support Cases");
    expect(CASES_TABLE_CLEAR_FILTERS_LABEL).toBe("Clear Filters");
    expect(DASHBOARD_CASES_VIEW_TABS.map((t) => t.id)).toEqual([
      DashboardCasesViewMode.MyCases,
      DashboardCasesViewMode.AllCases,
    ]);
  });
});

describe("dashboard stat config", () => {
  it("exports four dashboard stat cards", () => {
    expect(DASHBOARD_STATS).toHaveLength(4);
    expect(DASHBOARD_STATS[0]?.label).toBe("Action Required");
  });
});
