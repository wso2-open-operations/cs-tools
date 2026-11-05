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

import { describe, expect, it } from "vitest";
import {
  formatDateOnly,
  formatDateTimeLocal,
  isCompleteDate,
  parseDateOnly,
  parseDateTimeLocal,
} from "@utils/dateTimeLocal";

describe("parseDateTimeLocal", () => {
  it("keeps the wall-clock fields of a YYYY-MM-DDTHH:mm string", () => {
    const date = parseDateTimeLocal("2026-06-10T04:30");
    expect(date?.getFullYear()).toBe(2026);
    expect(date?.getMonth()).toBe(5);
    expect(date?.getDate()).toBe(10);
    expect(date?.getHours()).toBe(4);
    expect(date?.getMinutes()).toBe(30);
  });

  it("accepts seconds and ignores them", () => {
    expect(parseDateTimeLocal("2026-06-10T04:30:45")?.getMinutes()).toBe(30);
  });

  it.each([["", null], ["not a date", null], ["2026-06-10", null], [undefined, null], [null, null]])(
    "returns null for %j",
    (value, expected) => {
      expect(parseDateTimeLocal(value as string | null | undefined)).toBe(expected);
    },
  );
});

describe("impossible calendar and clock values", () => {
  it.each([
    ["30 February", "2026-02-30T10:00"],
    ["month 13", "2026-13-01T10:00"],
    ["month 0", "2026-00-10T10:00"],
    ["day 0", "2026-06-00T10:00"],
    ["hour 25", "2026-06-10T25:00"],
    ["minute 61", "2026-06-10T10:61"],
    ["a year below 100", "0050-06-10T10:00"],
  ])("rejects %s instead of rolling it over", (_label, value) => {
    expect(parseDateTimeLocal(value)).toBeNull();
  });

  it("accepts 29 February only in a leap year", () => {
    expect(parseDateTimeLocal("2028-02-29T10:00")?.getDate()).toBe(29);
    expect(parseDateTimeLocal("2027-02-29T10:00")).toBeNull();
  });

  it("applies the same checks to date-only values", () => {
    expect(parseDateOnly("2026-02-30")).toBeNull();
    expect(parseDateOnly("2026-13-01")).toBeNull();
    expect(parseDateOnly("0050-06-10")).toBeNull();
    expect(parseDateOnly("2028-02-29")?.getMonth()).toBe(1);
  });
});

describe("formatDateTimeLocal", () => {
  it("round-trips with parseDateTimeLocal", () => {
    expect(formatDateTimeLocal(parseDateTimeLocal("2026-01-05T09:05") as Date)).toBe("2026-01-05T09:05");
  });

  it("zero-pads every field", () => {
    expect(formatDateTimeLocal(new Date(2026, 0, 2, 3, 4))).toBe("2026-01-02T03:04");
  });
});

describe("date-only helpers", () => {
  it("parses to local midnight and formats back", () => {
    const date = parseDateOnly("2026-03-04");
    expect(date?.getHours()).toBe(0);
    expect(formatDateOnly(date as Date)).toBe("2026-03-04");
  });

  it("ignores a time part and returns null for nothing usable", () => {
    expect(formatDateOnly(parseDateOnly("2026-03-04T10:00") as Date)).toBe("2026-03-04");
    expect(parseDateOnly("")).toBeNull();
    expect(parseDateOnly("garbage")).toBeNull();
  });
});

describe("isCompleteDate", () => {
  it("accepts a valid date with a four-digit year", () => {
    expect(isCompleteDate(new Date(2026, 5, 10))).toBe(true);
  });

  it("rejects an invalid Date, a short year, null and non-dates", () => {
    expect(isCompleteDate(new Date(Number.NaN))).toBe(false);
    const shortYear = new Date(2026, 0, 10);
    shortYear.setFullYear(2);
    expect(isCompleteDate(shortYear)).toBe(false);
    expect(isCompleteDate(null)).toBe(false);
    expect(isCompleteDate("2026-06-10")).toBe(false);
  });
});
