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
import { safeRouteId } from "@features/spl/utils/routeId";

describe("safeRouteId", () => {
  it("accepts UUIDs, 32-hex ids and record numbers", () => {
    expect(safeRouteId("00000000-0000-0000-0000-000000000000")).toBe(
      "00000000-0000-0000-0000-000000000000",
    );
    expect(safeRouteId("0123456789abcdef0123456789abcdef")).toBe("0123456789abcdef0123456789abcdef");
    expect(safeRouteId("CS0000001")).toBe("CS0000001");
  });

  it("returns an empty id for missing or malformed values, without rewriting them", () => {
    expect(safeRouteId(undefined)).toBe("");
    expect(safeRouteId("")).toBe("");
    expect(safeRouteId("a/b")).toBe("");
    expect(safeRouteId("../x")).toBe("");
    expect(safeRouteId("<img src=x>")).toBe("");
    expect(safeRouteId("a".repeat(65))).toBe("");
  });
});
