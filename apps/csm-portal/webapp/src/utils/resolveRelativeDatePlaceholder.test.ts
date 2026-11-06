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
  describeRelativeDatePlaceholder,
  resolveRelativeDatePlaceholder,
  resolveRelativeDatePlaceholderToDate,
} from "./resolveRelativeDatePlaceholder";

// A fixed reference instant so every assertion is deterministic regardless of
// the machine/timezone running the test — 2026-08-15 14:30 in whatever
// timezone the test runner's `Date` constructor treats as local. Every
// assertion below re-derives its expectation from this same `now`'s own
// getters, so it holds under any local timezone (CI included).
const NOW = new Date(2026, 7, 15, 14, 30, 0, 0); // Aug 15, 2026, 14:30 local

function localMidnightIso(year: number, month: number, date: number): string {
  return new Date(year, month, date, 0, 0, 0, 0).toISOString();
}

/** One nanosecond-can't-be-represented-in-JS-so-one-millisecond before the
 * next day's local midnight — the `lte` "inclusive of the whole day" bound. */
function localEndOfDayIso(year: number, month: number, date: number): string {
  return new Date(new Date(year, month, date + 1, 0, 0, 0, 0).getTime() - 1).toISOString();
}

describe("resolveRelativeDatePlaceholder", () => {
  it("resolves __today__ for a gte bound to today's local midnight", () => {
    const resolved = resolveRelativeDatePlaceholder("__today__", "gte", NOW);
    expect(resolved).toBe(localMidnightIso(2026, 7, 15));
  });

  it("resolves __today__ for an lte bound to the end of today (local)", () => {
    const resolved = resolveRelativeDatePlaceholder("__today__", "lte", NOW);
    expect(resolved).toBe(localEndOfDayIso(2026, 7, 15));
  });

  it("resolves __daysAgo:N__ relative to today", () => {
    const resolvedGte = resolveRelativeDatePlaceholder("__daysAgo:7__", "gte", NOW);
    expect(resolvedGte).toBe(localMidnightIso(2026, 7, 8));

    const resolvedLte = resolveRelativeDatePlaceholder("__daysAgo:0__", "lte", NOW);
    expect(resolvedLte).toBe(localEndOfDayIso(2026, 7, 15));
  });

  it("does not resolve a negative __daysAgo__ offset", () => {
    expect(resolveRelativeDatePlaceholder("__daysAgo:-1__", "gte", NOW)).toBeUndefined();
  });

  it("resolves __startOfMonth:N__ / __endOfMonth:N__", () => {
    // This month (n=0): Aug 1 - Aug 31, 2026.
    expect(resolveRelativeDatePlaceholder("__startOfMonth:0__", "gte", NOW)).toBe(
      localMidnightIso(2026, 7, 1),
    );
    expect(resolveRelativeDatePlaceholder("__endOfMonth:0__", "lte", NOW)).toBe(
      localEndOfDayIso(2026, 7, 31),
    );
    // Last month (n=-1): Jul 1 - Jul 31, 2026.
    expect(resolveRelativeDatePlaceholder("__startOfMonth:-1__", "gte", NOW)).toBe(
      localMidnightIso(2026, 6, 1),
    );
    expect(resolveRelativeDatePlaceholder("__endOfMonth:-1__", "lte", NOW)).toBe(
      localEndOfDayIso(2026, 6, 31),
    );
  });

  it("resolves __startOfQuarter:N__ / __endOfQuarter:N__", () => {
    // Aug 2026 is in Q3 (Jul-Sep). This quarter (n=0): Jul 1 - Sep 30, 2026.
    expect(resolveRelativeDatePlaceholder("__startOfQuarter:0__", "gte", NOW)).toBe(
      localMidnightIso(2026, 6, 1),
    );
    expect(resolveRelativeDatePlaceholder("__endOfQuarter:0__", "lte", NOW)).toBe(
      localEndOfDayIso(2026, 8, 30),
    );
    // Next quarter (n=1): Oct 1 - Dec 31, 2026.
    expect(resolveRelativeDatePlaceholder("__startOfQuarter:1__", "gte", NOW)).toBe(
      localMidnightIso(2026, 9, 1),
    );
  });

  it("resolves __startOfWeek:N__ / __endOfWeek:N__ to weeks that run Monday to Sunday", () => {
    // Aug 15, 2026 is a Saturday. This week (n=0): Mon Aug 10 - Sun Aug 16.
    expect(resolveRelativeDatePlaceholder("__startOfWeek:0__", "gte", NOW)).toBe(
      localMidnightIso(2026, 7, 10),
    );
    expect(resolveRelativeDatePlaceholder("__endOfWeek:0__", "lte", NOW)).toBe(
      localEndOfDayIso(2026, 7, 16),
    );
    // Last week (n=-1): Mon Aug 3 - Sun Aug 9.
    expect(resolveRelativeDatePlaceholder("__startOfWeek:-1__", "gte", NOW)).toBe(
      localMidnightIso(2026, 7, 3),
    );
    expect(resolveRelativeDatePlaceholder("__endOfWeek:-1__", "lte", NOW)).toBe(
      localEndOfDayIso(2026, 7, 9),
    );
  });

  it("counts a Sunday as the last day of its week, and a Monday as the first", () => {
    const sunday = new Date(2026, 7, 16, 9, 0, 0, 0);
    expect(resolveRelativeDatePlaceholder("__startOfWeek:0__", "gte", sunday)).toBe(
      localMidnightIso(2026, 7, 10),
    );
    const monday = new Date(2026, 7, 10, 9, 0, 0, 0);
    expect(resolveRelativeDatePlaceholder("__startOfWeek:0__", "gte", monday)).toBe(
      localMidnightIso(2026, 7, 10),
    );
    expect(resolveRelativeDatePlaceholder("__endOfWeek:0__", "lte", monday)).toBe(
      localEndOfDayIso(2026, 7, 16),
    );
  });

  it("crosses a month and a year boundary for a week", () => {
    // Thu Jan 1, 2026: its week starts Mon Dec 29, 2025.
    const newYear = new Date(2026, 0, 1, 9, 0, 0, 0);
    expect(resolveRelativeDatePlaceholder("__startOfWeek:0__", "gte", newYear)).toBe(
      localMidnightIso(2025, 11, 29),
    );
    expect(resolveRelativeDatePlaceholder("__endOfWeek:0__", "lte", newYear)).toBe(
      localEndOfDayIso(2026, 0, 4),
    );
  });

  it("resolves __startOfYear:N__ / __endOfYear:N__", () => {
    expect(resolveRelativeDatePlaceholder("__startOfYear:0__", "gte", NOW)).toBe(
      localMidnightIso(2026, 0, 1),
    );
    expect(resolveRelativeDatePlaceholder("__endOfYear:0__", "lte", NOW)).toBe(
      localEndOfDayIso(2026, 11, 31),
    );
    expect(resolveRelativeDatePlaceholder("__startOfYear:-1__", "gte", NOW)).toBe(
      localMidnightIso(2025, 0, 1),
    );
    expect(resolveRelativeDatePlaceholder("__endOfYear:-1__", "lte", NOW)).toBe(
      localEndOfDayIso(2025, 11, 31),
    );
  });

  it("needs an offset on the week and year placeholders", () => {
    for (const v of ["__startOfWeek__", "__endOfWeek__", "__startOfYear__", "__endOfYear__"]) {
      expect(resolveRelativeDatePlaceholder(v, "gte", NOW)).toBeUndefined();
    }
  });

  it("resolves the week and year placeholders to plain dates for a time card's whole-day bounds", () => {
    expect(resolveRelativeDatePlaceholderToDate("__startOfWeek:0__", NOW)).toBe("2026-08-10");
    expect(resolveRelativeDatePlaceholderToDate("__endOfWeek:0__", NOW)).toBe("2026-08-16");
    expect(resolveRelativeDatePlaceholderToDate("__startOfYear:-1__", NOW)).toBe("2025-01-01");
    expect(resolveRelativeDatePlaceholderToDate("__endOfYear:-1__", NOW)).toBe("2025-12-31");
  });

  it("leaves a literal date value untouched", () => {
    expect(resolveRelativeDatePlaceholder("2026-01-01", "gte", NOW)).toBeUndefined();
    expect(
      resolveRelativeDatePlaceholder("2026-01-01T00:00:00Z", "gte", NOW),
    ).toBeUndefined();
  });

  it("leaves an unrecognized placeholder name untouched (falls through to the backend's own rejection)", () => {
    expect(resolveRelativeDatePlaceholder("__current_user_email__", "eq", NOW)).toBeUndefined();
    expect(resolveRelativeDatePlaceholder("__someUnknownThing__", "gte", NOW)).toBeUndefined();
  });

  it("leaves a malformed argument untouched, matching the backend's own fail-fast behavior", () => {
    expect(resolveRelativeDatePlaceholder("__daysAgo:abc__", "gte", NOW)).toBeUndefined();
    expect(resolveRelativeDatePlaceholder("__daysAgo__", "gte", NOW)).toBeUndefined();
    expect(resolveRelativeDatePlaceholder("__today:5__", "gte", NOW)).toBeUndefined();
  });

  // Regression: an extreme (but well-formed) offset computes a Date outside
  // JS's representable range, and `toISOString()` on it throws `RangeError:
  // Invalid time value` instead of returning a string -- which used to
  // escape this function and crash the caller (`resolveRelativeDateFilters`/
  // `useGetCsmCases`) rather than leaving the value unresolved like every
  // other malformed/unrecognized placeholder.
  it("leaves an out-of-range offset unresolved instead of throwing", () => {
    expect(
      resolveRelativeDatePlaceholder("__daysAgo:99999999999__", "gte", NOW),
    ).toBeUndefined();
    expect(
      resolveRelativeDatePlaceholder("__endOfMonth:100000000__", "lte", NOW),
    ).toBeUndefined();
  });
});

