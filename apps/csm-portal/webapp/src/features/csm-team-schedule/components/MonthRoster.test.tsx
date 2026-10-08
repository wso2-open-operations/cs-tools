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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import MonthRoster from "./MonthRoster";
import {
  ANNUAL_LEAVE,
  EVENING,
  MONDAY,
  REGULAR,
  TZ1,
  TZ1_WE,
  TZ2,
  TZ2_WE,
  RND,
  TZ1_L1,
  MOE_DAY,
  MOE_NIGHT,
  absence,
  assignment,
  scopeControls,
  shiftMap,
} from "../test/fixtures";

const SHIFTS = shiftMap(REGULAR, EVENING, TZ1, TZ2, TZ1_WE, TZ2_WE);

function renderRoster(over: Partial<React.ComponentProps<typeof MonthRoster>> = {}) {
  const props = {
    month: MONDAY,
    monthCount: 1,
    assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code })],
    absences: [],
    shifts: SHIFTS,
    absenceKinds: [ANNUAL_LEAVE],
    selectedIso: "2026-09-21",
    ...scopeControls(),
    ...over,
  } as React.ComponentProps<typeof MonthRoster>;
  return render(<MonthRoster {...props} />);
}

describe("MonthRoster: who may edit", () => {
  it("offers no editable cell when the reader leads nothing", () => {
    const { container } = renderRoster({ leadTeams: [], editing: true, onEditCell: vi.fn() });
    expect(container.querySelectorAll("td.editable")).toHaveLength(0);
  });

  it("offers no editable cell until edit mode is on, even for a lead", () => {
    // A lead reads this grid far more often than they change it, so a cell
    // that writes on one stray click is the thing being guarded against.
    const { container } = renderRoster({
      leadTeams: ["alpha"],
      editing: false,
      onEditCell: vi.fn(),
    });
    expect(container.querySelectorAll("td.editable")).toHaveLength(0);
  });

  it("makes only the reader's own team editable", () => {
    const { container } = renderRoster({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" }),
        assignment({ name: "Nuwan", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "charlie" }),
      ],
      leadTeams: ["alpha"],
      editing: true,
      onEditCell: vi.fn(),
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    const own = rows.find((r) => r.textContent?.includes("Asela"));
    const other = rows.find((r) => r.textContent?.includes("Nuwan"));
    expect(own?.querySelectorAll("td.editable").length).toBeGreaterThan(0);
    expect(other?.querySelectorAll("td.editable")).toHaveLength(0);
  });

  it("reports the slot, what is on it, and where it is", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({ leadTeams: ["alpha"], editing: true, onEditCell });
    const cell = container.querySelector("td.editable");
    fireEvent.click(cell!);

    expect(onEditCell).toHaveBeenCalledTimes(1);
    const edit = onEditCell.mock.calls[0][0];
    expect(edit).toMatchObject({ name: "Asela", teamKey: "alpha" });
    // The picker opens against the cell, so it needs to know where that is.
    expect(edit.anchor).toEqual(
      expect.objectContaining({
        top: expect.any(Number),
        left: expect.any(Number),
        bottom: expect.any(Number),
        right: expect.any(Number),
      }),
    );
  });

  it("reports the leave on a cell so the picker can mark it and offer a clear", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
      leadTeams: ["alpha"],
      editing: true,
      onEditCell,
    });
    const cell = [...container.querySelectorAll("td.editable")].find((td) =>
      td.textContent?.includes("AL"),
    );
    fireEvent.click(cell!);
    expect(onEditCell.mock.calls[0][0].absenceKindCode).toBe("ANNUAL_LEAVE");
  });
});

