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
import { formatDayLabel, formatHours } from "@features/csm-dashboard/utils/timeCardTotals";

describe("formatHours", () => {
  it("rounds to one decimal and adds h", () => {
    expect(formatHours(12.5)).toBe("12.5 h");
    expect(formatHours(4.25)).toBe("4.3 h");
    expect(formatHours(4)).toBe("4 h");
    expect(formatHours(0)).toBe("0 h");
    expect(formatHours(1234.56)).toBe("1,234.6 h");
  });
});

describe("formatDayLabel", () => {
  it("shows a day key as a short local date", () => {
    expect(formatDayLabel("2026-10-30")).toBe("Oct 30");
    expect(formatDayLabel("2026-11-01")).toBe("Nov 1");
    expect(formatDayLabel("2024-02-29")).toBe("Feb 29");
  });

  it("leaves anything that is not a day key unchanged", () => {
    expect(formatDayLabel("approved")).toBe("approved");
    expect(formatDayLabel("2026-10-30T00:00:00Z")).toBe("2026-10-30T00:00:00Z");
  });
});
