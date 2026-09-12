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
import { resolveRelativeDateFilters } from "./resolveRelativeDateFilters";

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

describe("resolveRelativeDateFilters", () => {
  it("resolves a createdOn gte/lte 'today' range in a widget's filters", () => {
    const filters = {
      filters: [
        { field: "type", op: "in", values: ["case"] },
        { field: "createdOn", op: "gte", values: ["__today__"] },
      ],
    };

    const resolved = resolveRelativeDateFilters(filters, NOW);

    expect(resolved).toEqual({
      filters: [
        { field: "type", op: "in", values: ["case"] },
        { field: "createdOn", op: "gte", values: [localMidnightIso(2026, 7, 15)] },
      ],
    });
  });

  it("leaves a literal (non-placeholder) filter value untouched", () => {
    const filters = {
      filters: [
        { field: "projectType", op: "in", values: ["cb4a87bd-0000-0000-0000-000000000000"] },
        { field: "createdOn", op: "gte", values: ["2026-01-01"] },
      ],
    };

    expect(resolveRelativeDateFilters(filters, NOW)).toBe(filters);
  });

  it("returns non-case-filter-shaped filters (other resourceTypes) unchanged", () => {
    const filters = { states: ["open"] };
    expect(resolveRelativeDateFilters(filters, NOW)).toBe(filters);
  });

  it("passes through an empty filters array unchanged", () => {
    const filters = { filters: [] };
    expect(resolveRelativeDateFilters(filters, NOW)).toBe(filters);
  });

  it("only resolves the placeholder entry, leaving sibling entries and other values in the same array alone", () => {
    const filters = {
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "resolvedOn", op: "lte", values: ["__daysAgo:30__"] },
        { field: "resolvedOn", op: "gte", values: ["2026-01-01"] },
      ],
    };

    const resolved = resolveRelativeDateFilters(filters, NOW);

    expect(resolved).toEqual({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "resolvedOn", op: "lte", values: [localEndOfDayIso(2026, 6, 16)] },
        { field: "resolvedOn", op: "gte", values: ["2026-01-01"] },
      ],
    });
  });

  it("defaults to real wall-clock time when no reference instant is passed", () => {
    // The function reads `new Date()` itself and the expectation below reads
    // it again; without a frozen clock the two reads can land on either side
    // of local midnight and belong to different calendar days.
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
    try {
      const filters = {
        filters: [{ field: "createdOn", op: "gte", values: ["__today__"] }],
      };

      const resolved = resolveRelativeDateFilters(filters) as {
        filters: { values: string[] }[];
      };

      expect(resolved.filters[0].values[0]).toBe(localMidnightIso(2026, 7, 15));
    } finally {
      vi.useRealTimers();
    }
  });
});
