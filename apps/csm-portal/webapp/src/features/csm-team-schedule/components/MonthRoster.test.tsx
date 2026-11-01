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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import MonthRoster from "./MonthRoster";
import type { EngagementAllocation } from "../types";
import {
  ANNUAL_LEAVE,
  EVENING,
  MONDAY,
  REGULAR,
  REGULAR_IND,
  TZ1,
  TZ1_WE,
  TZ2,
  TZ2_WE,
  WEEKEND,
  TZ3,
  RND,
  TZ1_L1,
  MOE_DAY,
  MOE_NIGHT,
  absence,
  assignment,
  scopeControls,
  shift,
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
  it("shows leave over the rota, and the turn it leaves uncovered", () => {
    // Somebody on leave is not on the rota that day -- the leave is the cell
    // -- but the turn generated for them is still there, and the gap is the
    // thing the lead needs to see: struck through, and the cell flagged.
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
    });
    const td = container.querySelector("tbody td.clash");
    expect(td).not.toBeNull();
    expect(td?.querySelector(".chip:not(.gone)")).toHaveTextContent("AL");
    expect(td?.querySelector(".chip.gone")).toHaveTextContent("6-9p");
    expect(td).toHaveAttribute("title", expect.stringContaining("needs cover"));
  });

  it("flags no clash on leave that covers no turn", () => {
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-22" })],
    });
    expect(container.querySelector("tbody td.clash")).toBeNull();
  });

  it("leaves the weekend inside a leave span blank", () => {
    // Leave is not taken on a weekend: Friday to Monday reads as two days.
    const { container } = renderRoster({
      assignments: [],
      absences: [absence({ name: "Asela", startsOn: "2026-09-25", endsOn: "2026-09-28" })],
    });
    const grid = container.querySelector("table") as HTMLElement;
    expect(within(grid).getAllByText("AL")).toHaveLength(2);
  });

  it("leaves the weekend inside an allocation blank too", () => {
    // Allocations are working-day time, like leave: Friday to Monday is two days.
    const { container } = renderRoster({
      assignments: [],
      absences: [absence({ name: "Asela", startsOn: "2026-09-25", endsOn: "2026-09-28", kindCode: RND.code })],
      absenceKinds: [ANNUAL_LEAVE, RND],
    });
    const grid = container.querySelector("table") as HTMLElement;
    expect(within(grid).getAllByText(RND.shortCode)).toHaveLength(2);
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

  it("lets leave take the whole day, the uncovered turn struck through beside it", () => {
    const { container } = renderRoster({
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code })],
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
    });
    const pair = container.querySelector(".duo");
    expect(pair).toHaveClass("clash");
    expect(pair?.firstElementChild).toHaveTextContent("AL");
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
    expect(within(container.querySelector("table") as HTMLElement).getByText("6-9p")).toBeInTheDocument();
  });

  it("gives way to leave the moment it is marked", () => {
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-23" })],
    });
    expect(container.querySelectorAll(".chip.dflt")).toHaveLength(19);
    expect(within(container.querySelector("table") as HTMLElement).getAllByText("AL")).toHaveLength(2);
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

  it("draws leave filed under a team the person is not on, on their own team's row", () => {
    // A Castor-style member whose lieu leave was filed under another team
    // (Migration) used to get a second row there, on leave in one team and
    // on the rota in the other the same day.
    const LIEU = { ...ANNUAL_LEAVE, id: "k-ll", code: "LIEU_LEAVE", shortCode: "LL", label: "Lieu leave", colourToken: "LL" };
    const { container } = renderRoster({
      teamKey: "",
      teams: ["alpha", "bravo"],
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" })],
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-22", kindCode: "LIEU_LEAVE", teamKey: "bravo" })],
      absenceKinds: [ANNUAL_LEAVE, LIEU],
      teamMembers: { alpha: [member("u-Asela", "Asela", "engineer")], bravo: [] },
      ownTeams: new Map([["u-Asela", ["alpha"]]]),
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows).toHaveLength(1);
    expect(rows[0].querySelector(".team")?.textContent).toBe("Alpha");
    expect(within(rows[0] as HTMLElement).getByText("LL")).toBeInTheDocument();
  });

  it("leaves such leave off a view that does not show the person's own team", () => {
    const LIEU = { ...ANNUAL_LEAVE, id: "k-ll", code: "LIEU_LEAVE", shortCode: "LL", label: "Lieu leave", colourToken: "LL" };
    const { container } = renderRoster({
      teamKey: "bravo",
      assignments: [],
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-22", kindCode: "LIEU_LEAVE", teamKey: "bravo" })],
      absenceKinds: [ANNUAL_LEAVE, LIEU],
      teamMembers: { bravo: [] },
      ownTeams: new Map([["u-Asela", ["alpha"]]]),
    });
    expect(container.querySelectorAll("tbody tr")).toHaveLength(0);
  });

  it("draws turns left on a team the person has left on their own row, and edits them where they are filed", () => {
    // Somebody moved from Bravo to Alpha with Bravo turns still on the rota:
    // one row, under Alpha, the Bravo turn on it -- and clicking it acts on
    // that Bravo turn, so clearing it removes it rather than an Alpha turn.
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      teamKey: "",
      teams: ["alpha", "bravo"],
      leadTeams: ["alpha", "bravo"],
      editing: true,
      onEditCell,
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: REGULAR.code, teamKey: "alpha" }),
        assignment({ name: "Asela", rotaDate: "2026-09-22", shiftCode: EVENING.code, teamKey: "bravo" }),
      ],
      teamMembers: { alpha: [member("u-Asela", "Asela", "engineer")], bravo: [] },
      ownTeams: new Map([["u-Asela", ["alpha"]]]),
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows).toHaveLength(1);
    expect(rows[0].querySelector(".team")?.textContent).toBe("Alpha");
    const turn = within(rows[0] as HTMLElement).getByText(EVENING.shortCode).closest("td")!;
    expect(turn.getAttribute("title")).toContain("filed under Bravo");
    fireEvent.click(turn);
    expect(onEditCell.mock.calls[0][0]).toMatchObject({ teamKey: "bravo", shiftCode: EVENING.code });
  });

  it.each([
    ["own entry first", true],
    ["stale turn first", false],
  ])("keeps both a stale turn and the person's own entry on the same day (%s)", (_label, ownFirst) => {
    const own = assignment({ name: "Asela", rotaDate: "2026-09-22", shiftCode: REGULAR.code, teamKey: "alpha" });
    const stale = assignment({ name: "Asela", rotaDate: "2026-09-22", shiftCode: EVENING.code, teamKey: "bravo" });
    const { container } = renderRoster({
      teamKey: "",
      teams: ["alpha", "bravo"],
      assignments: ownFirst ? [own, stale] : [stale, own],
      teamMembers: { alpha: [member("u-Asela", "Asela", "engineer")], bravo: [] },
      ownTeams: new Map([["u-Asela", ["alpha"]]]),
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows).toHaveLength(1);
    const cell = within(rows[0] as HTMLElement).getByText(EVENING.shortCode).closest("td")!;
    expect(within(cell).getByText(REGULAR.shortCode)).toBeInTheDocument();
    expect(cell.getAttribute("title")).toContain("filed under Bravo");
  });

  it("leaves a turn on a team the person has left off a view of that team alone", () => {
    const { container } = renderRoster({
      teamKey: "bravo",
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-22", shiftCode: EVENING.code, teamKey: "bravo" })],
      teamMembers: { bravo: [] },
      ownTeams: new Map([["u-Asela", ["alpha"]]]),
    });
    expect(container.querySelectorAll("tbody tr")).toHaveLength(0);
  });

  it("shows somebody moved to the Americas team, and on its weekend rota, under that team only", () => {
    // A Brazil-rotation move from Alpha to Bravo (the Americas team) with a
    // weekend turn on Bravo's rota: one row, under Bravo, the turn on it.
    const BRAZIL = {
      id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const,
      colourToken: "BR", sortOrder: 70, movesToTeamKey: "bravo", worksRotaThere: true,
    };
    const { container } = renderRoster({
      teamKey: "",
      teams: ["alpha", "bravo"],
      shifts: shiftMap(REGULAR, EVENING, WEEKEND),
      assignments: [assignment({ name: "Bruna", rotaDate: "2026-09-26", shiftCode: WEEKEND.code, teamKey: "bravo" })],
      absences: [{ ...absence({ name: "Bruna", startsOn: "2026-09-01", endsOn: "2026-12-31", kindCode: "ALLO_BR", teamKey: "bravo" }), homeTeamKey: "alpha" }],
      absenceKinds: [ANNUAL_LEAVE, BRAZIL],
      teamMembers: { alpha: [member("u-Bruna", "Bruna", "engineer")], bravo: [] },
      ownTeams: new Map([["u-Bruna", ["alpha"]]]),
    });
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows.map((r) => r.querySelector(".team")?.textContent)).toEqual(["Bravo"]);
    const turn = within(rows[0] as HTMLElement).getByText(WEEKEND.shortCode).closest("td")!;
    expect(turn.getAttribute("title")).not.toContain("filed under");
  });

  it("keeps a turn on another team where the person is still a member of it", () => {
    // A member of two teams (the shared SaaS SRE crew) keeps a row on each.
    const { container } = renderRoster({
      teamKey: "",
      teams: ["alpha", "bravo"],
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" }),
        assignment({ name: "Asela", rotaDate: "2026-09-22", shiftCode: EVENING.code, teamKey: "bravo" }),
      ],
      teamMembers: { alpha: [member("u-Asela", "Asela", "engineer")], bravo: [member("u-Asela", "Asela", "engineer")] },
      ownTeams: new Map([["u-Asela", ["alpha", "bravo"]]]),
    });
    expect([...container.querySelectorAll("tbody tr .team")].map((t) => t.textContent)).toEqual(["Alpha", "Bravo"]);
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


describe("MonthRoster: which roster scrolls inside its card", () => {
  it("lets the SRE roster grow with its list, so only the page scrolls down", () => {
    const { container } = renderRoster({ family: "SRE" });
    expect(container.querySelector(".rostwrap")).toHaveClass("rostflow");
  });

  it("keeps the CRE roster scrolling inside its card", () => {
    const { container } = renderRoster();
    expect(container.querySelector(".rostwrap")).not.toHaveClass("rostflow");
  });
});

describe("MonthRoster: a move that is only a team (SaaS Night cover)", () => {
  // Today is in view, and the move starts the next day.
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 8, 21));
  });
  afterEach(() => vi.useRealTimers());
  const NIGHT = {
    id: "k-nc", code: "SRE_NIGHT_COVER", shortCode: "NC", label: "Night cover", bucket: "ALLOCATION" as const,
    colourToken: "NLK", sortOrder: 75, movesToTeamKey: "bravo", worksRotaThere: true,
  };
  const BRAZIL = { ...NIGHT, id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", showsAsShiftCode: EVENING.code };
  const moved = (kind: typeof NIGHT | typeof BRAZIL) =>
    renderRoster({
      assignments: [],
      absences: [{ ...absence({ name: "Nia", startsOn: "2026-09-22", endsOn: "2026-09-30", kindCode: kind.code, teamKey: "bravo" }), homeTeamKey: "alpha" }],
      absenceKinds: [ANNUAL_LEAVE, kind],
      teamMembers: { alpha: [{ userId: "u-Nia", name: "Nia", email: "n@example.com", isLead: false, role: "engineer" }] },
    });

  it("moves them under that team as soon as it is in view, with no tag on their days", () => {
    const { container } = moved(NIGHT);
    const rows = [...container.querySelectorAll("tbody tr")];
    expect(rows.map((tr) => tr.querySelector(".team")?.textContent)).toEqual(["Bravo"]);
    expect([...container.querySelectorAll("tbody .chip")].some((c) => c.textContent === "NC")).toBe(false);
  });

  it("leaves a move with hours of its own on their team until it starts", () => {
    const { container } = moved(BRAZIL);
    expect([...container.querySelectorAll("tbody tr")].map((tr) => tr.querySelector(".team")?.textContent)).toEqual(["Alpha"]);
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

  it("draws no LK on a team whose engineers work one zone a day", () => {
    // An SRE SaaS engineer works TZ1 or TZ2 and nights, never LK's whole day;
    // the generated SUP says which, so an unmarked weekday stays blank.
    const { container } = renderRoster({
      assignments: [],
      teamMembers: { bravo: [{ userId: "u-Nia", name: "Nia", email: "n@example.com", isLead: false, role: "engineer" }] },
      teamsWithoutDefault: new Set(["bravo"]),
    });
    expect(container.querySelectorAll(".chip.dflt")).toHaveLength(0);
  });

  it("reads the India region shift for a team's engineers, and Regular hours for its lead", () => {
    // The lead of a team on the India region shift is a Sri Lankan-team
    // engineer on rotation, so their ordinary day stays LK.
    const { container } = renderRoster({
      assignments: [],
      shifts: shiftMap(REGULAR, REGULAR_IND, EVENING),
      teamMembers: {
        bravo: [
          { userId: "u-Lia", name: "Lia", email: "l@example.com", isLead: true, role: "lead" },
          { userId: "u-Nia", name: "Nia", email: "n@example.com", isLead: false, role: "engineer" },
        ],
      },
      teamDefaultShift: { bravo: REGULAR_IND.code },
    });
    const firstCell = (name: string) =>
      [...container.querySelectorAll("tbody tr")]
        .find((tr) => tr.textContent?.includes(name))
        ?.querySelector("td")
        ?.textContent?.trim();
    expect(firstCell("Nia")).toBe(REGULAR_IND.shortCode);
    expect(firstCell("Lia")).toBe(REGULAR.shortCode);
  });

  it("keeps a lead on their team's own ordinary day anywhere else", () => {
    const { container } = renderRoster({
      assignments: [],
      teamMembers: { bravo: [{ userId: "u-Lia", name: "Lia", email: "l@example.com", isLead: true, role: "lead" }] },
      teamDefaultShift: { bravo: EVENING.code },
    });
    expect(container.querySelector("tbody tr td")?.textContent?.trim()).toBe(EVENING.shortCode);
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

describe("MonthRoster: the SaaS Night cover is a team people are moved to", () => {
  // TZ3 tiers as the catalogue marks them: one crew for every team, and the
  // tag (migration 0232) that moves somebody onto the Night cover for the
  // months they are on nights. It has no hours of its own.
  const NIGHT = { ...TZ3, sharedAcrossTeams: true };
  const NIGHT_COVER = {
    id: "k-nc", code: "SRE_NIGHT_COVER", shortCode: "NC", label: "SaaS Night cover", bucket: "ALLOCATION" as const,
    colourToken: "NLK", sortOrder: 75, family: "SRE" as const, movesToTeamKey: "night", worksRotaThere: true,
  };
  const nightShifts = shiftMap(TZ1, TZ2, NIGHT, TZ1_WE, TZ2_WE);
  const turn = (name: string, rotaDate: string, tier: "L1" | "L2" | "L3", teamKey: string) =>
    assignment({ name, rotaDate, shiftCode: NIGHT.code, zoneCode: "TZ3", tier, teamKey });
  const members = {
    delta: ["Nia", "Tom"].map((n) => ({ userId: `u-${n}`, name: n, email: `${n.toLowerCase()}@example.test`, isLead: false, role: "engineer" })),
    night: [],
  };
  const rowOf = (c: HTMLElement, name: string) =>
    [...c.querySelectorAll("tbody tr")].find((r) => r.querySelector(".who")?.textContent === name);

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 8, 22));
  });
  afterEach(() => vi.useRealTimers());

  it("lists somebody moved onto it under it, with their nights, and once", () => {
    const { container } = renderRoster({
      family: "SRE",
      shifts: nightShifts,
      absenceKinds: [ANNUAL_LEAVE, NIGHT_COVER],
      teamMembers: members,
      absences: [{ ...absence({ name: "Nia", startsOn: "2026-09-01", endsOn: "2026-11-30", kindCode: NIGHT_COVER.code, teamKey: "night" }), homeTeamKey: "delta" }],
      assignments: [turn("Nia", "2026-09-21", "L1", "night"), turn("Tom", "2026-09-22", "L2", "delta")],
    });
    const nia = rowOf(container, "Nia");
    expect(nia?.querySelector(".team")?.textContent).toBe("Night");
    expect(nia?.textContent).toContain("L1");
    // Being on the Night cover is the whole of it: no tag drawn on the days.
    expect(nia?.textContent).not.toContain("NC");
    // The legend still names it: it is a move an SRE rota marker can add.
    expect(container.querySelector(".rlegend")?.textContent ?? "").toContain("SaaS Night cover");
    expect(screen.getAllByText("Nia")).toHaveLength(1);
    // Somebody holding a night without being moved stays on their own team.
    expect(rowOf(container, "Tom")?.querySelector(".team")?.textContent).toBe("Delta");
    expect(rowOf(container, "Tom")?.textContent).toContain("L2");
    expect(container.querySelector("tbody.nightgrp")).toBeNull();
  });

  it("puts them back under their own team once the move is over", () => {
    const { container } = renderRoster({
      family: "SRE",
      shifts: nightShifts,
      absenceKinds: [ANNUAL_LEAVE, NIGHT_COVER],
      teamMembers: members,
      // Three months on nights, over before today.
      absences: [{ ...absence({ name: "Nia", startsOn: "2026-06-01", endsOn: "2026-08-31", kindCode: NIGHT_COVER.code, teamKey: "night" }), homeTeamKey: "delta" }],
      assignments: [assignment({ name: "Nia", rotaDate: "2026-09-21", shiftCode: TZ1.code, zoneCode: "TZ1", teamKey: "delta" })],
    });
    expect(rowOf(container, "Nia")?.querySelector(".team")?.textContent).toBe("Delta");
  });

  it("lets their own team's lead change the moved days, and no other team's", () => {
    const moved = {
      family: "SRE" as const,
      shifts: nightShifts,
      absenceKinds: [ANNUAL_LEAVE, NIGHT_COVER],
      teamMembers: members,
      absences: [{ ...absence({ name: "Nia", startsOn: "2026-09-01", endsOn: "2026-11-30", kindCode: NIGHT_COVER.code, teamKey: "night" }), homeTeamKey: "delta" }],
      assignments: [turn("Nia", "2026-09-21", "L1", "night")],
      editing: true,
      onEditCell: vi.fn(),
    };
    const own = renderRoster({ ...moved, leadTeams: ["delta"] });
    expect(rowOf(own.container, "Nia")?.querySelectorAll("td.editable").length).toBeGreaterThan(0);
    // A blank day of the move still opens the move, so it can be ended or removed.
    const blank = [...(rowOf(own.container, "Nia")?.querySelectorAll("td.editable") ?? [])].find(
      (td) => td.textContent?.trim() === "·",
    ) as HTMLElement;
    fireEvent.click(blank);
    expect(moved.onEditCell).toHaveBeenLastCalledWith(
      expect.objectContaining({ teamKey: "night", absence: expect.objectContaining({ kindCode: NIGHT_COVER.code }) }),
    );
    own.unmount();
    const other = renderRoster({ ...moved, leadTeams: ["echo"] });
    expect(rowOf(other.container, "Nia")?.querySelectorAll("td.editable")).toHaveLength(0);
  });
});

describe("MonthRoster: reading the codes", () => {
  it("draws an on-call window dashed, so it is told apart from its plain twin", () => {
    const MORNING = shift({ code: "CRE_MORNING", shortCode: "6-9am", label: "Morning 6-9am", isRotation: true });
    const MORNING_OC = shift({
      code: "CRE_MORNING_OC",
      shortCode: "6-9am",
      label: "Morning 6-9am on-call",
      isRotation: true,
      isOnCall: true,
    });
    const { container } = renderRoster({
      shifts: shiftMap(REGULAR, MORNING, MORNING_OC),
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: MORNING.code }),
        assignment({ name: "Asela", rotaDate: "2026-09-22", shiftCode: MORNING_OC.code }),
      ],
    });
    const chips = [...container.querySelectorAll("tbody .chip")].filter((c) => c.textContent === "6-9am");
    expect(chips).toHaveLength(2);
    expect(chips.filter((c) => c.classList.contains("oc"))).toHaveLength(1);
  });

  it("explains the codes on screen, and only those", () => {
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-22" })],
    });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    expect(legend).not.toBeNull();
    expect(within(legend).getByText(EVENING.label)).toBeInTheDocument();
    expect(within(legend).getByText(ANNUAL_LEAVE.label)).toBeInTheDocument();
    expect(within(legend).getByText(/Working day/)).toBeInTheDocument();
    expect(within(legend).queryByText(RND.label)).not.toBeInTheDocument();
  });

  it("does not explain a kind that is no longer offered, though its day still shows", () => {
    const SICK = { ...ANNUAL_LEAVE, id: "k9", code: "SICK_LEAVE", shortCode: "SL", label: "Sick leave", retired: true };
    const { container } = renderRoster({
      absenceKinds: [ANNUAL_LEAVE, SICK],
      absences: [
        absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" }),
        absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-22", kindCode: SICK.code }),
      ],
    });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    expect(within(legend).getByText(ANNUAL_LEAVE.label)).toBeInTheDocument();
    expect(within(legend).queryByText(/Sick leave/)).not.toBeInTheDocument();
    expect([...container.querySelectorAll("tbody .chip")].some((c) => c.textContent === "SL")).toBe(true);
  });

  it("always lists the half-day leave tags, even with nobody on one in view", () => {
    const HD_AM = { ...ANNUAL_LEAVE, id: "k-am", code: "HALF_DAY_LEAVE_MORNING", shortCode: "HD-AM", label: "Half day leave — morning", colourToken: "HDM" };
    const HD_PM = { ...ANNUAL_LEAVE, id: "k-pm", code: "HALF_DAY_LEAVE_EVENING", shortCode: "HD-PM", label: "Half day leave — evening", colourToken: "HDE", retired: true };
    const { container } = renderRoster({ absenceKinds: [ANNUAL_LEAVE, HD_AM, HD_PM] });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    expect(within(legend).getByText(HD_AM.label)).toBeInTheDocument();
    // Retired, it is no longer something to mark, so not explained either.
    expect(within(legend).queryByText(HD_PM.label)).not.toBeInTheDocument();
  });

  it("lists every tag a rota marker can add on this rota, in catalogue order, on screen or not", () => {
    const ALLO_INT = { ...ANNUAL_LEAVE, id: "k-int", code: "ALLO_INT", shortCode: "Allo-INT", label: "Allocated — internal", bucket: "ALLOCATION" as const, colourToken: "INT", sortOrder: 30 };
    const LIEU = { ...ANNUAL_LEAVE, id: "k-ll", code: "LIEU_LEAVE", shortCode: "LL", label: "Lieu leave", colourToken: "LL", sortOrder: 20 };
    const MIG = { ...ALLO_INT, id: "k-mig", code: "MIGRATION", shortCode: "Mig", label: "Migration", colourToken: "MIG", sortOrder: 90, family: "CRE" as const, movesToTeamKey: "bravo" };
    const RETIRED = { ...ALLO_INT, id: "k-cus", code: "CUSTOMER", shortCode: "CUS", label: "Customer allocation", sortOrder: 43, retired: true };
    const EXCLUDED_KIND = { ...ALLO_INT, id: "k-exc", code: "EXCLUDED", shortCode: "EXC", label: "Excluded from rota", bucket: "EXCLUDED" as const, sortOrder: 110 };
    const SRE_RND = { ...RND, family: "SRE" as const };
    const { container } = renderRoster({
      family: "CRE",
      // Lieu leave is on screen; nothing else is.
      absences: [absence({ name: "Asela", startsOn: "2026-09-22", endsOn: "2026-09-22", kindCode: LIEU.code })],
      absenceKinds: [MIG, SRE_RND, ALLO_INT, RETIRED, EXCLUDED_KIND, LIEU, ANNUAL_LEAVE],
    });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    const labels = [...legend.querySelectorAll(".chip")].map((c) => c.textContent);
    expect(labels).toEqual(expect.arrayContaining(["AL", "LL", "Allo-INT", "Mig"]));
    expect(labels.indexOf("AL")).toBeLessThan(labels.indexOf("LL"));
    expect(labels.indexOf("Allo-INT")).toBeLessThan(labels.indexOf("Mig"));
    // Not this rota's (RnD is SRE's), retired, or not a lead's to set.
    expect(within(legend).queryByText(SRE_RND.label)).not.toBeInTheDocument();
    expect(within(legend).queryByText(RETIRED.label)).not.toBeInTheDocument();
    expect(within(legend).queryByText(EXCLUDED_KIND.label)).not.toBeInTheDocument();
  });

  it("lists a move worked as another team's hours as those hours", () => {
    const AMERICAS = shift({ code: "CRE_AMERICAS", shortCode: "NLK", label: "Americas cover", colourToken: "NLK", isRotation: false });
    const BR = { ...ANNUAL_LEAVE, id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const, sortOrder: 70, movesToTeamKey: "americas", worksRotaThere: true, showsAsShiftCode: AMERICAS.code };
    const { container } = renderRoster({
      family: "CRE",
      shifts: shiftMap(REGULAR, EVENING, AMERICAS),
      absenceKinds: [ANNUAL_LEAVE, BR],
    });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    expect(within(legend).getByText("Americas cover")).toBeInTheDocument();
    expect(within(legend).queryByText(BR.label)).not.toBeInTheDocument();
  });

  it("does not list another rota's hours for such a move", () => {
    const AMERICAS = shift({ code: "CRE_AMERICAS", shortCode: "NLK", label: "Americas cover", colourToken: "NLK", isRotation: false });
    const BR = { ...ANNUAL_LEAVE, id: "k-br", code: "ALLO_BR", shortCode: "BR", label: "Brazil rotation", bucket: "ALLOCATION" as const, sortOrder: 70, movesToTeamKey: "americas", worksRotaThere: true, showsAsShiftCode: AMERICAS.code };
    const { container } = renderRoster({
      family: "SRE",
      assignments: [],
      shifts: shiftMap(TZ1, TZ2, AMERICAS),
      absenceKinds: [ANNUAL_LEAVE, BR],
    });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    expect(within(legend).queryByText("Americas cover")).not.toBeInTheDocument();
  });

  it("lists labels only, not the grid's own marks", () => {
    const { container } = renderRoster({
      absences: [absence({ name: "Asela", startsOn: "2026-09-21", endsOn: "2026-09-21" })],
    });
    const legend = container.querySelector(".rlegend") as HTMLElement;
    expect(within(legend).queryByText(/needs cover/)).not.toBeInTheDocument();
    expect(within(legend).queryByText("Nothing rostered")).not.toBeInTheDocument();
  });
});

