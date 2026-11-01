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

import { fireEvent, render, screen } from "@testing-library/react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import DayLadder, { type LadderLane } from "./DayLadder";
import {
  ANNUAL_LEAVE,
  LIEU_LEAVE,
  RND,
  EVENING,
  REGULAR,
  REGULAR_IND,
  TZ,
  TZ1,
  TZ3,
  ZONES,
  absence,
  assignment,
  scopeControls,
  shift,
  shiftMap,
} from "../test/fixtures";

// A card on the ladder measures itself, to say when it is holding more than
// it can show. jsdom has no ResizeObserver and never lays anything out, so a
// stub that never fires is both enough and honest: these tests are about what
// the ladder puts in a card, not how tall the card turns out.
beforeAll(() => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe(): void {}
      unobserve(): void {}
      disconnect(): void {}
    },
  );
});
afterAll(() => vi.unstubAllGlobals());

/** Wednesday 23 September 2026. */
const WEDNESDAY = new Date(2026, 8, 23);
const ISO = "2026-09-23";

/** The morning window and the on-call variant of it, same hours. */
const MORNING = shift({
  code: "CRE_MORNING",
  shortCode: "6-9a",
  label: "Morning 6-9am",
  startMinute: 360,
  endMinute: 540,
  sortOrder: 10,
});
const MORNING_OC = shift({
  code: "CRE_MORNING_OC",
  shortCode: "6-9a",
  label: "Morning 6-9am on-call",
  startMinute: 360,
  endMinute: 540,
  isOnCall: true,
  sortOrder: 11,
});

const SHIFTS = shiftMap(REGULAR, REGULAR_IND, EVENING, MORNING, MORNING_OC);

function lane(assignments: LadderLane["assignments"]): LadderLane {
  return { name: "Rotations", assignments, layout: "flat" };
}

function renderLadder(assignments: LadderLane["assignments"], absences = [] as ReturnType<typeof absence>[]) {
  return render(
    <DayLadder
      day={WEDNESDAY}
      tz={TZ}
      zoneLabel="IST"
      lanes={[lane(assignments)]}
      shifts={SHIFTS}
      zones={ZONES}
      absences={absences}
      absenceKinds={[ANNUAL_LEAVE]}
      {...scopeControls()}
    />,
  );
}

/** Nine to five on the day under test, in the fixture clock. */
function nineToFive(name: string, shiftCode: string, teamKey = "alpha") {
  return assignment({
    name,
    teamKey,
    rotaDate: ISO,
    shiftCode,
    startsAt: `${ISO}T03:30:00.000Z`,
    endsAt: `${ISO}T12:30:00.000Z`,
  });
}