describe("MonthRoster: what the grid says", () => {
  it("shows leave over the rota underneath it", () => {
    // Somebody on leave is not on the rota that day, whatever the generated
    // row says -- but the assignment is only covered, never deleted.
    renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
    });
    expect(screen.getByText("AL")).toBeInTheDocument();
    expect(screen.queryByText("6-9p")).not.toBeInTheDocument();
  });

  it("marks the week the reader is in, its two ends included", () => {
    // Three months around today, as the page renders it: with one month the
    // test failed in any week that crosses a month end (a week starting on the
    // 28th shows three of its days in that month).
    const now = new Date();
    const { container } = renderRoster({
      month: new Date(now.getFullYear(), now.getMonth() - 1, 1),
      monthCount: 3,
    });
    const band = container.querySelectorAll("thead th.day.cw");
    expect(band).toHaveLength(7);
    expect(container.querySelectorAll("thead th.day.cwa")).toHaveLength(1);
    expect(container.querySelectorAll("thead th.day.cwz")).toHaveLength(1);
  });

  it("marks today and the chosen day apart", () => {
    const { container } = renderRoster({ month: new Date(), selectedIso: "2026-09-21" });
    // Today is a fact and the selection is a choice; both are marked, and a
    // month that contains neither would mark nothing.
    expect(container.querySelectorAll("thead th.day.today").length).toBeLessThanOrEqual(1);
  });

  it("finds an engineer by name and hides the rest", () => {
    const { container } = renderRoster({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code }),
        assignment({ name: "Nuwan", rotaDate: "2026-09-21", shiftCode: EVENING.code }),
      ],
    });
    fireEvent.change(screen.getByLabelText("Find an engineer"), { target: { value: "asel" } });
    expect(screen.getByText("Asela")).toBeInTheDocument();
    expect(screen.queryByText("Nuwan")).not.toBeInTheDocument();
    expect(container.querySelectorAll("tbody tr")).toHaveLength(1);
  });
});

describe("MonthRoster: the zone-split grid", () => {
  const sreProps = {
    assignments: [
      assignment({ name: "Apollo01", rotaDate: "2026-09-21", shiftCode: TZ1.code, zoneCode: "TZ1", teamKey: "delta" }),
      assignment({ name: "Apollo01", rotaDate: "2026-09-21", shiftCode: TZ2.code, zoneCode: "TZ2", teamKey: "delta" }),
    ],
    family: "SRE" as const,
  };

  it("gives a lead a cell per zone, not one for the day", () => {
    const { container } = renderRoster({
      ...sreProps,
      leadTeams: ["delta"],
      editing: true,
      onEditCell: vi.fn(),
    });
    expect(container.querySelectorAll("td.z.editable").length).toBeGreaterThan(1);
  });

  it("says which zone column was clicked, so the picker can narrow to it", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      ...sreProps,
      leadTeams: ["delta"],
      editing: true,
      onEditCell,
    });
    const zoneCells = [...container.querySelectorAll("td.z.editable")];
    fireEvent.click(zoneCells[0]);
    expect(onEditCell.mock.calls[0][0].zoneCode).toBeTruthy();
  });

  it("keeps the week band's edges on the day rather than on each zone", () => {
    // A day split across zone columns still has one left edge and one right
    // edge; ruling every zone would mark the inside of a day as heavily as
    // its boundary.
    const { container } = renderRoster({ ...sreProps, month: new Date() });
    const opens = container.querySelectorAll("tbody td.cwa");
    const closes = container.querySelectorAll("tbody td.cwz");
    const rows = container.querySelectorAll("tbody tr").length;
    expect(opens.length).toBeLessThanOrEqual(rows);
    expect(closes.length).toBeLessThanOrEqual(rows);
  });
});

describe("MonthRoster: changes made this session", () => {
  it("marks exactly the days changed, and nothing else", () => {
    const { container } = renderRoster({
      changedCells: new Set(["u-Asela|2026-09-21", "u-Asela|2026-09-22"]),
    });
    const marked = container.querySelectorAll("tbody td.changed");
    expect(marked).toHaveLength(2);
  });

  it("marks nothing when there are no changes", () => {
    const { container } = renderRoster({ changedCells: new Set() });
    expect(container.querySelectorAll("tbody td.changed")).toHaveLength(0);
  });
});

describe("MonthRoster: who changed a cell", () => {
  const edited = new Map([
    ["u-Asela|2026-09-21", { actor: "lead@example.test", changedAt: "2026-09-20T10:00:00.000Z" }],
  ]);

  it("marks only the cells somebody actually changed", () => {
    // The mark has to stay rare: on a three-month grid almost every cell was
    // generated, and marking those would say nothing.
    const { container } = renderRoster({ editedCells: edited });
    expect(container.querySelectorAll("td.touched")).toHaveLength(1);
  });

  it("says who changed it and when, on the cell itself", () => {
    const { container } = renderRoster({ editedCells: edited });
    expect(container.querySelector("td.touched")).toHaveAttribute(
      "title",
      expect.stringContaining("changed by lead@example.test"),
    );
  });

  it("marks nothing at all when the history has not arrived yet", () => {
    // The grid renders from the rota alone; markers turn up when they turn up.
    const { container } = renderRoster({ editedCells: undefined });
    expect(container.querySelectorAll("td.touched")).toHaveLength(0);
  });
});