describe("describeRelativeDatePlaceholder", () => {
  it("puts each placeholder in words, the same as the Cases list's presets", () => {
    expect(describeRelativeDatePlaceholder("__today__")).toBe("Today");
    expect(describeRelativeDatePlaceholder("__daysAgo:0__")).toBe("Today");
    expect(describeRelativeDatePlaceholder("__daysAgo:1__")).toBe("1 day ago");
    expect(describeRelativeDatePlaceholder("__daysAgo:30__")).toBe("30 days ago");
    expect(describeRelativeDatePlaceholder("__startOfMonth:0__")).toBe("Start of this month");
    expect(describeRelativeDatePlaceholder("__startOfMonth:-1__")).toBe("Start of last month");
    expect(describeRelativeDatePlaceholder("__endOfMonth:1__")).toBe("End of next month");
    expect(describeRelativeDatePlaceholder("__startOfMonth:-3__")).toBe("Start of the month 3 months ago");
    expect(describeRelativeDatePlaceholder("__endOfQuarter:-1__")).toBe("End of last quarter");
    expect(describeRelativeDatePlaceholder("__startOfQuarter:2__")).toBe("Start of the quarter 2 quarters ahead");
    expect(describeRelativeDatePlaceholder("__startOfWeek:0__")).toBe("Start of this week");
    expect(describeRelativeDatePlaceholder("__endOfWeek:-1__")).toBe("End of last week");
    expect(describeRelativeDatePlaceholder("__startOfWeek:-2__")).toBe("Start of the week 2 weeks ago");
    expect(describeRelativeDatePlaceholder("__startOfYear:0__")).toBe("Start of this year");
    expect(describeRelativeDatePlaceholder("__endOfYear:-1__")).toBe("End of last year");
  });

  it("describes nothing the resolver would not resolve", () => {
    for (const v of ["2026-10-04", "__today:5__", "__daysAgo:-1__", "__daysAgo__", "__weeksAgo:2__", "today"]) {
      expect(describeRelativeDatePlaceholder(v)).toBeUndefined();
    }
  });
});
