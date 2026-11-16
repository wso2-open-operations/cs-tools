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

import { computeQueryHoursSlices } from "@features/csm-reports/components/queryHoursSlices";

describe("computeQueryHoursSlices", () => {
  it("splits a normal balance", () => {
    expect(computeQueryHoursSlices("30h 0m", "40h 0m")).toEqual({
      consumed: 10,
      remaining: 30,
      consumedPct: 25,
      remainingPct: 75,
    });
  });
  it("clamps a negative remaining balance to zero", () => {
    const slices = computeQueryHoursSlices("-1h 30m", "10h 0m");
    expect(slices).toEqual({ consumed: 10, remaining: 0, consumedPct: 100, remainingPct: 0 });
    expect(computeQueryHoursSlices("-45m", "10h 0m").remaining).toBe(0);
  });
  it("clamps remaining above total", () => {
    expect(computeQueryHoursSlices("50h 0m", "40h 0m")).toMatchObject({ consumed: 0, remaining: 40, remainingPct: 100 });
  });
  it("guards a zero, negative or unparsable total", () => {
    const zero = { consumed: 0, remaining: 0, consumedPct: 0, remainingPct: 0 };
    expect(computeQueryHoursSlices("0", "0")).toEqual(zero);
    expect(computeQueryHoursSlices("5h 0m", "-2h 0m")).toEqual(zero);
    expect(computeQueryHoursSlices("5h 0m", "abc")).toEqual(zero);
  });
  it("never yields NaN or negative values", () => {
    for (const [r, t] of [["-1h -29m", "5h"], ["", ""], ["-3", "2"]]) {
      const s = computeQueryHoursSlices(r, t);
      for (const v of Object.values(s)) {
        expect(Number.isNaN(v)).toBe(false);
        expect(v).toBeGreaterThanOrEqual(0);
      }
    }
  });
});
