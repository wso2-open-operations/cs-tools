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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import WeekTable, { type WeekEditControls } from "./WeekTable";
import {
  ANNUAL_LEAVE,
  EVENING,
  MONDAY,
  REGULAR,
  REGULAR_IND,
  TZ1,
  TZ1_L1,
  TZ2,
  TZ3,
  absence,
  shift,
  assignment,
  scopeControls,
  shiftMap,
} from "../test/fixtures";

const SHIFTS = shiftMap(REGULAR, REGULAR_IND, EVENING);

function renderWeek(over: Partial<React.ComponentProps<typeof WeekTable>> = {}) {
  return render(
    <WeekTable
      weekStart={MONDAY}
      assignments={[]}
      shifts={SHIFTS}
      absences={[]}
      absenceKinds={[ANNUAL_LEAVE]}
      {...scopeControls()}
      {...over}
    />,
  );
}

describe("WeekTable", () => {
  it("holds regular hours and the India region shift in one row", () => {
    // Both are 09:00-18:00 in the same zone on the same days: the code says
    // which team works it, not a different working day. The row lists teams,
    // so the second row said nothing this one's second-team entry does not.
    renderWeek({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: REGULAR.code, teamKey: "alpha" }),
        assignment({ name: "Akhil", rotaDate: "2026-09-21", shiftCode: REGULAR_IND.code, teamKey: "bravo" }),
      ],
    });
    expect(screen.getByText("Regular hours")).toBeInTheDocument();
    expect(screen.queryByText("India region shift")).not.toBeInTheDocument();
    // Both engineers sit under that one row. The India shift's person is
    // there as one more name on regular hours, not as a row of their own.
    expect(screen.getByText("Asela")).toBeInTheDocument();
    expect(screen.getByText("Akhil")).toBeInTheDocument();
  });

  it("keeps a rotation in a row of its own", () => {
    renderWeek({
      assignments: [
        assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: REGULAR.code }),
        assignment({ name: "Nuwan", rotaDate: "2026-09-21", shiftCode: EVENING.code }),
      ],
    });
    expect(screen.getByText("Regular hours")).toBeInTheDocument();
    expect(screen.getByText("Evening 6-9pm")).toBeInTheDocument();
  });

  it("counts people, not rows: one engineer on two rotations is one engineer", () => {
    const { container } = renderWeek({
      assignments: [
        assignment({ name: "Asela", userId: "u1", rotaDate: "2026-09-21", shiftCode: REGULAR.code }),
        assignment({ name: "Asela", userId: "u1", rotaDate: "2026-09-22", shiftCode: EVENING.code }),
      ],
    });
    expect(container.querySelector(".card-head .count")).toHaveTextContent("1");
  });

  it("shows who is away over the week", () => {
    renderWeek({
      assignments: [assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: REGULAR.code })],
      absences: [absence({ name: "Nuwan", startsOn: "2026-09-22", endsOn: "2026-09-23" })],
    });
    // Once per day of the span, which is the point of a week grid.
    expect(screen.getAllByText("Nuwan").length).toBeGreaterThan(0);
  });
});

describe("WeekTable: SRE escalation, a row per zone and tier", () => {
  const SRE = shiftMap(TZ1, TZ1_L1, TZ2, TZ3);
  const tiered = (name: string, shiftCode: string, zoneCode: string, tier: "L1" | "L2" | "L3") => ({
    ...assignment({ name, rotaDate: "2026-09-21", shiftCode, zoneCode }),
    tier,
  });

  it("gives every zone an L1, L2 and L3 row, in that order", () => {
    const { container } = renderWeek({
      family: "SRE",
      shifts: SRE,
      assignments: [tiered("Jane", TZ1_L1.code, "TZ1", "L1"), tiered("John", TZ1.code, "TZ1", "L2")],
    } as never);
    const labels = [...container.querySelectorAll("tbody th.lab small")].map((el) => el.textContent);
    expect(labels).toEqual([
      "TZ1 L1 support", "TZ1 L2 support", "TZ1 L3 support",
      "TZ2 L1 support", "TZ2 L2 support", "TZ2 L3 support",
      "TZ3 L1 support", "TZ3 L2 support", "TZ3 L3 support",
    ]);
  });

  it("files each engineer under their own tier, whichever window holds it", () => {
    const { container } = renderWeek({
      family: "SRE",
      shifts: SRE,
      assignments: [tiered("Jane", TZ1_L1.code, "TZ1", "L1"), tiered("John", TZ1.code, "TZ1", "L2"), tiered("Ada", TZ2.code, "TZ2", "L3")],
    } as never);
    const rowOf = (label: string) =>
      [...container.querySelectorAll("tbody tr")].find((tr) => tr.querySelector("th.lab small")?.textContent === label);
    expect(rowOf("TZ1 L1 support")).toHaveTextContent("Jane");
    expect(rowOf("TZ1 L2 support")).toHaveTextContent("John");
    expect(rowOf("TZ2 L3 support")).toHaveTextContent("Ada");
    // The row already says the tier, so no badge repeats it.
    expect(rowOf("TZ1 L2 support")?.querySelector(".tier-t")).toBeNull();
  });
});