describe("DayLadder: cards that share their hours", () => {
  it("holds a window and its on-call variant as one rotation, tagging only the on-call person", () => {
    // Placed by time alone, two windows with the same hours drew on top of
    // each other and printed through one another. Held together they are one
    // rotation: its name once, one list, and "On-call" on the person who is.
    const { container } = renderLadder([
      assignment({
        name: "Asela", rotaDate: ISO, shiftCode: MORNING.code,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      }),
      assignment({
        name: "Nuwan", rotaDate: ISO, shiftCode: MORNING_OC.code, isOnCall: true,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      }),
    ]);
    const card = [...container.querySelectorAll(".zblk")].find((c) => c.textContent?.includes("Asela"))!;
    expect(card.querySelector(".zbt")).toHaveTextContent(/^Morning 6-9am$/);
    expect(card.querySelectorAll(".zsl")).toHaveLength(0);
    // On call is not working the hours: a column of its own, under a heading
    // whose tooltip says what on call means.
    const onCall = card.querySelector(".zoc")!;
    expect(onCall.querySelector(".zocl")).toHaveTextContent("On-call");
    expect(onCall.querySelector(".zocl")).toHaveAttribute("title", expect.stringContaining("emergency"));
    expect(onCall).toHaveTextContent("Nuwan");
    expect(onCall).not.toHaveTextContent("Asela");
    expect(card.querySelector(".zflow")).toHaveTextContent("Asela");
  });

  it("keeps the on-call column when a full shift and its on-call share a card", () => {
    // Twelve on the window and one on call is thirteen rows -- over the limit
    // that turns a card into a team list, which would take the on-call person
    // in as one more working. Counted on who works, it stays a list of names
    // with the on-call person in their own column.
    const working = Array.from({ length: 12 }, (_, i) =>
      assignment({
        name: `Morn${i}`, rotaDate: ISO, shiftCode: MORNING.code,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      }),
    );
    const { container } = renderLadder([
      ...working,
      assignment({
        name: "Nuwan", rotaDate: ISO, shiftCode: MORNING_OC.code, isOnCall: true,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      }),
    ]);
    const card = [...container.querySelectorAll(".zblk")].find((c) => c.textContent?.includes("Morn0"))!;
    expect(card.querySelector(".teamsplit")).toBeNull();
    expect(card.querySelectorAll(".zflow .lnm")).toHaveLength(12);
    expect(card.querySelector(".zoc")).toHaveTextContent("Nuwan");
  });

  it("lists whoever is on their team's own window first, ahead of those taking a turn", () => {
    const morning = (name: string, teamKey: string) =>
      assignment({
        name, rotaDate: ISO, shiftCode: MORNING.code, teamKey,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      });
    const { container } = render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[lane([morning("Asela", "alpha"), morning("Zara", "southern")])]}
        shifts={SHIFTS}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        teamDefaultShift={{ southern: MORNING.code }}
        {...scopeControls()}
      />,
    );
    const names = [...container.querySelectorAll(".zflow .lnm .who")].map((n) => n.textContent);
    expect(names).toEqual(["Zara", "Asela"]);
  });

  it("lists people on AEST regular hours in the Morning card's one list, tagged AEST", () => {
    // AEST regular hours are stored on the morning's hours so the team sees
    // who on Australian time is there. They are working hours, not a turn:
    // the card stays the Morning rotation, one list with the AEST person
    // tagged, and the on-call person in their own column.
    const aest = shift({
      code: "CRE_REGULAR_AEST",
      shortCode: "AEST",
      label: "AEST regular hours",
      startMinute: 360,
      endMinute: 540,
      isRotation: false,
      sortOrder: 36,
    });
    const at = { rotaDate: ISO, startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z` };
    const { container } = render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[
          lane([
            assignment({ ...at, name: "Ava", shiftCode: aest.code }),
            assignment({ ...at, name: "Asela", shiftCode: MORNING.code }),
            assignment({ ...at, name: "Nuwan", shiftCode: MORNING_OC.code, isOnCall: true }),
          ]),
        ]}
        shifts={shiftMap(REGULAR, EVENING, MORNING, MORNING_OC, aest)}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );
    const card = [...container.querySelectorAll(".zblk")].find((c) => c.textContent?.includes("Ava"))!;
    expect(card.querySelector(".zbt")).toHaveTextContent(/^Morning 6-9am$/);
    expect(card.querySelectorAll(".zsl")).toHaveLength(0);
    expect(card).not.toHaveTextContent(/AEST regular hours/i);
    const rows = [...card.querySelectorAll(".zflow .lnm")];
    expect(rows.map((r) => r.querySelector(".who")?.textContent)).toEqual(["Asela", "Ava"]);
    expect(rows[1].querySelector(".tier-t")).toHaveTextContent("AEST");
    expect(rows[0].querySelector(".tier-t")).toBeNull();
    expect(card.querySelector(".zoc")).toHaveTextContent("Nuwan");
  });

  it("keeps the Morning card on a day nobody is on the Morning rotation, listing the AEST person tagged", () => {
    const aest = shift({
      code: "CRE_REGULAR_AEST",
      shortCode: "AEST",
      label: "AEST regular hours",
      startMinute: 360,
      endMinute: 540,
      isRotation: false,
      sortOrder: 36,
    });
    const { container } = render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[
          lane([
            assignment({
              name: "Ava", rotaDate: ISO, shiftCode: aest.code,
              startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
            }),
          ]),
        ]}
        shifts={shiftMap(REGULAR, EVENING, MORNING, MORNING_OC, aest)}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );
    const card = [...container.querySelectorAll(".zblk")].find((c) => c.textContent?.includes("Ava"))!;
    expect(card.querySelector(".zbt")).toHaveTextContent(/^Morning 6-9am$/);
    expect(card).not.toHaveTextContent(/AEST regular hours/i);
    expect(card.querySelector(".zflow .lnm .tier-t")).toHaveTextContent("AEST");
  });

  it("folds a window that differs only by team into the crowded card", () => {
    // Regular hours and the India region shift are the same nine-to-five. A
    // card this size lists teams, which is the whole distinction, so a second
    // card said nothing the first one's second-team row would not.
    const people = Array.from({ length: 14 }, (_, i) => nineToFive(`Reg${i}`, REGULAR.code));
    renderLadder([...people, nineToFive("Akhil", REGULAR_IND.code, "bravo")]);

    expect(screen.getByText("Regular hours")).toBeInTheDocument();
    expect(screen.queryByText("India region shift")).not.toBeInTheDocument();
    // The card's own team row, not the team picker's option of the same name.
    expect(screen.getByText(/Bravo/, { ignore: "option" })).toBeInTheDocument();
  });

  it("never folds an on-call window into a crowded card", () => {
    // Being on call is not a fact about which team you are on, and a team
    // list cannot carry it.
    const people = Array.from({ length: 14 }, (_, i) =>
      assignment({
        name: `Morn${i}`, rotaDate: ISO, shiftCode: MORNING.code,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      }),
    );
    renderLadder([
      ...people,
      assignment({
        name: "OnCall", rotaDate: ISO, shiftCode: MORNING_OC.code,
        startsAt: `${ISO}T00:30:00.000Z`, endsAt: `${ISO}T03:30:00.000Z`,
      }),
    ]);
    expect(screen.getByText("Morning 6-9am on-call")).toBeInTheDocument();
  });
});

describe("DayLadder: who is not on the rota", () => {
  it("keeps somebody on half-day leave in the Leave card, flagged with their half", () => {
    const HD_AM = {
      ...ANNUAL_LEAVE, id: "k-hdam", code: "HALF_DAY_LEAVE_MORNING", shortCode: "HD-AM",
      label: "Half day leave — morning", colourToken: "HDM", sortOrder: 11,
    };
    const { container } = render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[lane([nineToFive("Asela", REGULAR.code)])]}
        shifts={SHIFTS}
        zones={ZONES}
        absences={[
          absence({ name: "Nuwan", startsOn: ISO, endsOn: ISO }),
          absence({ name: "Hiru", startsOn: ISO, endsOn: ISO, kindCode: HD_AM.code }),
        ]}
        absenceKinds={[ANNUAL_LEAVE, HD_AM]}
        {...scopeControls()}
      />,
    );
    const cards = [...container.querySelectorAll(".offcard")];
    expect(cards).toHaveLength(1);
    expect(cards[0].querySelector(".offh b")).toHaveTextContent("2");
    const person = (name: string) =>
      [...cards[0].querySelectorAll(".lnm")].find((n) => n.querySelector(".who")?.textContent === name);
    expect(person("Hiru")?.querySelector(".hdflag")).toHaveTextContent("HD-AM");
    // Whole-day leave still does not name its kind.
    expect(person("Nuwan")?.querySelector(".hdflag")).toBeNull();
  });

  it("lists leave ahead of the allocations", () => {
    renderLadder(
      [nineToFive("Asela", REGULAR.code)],
      [absence({ name: "Nuwan", startsOn: ISO, endsOn: ISO })],
    );
    expect(screen.getByText("Leave")).toBeInTheDocument();
    expect(screen.getByText("Nuwan")).toBeInTheDocument();
  });

  it("keeps every kind of leave in one card, without naming the kind, and allocations apart", () => {
    const { container } = render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[lane([nineToFive("Asela", REGULAR.code)])]}
        shifts={SHIFTS}
        zones={ZONES}
        absences={[
          absence({ name: "Nuwan", startsOn: ISO, endsOn: ISO }),
          absence({ name: "Lena", startsOn: ISO, endsOn: ISO, kindCode: LIEU_LEAVE.code }),
          absence({ name: "Omar", startsOn: ISO, endsOn: ISO, kindCode: RND.code }),
        ]}
        absenceKinds={[ANNUAL_LEAVE, LIEU_LEAVE, RND]}
        {...scopeControls()}
      />,
    );
    const cards = [...container.querySelectorAll(".offcard")];
    expect(cards).toHaveLength(2);
    expect(cards[0].querySelector(".offl")).toHaveTextContent("Leave");
    expect(cards[0].querySelector(".offh b")).toHaveTextContent("2");
    expect(cards[0]).toHaveTextContent("Nuwan");
    expect(cards[0]).toHaveTextContent("Lena");
    expect(screen.queryByText("Annual leave")).not.toBeInTheDocument();
    expect(screen.queryByText("Lieu leave")).not.toBeInTheDocument();
    expect(cards[1]).toHaveTextContent("R&D");
    expect(cards[1]).toHaveTextContent("Omar");
  });
});

describe("DayLadder: the escalation ladder", () => {
  function tiered(name: string, tier: "L1" | "L2" | "L3") {
    return { ...assignment({ name, rotaDate: ISO, shiftCode: TZ1.code, zoneCode: "TZ1" }), tier };
  }

  function renderZone(assignments: LadderLane["assignments"]) {
    return render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[{ name: "TZ1", assignments, layout: "zone" }]}
        shifts={shiftMap(TZ1)}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );
  }

  it("shows L1, L2 and L3 in order, saying when a tier has nobody", () => {
    const { container } = renderZone([tiered("Jane", "L1"), tiered("John", "L2")]);
    const labels = [...container.querySelectorAll(".zbp.tiers .zsl")].map((el) => el.firstChild?.textContent);
    expect(labels).toEqual(["L1 escalation", "L2 escalation", "L3 escalation"]);
    expect(screen.getByText("Nobody rostered")).toBeInTheDocument();
  });

  it("names whoever holds L3 like any other tier", () => {
    renderZone([tiered("Jane", "L1"), tiered("Ada", "L3")]);
    expect(screen.getByText("Ada")).toBeInTheDocument();
    // L2 is the empty one now.
    expect(screen.getAllByText("Nobody rostered")).toHaveLength(1);
  });
});

describe("DayLadder: TZ3 is one card", () => {
  it("folds regular hours that match the escalation window into its card", () => {
    // TZ3's regular hours and its escalation are the same 21:00-06:00: two
    // cards side by side would say the same hours twice.
    const TZ3_REGULAR = shift({
      code: "SRE_TZ3_REGULAR",
      label: "TZ3 regular hours",
      family: "SRE",
      zoneCode: "TZ3",
      isRotation: false,
      startMinute: 1260,
      endMinute: 1800,
    });
    const night = (name: string, shiftCode: string) =>
      assignment({
        name,
        rotaDate: ISO,
        shiftCode,
        zoneCode: "TZ3",
        startsAt: `${ISO}T15:30:00.000Z`, // 21:00 in the fixture clock
        endsAt: "2026-09-24T00:30:00.000Z", // 06:00 next morning
      });
    const { container } = render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[{ name: "TZ3", assignments: [{ ...night("Isuri", TZ3.code), tier: "L1" }, night("Nimal", TZ3_REGULAR.code)], layout: "zone" }]}
        shifts={shiftMap(TZ3, TZ3_REGULAR)}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );
    const lane = container.querySelector(".ladder .lane:not(.offlane)");
    expect(lane?.querySelectorAll(".lncol")).toHaveLength(1);
    const labels = [...container.querySelectorAll(".zbp.tiers .zsl")].map((el) => el.firstChild?.textContent);
    expect(labels).toEqual(["L1 escalation", "L2 escalation", "L3 escalation", "Regular hours"]);
    expect(screen.getByText("Nimal")).toBeInTheDocument();
    expect(container.textContent).not.toContain("Others in TZ3");
  });
});

describe("DayLadder: others in a zone, by team", () => {
  const day = (name: string, teamKey: string) =>
    assignment({ name, rotaDate: ISO, shiftCode: TZ1.code, zoneCode: "TZ1", teamKey });
  const renderZone = (assignments: ReturnType<typeof day>[]) =>
    render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[{ name: "TZ1", assignments, layout: "zone" }]}
        shifts={shiftMap(TZ1)}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );

  it("lists each team under its own heading and count, as the Leave card does", () => {
    const { container } = renderZone([
      day("Nadia", "orion_abt_cre_team"),
      day("Kasun", "lyra_abt_cre_team"),
      day("Ruwan", "orion_abt_cre_team"),
    ]);
    const card = [...container.querySelectorAll(".zblk")].find((el) => el.textContent?.includes("Others in TZ1"))!;
    const groups = [...card.querySelectorAll(".zteam")].map((g) => ({
      heading: g.querySelector(".offgh")?.firstChild?.textContent,
      count: g.querySelector(".offgh b")?.textContent,
      names: [...g.querySelectorAll(".lnm .who")].map((n) => n.textContent),
    }));
    expect(groups).toEqual([
      { heading: "Orion", count: "2", names: ["Nadia", "Ruwan"] },
      { heading: "Lyra", count: "1", names: ["Kasun"] },
    ]);
  });

  it("keeps the escalation card as tiers, not teams", () => {
    const { container } = renderZone([{ ...day("Nadia", "orion_abt_cre_team"), tier: "L1" }]);
    expect(container.querySelector(".zbp.tiers .zteam")).toBeNull();
  });
});

describe("DayLadder: a stint worked on another team's rota", () => {
  const BRAZIL = {
    id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
    colourToken: "BR", sortOrder: 70, movesToTeamKey: "americas", worksRotaThere: true,
  };
  const MIGRATION = {
    id: "k-mig", code: "MIGRATION", shortCode: "Mig", label: "Migration", bucket: "ALLOCATION" as const,
    colourToken: "MIG", sortOrder: 90, movesToTeamKey: "migration",
  };

  it("is not listed as off the rota, while other time away still is", () => {
    render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[lane([])]}
        shifts={SHIFTS}
        zones={ZONES}
        absences={[
          absence({ name: "Bruna", startsOn: "2026-09-01", endsOn: "2026-12-31", kindCode: "ALLO_BR", teamKey: "americas" }),
          absence({ name: "Milan", startsOn: "2026-09-01", endsOn: "2026-12-31", kindCode: "MIGRATION", teamKey: "migration" }),
          absence({ name: "Lena", startsOn: "2026-09-23", endsOn: "2026-09-23" }),
        ]}
        absenceKinds={[ANNUAL_LEAVE, BRAZIL, MIGRATION]}
        {...scopeControls()}
      />,
    );
    expect(screen.queryByText("Bruna")).not.toBeInTheDocument();
    expect(screen.getByText("Milan")).toBeInTheDocument();
    expect(screen.getByText("Lena")).toBeInTheDocument();
    expect(screen.getByText("2 not available")).toBeInTheDocument();
  });

  it("leaves a retired tag's entries out of who is off today", () => {
    const ONBOARDING = {
      id: "k-onb", code: "ONBOARDING", shortCode: "ONB", label: "Onboarding", bucket: "ALLOCATION" as const,
      colourToken: "ONB", sortOrder: 100, retired: true,
    };
    render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[lane([])]}
        shifts={SHIFTS}
        zones={ZONES}
        absences={[
          absence({ name: "Omar", startsOn: "2026-09-01", endsOn: "2026-12-31", kindCode: "ONBOARDING" }),
          absence({ name: "Lena", startsOn: "2026-09-23", endsOn: "2026-09-23" }),
        ]}
        absenceKinds={[ANNUAL_LEAVE, ONBOARDING]}
        {...scopeControls()}
      />,
    );
    expect(screen.queryByText("Omar")).not.toBeInTheDocument();
    expect(screen.getByText("Lena")).toBeInTheDocument();
    expect(screen.getByText("1 not available")).toBeInTheDocument();
  });

  it("leaves people excluded from the rota out of who is off today", () => {
    const EXCLUDED = {
      id: "k-exc", code: "EXCLUDED", shortCode: "EXC", label: "Excluded from rota", bucket: "EXCLUDED" as const,
      colourToken: "EXC", sortOrder: 110,
    };
    render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[lane([])]}
        shifts={SHIFTS}
        zones={ZONES}
        absences={[
          absence({ name: "Ezra", startsOn: "2026-01-01", kindCode: "EXCLUDED" }),
          absence({ name: "Lena", startsOn: "2026-09-23", endsOn: "2026-09-23" }),
        ]}
        absenceKinds={[ANNUAL_LEAVE, EXCLUDED]}
        {...scopeControls()}
      />,
    );
    expect(screen.queryByText("Ezra")).not.toBeInTheDocument();
    expect(screen.queryByText("Excluded from rota")).not.toBeInTheDocument();
    expect(screen.getByText("Lena")).toBeInTheDocument();
    expect(screen.getByText("1 not available")).toBeInTheDocument();
  });
});

describe("DayLadder: a crowded card's team pane", () => {
  const twoTeams = () => [
    ...Array.from({ length: 8 }, (_, i) => nineToFive(`Orion${i}`, REGULAR.code, "orion_abt_cre_team")),
    ...Array.from({ length: 7 }, (_, i) => nineToFive(`Lyra${i}`, REGULAR.code, "lyra_abt_cre_team")),
  ];
  const teamButton = (container: HTMLElement, name: string) =>
    [...container.querySelectorAll<HTMLElement>(".teamsplit .tl")].find((el) => el.textContent?.includes(name))!;

  it("opens on the team list alone, so no one team reads as the whole rotation", () => {
    const { container } = renderLadder(twoTeams());
    expect(container.querySelectorAll(".teamsplit .tl")).toHaveLength(2);
    expect(container.querySelector(".teampane .tlph")).toBeNull();
    expect(screen.queryByText("Orion0")).not.toBeInTheDocument();
    expect(screen.getByText("Hover over a team to see who is on it.")).toBeInTheDocument();
  });

  it("shows a team while it is hovered, and closes it when the cursor leaves", () => {
    const { container } = renderLadder(twoTeams());
    fireEvent.mouseEnter(teamButton(container, "Orion"));
    expect(screen.getByText("Orion0")).toBeInTheDocument();
    fireEvent.mouseLeave(container.querySelector(".teamsplit")!);
    expect(screen.queryByText("Orion0")).not.toBeInTheDocument();
  });

  it("keeps a clicked team open until it is clicked again", () => {
    const { container } = renderLadder(twoTeams());
    const orion = teamButton(container, "Orion");
    fireEvent.click(orion);
    fireEvent.mouseLeave(container.querySelector(".teamsplit")!);
    expect(screen.getByText("Orion0")).toBeInTheDocument();
    expect(orion).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(orion);
    expect(screen.queryByText("Orion0")).not.toBeInTheDocument();
  });

  it("heads the team by name, not by its directory key", () => {
    const { container } = renderLadder(twoTeams());
    fireEvent.mouseEnter(teamButton(container, "Orion"));
    const heading = container.querySelector(".teampane .tlph");
    expect(heading).toHaveTextContent("Orion team");
    expect(heading).not.toHaveTextContent("orion_abt_cre_team");
  });
});

describe("DayLadder: a reader west of the office", () => {
  // Monday 12 October 2026, read in Los Angeles. Colombo's Monday regular
  // hours (09:00-18:00 IST) run from 20:30 Sunday to 05:30 Monday here, and
  // Tuesday's morning crew (06:00-09:00 IST) is on 17:30-20:30 Monday.
  const MONDAY = new Date(2026, 9, 12);
  const LA = "America/Los_Angeles";

  function at(name: string, rotaDate: string, shiftCode: string, startsAt: string, endsAt: string) {
    return assignment({ name, rotaDate, shiftCode, startsAt, endsAt });
  }

  function renderLA(lanes: LadderLane[], shifts = SHIFTS) {
    return render(
      <DayLadder
        day={MONDAY}
        tz={LA}
        zoneLabel="PDT"
        lanes={lanes}
        shifts={shifts}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );
  }

  const READ = [
    // Sunday's rota: over by 05:30 Sunday here.
    at("Sunday Sam", "2026-10-11", REGULAR.code, "2026-10-11T03:30:00.000Z", "2026-10-11T12:30:00.000Z"),
    // Monday's: carried in from Sunday evening.
    at("Monday Mia", "2026-10-12", REGULAR.code, "2026-10-12T03:30:00.000Z", "2026-10-12T12:30:00.000Z"),
    // Tuesday's morning and regular hours: Monday evening here.
    at("Tuesday Tom", "2026-10-13", MORNING.code, "2026-10-13T00:30:00.000Z", "2026-10-13T03:30:00.000Z"),
    at("Tuesday Ted", "2026-10-13", REGULAR.code, "2026-10-13T03:30:00.000Z", "2026-10-13T12:30:00.000Z"),
    // Wednesday's: Tuesday evening here.
    at("Wednesday Wen", "2026-10-14", REGULAR.code, "2026-10-14T03:30:00.000Z", "2026-10-14T12:30:00.000Z"),
  ];

  it("shows the office's next morning crew on the reader's evening", () => {
    renderLA([lane(READ)]);
    expect(screen.getByText("17:30 – 20:30")).toBeInTheDocument();
    expect(screen.getByText("Tuesday Tom")).toBeInTheDocument();
    expect(screen.getByText("Monday Mia")).toBeInTheDocument();
    expect(screen.getByText("Tuesday Ted")).toBeInTheDocument();
  });

  it("leaves out rota days that do not touch the reader's day", () => {
    renderLA([lane(READ)]);
    expect(screen.queryByText("Sunday Sam")).not.toBeInTheDocument();
    expect(screen.queryByText("Wednesday Wen")).not.toBeInTheDocument();
  });

  it("counts only the people on during the reader's day", () => {
    const { container } = renderLA([lane(READ)]);
    expect(container.querySelector("h2 .count")?.textContent).toBe("3");
  });

  it("keeps two rota days' escalations as two cards", () => {
    const tiered = (name: string, rotaDate: string, startsAt: string, endsAt: string) => ({
      ...assignment({ name, rotaDate, shiftCode: TZ1.code, zoneCode: "TZ1", startsAt, endsAt }),
      tier: "L1" as const,
    });
    const { container } = renderLA(
      [
        {
          name: "TZ1",
          layout: "zone",
          assignments: [
            // Monday 18:00-21:00 IST and Tuesday 06:00-09:00 IST: 05:30 and
            // 17:30 Monday here, a working day apart.
            tiered("Evening Eve", "2026-10-12", "2026-10-12T12:30:00.000Z", "2026-10-12T15:30:00.000Z"),
            tiered("Morning Max", "2026-10-13", "2026-10-13T00:30:00.000Z", "2026-10-13T03:30:00.000Z"),
          ],
        },
      ],
      shiftMap(TZ1),
    );
    expect(container.querySelectorAll(".zbp.tiers")).toHaveLength(2);
  });
});

describe("DayLadder: where the Sri Lankan day and the Americas day meet", () => {
  /** 08:00-18:00 on a fixed UTC-7 clock: 20:30-06:30 IST all year. */
  const AMERICAS_PT = shift({
    code: "CRE_AMERICAS_PT",
    shortCode: "NLK",
    label: "Americas regular hours",
    startMinute: 480,
    endMinute: 1080,
    authoringTimeZone: "Etc/GMT+7",
    isRotation: false,
    colourToken: "NLK",
    sortOrder: 55,
  });
  /** The window as it was: 21:00-06:00 on the Colombo clock. */
  const AMERICAS_COVER = shift({
    code: "CRE_AMERICAS",
    shortCode: "NLK",
    label: "Americas cover",
    startMinute: 1260,
    endMinute: 1800,
    crossesMidnight: true,
    isRotation: false,
    colourToken: "NLK",
    sortOrder: 50,
  });
  const shifts = shiftMap(REGULAR, EVENING, MORNING, AMERICAS_PT, AMERICAS_COVER);

  const evening = assignment({
    name: "Eve Ning",
    rotaDate: ISO,
    shiftCode: "CRE_EVENING",
    startsAt: `${ISO}T12:30:00.000Z`,
    endsAt: `${ISO}T15:30:00.000Z`,
  });
  const americas = (shiftCode: string, startsAt: string, endsAt: string) =>
    assignment({ name: "Ann Merica", teamKey: "americas", rotaDate: ISO, shiftCode, startsAt, endsAt });

  function renderCre(assignments: LadderLane["assignments"], regions = true) {
    return render(
      <DayLadder
        day={WEDNESDAY}
        tz={TZ}
        zoneLabel="IST"
        lanes={[{ name: "Rotations", assignments, layout: "flat", regions }]}
        shifts={shifts}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        {...scopeControls()}
      />,
    );
  }

  it("marks the half hour both are on, and gives each side its own column", () => {
    const { container } = renderCre([
      nineToFive("Reg Ular", "CRE_REGULAR"),
      evening,
      americas("CRE_AMERICAS_PT", `${ISO}T15:00:00.000Z`, "2026-09-24T01:00:00.000Z"),
    ]);
    const overlap = screen.getByRole("note", { name: /Overlap 20:30–21:00, 30 min/ });
    // A mark only: the time is in its tooltip, never written across the cards.
    expect(overlap.textContent).not.toMatch(/\d/);
    expect(overlap).toHaveAttribute("title", expect.stringContaining("20:30–21:00"));
    const columns = container.querySelectorAll(".lane:not(.offlane) .lncol");
    expect(columns).toHaveLength(2);
    // On the edge the two columns share: it lives in the US column, not over
    // the whole lane.
    expect(columns[1]).toContainElement(overlap);
    expect(columns[0]).toHaveTextContent("Eve Ning");
    expect(columns[0]).toHaveTextContent("Reg Ular");
    expect(columns[0]).not.toHaveTextContent("Ann Merica");
    expect(columns[1]).toHaveTextContent("Ann Merica");
  });

  it("keeps the whole Sri Lankan day in one column, the morning handover included", () => {
    // Packed by time, last night's Americas crew (in until 06:30) would push
    // the morning into a second column and leave regular hours in the first.
    const { container } = renderCre([
      assignment({
        name: "Ann Merica",
        teamKey: "americas",
        rotaDate: "2026-09-22",
        shiftCode: "CRE_AMERICAS_PT",
        startsAt: "2026-09-22T15:00:00.000Z",
        endsAt: `${ISO}T01:00:00.000Z`,
      }),
      assignment({
        name: "Mor Ning",
        rotaDate: ISO,
        shiftCode: "CRE_MORNING",
        startsAt: `${ISO}T00:30:00.000Z`,
        endsAt: `${ISO}T03:30:00.000Z`,
      }),
      nineToFive("Reg Ular", "CRE_REGULAR"),
      evening,
      americas("CRE_AMERICAS_PT", `${ISO}T15:00:00.000Z`, "2026-09-24T01:00:00.000Z"),
    ]);
    expect(screen.getByRole("note", { name: /Overlap 06:00–06:30/ })).toBeInTheDocument();
    expect(screen.getByRole("note", { name: /Overlap 20:30–21:00/ })).toBeInTheDocument();
    const columns = container.querySelectorAll(".lane:not(.offlane) .lncol");
    expect(columns).toHaveLength(2);
    expect(columns[0]).toHaveTextContent("Mor Ning");
    expect(columns[0]).toHaveTextContent("Reg Ular");
    expect(columns[0]).not.toHaveTextContent("Ann Merica");
  });

  it("marks nothing for a hole between the two days, and still keeps LK left and the US right", () => {
    const { container } = renderCre([
      evening,
      americas("CRE_AMERICAS_PT", `${ISO}T16:00:00.000Z`, "2026-09-24T02:00:00.000Z"),
    ]);
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
    const columns = container.querySelectorAll(".lane:not(.offlane) .lncol");
    expect(columns).toHaveLength(2);
    expect(columns[0]).toHaveTextContent("Eve Ning");
    expect(columns[1]).toHaveTextContent("Ann Merica");
  });

  it("marks nothing where one day starts as the other ends, and still keeps LK left and the US right", () => {
    const { container } = renderCre([
      evening,
      americas("CRE_AMERICAS", `${ISO}T15:30:00.000Z`, "2026-09-24T00:30:00.000Z"),
    ]);
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
    const columns = container.querySelectorAll(".lane:not(.offlane) .lncol");
    expect(columns).toHaveLength(2);
    expect(columns[0]).toHaveTextContent("Eve Ning");
    expect(columns[1]).toHaveTextContent("Ann Merica");
  });

  it("keeps the US card on the right even when it comes first in the day", () => {
    // Last night's US crew is the first card of the morning, and packed by
    // time it took the left column.
    const { container } = renderCre([
      assignment({
        name: "Ann Merica",
        teamKey: "americas",
        rotaDate: "2026-09-22",
        shiftCode: "CRE_AMERICAS",
        startsAt: "2026-09-22T15:30:00.000Z",
        endsAt: `${ISO}T00:30:00.000Z`,
      }),
      nineToFive("Reg Ular", "CRE_REGULAR"),
    ]);
    const columns = container.querySelectorAll(".lane:not(.offlane) .lncol");
    expect(columns).toHaveLength(2);
    expect(columns[0]).toHaveTextContent("Reg Ular");
    expect(columns[1]).toHaveTextContent("Ann Merica");
  });

  it("is one column when only one side is on screen", () => {
    const { container } = renderCre([nineToFive("Reg Ular", "CRE_REGULAR"), evening]);
    expect(container.querySelectorAll(".lane:not(.offlane) .lncol")).toHaveLength(1);
  });

  it("leaves a lane that does not ask for it alone", () => {
    renderCre([evening, americas("CRE_AMERICAS_PT", `${ISO}T15:00:00.000Z`, "2026-09-24T01:00:00.000Z")], false);
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
  });
});

describe("DayLadder: every name on a one-team card, and the clock", () => {
  const AMERICAS_PT = shift({
    code: "CRE_AMERICAS_PT",
    shortCode: "NLK",
    label: "Americas regular hours",
    startMinute: 480,
    endMinute: 1080,
    authoringTimeZone: "Etc/GMT+7",
    isRotation: false,
    colourToken: "NLK",
    sortOrder: 55,
  });
  const shifts = shiftMap(REGULAR, AMERICAS_PT);
  const americasCrew = Array.from({ length: 15 }, (_, i) =>
    assignment({
      name: `Americas Engineer ${String(i + 1).padStart(2, "0")}`,
      teamKey: "americas",
      rotaDate: ISO,
      shiftCode: "CRE_AMERICAS_PT",
      startsAt: `${ISO}T15:00:00.000Z`,
      endsAt: "2026-09-24T01:00:00.000Z",
    }),
  );

  function renderWith(
    assignments: LadderLane["assignments"],
    clock?: { tz: string; onClockChange: (tz: string) => void },
  ) {
    return render(
      <DayLadder
        day={WEDNESDAY}
        tz={clock?.tz ?? TZ}
        zoneLabel="IST"
        lanes={[{ name: "Rotations", assignments, layout: "flat", regions: true }]}
        shifts={shifts}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        clockOptions={
          clock
            ? [
                { value: "Asia/Colombo", label: "IST" },
                { value: "Etc/GMT+7", label: "PDT" },
              ]
            : undefined
        }
        onClockChange={clock?.onClockChange}
        {...scopeControls()}
      />,
    );
  }

  it("lists all fifteen people of a one-team card rather than splitting it by team", () => {
    renderWith(americasCrew);
    for (const a of americasCrew) expect(screen.getByText(a.engineer.name)).toBeInTheDocument();
    expect(screen.queryByText(/more$/)).not.toBeInTheDocument();
  });

  it("keeps a cut-off card at its own hours, and opens it on hover with everyone", () => {
    // jsdom lays nothing out: make every card report more content than room.
    const scroll = vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(900);
    const client = vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(100);
    try {
      const { container } = renderWith(americasCrew);
      const cards = [...container.querySelectorAll<HTMLElement>(".zblk")];
      expect(cards.length).toBeGreaterThan(0);
      for (const card of cards) {
        // At rest, exactly its hours.
        expect(card).toHaveClass("canopen");
        expect(card.style.height).toMatch(/^\d+(\.\d+)?px$/);
        expect(card).toHaveTextContent("Show all 15");
        // On hover, the whole list.
        fireEvent.mouseEnter(card);
        expect(card).toHaveClass("open");
        expect(card.style.height).toBe("auto");
        for (const a of americasCrew) expect(card).toHaveTextContent(a.engineer.name);
        fireEvent.mouseLeave(card);
        expect(card).not.toHaveClass("open");
      }
    } finally {
      scroll.mockRestore();
      client.mockRestore();
    }
  });

  it("still splits a crowded card of several teams by team", () => {
    const mixed = Array.from({ length: 14 }, (_, i) =>
      nineToFive(`Engineer ${String(i + 1).padStart(2, "0")}`, "CRE_REGULAR", i % 2 === 0 ? "alpha" : "beta"),
    );
    renderWith(mixed);
    expect(screen.queryByText("Engineer 01")).not.toBeInTheDocument();
  });

  it("draws no clock picker without choices", () => {
    renderWith(americasCrew);
    expect(screen.queryByRole("combobox", { name: "Read the day on" })).not.toBeInTheDocument();
  });

  it("offers IST and PDT, and reads the day on the one picked", () => {
    const onClockChange = vi.fn();
    const { rerender } = renderWith(americasCrew, { tz: TZ, onClockChange });
    const picker = screen.getByRole("combobox", { name: "Read the day on" });
    expect(picker).toHaveValue("Asia/Colombo");
    fireEvent.change(picker, { target: { value: "Etc/GMT+7" } });
    expect(onClockChange).toHaveBeenCalledWith("Etc/GMT+7");
    // On the PDT clock the Americas day reads 08:00-18:00.
    rerender(
      <DayLadder
        day={WEDNESDAY}
        tz="Etc/GMT+7"
        zoneLabel="PDT"
        lanes={[{ name: "Rotations", assignments: americasCrew, layout: "flat", regions: true }]}
        shifts={shifts}
        zones={ZONES}
        absences={[]}
        absenceKinds={[ANNUAL_LEAVE]}
        clockOptions={[
          { value: "Asia/Colombo", label: "IST" },
          { value: "Etc/GMT+7", label: "PDT" },
        ]}
        onClockChange={onClockChange}
        {...scopeControls()}
      />,
    );
    expect(screen.getByText("08:00 – 18:00")).toBeInTheDocument();
  });
});
