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
  clampDateInput,
  daysAgoLocalStr,
  localDateStr,
  todayLocalStr,
} from "@features/spl/usage-metrics/utils/dateBounds";

describe("usage metrics date bounds", () => {
  it("formats the local calendar date, not the UTC one", () => {
    // 00:30 local on the 2nd: toISOString() would be the 1st for any zone east of UTC.
    expect(localDateStr(new Date(2026, 9, 2, 0, 30))).toBe("2026-10-02");
    expect(todayLocalStr(new Date(2026, 9, 2, 0, 30))).toBe("2026-10-02");
  });

  it("computes days-ago per call from the given 'now'", () => {
    expect(daysAgoLocalStr(30, new Date(2026, 9, 2))).toBe("2026-09-02");
    expect(daysAgoLocalStr(30, new Date(2026, 9, 5))).toBe("2026-09-05");
  });

  it("clamps typed dates into the allowed window", () => {
    expect(clampDateInput("2026-12-01", "2025-10-01", "2026-10-02")).toBe("2026-10-02");
    expect(clampDateInput("2020-01-01", "2025-10-01", "2026-10-02")).toBe("2025-10-01");
    expect(clampDateInput("2026-09-01", "2025-10-01", "2026-10-02")).toBe("2026-09-01");
    expect(clampDateInput("2026-0", "2025-10-01", "2026-10-02")).toBe("2026-0");
  });
});