describe("WeekTable: no tier on a zone's escalation window", () => {
  it("reads as the zone's regular hours, not as a turn missing its tier", () => {
    const TZ3_REGULAR = shift({
      code: "SRE_TZ3_REGULAR",
      label: "TZ3 regular hours",
      family: "SRE",
      zoneCode: "TZ3",
      isRotation: false,
      startMinute: 1260,
      endMinute: 1800,
      sortOrder: 103,
    });
    const { container } = renderWeek({
      family: "SRE",
      shifts: shiftMap(TZ1, TZ1_L1, TZ2, TZ3, TZ3_REGULAR),
      assignments: [assignment({ name: "Isuri", rotaDate: "2026-09-21", shiftCode: TZ3.code, zoneCode: "TZ3" })],
    } as never);
    const rowOf = (label: string) =>
      [...container.querySelectorAll("tbody tr")].find((tr) => tr.querySelector("th.lab small")?.textContent === label);
    expect(rowOf("TZ3 regular hours")).toHaveTextContent("Isuri");
    expect(container.textContent).not.toContain("tier not set");
  });
});

describe("WeekTable: a rota admin editing the week", () => {
  const PEOPLE = [
    { userId: "u-jane", name: "Jane Doe", teamKey: "alpha" },
    { userId: "u-john", name: "John Roe", teamKey: "bravo" },
  ];

  function editControls(over: Partial<WeekEditControls> = {}): WeekEditControls {
    return { people: PEOPLE, onAdd: vi.fn(), onRemove: vi.fn(), busy: false, ...over };
  }

  const WEEK = [
    assignment({ name: "Asela", rotaDate: "2026-09-21", shiftCode: REGULAR.code, teamKey: "alpha" }),
    assignment({ name: "Jane Doe", userId: "u-jane", rotaDate: "2026-09-21", shiftCode: EVENING.code, teamKey: "alpha" }),
  ];

  it("offers nothing to change while not editing", () => {
    renderWeek({ assignments: WEEK });
    expect(screen.queryByRole("combobox", { name: /Add someone/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Take .* off/ })).not.toBeInTheDocument();
  });

  it("adds someone to a rotation on a day, offering only people not already on it", () => {
    const edit = editControls();
    renderWeek({ assignments: WEEK, edit });
    const monday = screen.getByRole("combobox", { name: "Add someone to Evening 6-9pm on Mon 21" });
    // Jane is already on Monday's evening turn: only John is offered.
    expect(within(monday).queryByRole("option", { name: /Jane Doe/ })).not.toBeInTheDocument();
    fireEvent.change(monday, { target: { value: "u-john" } });
    expect(edit.onAdd).toHaveBeenCalledWith(
      { shiftCode: EVENING.code, tier: undefined, label: "Evening 6-9pm" },
      "2026-09-21",
      PEOPLE[1],
    );
    // Tuesday is empty, so both are offered there.
    const tuesday = screen.getByRole("combobox", { name: "Add someone to Evening 6-9pm on Tue 22" });
    expect(within(tuesday).getAllByRole("option")).toHaveLength(3);
  });

  it("takes someone off the turn they hold in that cell", () => {
    const edit = editControls();
    renderWeek({ assignments: WEEK, edit });
    fireEvent.click(screen.getByRole("button", { name: "Take Jane Doe off Evening 6-9pm on Mon 21" }));
    expect(edit.onRemove).toHaveBeenCalledWith(expect.objectContaining({ shiftCode: EVENING.code, rotaDate: "2026-09-21" }));
  });

  it("leaves regular hours and a weekday rotation's weekend alone", () => {
    renderWeek({ assignments: WEEK, edit: editControls() });
    expect(screen.queryByRole("combobox", { name: /Regular hours/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Take Asela off/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /Evening 6-9pm on (Sat|Sun)/ })).not.toBeInTheDocument();
    expect(screen.getAllByRole("combobox", { name: /Add someone to Evening 6-9pm/ })).toHaveLength(5);
  });

  it("holds the controls while a change is saving", () => {
    renderWeek({ assignments: WEEK, edit: editControls({ busy: true }) });
    expect(screen.getByRole("combobox", { name: "Add someone to Evening 6-9pm on Tue 22" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Take Jane Doe off Evening 6-9pm on Mon 21" })).toBeDisabled();
  });

  it("adds to an SRE tier row with that tier, on the tier's own window", () => {
    const edit = editControls();
    renderWeek({
      shifts: shiftMap(TZ1_L1),
      ...scopeControls(),
      family: "SRE",
      assignments: [assignment({ name: "Kasun", rotaDate: "2026-09-21", shiftCode: TZ1_L1.code, zoneCode: "TZ1", tier: "L1" })],
      edit,
    });
    const tue = screen.getByRole("combobox", { name: /Add someone to .* on Tue 22/ });
    fireEvent.change(tue, { target: { value: "u-jane" } });
    // The window carries its own tier, so none is sent with it.
    expect(edit.onAdd).toHaveBeenCalledWith(expect.objectContaining({ shiftCode: TZ1_L1.code, tier: undefined }), "2026-09-22", PEOPLE[0]);
  });
});