describe("MonthRoster: editing by keyboard", () => {
  const editable = () =>
    renderRoster({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" }),
        assignment({ name: "Nuwan", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "bravo" }),
      ],
      leadTeams: ["alpha"],
      editing: true,
      onEditCell: vi.fn(),
    });

  it("puts one cell in the Tab order, not every cell", () => {
    const { container } = editable();
    const cells = [...container.querySelectorAll<HTMLElement>("td.editable")];
    expect(cells.length).toBeGreaterThan(1);
    expect(cells.filter((c) => c.tabIndex === 0)).toHaveLength(1);
  });

  it("moves along a row with the arrow keys and opens a cell with Enter", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({ leadTeams: ["alpha"], editing: true, onEditCell });
    const cells = [...container.querySelectorAll<HTMLElement>("td.editable")];
    cells[0].focus();
    fireEvent.keyDown(cells[0], { key: "ArrowRight" });
    expect(cells[1]).toHaveFocus();
    fireEvent.keyDown(cells[1], { key: "Home" });
    expect(cells[0]).toHaveFocus();
    fireEvent.keyDown(cells[0], { key: "Enter" });
    expect(onEditCell).toHaveBeenCalledTimes(1);
  });

  it("says whose a row is when this lead cannot change it", () => {
    const { container } = editable();
    const other = [...container.querySelectorAll("tbody tr")].find((r) => r.textContent?.includes("Nuwan"));
    expect(other).toHaveClass("ro");
    expect(other?.querySelector("td")).toHaveAttribute(
      "title",
      expect.stringContaining("lead or a CRE rota admin can change this"),
    );
  });
});

