/*
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
import type {
  ScheduleAbsence,
  ScheduleAbsenceKind,
  ScheduleAssignment,
  ScheduleShift,
  ScheduleTeam,
  ScheduleTeamMember,
} from "../types";
import { atTeamOrdinaryHours, atWorkOnTheDay, creRowsAtWork, instantInZone, onItsStandingCard, STANDING_DAY_SOURCE, standingDayRows } from "./standingDay";

const COLOMBO = "Asia/Colombo";

function shift(code: string, startMinute: number, endMinute: number, extra: Partial<ScheduleShift> = {}): ScheduleShift {
  return {
    id: code,
    code,
    shortCode: code,
    label: code,
    family: "CRE",
    dayScope: "WEEKDAY",
    startMinute,
    endMinute,
    authoringTimeZone: COLOMBO,
    isOnCall: false,
    isEscalation: false,
    isRotation: true,
    crossesMidnight: endMinute > 1440,
    colourToken: code,
    sortOrder: 0,
    ...extra,
  };
}

const SHIFTS = new Map(
  [
    shift("CRE_REGULAR", 540, 1080, { isRotation: false }),
    shift("CRE_AMERICAS", 1260, 1800, { isRotation: false }),
    shift("CRE_AMERICAS_PT", 480, 1080, { isRotation: false, authoringTimeZone: "Etc/GMT+7" }),
    shift("CRE_REGULAR_IND", 540, 1080, { isRotation: false, authoringTimeZone: "Asia/Kolkata" }),
    shift("CRE_EVENING", 1080, 1260),
    shift("CRE_MORNING_OC", 360, 540, { isOnCall: true }),
    shift("CRE_LATE", 1260, 1380),
    shift("CRE_ESC_EVE", 1080, 1260, { isEscalation: true }),
  ].map((s) => [s.code, s]),
);

const member = (id: string, role = "engineer"): ScheduleTeamMember => ({
  userId: id,
  name: `Engineer ${id}`,
  email: `${id}@example.com`,
  isLead: role === "lead",
  role,
});

const team = (key: string, members: ScheduleTeamMember[], defaultShiftCode?: string): ScheduleTeam => ({
  key,
  name: key,
  family: "CRE",
  sortOrder: 0,
  members,
  defaultShiftCode,
});

const row = (id: string, teamKey: string, shiftCode: string, rotaDate: string): ScheduleAssignment => ({
  id: `${id}-${shiftCode}`,
  engineer: { userId: id, name: `Engineer ${id}`, email: `${id}@example.com`, isLead: false },
  teamKey,
  shiftCode,
  rotaDate,
  startsAt: "2026-10-12T00:00:00Z",
  endsAt: "2026-10-12T01:00:00Z",
  isOnCall: false,
  source: "IMPORTED",
});

const away = (id: string, startsOn: string, endsOn?: string): ScheduleAbsence => ({
  id: `ab-${id}`,
  engineer: { userId: id, name: `Engineer ${id}`, email: `${id}@example.com`, isLead: false },
  teamKey: "draco",
  kindCode: "ANNUAL_LEAVE",
  startsOn,
  endsOn,
});

const MONDAY = "2026-10-12";
const SATURDAY = "2026-10-17";

describe("standingDayRows", () => {
  it("puts an India region team's engineers on IND and its lead on Regular hours", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY],
      teams: [team("phoenix", [member("lead", "lead"), member("e")], "CRE_REGULAR_IND")],
      shifts: SHIFTS,
      assignments: [],
      absences: [],
    });
    const codeOf = (id: string) => out.find((r) => r.engineer.userId === id)?.shiftCode;
    expect(codeOf("e")).toBe("CRE_REGULAR_IND");
    expect(codeOf("lead")).toBe("CRE_REGULAR");
  });

  it("keeps the Americas team's leads on Americas cover", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY],
      teams: [team("americas", [member("lead", "lead")], "CRE_AMERICAS")],
      shifts: SHIFTS,
      assignments: [],
      absences: [],
    });
    expect(out.map((r) => r.shiftCode)).toEqual(["CRE_AMERICAS"]);
  });

  it("lists a member with no row and no leave as working their team's regular hours", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY],
      teams: [team("draco", [member("a"), member("b")])],
      shifts: SHIFTS,
      assignments: [row("a", "draco", "CRE_REGULAR", MONDAY)],
      absences: [],
    });
    expect(out).toHaveLength(1);
    expect(out[0]).toMatchObject({
      engineer: { userId: "b" },
      teamKey: "draco",
      shiftCode: "CRE_REGULAR",
      rotaDate: MONDAY,
      source: STANDING_DAY_SOURCE,
      startsAt: "2026-10-12T03:30:00.000Z",
      endsAt: "2026-10-12T12:30:00.000Z",
    });
  });

  it("leaves out people on leave, managers, and weekends", () => {
    const teams = [team("draco", [member("a"), member("h", "cre_head"), member("l", "lead")])];
    expect(
      standingDayRows({ rotaDates: [MONDAY], teams, shifts: SHIFTS, assignments: [], absences: [away("a", "2026-10-10")] })
        .map((r) => r.engineer.userId),
    ).toEqual(["l"]);
    expect(
      standingDayRows({ rotaDates: [SATURDAY], teams, shifts: SHIFTS, assignments: [], absences: [] }),
    ).toEqual([]);
  });

  it("keeps someone on half-day leave on their ordinary day", () => {
    const teams = [team("draco", [member("a")])];
    const halfDay = { ...away("a", MONDAY, MONDAY), kindCode: "HALF_DAY_LEAVE_MORNING" };
    expect(
      standingDayRows({ rotaDates: [MONDAY], teams, shifts: SHIFTS, assignments: [], absences: [halfDay] }).map(
        (r) => r.engineer.userId,
      ),
    ).toEqual(["a"]);
  });

  it("uses the team's own ordinary window -- Americas cover on the Americas team", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY],
      teams: [team("americas", [member("n")], "CRE_AMERICAS")],
      shifts: SHIFTS,
      assignments: [],
      absences: [],
    });
    expect(out[0]).toMatchObject({
      shiftCode: "CRE_AMERICAS",
      startsAt: "2026-10-12T15:30:00.000Z",
      endsAt: "2026-10-13T00:30:00.000Z",
    });
  });

  it("puts Americas regular hours on a fixed UTC-7 clock, the same all year", () => {
    const day = (iso: string) =>
      standingDayRows({
        rotaDates: [iso],
        teams: [team("americas", [member("n")], "CRE_AMERICAS_PT")],
        shifts: SHIFTS,
        assignments: [],
        absences: [],
      })[0];
    // 08:00-18:00 PDT is 20:30-06:30 IST.
    expect(day(MONDAY)).toMatchObject({
      shiftCode: "CRE_AMERICAS_PT",
      startsAt: "2026-10-12T15:00:00.000Z",
      endsAt: "2026-10-13T01:00:00.000Z",
    });
    // Still 20:30-06:30 IST after the US clocks go back (1 November 2026).
    expect(day("2026-11-02")).toMatchObject({
      startsAt: "2026-11-02T15:00:00.000Z",
      endsAt: "2026-11-03T01:00:00.000Z",
    });
  });

  it("lists the evening and morning crews (on-call included) in regular hours too", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY],
      teams: [team("draco", [member("e"), member("m"), member("x")])],
      shifts: SHIFTS,
      assignments: [
        row("e", "draco", "CRE_EVENING", MONDAY),
        row("m", "draco", "CRE_MORNING_OC", MONDAY),
        // A window that does not touch the regular hours adds nothing.
        row("x", "draco", "CRE_LATE", MONDAY),
      ],
      absences: [],
    });
    expect(out.map((r) => [r.engineer.userId, r.shiftCode])).toEqual([
      ["e", "CRE_REGULAR"],
      ["m", "CRE_REGULAR"],
    ]);
  });

  it("adds nothing for an escalation tier, even one whose window touches the regular hours", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY],
      teams: [team("draco", [member("t")])],
      shifts: SHIFTS,
      assignments: [row("t", "draco", "CRE_ESC_EVE", MONDAY)],
      absences: [],
    });
    expect(out).toEqual([]);
  });

  it("does not add regular hours for an evening person who already holds them, or who is away", () => {
    const teams = [team("draco", [member("e")])];
    const evening = row("e", "draco", "CRE_EVENING", MONDAY);
    expect(
      standingDayRows({
        rotaDates: [MONDAY],
        teams,
        shifts: SHIFTS,
        assignments: [evening, row("e", "draco", "CRE_REGULAR", MONDAY)],
        absences: [],
      }),
    ).toEqual([]);
    expect(
      standingDayRows({ rotaDates: [MONDAY], teams, shifts: SHIFTS, assignments: [evening], absences: [away("e", MONDAY, MONDAY)] }),
    ).toEqual([]);
  });

  it("lists someone on two teams once, and each rota day on its own", () => {
    const out = standingDayRows({
      rotaDates: [MONDAY, "2026-10-13"],
      teams: [team("draco", [member("x")]), team("migration", [member("x")])],
      shifts: SHIFTS,
      assignments: [],
      absences: [],
    });
    expect(out.map((r) => [r.rotaDate, r.teamKey])).toEqual([
      [MONDAY, "draco"],
      ["2026-10-13", "draco"],
    ]);
  });
});

describe("instantInZone", () => {
  it("reads a wall-clock minute in the zone, including past midnight", () => {
    expect(instantInZone(MONDAY, 540, COLOMBO)).toBe("2026-10-12T03:30:00.000Z");
    expect(instantInZone(MONDAY, 1800, COLOMBO)).toBe("2026-10-13T00:30:00.000Z");
    // A zone with daylight saving: 09:00 in New York on 2026-11-02 is EST.
    expect(instantInZone("2026-11-02", 540, "America/New_York")).toBe("2026-11-02T14:00:00.000Z");
  });
});

describe("onItsStandingCard", () => {
  const TEAMS: ScheduleTeam[] = [
    { ...team("vega", []), isAbt: true },
    { ...team("americas", [], "CRE_AMERICAS"), isAbt: false },
    { ...team("migration", []), isAbt: false },
  ];
  const keep = (teamKey: string, shiftCode: string, teams = TEAMS) =>
    onItsStandingCard(row("p", teamKey, shiftCode, MONDAY), SHIFTS, teams);

  it("lists only the Americas team on Americas cover", () => {
    expect(keep("americas", "CRE_AMERICAS")).toBe(true);
    expect(keep("vega", "CRE_AMERICAS")).toBe(false);
  });

  it("lists only ABTs on regular hours", () => {
    expect(keep("vega", "CRE_REGULAR")).toBe(true);
    expect(keep("migration", "CRE_REGULAR")).toBe(false);
  });

  it("leaves turns alone, whatever the team", () => {
    expect(keep("migration", "CRE_EVENING")).toBe(true);
    expect(keep("americas", "CRE_MORNING_OC")).toBe(true);
  });

  it("keeps Americas cover on the Americas team once its day moved to Pacific time", () => {
    const moved = TEAMS.map((t) => (t.key === "americas" ? { ...t, defaultShiftCode: "CRE_AMERICAS_PT" } : t));
    expect(keep("americas", "CRE_AMERICAS_PT", moved)).toBe(true);
    expect(keep("vega", "CRE_AMERICAS_PT", moved)).toBe(false);
    expect(keep("americas", "CRE_AMERICAS", moved)).toBe(true);
    expect(keep("vega", "CRE_AMERICAS", moved)).toBe(false);
    expect(keep("vega", "CRE_REGULAR", moved)).toBe(true);
  });

  it("does not apply the ABT rule until the server says which teams are ABTs", () => {
    const old = TEAMS.map(({ isAbt: _isAbt, ...t }) => t);
    expect(keep("migration", "CRE_REGULAR", old)).toBe(true);
    expect(keep("vega", "CRE_AMERICAS", old)).toBe(false);
  });
});

describe("atTeamOrdinaryHours", () => {
  const SHIFTS_PT = new Map([
    ...SHIFTS,
    ["CRE_WEEKEND_NIGHT", shift("CRE_WEEKEND_NIGHT", 1260, 1800, { dayScope: "WEEKEND" })],
    ["CRE_WEEKEND_NIGHT_OC", shift("CRE_WEEKEND_NIGHT_OC", 1260, 1800, { dayScope: "WEEKEND", isOnCall: true })],
    ["CRE_WEEKEND", shift("CRE_WEEKEND", 360, 1260, { dayScope: "WEEKEND" })],
    ["CRE_WEEKEND_ESC", shift("CRE_WEEKEND_ESC", 1260, 1800, { dayScope: "WEEKEND", isEscalation: true })],
  ]);
  const TEAMS: ScheduleTeam[] = [
    { ...team("americas", [], "CRE_AMERICAS_PT"), isAbt: false },
    { ...team("vega", []), isAbt: true },
  ];
  const draw = (r: ScheduleAssignment) => atTeamOrdinaryHours([r], SHIFTS_PT, TEAMS)[0];

  it("draws an Americas team member's stored Americas cover at the team's hours, all year", () => {
    expect(draw(row("n", "americas", "CRE_AMERICAS", MONDAY))).toMatchObject({
      shiftCode: "CRE_AMERICAS_PT",
      rotaDate: MONDAY,
      startsAt: "2026-10-12T15:00:00.000Z",
      endsAt: "2026-10-13T01:00:00.000Z",
    });
    expect(draw(row("n", "americas", "CRE_AMERICAS", "2026-11-02"))).toMatchObject({
      startsAt: "2026-11-02T15:00:00.000Z",
      endsAt: "2026-11-03T01:00:00.000Z",
    });
  });

  it("leaves other teams, turns, weekends and the team's own day alone", () => {
    const untouched = [
      row("v", "vega", "CRE_AMERICAS", MONDAY),
      row("n", "americas", "CRE_AMERICAS", SATURDAY),
      row("v", "vega", "CRE_WEEKEND", SATURDAY),
      row("v", "vega", "CRE_WEEKEND_ESC", SATURDAY),
      row("n", "americas", "CRE_AMERICAS_PT", MONDAY),
      row("n", "americas", "CRE_REGULAR", MONDAY),
    ];
    for (const r of untouched) expect(draw(r)).toBe(r);
  });

  it("draws the Americas weekend turn and its on-call at the Americas hours, whoever's team holds it", () => {
    // Stored 21:00-06:00 IST, back to back with the Sri Lankan weekend; drawn
    // 08:00-18:00 at UTC-7 = 20:30-06:30 IST, so it overlaps half an hour at
    // each end like every weekday.
    for (const r of [
      row("n", "americas", "CRE_WEEKEND_NIGHT", SATURDAY),
      row("v", "vega", "CRE_WEEKEND_NIGHT_OC", SATURDAY),
    ]) {
      expect(draw(r)).toMatchObject({
        shiftCode: r.shiftCode,
        teamKey: r.teamKey,
        rotaDate: SATURDAY,
        startsAt: "2026-10-17T15:00:00.000Z",
        endsAt: "2026-10-18T01:00:00.000Z",
      });
    }
  });

  it("leaves the weekend turn as stored while no team keeps an Americas ordinary day", () => {
    const r = row("n", "americas", "CRE_WEEKEND_NIGHT", SATURDAY);
    expect(atTeamOrdinaryHours([r], SHIFTS_PT, [{ ...team("vega", []), isAbt: true }])[0]).toBe(r);
  });

  it("changes nothing while the team's own day is Americas cover", () => {
    const old: ScheduleTeam[] = [{ ...team("americas", [], "CRE_AMERICAS"), isAbt: false }];
    const r = row("n", "americas", "CRE_AMERICAS", MONDAY);
    expect(atTeamOrdinaryHours([r], SHIFTS_PT, old)[0]).toBe(r);
  });
});

describe("atWorkOnTheDay", () => {
  const kind = (code: string, bucket: ScheduleAbsenceKind["bucket"], extra: Partial<ScheduleAbsenceKind> = {}) =>
    ({ code, label: code, shortCode: code, bucket, colourToken: code, sortOrder: 0, ...extra }) as ScheduleAbsenceKind;
  const KINDS = [
    kind("ANNUAL_LEAVE", "LEAVE"),
    kind("HALF_DAY_LEAVE_MORNING", "LEAVE"),
    kind("MIGRATION", "ALLOCATION", { movesToTeamKey: "migration" }),
    kind("ALLO_BR", "ALLOCATION", { movesToTeamKey: "americas", worksRotaThere: true }),
    kind("EXCLUDED", "EXCLUDED"),
    kind("ONBOARDING", "ALLOCATION", { retired: true }),
  ];
  const span = (id: string, kindCode: string, teamKey = "draco", startsOn = MONDAY, endsOn = MONDAY): ScheduleAbsence => ({
    ...away(id, startsOn, endsOn),
    kindCode,
    teamKey,
  });
  const kept = (rows: ScheduleAssignment[], absences: ScheduleAbsence[]) =>
    atWorkOnTheDay(rows, absences, KINDS).map((r) => `${r.engineer.userId}:${r.shiftCode}`);

  it("leaves out every stored row of someone on full-day leave that day, turns included", () => {
    const rows = [
      row("a", "draco", "CRE_REGULAR", MONDAY),
      row("a", "draco", "CRE_EVENING", MONDAY),
      row("b", "draco", "CRE_REGULAR", MONDAY),
    ];
    expect(kept(rows, [span("a", "ANNUAL_LEAVE")])).toEqual(["b:CRE_REGULAR"]);
  });

  it("only on the days the leave covers", () => {
    const rows = [row("a", "draco", "CRE_REGULAR", MONDAY), row("a", "draco", "CRE_REGULAR", "2026-10-13")];
    expect(kept(rows, [span("a", "ANNUAL_LEAVE")])).toEqual(["a:CRE_REGULAR"]);
    expect(atWorkOnTheDay(rows, [span("a", "ANNUAL_LEAVE")], KINDS)[0].rotaDate).toBe("2026-10-13");
  });

  it("leaves out someone on an allocation, such as Migration", () => {
    expect(kept([row("a", "vega", "CRE_REGULAR", MONDAY)], [span("a", "MIGRATION", "migration")])).toEqual([]);
  });

  it("keeps the rotation turn of someone on a customer allocation, but not their regular hours", () => {
    const allocated = [...KINDS, kind("ALLO_EXT", "ALLOCATION")];
    const shifts = new Map([
      ["CRE_REGULAR", { code: "CRE_REGULAR", isRotation: false } as ScheduleShift],
      ["CRE_EVENING", { code: "CRE_EVENING", isRotation: true } as ScheduleShift],
    ]);
    const rows = [row("a", "draco", "CRE_REGULAR", MONDAY), row("a", "draco", "CRE_EVENING", MONDAY)];
    expect(
      atWorkOnTheDay(rows, [span("a", "ALLO_EXT")], allocated, shifts).map((r) => `${r.engineer.userId}:${r.shiftCode}`),
    ).toEqual(["a:CRE_EVENING"]);
    // Leave and a move to a team with no rota still take the turn.
    expect(atWorkOnTheDay(rows, [span("a", "ANNUAL_LEAVE")], allocated, shifts)).toEqual([]);
    expect(atWorkOnTheDay(rows, [span("a", "MIGRATION", "migration")], allocated, shifts)).toEqual([]);
  });

  it("keeps half-day leave, the Brazil rotation on the Americas team, Excluded and retired kinds", () => {
    expect(kept([row("a", "draco", "CRE_REGULAR", MONDAY)], [span("a", "HALF_DAY_LEAVE_MORNING")])).toEqual([
      "a:CRE_REGULAR",
    ]);
    expect(kept([row("a", "americas", "CRE_AMERICAS", MONDAY)], [span("a", "ALLO_BR", "americas")])).toEqual([
      "a:CRE_AMERICAS",
    ]);
    expect(kept([row("a", "draco", "CRE_REGULAR", MONDAY)], [span("a", "EXCLUDED")])).toEqual(["a:CRE_REGULAR"]);
    expect(kept([row("a", "draco", "CRE_REGULAR", MONDAY)], [span("a", "ONBOARDING")])).toEqual(["a:CRE_REGULAR"]);
  });

  it("leaves out a home-team row of someone away on the Brazil rotation", () => {
    expect(kept([row("a", "draco", "CRE_REGULAR", MONDAY)], [span("a", "ALLO_BR", "americas")])).toEqual([]);
  });

  it("reads an open-ended span as still running", () => {
    expect(kept([row("a", "draco", "CRE_REGULAR", MONDAY)], [{ ...span("a", "MIGRATION"), endsOn: undefined }])).toEqual(
      [],
    );
  });
});

describe("creRowsAtWork", () => {
  // One ABT over a whole week, the shape This week reads: a stored Regular
  // hours (LK) day, a blank weekday, an evening turn, and two people on an
  // allocation -- one with a stored LK row under it, one without.
  const KINDS = [
    { code: "ALLO_INT", label: "Allocated — internal", shortCode: "Allo-INT", bucket: "ALLOCATION", colourToken: "INT", sortOrder: 30 },
    { code: "ALLO_EXT", label: "Allocated — external", shortCode: "Allo-EXT", bucket: "ALLOCATION", colourToken: "EXT", sortOrder: 40 },
  ] as ScheduleAbsenceKind[];
  const WEEK = ["2026-10-12", "2026-10-13", "2026-10-14", "2026-10-15", "2026-10-16", "2026-10-17", "2026-10-18"];
  const draco = { ...team("draco", ["lk", "blank", "eve", "int", "ext"].map((id) => member(id))), isAbt: true };
  const allocated = (id: string, kindCode: string): ScheduleAbsence => ({ ...away(id, WEEK[0], WEEK[6]), kindCode });
  const assignments = WEEK.slice(0, 5).flatMap((iso) => [
    row("lk", "draco", "CRE_REGULAR", iso),
    row("int", "draco", "CRE_REGULAR", iso),
    row("eve", "draco", "CRE_EVENING", iso),
  ]);
  const read = (teamKeysInView: string[] = []) =>
    creRowsAtWork({
      rotaDates: WEEK,
      teams: [draco],
      teamKeysInView,
      shifts: SHIFTS,
      assignments,
      absences: [allocated("int", "ALLO_INT"), allocated("ext", "ALLO_EXT")],
      absenceKinds: KINDS,
    });
  const regularOn = (rows: ScheduleAssignment[], iso: string) =>
    rows
      .filter((r) => r.rotaDate === iso && r.shiftCode === "CRE_REGULAR")
      .map((r) => r.engineer.userId)
      .sort();

  it("counts each weekday's Regular hours as Today does: LK, blank weekdays and the crews around them", () => {
    const rows = read();
    for (const iso of WEEK.slice(0, 5)) expect(regularOn(rows, iso)).toEqual(["blank", "eve", "lk"]);
  });

  it("leaves out people on an allocation, with or without a stored row under it", () => {
    const ids = new Set(read().map((r) => r.engineer.userId));
    expect(ids.has("int")).toBe(false);
    expect(ids.has("ext")).toBe(false);
  });

  it("adds no Regular hours at the weekend", () => {
    const rows = read();
    expect(regularOn(rows, WEEK[5])).toEqual([]);
    expect(regularOn(rows, WEEK[6])).toEqual([]);
  });

  it("makes up standing days only for the teams in view", () => {
    const rows = read(["vega"]);
    expect(rows.some((r) => r.source === STANDING_DAY_SOURCE)).toBe(false);
  });
});
