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
  basePathForKind,
  matchCaseLocation,
  pathForTab,
} from "@context/case-tabs/caseRoutePatterns";

describe("matchCaseLocation", () => {
  it("matches every known route base", () => {
    expect(matchCaseLocation("/cases/CS1")).toEqual({ kind: "case", caseId: "CS1" });
    expect(matchCaseLocation("/engagements/CS2")).toEqual({
      kind: "engagement",
      caseId: "CS2",
    });
    expect(matchCaseLocation("/announcements/CS3")).toEqual({
      kind: "announcement",
      caseId: "CS3",
    });
    expect(matchCaseLocation("/operations/service-requests/CS4")).toEqual({
      kind: "service_request",
      caseId: "CS4",
    });
    expect(matchCaseLocation("/security-center/security-reports/CS5")).toEqual({
      kind: "security_report_analysis",
      caseId: "CS5",
    });
    expect(matchCaseLocation("/operations/incidents/INC1")).toEqual({
      kind: "incident",
      caseId: "INC1",
    });
    expect(matchCaseLocation("/operations/change-requests/CR1")).toEqual({
      kind: "change_request",
      caseId: "CR1",
    });
  });

  it("does not match a list/index path with no id segment", () => {
    expect(matchCaseLocation("/cases")).toBeUndefined();
    expect(matchCaseLocation("/cases/")).toBeUndefined();
  });

  it("does not match an unrelated route", () => {
    expect(matchCaseLocation("/dashboard")).toBeUndefined();
    expect(matchCaseLocation("/operations/problems/CS1")).toBeUndefined();
  });

  it("only takes the first path segment after the base as the id", () => {
    expect(matchCaseLocation("/cases/CS1/extra")).toEqual({ kind: "case", caseId: "CS1" });
  });

  it("does not treat a sibling create route's 'new' segment as an id", () => {
    expect(matchCaseLocation("/cases/new")).toBeUndefined();
    expect(matchCaseLocation("/engagements/new")).toBeUndefined();
    expect(matchCaseLocation("/announcements/new")).toBeUndefined();
    expect(matchCaseLocation("/operations/service-requests/new")).toBeUndefined();
    expect(matchCaseLocation("/security-center/security-reports/new")).toBeUndefined();
    expect(matchCaseLocation("/operations/incidents/new")).toBeUndefined();
    expect(matchCaseLocation("/operations/change-requests/new")).toBeUndefined();
  });
});

describe("basePathForKind / pathForTab", () => {
  it("round-trip consistently for every kind", () => {
    for (const kind of [
      "case",
      "engagement",
      "announcement",
      "service_request",
      "security_report_analysis",
      "incident",
      "change_request",
    ] as const) {
      const path = pathForTab(kind, "CS1");
      expect(matchCaseLocation(path)).toEqual({ kind, caseId: "CS1" });
      expect(path.startsWith(basePathForKind(kind))).toBe(true);
    }
  });
});

