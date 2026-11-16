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
import { formatDayLabel, formatHours, formatMttrBarLabel, formatMttrCases, formatMttrHours } from "@features/csm-dashboard/utils/timeCardTotals";

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

describe("formatMttrHours", () => {
  it("keeps at most two decimals and appends the unit", () => {
    expect(formatMttrHours(1259.6395427034176)).toBe("1,259.64 h");
    expect(formatMttrHours(214.5)).toBe("214.5 h");
    expect(formatMttrHours(266)).toBe("266 h");
    expect(formatMttrHours(0)).toBe("0 h");
  });
});

describe("formatMttrCases", () => {
  it("labels the count with a singular or plural noun", () => {
    expect(formatMttrCases(1)).toBe("1 case");
    expect(formatMttrCases(5)).toBe("5 cases");
    expect(formatMttrCases(0)).toBe("0 cases");
  });
});

describe("formatMttrBarLabel", () => {
  it("stays short enough for a bar of a crowded chart", () => {
    expect(formatMttrBarLabel(3435.8522618300003)).toBe("3436h");
    expect(formatMttrBarLabel(82.12713216074074)).toBe("82.1h");
    expect(formatMttrBarLabel(0.250268441944)).toBe("0.3h");
    expect(formatMttrBarLabel(0)).toBe("0h");
  });
});
