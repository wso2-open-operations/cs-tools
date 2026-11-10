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

import { parseHours } from "@features/csm-reports/components/parseHours";

describe("parseHours", () => {
  it("parses the display strings the backend returns", () => {
    expect(parseHours("70h 0m")).toBe(70);
    expect(parseHours("36h 36m")).toBeCloseTo(36.6, 5);
  });
  it("keeps parsing plain numbers and minutes", () => {
    expect(parseHours("12.5")).toBe(12.5);
    expect(parseHours("30m")).toBe(0.5);
    expect(parseHours("")).toBe(0);
  });
  it("parses a single leading sign as a negative balance", () => {
    expect(parseHours("-1h 30m")).toBe(-1.5);
    expect(parseHours("-45m")).toBe(-0.75);
    expect(parseHours("-1h 0m")).toBe(-1);
    expect(parseHours("-0m")).toBe(0);
    expect(parseHours("-2.5")).toBe(-2.5);
  });
});
