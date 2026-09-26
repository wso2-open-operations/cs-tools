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
import { settleWithConcurrencyLimit } from "./settleWithConcurrencyLimit";

describe("settleWithConcurrencyLimit", () => {
  it("never runs more than `limit` invocations at once", async () => {
    const items = Array.from({ length: 20 }, (_, i) => i);
    let active = 0;
    let maxActive = 0;

    await settleWithConcurrencyLimit(items, 5, async (i) => {
      active++;
      maxActive = Math.max(maxActive, active);
      await new Promise((resolve) => setTimeout(resolve, 1));
      active--;
      return i * 2;
    });

    expect(maxActive).toBeLessThanOrEqual(5);
  });

  it("returns one settled result per item, in the original order, matching Promise.allSettled's shape", async () => {
    const items = ["a", "b", "c"];

    const results = await settleWithConcurrencyLimit(items, 2, async (item) => {
      if (item === "b") throw new Error("b failed");
      return `${item}-ok`;
    });

    expect(results).toEqual([
      { status: "fulfilled", value: "a-ok" },
      { status: "rejected", reason: new Error("b failed") },
      { status: "fulfilled", value: "c-ok" },
    ]);
  });

  it("handles an empty item list", async () => {
    const results = await settleWithConcurrencyLimit([], 5, async (x) => x);
    expect(results).toEqual([]);
  });

  it("handles limit greater than the item count without over-spawning workers", async () => {
    const results = await settleWithConcurrencyLimit([1, 2], 10, async (x) => x * 10);
    expect(results).toEqual([
      { status: "fulfilled", value: 10 },
      { status: "fulfilled", value: 20 },
    ]);
  });
});