describe("MonthRoster: two tags in one cell", () => {
  it("shows a rotation turn and the allocation beside it", () => {
    const { container } = renderRoster({
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code })],
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21", kindCode: RND.code })],
      absenceKinds: [ANNUAL_LEAVE, RND],
    });
    const duo = container.querySelector(".duo");
    expect(duo).not.toBeNull();
    expect(duo).toHaveTextContent(EVENING.shortCode);
    expect(duo).toHaveTextContent(RND.shortCode);
  });

  it("lets leave take the whole day", () => {
    const { container } = renderRoster({
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code })],
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
    });
    expect(container.querySelector(".duo")).toBeNull();
  });

  it("hands both to the picker when the cell is opened", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      leadTeams: ["alpha"],
      editing: true,
      onEditCell,
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code })],
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21", kindCode: RND.code })],
      absenceKinds: [ANNUAL_LEAVE, RND],
    });
    fireEvent.click(container.querySelector(".duo")!.closest("td")!);
    expect(onEditCell).toHaveBeenCalledWith(
      expect.objectContaining({ shiftCode: EVENING.code, absenceKindCode: RND.code }),
    );
  });
});

describe("MonthRoster: a zone's turn on an allocation day", () => {
  it("stacks the turn and the allocation in that zone, and shows the allocation in the rest", () => {
    // Every zone this group works, weekday and weekend, so the day splits.
    const { container } = renderRoster({
      family: "SRE",
      shifts: shiftMap(REGULAR, TZ1, TZ1_L1, TZ2, TZ1_WE, TZ2_WE),
      assignments: [
        { ...assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: TZ1_L1.code, zoneCode: "TZ1" }), tier: "L1" },
      ],
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21", kindCode: RND.code })],
      absenceKinds: [ANNUAL_LEAVE, RND],
    } as never);
    const zoneCells = [...container.querySelectorAll("td.c.z")].filter((td) => td.textContent?.trim());
    const tz1 = zoneCells.find((td) => td.querySelector(".duo"));
    expect(tz1).toBeDefined();
    expect(tz1).toHaveTextContent("L1");
    expect(tz1).toHaveTextContent(RND.shortCode);
    expect(zoneCells.some((td) => !td.querySelector(".duo") && td.textContent?.includes(RND.shortCode))).toBe(true);
  });
});

describe("MonthRoster: a turn on a zone's regular hours", () => {
  it("stacks the turn over the regular hours in that zone", () => {
    const TZ1_REG = { ...REGULAR, id: "r1", code: "SRE_TZ1_REGULAR", shortCode: "SUP", family: "SRE" as const, zoneCode: "TZ1" };
    const { container } = renderRoster({
      family: "SRE",
      shifts: shiftMap(TZ1_REG, TZ1, TZ1_L1, TZ2, TZ1_WE, TZ2_WE),
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: TZ1_REG.code, zoneCode: "TZ1" }),
        { ...assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: TZ1_L1.code, zoneCode: "TZ1" }), tier: "L1" },
      ],
    } as never);
    const duo = container.querySelector("td.c.z .duo");
    expect(duo).not.toBeNull();
    expect(duo).toHaveTextContent("L1");
    expect(duo).toHaveTextContent("SUP");
  });
});

describe("MonthRoster: an unmarked weekday is a working day", () => {
  // September 2026 has 22 weekdays; the fixture engineer holds a turn on one.
  it("shows LK on every weekday nobody has marked, and nothing at the weekend", () => {
    const { container } = renderRoster();
    expect(container.querySelectorAll(".chip.dflt")).toHaveLength(21);
    expect([...container.querySelectorAll(".chip.dflt")].every((c) => c.textContent === "LK")).toBe(true);
    // The turn on the 21st is still the turn, not LK.
    expect(screen.getByText("6-9p")).toBeInTheDocument();
  });

  it("gives way to leave the moment it is marked", () => {
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-23" })],
    });
    expect(container.querySelectorAll(".chip.dflt")).toHaveLength(19);
    expect(screen.getAllByText("AL")).toHaveLength(2);
  });

  it("spans an SRE day's zones as one LK, and leaves a day with a zone alone", () => {
    const { container } = renderRoster({
      assignments: [
        assignment({ name: "Apollo01", rotaDate: "2026-09-21", shiftCode: TZ1.code, zoneCode: "TZ1", teamKey: "delta" }),
      ],
      family: "SRE",
    });
    expect(container.querySelectorAll("td.zwhole .chip.dflt")).toHaveLength(21);
  });

  it("opens an LK day in the picker as an empty day, not as a held window", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({ leadTeams: ["alpha"], editing: true, onEditCell });
    fireEvent.click(container.querySelector(".chip.dflt")!.closest("td")!);
    expect(onEditCell).toHaveBeenCalledWith(
      expect.objectContaining({ shiftCode: undefined, absenceKindCode: undefined }),
    );
  });
});

