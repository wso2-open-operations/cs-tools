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

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  buildCaseResolutionTimeRequest,
  toCaseResolutionTimeResult,
} from "@features/csm-custom-dashboards/api/codeDashboardResolutionTime";

const s1 = [{ field: "severity", op: "in", values: ["critical"] }];

describe("buildCaseResolutionTimeRequest", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("asks for every resolved case of the default window when nothing is given", () => {
    expect(buildCaseResolutionTimeRequest(undefined)).toEqual({ body: { filters: { filters: [] } }, grouped: false });
  });

  it("passes the filters, the window as written and the grouping", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(Date.UTC(2026, 10, 16, 12, 0)));
    expect(
      buildCaseResolutionTimeRequest({
        filters: s1,
        startDate: "__startOfQuarter:-1__",
        endDate: "__endOfQuarter:-1__",
        groupBy: "severity",
        maxGroups: 5,
      }),
    ).toEqual({
      body: {
        filters: { filters: s1 },
        resolvedFrom: "__startOfQuarter:-1__",
        resolvedTo: "__endOfQuarter:-1__",
        groupBy: "severity",
        maxGroups: 5,
      },
      grouped: true,
    });
  });

  it("takes one day, and a window of just under 24 months", () => {
    expect(buildCaseResolutionTimeRequest({ startDate: "2026-03-01", endDate: "2026-03-01" }).body).toMatchObject({
      resolvedFrom: "2026-03-01",
      resolvedTo: "2026-03-01",
    });
    expect(buildCaseResolutionTimeRequest({ startDate: "2024-11-17", endDate: "2026-11-16" }).body).toMatchObject({
      resolvedTo: "2026-11-16",
    });
  });

  it.each([
    [{ days: 30 }, 'caseResolutionTime: unknown argument "days"'],
    [{ filters: "severity" }, "caseResolutionTime: filters must be an array of {field, op, values}"],
    [{ startDate: "2026-01-01" }, "caseResolutionTime: startDate and endDate go together"],
    [{ endDate: "2026-01-31" }, "caseResolutionTime: startDate and endDate go together"],
    [{ startDate: "01/01/2026", endDate: "2026-01-31" }, "caseResolutionTime: startDate must be a date (YYYY-MM-DD) or a date token"],
    [{ startDate: "2026-01-01", endDate: 20260131 }, "caseResolutionTime: endDate must be a date (YYYY-MM-DD) or a date token"],
    [{ startDate: "2026-02-01", endDate: "2026-01-31" }, "caseResolutionTime: endDate must not be before startDate"],
    [{ startDate: "2024-11-16", endDate: "2026-11-16" }, "caseResolutionTime: the window covers at most 24 months"],
    [{ groupBy: "state" }, "caseResolutionTime: groupBy must be one of severity, type, account, product, project, region"],
    [{ groupBy: "productVersion" }, "caseResolutionTime: groupBy must be one of"],
    [{ maxGroups: 5 }, "caseResolutionTime: maxGroups must be a positive integer, with groupBy"],
    [{ groupBy: "project", maxGroups: 0 }, "caseResolutionTime: maxGroups must be a positive integer, with groupBy"],
    [{ groupBy: "project", maxGroups: 2.5 }, "caseResolutionTime: maxGroups must be a positive integer, with groupBy"],
  ])("refuses %j", (args, message) => {
    expect(() => buildCaseResolutionTimeRequest(args)).toThrow(message);
  });
});

describe("toCaseResolutionTimeResult", () => {
  const answer = {
    resolvedFrom: "2026-07-01",
    resolvedTo: "2026-09-30",
    total: { count: 9, measured: 7, averageHours: 20.5 },
    groups: [
      { key: "critical", label: "critical", count: 4, measured: 4, averageHours: 10 },
      { key: "high", label: "", count: 3, measured: 0, averageHours: null },
    ],
    others: { count: 2, measured: 3, averageHours: 45 },
  };

  it("puts the total's figures at the top, without groups when none was asked for", () => {
    expect(toCaseResolutionTimeResult(answer, false)).toEqual({
      startDate: "2026-07-01",
      endDate: "2026-09-30",
      count: 9,
      measured: 7,
      averageHours: 20.5,
    });
  });

  it("lists the groups by label (else key) and the rest as Others", () => {
    expect(toCaseResolutionTimeResult(answer, true).groups).toEqual([
      { label: "critical", count: 4, measured: 4, averageHours: 10 },
      { label: "high", count: 3, measured: 0, averageHours: null },
      { label: "Others", count: 2, measured: 3, averageHours: 45 },
    ]);
  });

  it("leaves out an empty Others, and reads an empty answer as nothing resolved", () => {
    expect(toCaseResolutionTimeResult({ ...answer, others: { count: 0, measured: 0, averageHours: null } }, true).groups).toHaveLength(2);
    expect(toCaseResolutionTimeResult({}, true)).toEqual({
      startDate: null,
      endDate: null,
      count: 0,
      measured: 0,
      averageHours: null,
      groups: [],
    });
  });
});
