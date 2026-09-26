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
import type { ScheduleAssignment, ScheduleShift } from "../types";
import {
  dayLabel,
  zoneAbbreviation,
  isPeerRotation,
  isRotationShift,
  mondayOf,
  partsInZone,
  placeOnDay,
  timeOf,
  toIsoDate,
} from "./rota";

const IST = "Asia/Colombo";

/** A night block: 21:00-06:00 IST on Monday 2026-09-21, which is 15:30Z-00:30Z. */
function nightBlock(): ScheduleAssignment {
  return {
    id: "a1",
    engineer: { userId: "u1", name: "Night Engineer", email: "n@example.com", isLead: false },
    teamKey: "americas",
    shiftCode: "CRE_AMERICAS",
    rotaDate: "2026-09-21",
    startsAt: "2026-09-21T15:30:00Z",
    endsAt: "2026-09-22T00:30:00Z",
    isOnCall: false,
    source: "GENERATED",
  };
}

const shift = (code: string, isRotation = true): ScheduleShift => ({
  id: code,
  code,
  shortCode: code,
  label: code,
  family: "CRE",
  dayScope: "WEEKDAY",
  startMinute: 0,
  endMinute: 60,
  authoringTimeZone: IST,
  isOnCall: false,
  isEscalation: false,
  isRotation,
  crossesMidnight: false,
  colourToken: "LK",
  sortOrder: 1,
});

describe("partsInZone", () => {
  it("reads an instant on the clock asked for, not the machine's", () => {
    // 15:30Z is 21:00 in Colombo (+5:30) and 12:30 in São Paulo (-3).
    expect(partsInZone("2026-09-21T15:30:00Z", IST)).toEqual({
      date: "2026-09-21",
      minutes: 21 * 60,
    });
    expect(partsInZone("2026-09-21T15:30:00Z", "America/Sao_Paulo")).toEqual({
      date: "2026-09-21",
      minutes: 12 * 60 + 30,
    });
  });

  it("rolls the date when the clock crosses midnight", () => {
    // 00:30Z on the 22nd is still the 21st in Los Angeles.
    expect(partsInZone("2026-09-22T00:30:00Z", "America/Los_Angeles").date).toBe("2026-09-21");
  });
});

describe("placeOnDay", () => {
  it("draws a night block running off the bottom of the day it belongs to", () => {
    const p = placeOnDay(nightBlock(), new Date(2026, 8, 21), IST);
    expect(p).not.toBeNull();
    expect(p?.startMin).toBe(21 * 60);
    expect(p?.endMin).toBe(1440);
    expect(p?.continuesNextDay).toBe(true);
    expect(p?.startedPreviousDay).toBe(false);
  });

  it("draws the same block arriving at the top of the following day", () => {
    // The crew rostered for Monday is still working on Tuesday morning, and a
    // reader looking at Tuesday has to see them.
    const p = placeOnDay(nightBlock(), new Date(2026, 8, 22), IST);
    expect(p).not.toBeNull();
    expect(p?.startMin).toBe(0);
    expect(p?.endMin).toBe(6 * 60);
    expect(p?.startedPreviousDay).toBe(true);
    expect(p?.continuesNextDay).toBe(false);
  });

  it("places nothing on a day the block does not touch", () => {
    expect(placeOnDay(nightBlock(), new Date(2026, 8, 23), IST)).toBeNull();
  });

  it("moves the block when the reader changes clock", () => {
    // Read from São Paulo the same block runs 12:30-21:30 on the 21st, so it
    // no longer crosses midnight at all.
    const p = placeOnDay(nightBlock(), new Date(2026, 8, 21), "America/Sao_Paulo");
    expect(p?.startMin).toBe(12 * 60 + 30);
    expect(p?.endMin).toBe(21 * 60 + 30);
    expect(p?.continuesNextDay).toBe(false);
  });

  it("gives a window ending exactly at midnight to the day it worked", () => {
    const a = { ...nightBlock(), endsAt: "2026-09-21T18:30:00Z" }; // 00:00 IST on the 22nd
    expect(placeOnDay(a, new Date(2026, 8, 22), IST)).toBeNull();
    expect(placeOnDay(a, new Date(2026, 8, 21), IST)?.endMin).toBe(1440);
  });
});

describe("dayLabel", () => {
  it("names the day the instant falls on, in the reader's zone", () => {
    // 15:30Z on the 21st is still Monday the 21st in Colombo.
    // en-GB abbreviates September as "Sept", not "Sep".
    expect(dayLabel("2026-09-21T15:30:00Z", IST)).toBe("Mon, 21 Sept 2026");
  });

  it("rolls to the previous day when the reader is far enough west", () => {
    // 00:30Z on the 22nd is the evening of the 21st in Los Angeles.
    expect(dayLabel("2026-09-22T00:30:00Z", "America/Los_Angeles")).toBe("Mon, 21 Sept 2026");
  });

  it("does not change shape with the reader's own locale", () => {
    // A numeric date would read as two different days in the US and the UK;
    // this rota is read in both, so the format is pinned.
    expect(dayLabel("2026-09-21T15:30:00Z", IST)).not.toMatch(/\d+\/\d+/);
  });
});

describe("timeOf", () => {
  it("renders the instant on the chosen clock", () => {
    expect(timeOf("2026-09-21T15:30:00Z", IST)).toBe("21:00");
    expect(timeOf("2026-09-21T15:30:00Z", "America/Chicago")).toBe("10:30");
  });
});

describe("mondayOf", () => {
  it("returns the Monday of the week, including when given a Sunday", () => {
    expect(toIsoDate(mondayOf(new Date(2026, 8, 23)))).toBe("2026-09-21"); // Wednesday
    expect(toIsoDate(mondayOf(new Date(2026, 8, 27)))).toBe("2026-09-21"); // Sunday
    expect(toIsoDate(mondayOf(new Date(2026, 8, 21)))).toBe("2026-09-21"); // Monday itself
  });
});

describe("isRotationShift", () => {
  it("counts a rotation but not ordinary working hours", () => {
    expect(isRotationShift(shift("CRE_EVENING"))).toBe(true);
    expect(isRotationShift(shift("SRE_TZ1_L1"))).toBe(true);
    expect(isRotationShift(shift("CRE_REGULAR", false))).toBe(false);
    expect(isRotationShift(shift("CRE_REGULAR_IND", false))).toBe(false);
    expect(isRotationShift(shift("SRE_REGULAR", false))).toBe(false);
    expect(isRotationShift(undefined)).toBe(false);
  });
});

describe("zoneAbbreviation", () => {
  it("uses the name the teams actually say", () => {
    // Intl returns "GMT+5:30" for Colombo; the rota is written in IST and
    // everyone calls it that.
    expect(zoneAbbreviation("Asia/Colombo")).toBe("IST");
    expect(zoneAbbreviation("America/Sao_Paulo")).toBe("BRT");
  });

  it("falls through to Intl elsewhere, so it follows daylight saving", () => {
    expect(["CDT", "CST"]).toContain(zoneAbbreviation("America/Chicago"));
  });
});

describe("isPeerRotation", () => {
  it("excludes regular hours and the Americas night cover", () => {
    // Americas work their own standing shift every day rather than taking a
    // turn in the ABT rotation, so they are not "on with" a CRE engineer.
    expect(isPeerRotation(shift("CRE_EVENING"))).toBe(true);
    expect(isPeerRotation(shift("CRE_MORNING_OC"))).toBe(true);
    expect(isPeerRotation(shift("CRE_AMERICAS", false))).toBe(false);
    expect(isPeerRotation(shift("CRE_REGULAR", false))).toBe(false);
  });
});