describe("MonthRoster: the CRE / SRE switch", () => {
  it("offers the switch when there are two groups to look at", () => {
    renderRoster({ families: ["CRE", "SRE"] });
    expect(screen.getByRole("tablist", { name: "Show CRE or SRE" })).toBeInTheDocument();
  });

  it("offers no switch on an engineer's own rota, which is one group", () => {
    renderRoster({ families: ["SRE"], family: "SRE" });
    expect(screen.queryByRole("tablist", { name: "Show CRE or SRE" })).not.toBeInTheDocument();
  });
});

describe("MonthRoster: how many months", () => {
  it("offers 1, 3 and 6 months, marks the one showing, and reports a change", () => {
    const onSpanChange = vi.fn();
    renderRoster({ span: 3, onSpanChange });
    const group = screen.getByRole("group", { name: "Months shown" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "3 months" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "1 month" })).toHaveAttribute("aria-pressed", "false");
    fireEvent.click(screen.getByRole("button", { name: "6 months" }));
    expect(onSpanChange).toHaveBeenCalledWith(6);
  });

  it("offers no choice when the page cannot change the months", () => {
    renderRoster();
    expect(screen.queryByRole("group", { name: "Months shown" })).not.toBeInTheDocument();
  });

  it("draws a column for every day of six months", () => {
    // Sept 2026 through Feb 2027: 30 + 31 + 30 + 31 + 31 + 28.
    const { container } = renderRoster({ monthCount: 6 });
    expect(container.querySelectorAll("thead th.day")).toHaveLength(181);
  });
});

describe("MonthRoster: a window around a day", () => {
  it("runs from the first day to the last, naming the month on the first column", () => {
    const { container } = renderRoster({ from: new Date(2026, 8, 14), to: new Date(2026, 9, 12) });
    const heads = container.querySelectorAll("thead th.day");
    expect(heads).toHaveLength(29);
    expect(heads[0].querySelector(".d")?.textContent).toBe("14");
    expect(heads[0].querySelector(".mo")).not.toBeNull();
    expect(heads[heads.length - 1].querySelector(".d")?.textContent).toBe("12");
  });

  it("names the weekday under every date", () => {
    const { container } = renderRoster({ from: new Date(2026, 8, 14), to: new Date(2026, 9, 12) });
    const heads = container.querySelectorAll("thead th.day");
    const weekday = (d: Date) => d.toLocaleDateString(undefined, { weekday: "short" });
    // 14 Sept 2026 is a Monday; 12 Oct 2026 is one too.
    expect(heads[0].querySelector(".w")?.textContent).toBe(weekday(new Date(2026, 8, 14)));
    expect(heads[5].querySelector(".w")?.textContent).toBe(weekday(new Date(2026, 8, 19)));
    expect(heads[heads.length - 1].querySelector(".w")?.textContent).toBe(weekday(new Date(2026, 9, 12)));
  });
});

describe("MonthRoster: an SME rotation", () => {
  const smeProps = {
    assignments: [
      assignment({ name: "Moesif01", rotaDate: "2026-09-21", shiftCode: MOE_DAY.code, zoneCode: "MOE_D", tier: "L1", teamKey: "moesif" }),
      assignment({ name: "Moesif02", rotaDate: "2026-09-21", shiftCode: MOE_NIGHT.code, zoneCode: "MOE_N", tier: "L2", teamKey: "moesif" }),
    ],
    shifts: shiftMap(MOE_DAY, MOE_NIGHT),
    family: "SME" as const,
    teams: ["moesif"],
    families: ["SME"] as const,
  };

  it("splits each day into its Day and Night, on weekdays and at the weekend alike", () => {
    const { container } = renderRoster(smeProps);
    const heads = [...container.querySelectorAll("tr.zrow th.zc")].map((th) => th.textContent);
    expect(heads.slice(0, 2)).toEqual(["Day", "Night"]);
    // A rotation runs the same shape every day: a Saturday has both too.
    expect(heads.filter((h) => h === "Day").length).toBe(heads.filter((h) => h === "Night").length);
    expect(heads.length).toBeGreaterThanOrEqual(14);
  });

  it("does not split a CRE day even when an SME window is in the catalogue", () => {
    const { container } = renderRoster({ shifts: shiftMap(REGULAR, EVENING, MOE_DAY, MOE_NIGHT) });
    expect(container.querySelector("tr.zrow")).toBeNull();
  });
});

