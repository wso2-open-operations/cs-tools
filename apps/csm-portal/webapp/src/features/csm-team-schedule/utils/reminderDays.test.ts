/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it } from "vitest";
import { describeDays, reminderDays } from "./reminderDays";

// Midday in Colombo (UTC+05:30) on each day of the week of 19 October 2026.
const MON = new Date("2026-10-19T06:30:00Z");
const TUE = new Date("2026-10-20T06:30:00Z");
const WED = new Date("2026-10-21T06:30:00Z");
const THU = new Date("2026-10-22T06:30:00Z");
const FRI = new Date("2026-10-23T06:30:00Z");
const SAT = new Date("2026-10-24T06:30:00Z");
const SUN = new Date("2026-10-25T06:30:00Z");

describe("reminderDays: morning and evening", () => {
  it("are the next working day, like weekday", () => {
    for (const now of [MON, THU, FRI, SAT, SUN]) {
      expect(reminderDays("morning", now)).toEqual(reminderDays("weekday", now));
      expect(reminderDays("evening", now)).toEqual(reminderDays("weekday", now));
    }
  });
});

describe("reminderDays: weekday", () => {
  it("is the next day from Monday to Thursday", () => {
    expect(reminderDays("weekday", MON)).toEqual(["2026-10-20"]);
    expect(reminderDays("weekday", TUE)).toEqual(["2026-10-21"]);
    expect(reminderDays("weekday", WED)).toEqual(["2026-10-22"]);
    expect(reminderDays("weekday", THU)).toEqual(["2026-10-23"]);
  });

  it("skips the weekend: Friday, Saturday and Sunday all give Monday", () => {
    expect(reminderDays("weekday", FRI)).toEqual(["2026-10-26"]);
    expect(reminderDays("weekday", SAT)).toEqual(["2026-10-26"]);
    expect(reminderDays("weekday", SUN)).toEqual(["2026-10-26"]);
  });

  it("crosses a month and a year", () => {
    expect(reminderDays("weekday", new Date("2026-10-30T06:30:00Z"))).toEqual(["2026-11-02"]);
    expect(reminderDays("weekday", new Date("2026-12-31T06:30:00Z"))).toEqual(["2027-01-01"]);
  });
});

describe("reminderDays: weekend", () => {
  it("is the coming Saturday and Sunday from Monday to Friday", () => {
    for (const day of [MON, TUE, WED, THU, FRI]) {
      expect(reminderDays("weekend", day)).toEqual(["2026-10-24", "2026-10-25"]);
    }
  });

  it("is today and tomorrow on a Saturday", () => {
    expect(reminderDays("weekend", SAT)).toEqual(["2026-10-24", "2026-10-25"]);
  });

  it("is yesterday and today on a Sunday", () => {
    expect(reminderDays("weekend", SUN)).toEqual(["2026-10-24", "2026-10-25"]);
  });

  it("crosses a month", () => {
    expect(reminderDays("weekend", new Date("2026-10-26T06:30:00Z"))).toEqual(["2026-10-31", "2026-11-01"]);
    expect(reminderDays("weekend", new Date("2026-11-01T06:30:00Z"))).toEqual(["2026-10-31", "2026-11-01"]);
  });
});

describe("reminderDays: the day is Colombo's, not UTC's", () => {
  it("reads 20:00 UTC on a Friday as Saturday in Colombo", () => {
    const now = new Date("2026-10-23T20:00:00Z");
    expect(reminderDays("weekday", now)).toEqual(["2026-10-26"]);
    expect(reminderDays("weekend", now)).toEqual(["2026-10-24", "2026-10-25"]);
  });

  it("reads 20:00 UTC on a Saturday as Sunday in Colombo", () => {
    const now = new Date("2026-10-24T20:00:00Z");
    expect(reminderDays("weekday", now)).toEqual(["2026-10-26"]);
    expect(reminderDays("weekend", now)).toEqual(["2026-10-24", "2026-10-25"]);
  });

  it("reads 20:00 UTC on a Thursday as Friday in Colombo", () => {
    const now = new Date("2026-10-22T20:00:00Z");
    expect(reminderDays("weekday", now)).toEqual(["2026-10-26"]);
    expect(reminderDays("weekend", now)).toEqual(["2026-10-24", "2026-10-25"]);
  });

  it("keeps a Colombo morning on the same day UTC is still the day before", () => {
    // 02:00 Colombo on Monday 19 October is 20:30 UTC on Sunday 18 October.
    const now = new Date("2026-10-18T20:30:00Z");
    expect(reminderDays("weekday", now)).toEqual(["2026-10-20"]);
    expect(reminderDays("weekend", now)).toEqual(["2026-10-24", "2026-10-25"]);
  });

  it("rolls into the new year with Colombo", () => {
    // 31 December 2026 at 20:00 UTC is Friday 1 January 2027 in Colombo.
    const now = new Date("2026-12-31T20:00:00Z");
    expect(reminderDays("weekday", now)).toEqual(["2027-01-04"]);
    expect(reminderDays("weekend", now)).toEqual(["2027-01-02", "2027-01-03"]);
  });
});

describe("describeDays", () => {
  it("names one day", () => {
    expect(describeDays(["2026-10-20"])).toBe("Tuesday, 20 October");
  });

  it("names two days of one month together", () => {
    expect(describeDays(["2026-10-24", "2026-10-25"])).toBe("Saturday, 24 and Sunday, 25 October");
  });

  it("names two days of two months in full", () => {
    expect(describeDays(["2026-10-31", "2026-11-01"])).toBe("Saturday, 31 October and Sunday, 1 November");
  });
});
