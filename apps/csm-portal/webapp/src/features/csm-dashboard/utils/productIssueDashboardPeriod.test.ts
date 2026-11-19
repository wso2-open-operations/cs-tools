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
  productIssuePeriodRange,
  productIssueTrendPoints,
} from "@features/csm-dashboard/utils/productIssueDashboardPeriod";

describe("productIssuePeriodRange", () => {
  // Wed 18 Nov 2026 12:00 UTC = 17:30 IST; the IST Monday is 16 Nov.
  const now = new Date("2026-11-18T12:00:00Z");

  it("ends tomorrow and starts on the Monday N-1 weeks before this week's", () => {
    expect(productIssuePeriodRange("4w", now)).toEqual({ from: "2026-10-26", to: "2026-11-19", bucket: "week" });
    expect(productIssuePeriodRange("12w", now)).toEqual({ from: "2026-08-31", to: "2026-11-19", bucket: "week" });
    expect(productIssuePeriodRange("26w", now).from).toBe("2026-05-25");
  });

  it("uses months from the 1st for the month periods", () => {
    expect(productIssuePeriodRange("6m", now)).toEqual({ from: "2026-06-01", to: "2026-11-19", bucket: "month" });
    expect(productIssuePeriodRange("12m", now)).toEqual({ from: "2025-12-01", to: "2026-11-19", bucket: "month" });
  });

  it("reads today in Asia/Kolkata, not UTC", () => {
    // 20:00 UTC on Sunday 15 Nov is already Monday 16 Nov 01:30 IST.
    expect(productIssuePeriodRange("4w", new Date("2026-11-15T20:00:00Z"))).toEqual({
      from: "2026-10-26",
      to: "2026-11-17",
      bucket: "week",
    });
    // Sunday evening UTC-wise and IST-wise stays in the old week.
    expect(productIssuePeriodRange("4w", new Date("2026-11-15T10:00:00Z")).from).toBe("2026-10-19");
  });

  it("stays inside the 400 day limit", () => {
    for (const id of ["4w", "12w", "26w", "6m", "12m"] as const) {
      const r = productIssuePeriodRange(id, new Date("2026-12-31T23:00:00Z"));
      const days = (Date.parse(r.to) - Date.parse(r.from)) / 86_400_000;
      expect(days).toBeLessThanOrEqual(400);
    }
  });
});

describe("productIssueTrendPoints", () => {
  it("turns the exclusive end into the bucket's last day", () => {
    expect(
      productIssueTrendPoints([
        { start: "2026-11-09T00:00:00+05:30", end: "2026-11-16T00:00:00+05:30", opened: 3, resolved: 2 },
      ]),
    ).toEqual([{ date: "2026-11-09", endDate: "2026-11-15", opened: 3, closed: 2 }]);
  });
});