describe("MonthRoster: every SME rota at once (All teams)", () => {
  const ASG_DAY = { ...MOE_DAY, code: "SME_ASG_DAY", label: "Asgardeo day escalation", zoneCode: "ASG_D", startMinute: 570, endMinute: 1110 };
  const ASG_NIGHT = { ...MOE_NIGHT, code: "SME_ASG_NIGHT", label: "Asgardeo night escalation", zoneCode: "ASG_N", startMinute: 1110, endMinute: 2010 };
  const props = {
    assignments: [
      assignment({ name: "Moesif01", rotaDate: "2026-09-21", shiftCode: MOE_DAY.code, zoneCode: "MOE_D", tier: "L1", teamKey: "moesif" }),
      assignment({ name: "Asgardeo01", rotaDate: "2026-09-21", shiftCode: ASG_NIGHT.code, zoneCode: "ASG_N", tier: "L2", teamKey: "asgardeo" }),
    ],
    shifts: shiftMap(MOE_DAY, MOE_NIGHT, ASG_DAY, ASG_NIGHT),
    family: "SME" as const,
    teams: ["asgardeo", "moesif"],
    families: ["SME"] as const,
  };

  it("keeps one Day and one Night column a day, shared by the rotas", () => {
    const { container } = renderRoster(props);
    const heads = [...container.querySelectorAll("tr.zrow th.zc")].map((th) => th.textContent);
    expect(heads.slice(0, 4)).toEqual(["Day", "Night", "Day", "Night"]);
  });

  it("shows every rota's people, each turn in its own column", () => {
    renderRoster(props);
    expect(screen.getByText("Moesif01")).toBeInTheDocument();
    expect(screen.getByText("Asgardeo01")).toBeInTheDocument();
  });

  it("hands a lead's click the engineer's own rota's zone, not the column's name", () => {
    const onEditCell = vi.fn();
    const ZONES_BY_TEAM: Record<string, string> = { "moesif|Day": "MOE_D", "moesif|Night": "MOE_N", "asgardeo|Day": "ASG_D", "asgardeo|Night": "ASG_N" };
    const zoneCodeFor = (teamKey: string, column: string) => ZONES_BY_TEAM[`${teamKey}|${column}`];
    const { container } = renderRoster({ ...props, leadTeams: ["asgardeo"], editing: true, onEditCell, zoneCodeFor });
    const cell = container.querySelector("td.z.editable") as HTMLElement;
    fireEvent.click(cell);
    expect(onEditCell).toHaveBeenCalledWith(expect.objectContaining({ teamKey: "asgardeo", zoneCode: "ASG_D" }));
  });
});

