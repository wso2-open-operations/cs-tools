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
  buildDashboardCaseSearchFilters,
  getDashboardOutstandingCasesDescription,
  getDashboardOutstandingCasesTitle,
  getDashboardSeverityHeadingLabel,
} from "@features/dashboard/utils/dashboardNavigation";

describe("getDashboardSeverityHeadingLabel", () => {
  it("maps known severity IDs", () => {
    expect(getDashboardSeverityHeadingLabel("10")).toBe("S1");
    expect(getDashboardSeverityHeadingLabel("11")).toBe("S2");
  });

  it("returns undefined for unknown severity IDs", () => {
    expect(getDashboardSeverityHeadingLabel("999")).toBeUndefined();
  });
});

describe("getDashboardOutstandingCasesTitle", () => {
  it("builds Outstanding Sx title", () => {
    expect(getDashboardOutstandingCasesTitle("10")).toBe("Outstanding S1 Cases");
  });
});

describe("getDashboardOutstandingCasesDescription", () => {
  it("builds severity-specific description", () => {
    expect(getDashboardOutstandingCasesDescription("10")).toBe(
      "Manage and track S1 outstanding support cases",
    );
  });
});

describe("buildDashboardCaseSearchFilters", () => {
  it("uses explicit statusId when selected", () => {
    expect(
      buildDashboardCaseSearchFilters({
        statusId: "10",
        severityId: "11",
        searchQuery: " test ",
      }),
    ).toEqual({
      statusIds: [10],
      severityId: 11,
      issueId: undefined,
      deploymentId: undefined,
      searchQuery: "test",
      createdByMe: undefined,
    });
  });

  it("uses outstanding statusIds for dashboard severity navigation", () => {
    expect(
      buildDashboardCaseSearchFilters({
        severityId: "11",
        isDashboardSeverityNavigation: true,
        caseStates: [
          { id: "1", label: "Open" },
          { id: "3", label: "Closed" },
          { id: "1006", label: "Reopened" },
        ],
      }),
    ).toEqual({
      statusIds: [1, 1006],
      severityId: 11,
      issueId: undefined,
      deploymentId: undefined,
      searchQuery: undefined,
      createdByMe: undefined,
    });
  });
});