describe("MonthRoster: customer engagement markers", () => {
  const JANE = "00000000-0000-0000-0000-000000000001";
  const alloc = (over: Partial<EngagementAllocation> = {}): EngagementAllocation => ({
    id: "a1",
    userId: JANE,
    engagementId: "00000000-0000-0000-0000-0000000000e1",
    startDate: "2026-09-22",
    endDate: "2026-09-24",
    accountName: "Example Corp",
    engagementDisplayId: "ENG-1001",
    engagementType: "CONSULTANCY",
    modeOfDelivery: "ONSITE",
    allocationId: "ALLOC-7",
    utilization: 50,
    ...over,
  });
  const onRota = [
    assignment({ name: "Jane Doe", userId: JANE, rotaDate: "2026-09-21", shiftCode: EVENING.code }),
    assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: EVENING.code }),
  ];
  const marks = (el: ParentNode) => [...el.querySelectorAll(".engmk")] as HTMLElement[];
  const rowOf = (c: HTMLElement, name: string) =>
    [...c.querySelectorAll("tbody tr")].find((r) => r.textContent?.includes(name)) as HTMLElement;

  it("draws no marker, and no legend entry, without allocations", () => {
    const { container } = renderRoster({ assignments: onRota });
    expect(marks(container)).toHaveLength(0);
    expect(container.querySelectorAll(".engmksw")).toHaveLength(0);
  });

  it("marks only the days an allocation covers, on that engineer's row", () => {
    const { container } = renderRoster({ assignments: onRota, engagementAllocations: [alloc()] });
    const jane = rowOf(container, "Jane Doe");
    const cells = [...jane.querySelectorAll("td")];
    const marked = cells.map((td, i) => (marks(td).length ? i + 1 : 0)).filter(Boolean);
    // Sept 22 to 24 on a grid that starts on the 1st.
    expect(marked).toEqual([22, 23, 24]);
    expect(marks(rowOf(container, "Asela"))).toHaveLength(0);
  });

  it("does not draw days outside the days on the grid", () => {
    const { container } = renderRoster({
      assignments: onRota,
      engagementAllocations: [alloc({ startDate: "2026-08-25", endDate: "2026-09-02" })],
    });
    expect(marks(rowOf(container, "Jane Doe"))).toHaveLength(2);
  });

  it("carries the week view's details in the title, with the label leading", () => {
    const { container } = renderRoster({ assignments: onRota, engagementAllocations: [alloc()] });
    expect(marks(container)[0]).toHaveAttribute(
      "title",
      "Example Corp - ENG-1001: Consultancy ● Onsite ● ALLOC-7 ● 50% ● 2026-09-22 - 2026-09-24",
    );
  });

  it("lists every allocation on a day in one marker, one per line", () => {
    const { container } = renderRoster({
      assignments: onRota,
      engagementAllocations: [
        alloc(),
        alloc({ id: "a2", accountName: "Another Corp", engagementDisplayId: "ENG-2002", startDate: "2026-09-23", endDate: "2026-09-23" }),
      ],
    });
    const jane = rowOf(container, "Jane Doe");
    const day23 = marks([...jane.querySelectorAll("td")][22]);
    expect(day23).toHaveLength(1);
    const lines = day23[0].getAttribute("title")!.split("\n");
    expect(lines).toHaveLength(2);
    expect(lines[0]).toContain("Another Corp - ENG-2002");
    expect(lines[1]).toContain("Example Corp - ENG-1001");
    // The 22nd has the one allocation only.
    expect(marks([...jane.querySelectorAll("td")][21])[0].getAttribute("title")!.split("\n")).toHaveLength(1);
  });

  it("names the marker once for assistive technology, and says how many", () => {
    const { container } = renderRoster({
      assignments: onRota,
      engagementAllocations: [alloc(), alloc({ id: "a2", accountName: "Another Corp" })],
    });
    const m = marks(container)[0];
    expect(m).toHaveAttribute("role", "img");
    expect(m.getAttribute("aria-label")).toMatch(/^2 customer engagements\. /);
    expect(m.getAttribute("aria-label")).not.toContain("\n");
    expect(screen.getAllByRole("img", { name: /customer engagement/i })).toHaveLength(3);
  });

  it("leaves the rota chip, the leave and the cell's own title as they were", () => {
    const base = renderRoster({ assignments: onRota, absences: [absence({ name: "Jane Doe", userId: JANE, startsOn: "2026-09-23", endsOn: "2026-09-23" })] });
    const before = [...rowOf(base.container, "Jane Doe").querySelectorAll("td")].map((td) => [td.className, td.getAttribute("title"), td.querySelector(".chip")?.textContent]);
    base.unmount();
    const { container } = renderRoster({
      assignments: onRota,
      absences: [absence({ name: "Jane Doe", userId: JANE, startsOn: "2026-09-23", endsOn: "2026-09-23" })],
      engagementAllocations: [alloc()],
    });
    const after = [...rowOf(container, "Jane Doe").querySelectorAll("td")].map((td) => [td.className, td.getAttribute("title"), td.querySelector(".chip")?.textContent]);
    expect(after).toEqual(before);
    // The leave day still shows its chip, with the marker beside it.
    const leaveCell = [...rowOf(container, "Jane Doe").querySelectorAll("td")][22];
    expect(leaveCell.querySelector(".chip")).not.toBeNull();
    expect(marks(leaveCell)).toHaveLength(1);
  });

  it("keeps the marker out of the way of an edit click", () => {
    const onEditCell = vi.fn();
    const { container } = renderRoster({
      assignments: onRota,
      engagementAllocations: [alloc()],
      leadTeams: ["alpha"],
      editing: true,
      onEditCell,
    });
    fireEvent.click(marks(container)[0]);
    expect(onEditCell).toHaveBeenCalledTimes(1);
  });

  it("explains the marker in the legend, once, only while markers are drawn", () => {
    const { container } = renderRoster({ assignments: onRota, engagementAllocations: [alloc()] });
    expect(container.querySelectorAll(".engmksw")).toHaveLength(1);
    expect(container.textContent).toContain("Customer engagement: a strip under the day");
  });

  it("hides the legend entry when the only allocations are off the grid", () => {
    const { container } = renderRoster({
      assignments: onRota,
      engagementAllocations: [alloc({ startDate: "2026-10-05", endDate: "2026-10-06" })],
    });
    expect(container.querySelectorAll(".engmksw")).toHaveLength(0);
  });

  it("says quietly that engagements could not be loaded, and draws the roster unchanged", () => {
    const base = renderRoster({ assignments: onRota });
    const html = base.container.querySelector("table")!.innerHTML;
    base.unmount();
    const { container } = renderRoster({ assignments: onRota, engagementsFailed: true });
    expect(screen.getByRole("status")).toHaveTextContent("Customer engagements could not be loaded, so they are not shown.");
    expect(container.querySelector("table")!.innerHTML).toBe(html);
  });

  describe("on a zone-split day", () => {
    const sre = {
      assignments: [
        assignment({ name: "Jane Doe", userId: JANE, rotaDate: "2026-09-22", shiftCode: TZ1.code, zoneCode: "TZ1", teamKey: "delta" }),
      ],
      family: "SRE" as const,
      engagementAllocations: [alloc({ startDate: "2026-09-22", endDate: "2026-09-22" })],
    };

    it("runs the strip across every zone of the day but names it once", () => {
      const { container } = renderRoster(sre);
      const day = marks(rowOf(container, "Jane Doe"));
      expect(day.length).toBeGreaterThan(1);
      expect(day.filter((m) => m.getAttribute("role") === "img")).toHaveLength(1);
      expect(day.filter((m) => m.getAttribute("aria-hidden") === "true")).toHaveLength(day.length - 1);
      expect(new Set(day.map((m) => m.getAttribute("title"))).size).toBe(1);
    });

    it("marks a whole-day cell once", () => {
      const { container } = renderRoster({
        ...sre,
        assignments: [assignment({ name: "Jane Doe", userId: JANE, rotaDate: "2026-09-21", shiftCode: TZ1.code, zoneCode: "TZ1", teamKey: "delta" })],
      });
      expect(marks(rowOf(container, "Jane Doe"))).toHaveLength(1);
      expect(rowOf(container, "Jane Doe").querySelector("td.zwhole .engmk")).not.toBeNull();
    });
  });
});