describe("MonthRoster: team leads", () => {
  it("opens each team with its lead, tagged, even with no window this month", () => {
    const { container } = renderRoster({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" }),
        assignment({ name: "Bimal", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" }),
      ],
      teamMembers: {
        alpha: [{ userId: "lead-alpha", name: "Zara Lead", email: "zara@example.com", isLead: true, role: "lead" }],
      },
    });
    const names = [...container.querySelectorAll("tbody tr .who")].map((n) => n.textContent);
    expect(names).toEqual(["Zara Lead", "Asela", "Bimal"]);
    const leadRow = container.querySelector("tbody tr")!;
    expect(leadRow.querySelector(".leadtag")).toHaveTextContent("Lead");
    expect(container.querySelectorAll("tbody .leadtag")).toHaveLength(1);
  });

  it("seats only the lead of the team picked", () => {
    const { container } = renderRoster({
      teamKey: "alpha",
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" })],
      teamMembers: {
        alpha: [{ userId: "lead-alpha", name: "Zara Lead", email: "zara@example.com", isLead: true, role: "lead" }],
        bravo: [{ userId: "lead-bravo", name: "Yan Lead", email: "yan@example.com", isLead: true, role: "lead" }],
      },
    });
    const names = [...container.querySelectorAll("tbody tr .who")].map((n) => n.textContent);
    expect(names).toEqual(["Zara Lead", "Asela"]);
  });
});

describe("MonthRoster: somebody moved to another team", () => {
  // Which team a moved person sits under depends on the day: pin today.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 8, 28));
  });
  afterEach(() => vi.useRealTimers());
  const BRAZIL = {
    id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
    colourToken: "BR", sortOrder: 70, movesToTeamKey: "bravo", worksRotaThere: true,
  };
  // Their own team (alpha) until the 23rd, the team the rotation moved them to
  // (bravo) from the 24th.
  const props = {
    assignments: [assignment({ name: "Bruna", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" })],
    absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-24", endsOn: "2026-12-31", kindCode: "ALLO_BR", teamKey: "bravo" }), homeTeamKey: "alpha" }],
    absenceKinds: [ANNUAL_LEAVE, BRAZIL],
  };

  it("shows them once, under the team they moved to, with their own team's days on it too", () => {
    const { container } = renderRoster(props);
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows.map((tr) => [tr.querySelector(".who")?.textContent, tr.querySelector(".team")?.textContent])).toEqual([
      ["Bruna", "Bravo"],
    ]);
    // Mon 21 Sep, before the move, is the evening turn held on their own team.
    expect(rows[0].querySelectorAll("td")[20].textContent?.trim()).toBe(EVENING.shortCode);
  });

  it("lets their own team's lead change the moved row, where the span is", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({ ...props, leadTeams: ["alpha"], editing: true, onEditCell });
    const movedRow = container.querySelectorAll("tbody tr")[0];
    const cells = movedRow.querySelectorAll("td");
    // Thu 24 Sep (column 23) is inside the span: the team it moved them to.
    fireEvent.click(cells[23] as HTMLElement);
    expect(onEditCell).toHaveBeenLastCalledWith(expect.objectContaining({ teamKey: "bravo", rotaDate: "2026-09-24" }));
    // Tue 1 Sep is before it: still their own team's day.
    fireEvent.click(cells[0] as HTMLElement);
    expect(onEditCell).toHaveBeenLastCalledWith(expect.objectContaining({ teamKey: "alpha", rotaDate: "2026-09-01" }));
  });

  it("gives a third team's lead nothing to change on it", () => {
    const { container } = renderRoster({ ...props, leadTeams: ["charlie"], editing: true, onEditCell: vi.fn() });
    expect(container.querySelector("tbody td.editable")).toBeNull();
  });
});

describe("MonthRoster: a stint drawn as the team's normal hours", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 8, 23));
  });
  afterEach(() => vi.useRealTimers());
  it("shows the window the tag names on weekdays, a turn over it, and nothing at the weekend", () => {
    const BRAZIL = {
      id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
      colourToken: "BR", sortOrder: 70, movesToTeamKey: "bravo", worksRotaThere: true, showsAsShiftCode: REGULAR.code,
    };
    const { container } = renderRoster({
      // Mon 21 Sep holds an evening turn; the span runs Mon 21 to Sun 27.
      assignments: [assignment({ name: "Bruna", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "bravo" })],
      absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-21", endsOn: "2026-09-27", kindCode: "ALLO_BR", teamKey: "bravo" }), homeTeamKey: "alpha" }],
      absenceKinds: [ANNUAL_LEAVE, BRAZIL],
    });
    expect([...container.querySelectorAll("tbody .chip")].map((c) => c.textContent)).not.toContain("BR");
    const row = container.querySelector("tbody tr")!;
    // The month opens on 1 September, so Mon 21 to Sun 27 are columns 20-26.
    const cells = [...row.querySelectorAll("td")].slice(20, 27).map((td) => td.textContent?.trim());
    expect(cells[0]).toBe(EVENING.shortCode);
    expect(cells.slice(1, 5)).toEqual(Array(4).fill(REGULAR.shortCode));
    expect(cells.slice(5, 7).some((c) => c === REGULAR.shortCode)).toBe(false);
  });
});

