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
import { MOE_DAY, MOE_NIGHT, TZ1, TZ1_L1, TZ1_WE, TZ2, TZ2_WE, TZ3, REGULAR } from "../test/fixtures";
import { escalationGrid, kindsOfferedOn, monthPieces, moveKindFor, movesOfferedOn, readerFamily, rosterRange, rotaZoneName, zoneColumnOf, zoneDisplayName, zoneLabelOn } from "./rota";
import {
  dayLabel,
  zoneAbbreviation,
  isPeerRotation,
  isRotationShift,
  mondayOf,
  partsInZone,
  placeOnDay,
  standingWindowKey,
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

describe("standingWindowKey", () => {
  /** Regular hours and the India region shift, as the catalogue actually has
   *  them: the same nine-to-five in the same zone on the same days. */
  const regular = { ...shift("CRE_REGULAR", false), startMinute: 540, endMinute: 1080 };
  const india = { ...shift("CRE_REGULAR_IND", false), startMinute: 540, endMinute: 1080 };

  it("folds two standing windows that are the same working day", () => {
    expect(standingWindowKey(regular, regular.code)).toBe(standingWindowKey(india, india.code));
  });

  it("keeps a window with different hours apart", () => {
    const americas = { ...shift("CRE_AMERICAS", false), startMinute: 1260, endMinute: 1800 };
    expect(standingWindowKey(americas, americas.code)).not.toBe(
      standingWindowKey(regular, regular.code),
    );
  });

  it("keeps the same hours apart across CRE and SRE", () => {
    const sre = { ...regular, code: "SRE_REGULAR", family: "SRE" as const };
    expect(standingWindowKey(sre, sre.code)).not.toBe(standingWindowKey(regular, regular.code));
  });

  it("never folds a rotation, however its hours line up", () => {
    const rota = { ...shift("CRE_EVENING"), startMinute: 540, endMinute: 1080 };
    expect(standingWindowKey(rota, rota.code)).toBe(rota.code);
  });

  it("never folds an on-call or escalation window", () => {
    // Being on call is not a fact about which team you are on, so a card that
    // lists teams cannot carry it and the window has to keep its own row.
    const oc = { ...regular, code: "CRE_REGULAR_OC", isOnCall: true };
    const esc = { ...regular, code: "SRE_TZ1", isEscalation: true };
    expect(standingWindowKey(oc, oc.code)).toBe(oc.code);
    expect(standingWindowKey(esc, esc.code)).toBe(esc.code);
  });

  it("falls back to the code when the catalogue has no such shift", () => {
    expect(standingWindowKey(undefined, "MYSTERY")).toBe("MYSTERY");
  });
});

describe("escalationGrid", () => {
  const all = [TZ1, TZ1_L1, TZ2, TZ3, TZ1_WE, TZ2_WE, REGULAR];

  it("offers L1, L2 and L3 in every weekday zone", () => {
    const grid = escalationGrid(all, "2026-09-23"); // a Wednesday
    expect(grid.map((r) => r.zoneCode)).toEqual(["TZ1", "TZ2", "TZ3"]);
    for (const row of grid) expect(row.tiers.map((t) => t.tier)).toEqual(["L1", "L2", "L3"]);
  });

  it("puts a tier on the window that fixes it, else on the zone's open window", () => {
    const tz1 = escalationGrid(all, "2026-09-23")[0];
    expect(tz1.tiers.map((t) => t.shift?.code)).toEqual(["SRE_TZ1_L1", "SRE_TZ1", "SRE_TZ1"]);
    const tz3 = escalationGrid(all, "2026-09-23")[2];
    expect(tz3.tiers.map((t) => t.shift?.code)).toEqual(["SRE_TZ3", "SRE_TZ3", "SRE_TZ3"]);
  });

  it("uses the weekend windows on a weekend", () => {
    const grid = escalationGrid(all, "2026-09-26"); // a Saturday
    expect(grid.map((r) => r.zoneCode)).toEqual(["TZ1", "TZ2"]);
    expect(grid[0].tiers[2].shift?.code).toBe("SRE_WE_TZ1");
  });

  it("is empty for a group with no escalation windows", () => {
    expect(escalationGrid([REGULAR], "2026-09-23")).toEqual([]);
  });
});

describe("zoneLabelOn", () => {
  const WE_TZ12 = { ...TZ1_WE, shortCode: "TZ1+2", label: "Weekend TZ1 + TZ2" };
  it("names the combined weekend crew from its window", () => {
    expect(zoneLabelOn([TZ1, WE_TZ12, TZ3], "TZ1", true)).toBe("TZ1+2");
  });
  it("is the zone's own code on a weekday, and for a zone with no weekend window of its own", () => {
    expect(zoneLabelOn([TZ1, WE_TZ12, TZ3], "TZ1", false)).toBe("TZ1");
    expect(zoneLabelOn([TZ1, WE_TZ12, TZ3], "TZ3", true)).toBe("TZ3");
  });
});

describe("kindsOfferedOn", () => {
  const kind = (code: string, over: Partial<import("../types").ScheduleAbsenceKind> = {}) => ({
    id: code,
    code,
    shortCode: code,
    label: code,
    bucket: "ALLOCATION" as const,
    colourToken: "",
    sortOrder: 0,
    ...over,
  });
  const catalogue = [
    kind("ANNUAL_LEAVE", { bucket: "LEAVE" }),
    kind("ALLO_INT"),
    kind("ALLO_EXT"),
    kind("ALLO_BR"),
    kind("RND", { family: "SRE" }),
    kind("MIGRATION", { family: "CRE" }),
    kind("CUSTOMER_OFFSITE", { retired: true }),
    kind("EXCLUDED", { bucket: "EXCLUDED" }),
  ];
  const codes = (family: "CRE" | "SRE") => kindsOfferedOn(catalogue, family).map((k) => k.code);

  it("offers CRE its own allocations and the shared ones, never RnD", () => {
    expect(codes("CRE")).toEqual(["ANNUAL_LEAVE", "ALLO_INT", "ALLO_EXT", "ALLO_BR", "MIGRATION"]);
  });

  it("offers SRE RnD and the shared ones, never Migration", () => {
    expect(codes("SRE")).toEqual(["ANNUAL_LEAVE", "ALLO_INT", "ALLO_EXT", "ALLO_BR", "RND"]);
  });

  it("never offers a retired kind or EXCLUDED", () => {
    for (const f of ["CRE", "SRE"] as const) {
      expect(codes(f)).not.toContain("CUSTOMER_OFFSITE");
      expect(codes(f)).not.toContain("EXCLUDED");
    }
  });
});

describe("rosterRange and monthPieces", () => {
  const MON = new Date(2026, 8, 28); // Mon 28 Sept 2026

  it("centres each span on the selected day", () => {
    const one = rosterRange(MON, 1);
    expect([toIsoDate(one.start), toIsoDate(one.end)]).toEqual(["2026-09-14", "2026-10-12"]);
    const three = rosterRange(MON, 3);
    expect([toIsoDate(three.start), toIsoDate(three.end)]).toEqual(["2026-08-14", "2026-11-12"]);
    const six = rosterRange(MON, 6);
    expect([toIsoDate(six.start), toIsoDate(six.end)]).toEqual(["2026-06-29", "2026-12-28"]);
  });

  it("cuts a range into the months it touches, clipped at both ends", () => {
    expect(monthPieces(new Date(2026, 8, 14), new Date(2026, 9, 12))).toEqual([
      { from: "2026-09-14", to: "2026-09-30" },
      { from: "2026-10-01", to: "2026-10-12" },
    ]);
  });

  it("keeps a whole month whole, across a year end", () => {
    expect(monthPieces(new Date(2026, 10, 20), new Date(2027, 1, 3))).toEqual([
      { from: "2026-11-20", to: "2026-11-30" },
      { from: "2026-12-01", to: "2026-12-31" },
      { from: "2027-01-01", to: "2027-01-31" },
      { from: "2027-02-01", to: "2027-02-03" },
    ]);
  });
});

describe("readerFamily", () => {
  it("takes the reader's team where they have one", () => {
    expect(readerFamily("SRE-ABT", ["cre_rota_admin"])).toBe("SRE");
    expect(readerFamily("cre-abt", [])).toBe("CRE");
  });

  it("gives a rota admin with no team the group they run", () => {
    expect(readerFamily(undefined, ["cre_rota_admin"])).toBe("CRE");
    expect(readerFamily(null, ["directory.sre_rota_admin"])).toBe("SRE");
  });

  it("reads a rota admin's group from the teams they may edit, when the sign-in has no role", () => {
    expect(readerFamily(undefined, ["cs_engineer"], ["CRE", "CRE"])).toBe("CRE");
    expect(readerFamily(undefined, [], ["SRE"])).toBe("SRE");
    expect(readerFamily(undefined, [], ["CRE", "SRE"])).toBeUndefined();
  });

  it("is nobody's group with neither role, or both", () => {
    expect(readerFamily(undefined, ["admin"])).toBeUndefined();
    expect(readerFamily(undefined, ["cre_rota_admin", "sre_rota_admin"])).toBeUndefined();
  });

  // SME, the product special rotations: read from the team type the way SRE
  // is, so an SME engineer is never quietly taken for CRE.
  it("reads SME from an SME team, an SME rota admin, or SME teams they edit", () => {
    expect(readerFamily("sme-moesif", [])).toBe("SME");
    expect(readerFamily("SME-Asgardeo", ["cre_rota_admin"])).toBe("SME");
    expect(readerFamily(undefined, ["sme_rota_admin"])).toBe("SME");
    expect(readerFamily(undefined, [], ["SME", "SME"])).toBe("SME");
    expect(readerFamily(undefined, ["sme_rota_admin", "sre_rota_admin"])).toBeUndefined();
    expect(readerFamily(undefined, [], ["SME", "SRE"])).toBeUndefined();
  });
});

describe("a rotation's Day and Night zones", () => {
  it("names them Day and Night, and leaves SRE's time zones as they are", () => {
    expect(zoneDisplayName("MOE_D")).toBe("Day");
    expect(zoneDisplayName("IAAS_N")).toBe("Night");
    expect(zoneDisplayName("TZ1")).toBe("TZ1");
    expect(zoneLabelOn([MOE_DAY, MOE_NIGHT], "MOE_N", true)).toBe("Night");
    expect(zoneLabelOn([TZ1_WE], "TZ1", true)).toBe("TZ1"); // a chip short code ("L1") is no crew name
  });

  it("puts L1, L2 and L3 on each, every day of the week", () => {
    for (const iso of ["2026-09-21", "2026-09-26"]) {
      const grid = escalationGrid([MOE_DAY, MOE_NIGHT], iso);
      // named with the rotation, so several rotations' rows can sit together
      expect(grid.map((r) => [r.zoneCode, r.label])).toEqual([["MOE_D", "Moesif Day"], ["MOE_N", "Moesif Night"]]);
      expect(grid[0].tiers.map((t) => t.shift?.code)).toEqual(["SME_MOE_DAY", "SME_MOE_DAY", "SME_MOE_DAY"]);
    }
  });
});

describe("rotaZoneName and zoneColumnOf", () => {
  it("names a rotation's zone with its rotation, and leaves SRE's time zones alone", () => {
    expect(rotaZoneName([MOE_DAY, MOE_NIGHT], "MOE_N")).toBe("Moesif Night");
    expect(rotaZoneName([TZ1], "TZ1")).toBe("TZ1");
  });
  it("puts every rotation's Day in one roster column and its Night in another", () => {
    expect(zoneColumnOf("ASG_D")).toBe(zoneColumnOf("MOE_D"));
    expect(zoneColumnOf("IAAS_N")).toBe("Night");
    expect(zoneColumnOf("TZ2")).toBe("TZ2");
  });
});


describe("a tag worked as another team's normal hours", () => {
  const BR = {
    id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
    colourToken: "BR", sortOrder: 70, movesToTeamKey: "americas", worksRotaThere: true, showsAsShiftCode: "CRE_AMERICAS",
  };
  const MIG = {
    id: "k-mig", code: "MIGRATION", shortCode: "Mig", label: "Migration", bucket: "ALLOCATION" as const,
    colourToken: "MIG", sortOrder: 90, movesToTeamKey: "migration", showsAsShiftCode: "CRE_REGULAR",
  };

  it("is not offered as time away; a plain move is offered as a move, a worked one as its shift", () => {
    expect(kindsOfferedOn([BR, MIG], "CRE").map((k) => k.code)).toEqual([]);
    expect(movesOfferedOn([BR, MIG], "CRE").map((k) => k.code)).toEqual(["MIGRATION"]);
  });

  it("is what picking that shift means for somebody on another team", () => {
    expect(moveKindFor([BR, MIG], "CRE_AMERICAS", "vega")?.code).toBe("ALLO_BR");
  });

  it("is not involved for somebody already on that team, nor for any other shift", () => {
    expect(moveKindFor([BR, MIG], "CRE_AMERICAS", "Americas")).toBeUndefined();
    // Regular hours is everybody's: Migration, which is not rota work there,
    // is never picked by choosing it.
    expect(moveKindFor([BR, MIG], "CRE_REGULAR", "vega")).toBeUndefined();
  });
});
