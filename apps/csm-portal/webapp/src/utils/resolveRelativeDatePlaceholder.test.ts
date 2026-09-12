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
import { resolveRelativeDatePlaceholder } from "./resolveRelativeDatePlaceholder";

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