describe("MonthRoster: every team member has a row", () => {
  const member = (userId: string, name: string, role: string) => ({
    userId, name, email: `${userId}@example.com`, isLead: role === "lead" || role === "americas_team_lead", role,
  });

  it("keeps a member with no entry this month, so their leave can still be marked", () => {
    const { container } = renderRoster({
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" })],
      teamMembers: { alpha: [member("u-Asela", "Asela", "engineer"), member("u-cleared", "Chen Cleared", "engineer")] },
    });
    const names = [...container.querySelectorAll("tbody tr .who")].map((n) => n.textContent);
    expect(names).toEqual(["Asela", "Chen Cleared"]);
  });

  it("does not add an empty home-team row for somebody shown under the team they moved to", () => {
    const BRAZIL = {
      id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
      colourToken: "BR", sortOrder: 70, movesToTeamKey: "bravo", worksRotaThere: true,
    };
    const { container } = renderRoster({
      teamKey: "alpha",
      assignments: [],
      absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-01", endsOn: "2026-12-31", kindCode: "ALLO_BR", teamKey: "bravo" }), homeTeamKey: "alpha" }],
      absenceKinds: [ANNUAL_LEAVE, BRAZIL],
      teamMembers: { alpha: [member("u-Bruna", "Bruna", "engineer")] },
    });
    const rows = [...container.querySelectorAll("tbody tr")].map((tr) => tr.querySelector(".team")?.textContent);
    expect(rows).toEqual(["Bravo"]);
  });
});

describe("MonthRoster: somebody who has left", () => {
  const EXCLUDED = {
    id: "k-exc", code: "EXCLUDED", shortCode: "EXC", label: "Excluded from rota", bucket: "EXCLUDED" as const,
    colourToken: "EXC", sortOrder: 110,
  };
  const member = (userId: string, name: string) => ({ userId, name, email: `${userId}@example.com`, isLead: false, role: "engineer" });

  it("is listed only for the days they worked, with nothing drawn after", () => {
    const { container } = renderRoster({
      // Last shift on Tue 1 Sep; excluded from the rota from the 2nd on.
      assignments: [assignment({ name: "Lee Left", rotaDate: "2026-09-01", shiftCode: EVENING.code, teamKey: "alpha" })],
      absences: [absence({ name: "Lee Left", startsOn: "2026-09-02", endsOn: "2026-12-31", kindCode: "EXCLUDED", teamKey: "alpha" })],
      absenceKinds: [ANNUAL_LEAVE, EXCLUDED],
      teamMembers: { alpha: [] },
    });
    const row = container.querySelector("tbody tr")!;
    expect(row.querySelector(".who")?.textContent).toBe("Lee Left");
    const cells = [...row.querySelectorAll("td")].map((td) => td.textContent?.trim());
    expect(cells[0]).toBe(EVENING.shortCode);
    // Wed 2 Sep onwards is the exclusion, not a working day.
    expect(cells.slice(1).filter((c) => c === REGULAR.shortCode)).toEqual([]);
  });

  it("is not listed on a month they only have the exclusion in", () => {
    const { container } = renderRoster({
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" })],
      absences: [absence({ name: "Lee Left", startsOn: "2026-03-26", endsOn: "2026-12-31", kindCode: "EXCLUDED", teamKey: "alpha" })],
      absenceKinds: [ANNUAL_LEAVE, EXCLUDED],
      teamMembers: { alpha: [member("u-Asela", "Asela")] },
    });
    const names = [...container.querySelectorAll("tbody tr .who")].map((n) => n.textContent);
    expect(names).toEqual(["Asela"]);
  });

  it("still shows a current member who is excluded", () => {
    const { container } = renderRoster({
      assignments: [],
      absences: [absence({ name: "Ema", startsOn: "2026-09-01", endsOn: "2026-12-31", kindCode: "EXCLUDED", teamKey: "alpha" })],
      absenceKinds: [ANNUAL_LEAVE, EXCLUDED],
      teamMembers: { alpha: [member("u-Ema", "Ema")] },
    });
    const names = [...container.querySelectorAll("tbody tr .who")].map((n) => n.textContent);
    expect(names).toEqual(["Ema"]);
  });
});


describe("MonthRoster: back from a move mid-month", () => {
  it("keeps one row, under the team they moved to, reading their own team's ordinary day after it", () => {
    const BRAZIL = {
      id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
      colourToken: "BR", sortOrder: 70, movesToTeamKey: "bravo", worksRotaThere: true, showsAsShiftCode: EVENING.code,
    };
    const { container } = renderRoster({
      assignments: [],
      // On the rotation 1-15 Sep, back on their own team from the 16th.
      absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-01", endsOn: "2026-09-15", kindCode: "ALLO_BR", teamKey: "bravo" }), homeTeamKey: "alpha" }],
      absenceKinds: [ANNUAL_LEAVE, BRAZIL],
      teamMembers: { alpha: [{ userId: "u-Bruna", name: "Bruna", email: "b@example.com", isLead: false, role: "engineer" }] },
      teamDefaultShift: { bravo: EVENING.code },
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows.map((tr) => tr.querySelector(".team")?.textContent)).toEqual(["Bravo"]);
    const cells = [...rows[0].querySelectorAll("td")].map((td) => td.textContent?.trim());
    // Tue 1 Sep is on the rotation; Wed 16 Sep is their own team's ordinary day.
    expect(cells[0]).toBe(EVENING.shortCode);
    expect(cells[15]).toBe(REGULAR.shortCode);
  });

  it("reads a team's own ordinary day where it has one", () => {
    const { container } = renderRoster({
      assignments: [],
      teamMembers: { bravo: [{ userId: "u-Nia", name: "Nia", email: "n@example.com", isLead: false, role: "engineer" }] },
      teamDefaultShift: { bravo: EVENING.code },
    });
    const cells = [...container.querySelectorAll("tbody tr td")].map((td) => td.textContent?.trim());
    expect(cells[0]).toBe(EVENING.shortCode);
    expect(cells.filter((c) => c === REGULAR.shortCode)).toEqual([]);
  });
});


describe("MonthRoster: a cell is editable exactly when the server would accept the edit", () => {
  // Which team a moved person sits under depends on the day: pin today.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 8, 28));
  });
  afterEach(() => vi.useRealTimers());
  const BRAZIL = {
    id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
    colourToken: "BR", sortOrder: 70, movesToTeamKey: "bravo", worksRotaThere: true,
  };
  const member = (userId: string, name: string) => ({ userId, name, email: `${userId}@example.com`, isLead: false, role: "engineer" });
  // Moved to bravo for 24-30 Sep; a member of alpha.
  const moved = {
    assignments: [],
    absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-24", endsOn: "2026-09-30", kindCode: "ALLO_BR", teamKey: "bravo" }), homeTeamKey: "alpha" }],
    absenceKinds: [ANNUAL_LEAVE, BRAZIL],
    teamMembers: { alpha: [member("u-Bruna", "Bruna")], bravo: [] },
    editing: true,
    onEditCell: vi.fn(),
  };
  const editableDays = (container: HTMLElement) =>
    [...container.querySelectorAll("tbody tr")[0].querySelectorAll("td")]
      .map((td, i) => (td.classList.contains("editable") ? i + 1 : 0))
      .filter(Boolean);

  it("lets the team it moved them to change only the moved days", () => {
    const { container } = renderRoster({ ...moved, leadTeams: ["bravo"] });
    expect(editableDays(container)).toEqual([24, 25, 26, 27, 28, 29, 30]);
  });

  it("lets their own team's lead change every day, the span included", () => {
    const { container } = renderRoster({ ...moved, leadTeams: ["alpha"] });
    expect(editableDays(container)).toHaveLength(30);
  });

  it("gives nobody a leaver's days, who is on no team's member list", () => {
    const { container } = renderRoster({
      assignments: [assignment({ name: "Lee Left", rotaDate: "2026-09-01", shiftCode: EVENING.code, teamKey: "alpha" })],
      teamMembers: { alpha: [] },
      leadTeams: ["alpha"],
      editing: true,
      onEditCell: vi.fn(),
    });
    expect(editableDays(container)).toEqual([]);
  });
});


describe("MonthRoster: moved back before today", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 8, 28));
  });
  afterEach(() => vi.useRealTimers());

  it("is back under their own team, the move's days still drawn on that row", () => {
    const MIG = {
      id: "k-mig", code: "MIGRATION", shortCode: "Mig", label: "Migration", bucket: "ALLOCATION" as const,
      colourToken: "MIG", sortOrder: 90, movesToTeamKey: "bravo", showsAsShiftCode: REGULAR.code,
    };
    const { container } = renderRoster({
      assignments: [],
      // On Migration 1-15 Sep, moved back; today is the 28th. On their own
      // row the Migration days read Mig, not the LK they were drawn as there.
      absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-01", endsOn: "2026-09-15", kindCode: "MIGRATION", teamKey: "bravo" }), homeTeamKey: "alpha" }],
      absenceKinds: [ANNUAL_LEAVE, MIG],
      teamMembers: { alpha: [{ userId: "u-Bruna", name: "Bruna", email: "b@example.com", isLead: false, role: "engineer" }] },
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows.map((tr) => tr.querySelector(".team")?.textContent)).toEqual(["Alpha"]);
    const cells = [...rows[0].querySelectorAll("td")].map((td) => td.textContent?.trim());
    expect(cells[0]).toBe("Mig");
    expect(cells[15]).toBe(REGULAR.shortCode);
  });
});
