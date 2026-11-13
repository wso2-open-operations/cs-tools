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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CellPicker, { type CellEngagement, type CellPickerTarget } from "./CellPicker";
import type { CellAbsence } from "../types";
import type { ScheduleAbsenceKind } from "../types";
import { ANNUAL_LEAVE, EVENING, LIEU_LEAVE, REGULAR, REGULAR_IND, RND, TZ1, TZ1_L1, TZ2, TZ3, WEEKEND, shift } from "../test/fixtures";

const ANCHOR = { top: 100, left: 100, bottom: 130, right: 144 };

function target(over: Partial<CellPickerTarget> = {}): CellPickerTarget {
  return {
    userId: "u1",
    name: "Asela",
    teamKey: "alpha",
    rotaDate: "2026-09-23", // a Wednesday
    anchor: ANCHOR,
    ...over,
  };
}

function renderPicker(over: {
  target?: CellPickerTarget;
  onApply?: () => void;
  onMarkAway?: () => void;
  onClear?: () => void;
} = {}) {
  const handlers = {
    onApply: over.onApply ?? vi.fn(),
    onMarkAway: over.onMarkAway ?? vi.fn(),
    onClear: over.onClear ?? vi.fn(),
    onClose: vi.fn(),
  };
  render(
    <CellPicker
      target={over.target ?? target()}
      shifts={[REGULAR, EVENING, WEEKEND]}
      awayKinds={[ANNUAL_LEAVE, LIEU_LEAVE, RND]}
      {...handlers}
    />,
  );
  return handlers;
}

describe("CellPicker: what it offers", () => {
  it("groups the rotations apart from the standing hours", () => {
    renderPicker();
    expect(screen.getByText("Rotations")).toBeInTheDocument();
    expect(screen.getByText("Standing hours")).toBeInTheDocument();
    expect(screen.getByText("Leave")).toBeInTheDocument();
    expect(screen.getByText("Allocations")).toBeInTheDocument();
  });

  it("disables a weekend rotation on a weekday, and says why", () => {
    // The rule is shown where it applies rather than left for a lead to infer
    // from a window's absence.
    renderPicker();
    const weekendOption = screen.getByRole("button", { name: /Weekend rotation/ });
    expect(weekendOption).toBeDisabled();
    expect(weekendOption).toHaveTextContent("weekends only");
  });

  it("enables that same rotation on a Saturday", () => {
    renderPicker({ target: target({ rotaDate: "2026-09-26" }) });
    expect(screen.getByRole("button", { name: /Weekend rotation/ })).toBeEnabled();
  });

  it("marks the window the engineer already holds", () => {
    const { container } = render(
      <CellPicker
        target={target({ shiftCode: EVENING.code })}
        shifts={[REGULAR, EVENING, WEEKEND]}
        awayKinds={[ANNUAL_LEAVE]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const on = container.querySelector(".pk-c.on");
    expect(on).toHaveTextContent("Evening 6-9pm");
  });

  it("offers leave with no weekday rule of its own", () => {
    // Leave is stored as a span, so a weekend inside it is covered too.
    renderPicker();
    expect(screen.getByRole("button", { name: /Annual leave/ })).toBeEnabled();
    expect(screen.getByRole("button", { name: /Lieu leave/ })).toBeEnabled();
  });
});

describe("CellPicker: the span it applies to", () => {
  it("starts closed on the day that was clicked", () => {
    renderPicker();
    expect(screen.getByText("1 day")).toBeInTheDocument();
  });

  it("counts the days as the end moves", () => {
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-25" } });
    expect(screen.getByText("3 days")).toBeInTheDocument();
  });

  it("treats an end before the start as no end chosen yet", () => {
    // A date input reports a whole date per segment, so the first digit of a
    // two-digit day is briefly an earlier date. Acting on it would apply a
    // backwards range.
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-02" } });
    expect(screen.getByText("1 day")).toBeInTheDocument();
  });

  it("treats a year the field will accept but no one meant as no end either", () => {
    // The year segment takes six digits, so "202609-02-09" is a value this
    // input genuinely hands over. A plain string comparison let it through
    // and the day count read "NaN days".
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "202609-02-09" } });
    expect(screen.getByText("1 day")).toBeInTheDocument();
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
  });

  it("applies the window over the whole span", () => {
    const { onApply } = renderPicker();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-25" } });
    fireEvent.click(screen.getByRole("button", { name: /Evening 6-9pm/ }));
    expect(onApply).toHaveBeenCalledWith(EVENING.code, "2026-09-23", "2026-09-25");
  });

  it("marks leave over the whole span", () => {
    const { onMarkAway } = renderPicker();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-25" } });
    fireEvent.click(screen.getByRole("button", { name: /Annual leave/ }));
    expect(onMarkAway).toHaveBeenCalledWith("ANNUAL_LEAVE", "2026-09-23", "2026-09-25", undefined);
  });

  it("warns that a span will skip the days a rotation is not worked on", () => {
    renderPicker();
    expect(screen.queryByText(/are skipped/)).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-29" } });
    expect(screen.getByText(/are skipped/)).toBeInTheDocument();
  });
});

describe("CellPicker: choosing the start as well as the end", () => {
  it("starts on the clicked day and lets the start move earlier", () => {
    // Marking a fortnight of leave from its middle should not mean finding
    // its first day on the grid first.
    const { onMarkAway } = renderPicker();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2026-09-21" } });
    expect(screen.getByText("3 days")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Annual leave/ }));
    expect(onMarkAway).toHaveBeenCalledWith("ANNUAL_LEAVE", "2026-09-21", "2026-09-23", undefined);
  });

  it("applies a rotation from the chosen start too", () => {
    const { onApply } = renderPicker();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2026-09-21" } });
    fireEvent.click(screen.getByRole("button", { name: /Evening 6-9pm/ }));
    expect(onApply).toHaveBeenCalledWith(EVENING.code, "2026-09-21", "2026-09-23");
  });

  it("keeps a start that is not a date yet on the clicked day", () => {
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "202609-02-09" } });
    expect(screen.getByText("1 day")).toBeInTheDocument();
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument();
  });

  it("takes the end along when the start moves past it", () => {
    renderPicker();
    const from = screen.getByLabelText("Mark from") as HTMLInputElement;
    fireEvent.change(from, { target: { value: "2026-09-28" } });
    fireEvent.blur(from);
    expect((screen.getByLabelText("Mark until") as HTMLInputElement).value).toBe("2026-09-28");
    expect(screen.getByText("1 day")).toBeInTheDocument();
  });

  it("blocks leave and allocations from a weekend start", () => {
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2026-09-26" } });
    expect(screen.getByRole("button", { name: /Annual leave/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: new RegExp(RND.label) })).toBeDisabled();
  });
});

describe("CellPicker: allocations", () => {
  it("sends who an allocation is for", () => {
    const { onMarkAway } = renderPicker();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-30" } });
    fireEvent.change(screen.getByLabelText("Allocated to"), { target: { value: "  Acme Corp " } });
    fireEvent.click(screen.getByRole("button", { name: new RegExp(RND.label) }));
    expect(onMarkAway).toHaveBeenCalledWith(RND.code, "2026-09-23", "2026-09-30", "Acme Corp");
  });

  it("leaves it out when nobody was named", () => {
    const { onMarkAway } = renderPicker();
    fireEvent.click(screen.getByRole("button", { name: new RegExp(RND.label) }));
    expect(onMarkAway).toHaveBeenCalledWith(RND.code, "2026-09-23", "2026-09-23", undefined);
  });

  it("never sends it with leave", () => {
    // Leave is not for anybody; a name typed for an allocation must not ride
    // along on a leave click.
    const { onMarkAway } = renderPicker();
    fireEvent.change(screen.getByLabelText("Allocated to"), { target: { value: "Acme Corp" } });
    fireEvent.click(screen.getByRole("button", { name: /Annual leave/ }));
    expect(onMarkAway).toHaveBeenCalledWith("ANNUAL_LEAVE", "2026-09-23", "2026-09-23", undefined);
  });
});

describe("CellPicker: what clearing means here", () => {
  it("puts a weekday back on the standing window the engineer already holds", () => {
    const { onClear } = renderPicker({
      target: target({ baseShiftCode: REGULAR.code }),
    });
    const clear = screen.getByRole("button", { name: /Clear/ });
    expect(clear).toHaveTextContent("Regular hours");
    fireEvent.click(clear);
    expect(onClear).toHaveBeenCalledWith(REGULAR.code, "2026-09-23", "2026-09-23");
  });

  it("empties a weekend, which has no standing window to fall back to", () => {
    const { onClear } = renderPicker({
      target: target({ rotaDate: "2026-09-26", baseShiftCode: REGULAR.code }),
    });
    fireEvent.click(screen.getByRole("button", { name: /Clear/ }));
    expect(onClear).toHaveBeenCalledWith("", "2026-09-26", "2026-09-26");
  });

  it("brings somebody marked away back on the rota instead", () => {
    // The rota underneath was covered, not deleted, so it shows through
    // again; rewriting the assignment would churn the history for a change
    // that did not happen.
    const { onClear } = renderPicker({
      target: target({ absenceKindCode: "ANNUAL_LEAVE", baseShiftCode: REGULAR.code }),
    });
    const clear = screen.getByRole("button", { name: /Clear/ });
    expect(clear).toHaveTextContent("back on the rota");
    fireEvent.click(clear);
    expect(onClear).toHaveBeenCalledWith(REGULAR.code, "2026-09-23", "2026-09-23");
  });
});

describe("CellPicker: removing a whole span", () => {
  const leave: CellAbsence = {
    id: "ab1",
    kindCode: "ANNUAL_LEAVE",
    startsOn: "2026-09-21",
    endsOn: "2026-09-25",
  };

  function renderWithAbsence(absence: CellAbsence, onRemoveAbsence = vi.fn()) {
    render(
      <CellPicker
        target={target({ absenceKindCode: absence.kindCode, absence })}
        shifts={[REGULAR, EVENING, WEEKEND]}
        awayKinds={[ANNUAL_LEAVE, LIEU_LEAVE, RND]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onRemoveAbsence={onRemoveAbsence}
        onClose={vi.fn()}
      />,
    );
    return onRemoveAbsence;
  }

  it("names the span and removes all of it in one click", () => {
    const onRemoveAbsence = renderWithAbsence(leave);
    expect(screen.getByText("21 Sept – 25 Sept")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(onRemoveAbsence).toHaveBeenCalledWith("ab1");
  });

  it("says when a span has no end", () => {
    const { endsOn: _drop, ...openEnded } = leave;
    void _drop;
    renderWithAbsence({ ...openEnded, kindCode: RND.code });
    expect(screen.getByText(/until further notice/)).toBeInTheDocument();
  });

  it("offers nothing to remove on a day nobody is away", () => {
    renderPicker({ target: target() });
    expect(screen.queryByRole("button", { name: "Remove" })).not.toBeInTheDocument();
  });
});

describe("CellPicker: adding a tag", () => {
  function renderWithCreate(onCreateKind: (k: unknown) => Promise<void>) {
    render(
      <CellPicker
        target={target()}
        shifts={[REGULAR]}
        awayKinds={[ANNUAL_LEAVE, RND]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onCreateKind={onCreateKind}
        onClose={vi.fn()}
      />,
    );
  }

  it("creates a tag with what the lead typed", async () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: " Trn " } });
    fireEvent.click(screen.getByRole("radio", { name: "Colour MIG" }));
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    // One name: it is both the chip the roster draws and the tag's name.
    expect(onCreateKind).toHaveBeenCalledWith({
      shortCode: "Trn",
      label: "Trn",
      bucket: "ALLOCATION",
      colourToken: "MIG",
      excludesFromTimesheet: false,
      description: "",
    });
    expect(await screen.findByRole("button", { name: "+ New tag" })).toBeInTheDocument();
  });

  it("asks the tag type and the timesheet as labelled dropdowns, an allocation kept on the timesheet by default", () => {
    renderWithCreate(vi.fn(() => Promise.resolve()));
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    expect(screen.getByRole("combobox", { name: "Tag type" })).toHaveValue("ALLOCATION");
    expect(screen.getByRole("combobox", { name: "Timesheet" })).toHaveValue("INCLUDE");
    expect(screen.getByText(/working on something else/)).toBeInTheDocument();
    expect(screen.getByText(/stay on the timesheet as usual/)).toBeInTheDocument();
  });

  it("creates an exclude-from-timesheet tag with its description", () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "SRE" } });
    fireEvent.change(screen.getByLabelText("Tag description"), { target: { value: "  On the SRE rota for the span  " } });
    fireEvent.change(screen.getByRole("combobox", { name: "Timesheet" }), { target: { value: "EXCLUDE" } });
    expect(screen.getByText(/left off the timesheet on those days/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).toHaveBeenCalledWith(
      expect.objectContaining({
        shortCode: "SRE",
        bucket: "ALLOCATION",
        excludesFromTimesheet: true,
        description: "On the SRE rota for the span",
      }),
    );
  });

  it("never sends a leave tag as excluded: leave shows on the timesheet as leave", () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByRole("combobox", { name: "Timesheet" }), { target: { value: "EXCLUDE" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Tag type" }), { target: { value: "LEAVE" } });
    const timesheet = screen.getByRole("combobox", { name: "Timesheet" });
    expect(timesheet).toBeDisabled();
    expect(timesheet).toHaveValue("LEAVE");
    expect(screen.getByText("Leave days show on the timesheet as leave.")).toBeInTheDocument();
    expect(screen.getByText(/the person is away/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "Vac" } });
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).toHaveBeenCalledWith(expect.objectContaining({ bucket: "LEAVE", excludesFromTimesheet: false }));
  });

  it("shows a custom tag's description beside its chip, never its name twice", () => {
    render(
      <CellPicker
        target={target()}
        shifts={[REGULAR]}
        awayKinds={[
          ANNUAL_LEAVE,
          { ...RND, code: "SRE", shortCode: "SRE", label: "SRE", custom: true, description: "On the SRE rota" },
        ]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const mark = screen.getByText("On the SRE rota").closest("button");
    expect(mark).not.toBeNull();
    expect(mark?.textContent).toBe("SREOn the SRE rota");
  });

  it("marks a tag that takes people off the timesheet in the list, and shows its description on hover", () => {
    render(
      <CellPicker
        target={target()}
        shifts={[REGULAR]}
        awayKinds={[
          ANNUAL_LEAVE,
          { ...RND, excludesFromTimesheet: true, description: "On the SRE rota" },
        ]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const mark = screen.getByText("Exclude from timesheet").closest("button");
    expect(mark).not.toBeNull();
    expect(mark?.getAttribute("title")).toContain("On the SRE rota");
    expect(mark?.getAttribute("title")).toContain("Exclude from timesheet");
  });

  it("says the new tag is for the roster's family only", () => {
    render(
      <CellPicker
        target={target()}
        shifts={[REGULAR]}
        awayKinds={[ANNUAL_LEAVE, RND]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onCreateKind={vi.fn(() => Promise.resolve())}
        tagFamily="CRE"
        onClose={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    expect(screen.getByText("For the CRE family only: only CRE rosters will offer this tag.")).toBeInTheDocument();
    expect(screen.queryByText("Shared with every team once added.")).not.toBeInTheDocument();
  });

  it("keeps Add tag off until a name is typed, and previews the tag as it will look", () => {
    renderWithCreate(vi.fn(() => Promise.resolve()));
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    const add = screen.getByRole("button", { name: "Add tag" });
    expect(add).toBeDisabled();
    const preview = screen.getByLabelText("Preview of the new tag");
    expect(preview).toHaveTextContent("Type a name to see the tag");
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "SRE" } });
    fireEvent.change(screen.getByLabelText("Tag description"), { target: { value: "SRE rotation" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Timesheet" }), { target: { value: "EXCLUDE" } });
    expect(add).toBeEnabled();
    expect(preview).toHaveTextContent("SRE");
    expect(preview).toHaveTextContent("SRE rotation");
    expect(preview).toHaveTextContent("Allocation · Exclude from timesheet");
    expect(screen.getByRole("group", { name: "What the roster shows" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "How it counts" })).toBeInTheDocument();
  });

  it("has no short code field: one name of at most 10 characters, explained in the description", () => {
    renderWithCreate(vi.fn(() => Promise.resolve()));
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    expect(screen.queryByLabelText("Tag short code")).not.toBeInTheDocument();
    expect(screen.getByText(/up to 10 characters\. Explain it in the description/)).toBeInTheDocument();
  });

  it("counts the name in characters as the server does, so an emoji counts once", () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    // Ten characters, twenty UTF-16 units: the server accepts it, so must the form.
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "🔥".repeat(10) } });
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).toHaveBeenCalled();
  });

  it("refuses a name longer than 10 characters without asking the server", () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "🔥".repeat(11) } });
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("at most 10 characters");
  });

  it("refuses a name another tag already draws, without asking the server", () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "al" } });
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("already a tag");
  });

  it("shows why the server refused it", async () => {
    renderWithCreate(() => Promise.reject(new Error("A tag with that name or short code already exists.")));
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "Xyz" } });
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(await screen.findByText(/already exists/)).toBeInTheDocument();
  });
});

describe("CellPicker: the SRE escalation grid", () => {
  function renderSre(over: Partial<CellPickerTarget> = {}, onApply = vi.fn()) {
    render(
      <CellPicker
        target={target({ zoneCode: "TZ1", ...over })}
        shifts={[TZ1, TZ1_L1, TZ2, TZ3]}
        awayKinds={[ANNUAL_LEAVE, RND]}
        onApply={onApply}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    return onApply;
  }

  it("offers L1, L2 and L3 for every zone, not just the one clicked", () => {
    renderSre();
    for (const zone of ["TZ1", "TZ2", "TZ3"]) {
      for (const tier of ["L1", "L2", "L3"]) {
        expect(screen.getByRole("button", { name: `${tier} for ${zone}` })).toBeEnabled();
      }
    }
    // The escalation windows are not listed a second time as rotations.
    expect(screen.queryByRole("button", { name: /TZ2 escalation/ })).not.toBeInTheDocument();
  });

  it("rosters L3 for another zone on that zone's open window", () => {
    const onApply = renderSre();
    fireEvent.click(screen.getByRole("button", { name: "L3 for TZ2" }));
    expect(onApply).toHaveBeenCalledWith("SRE_TZ2", "2026-09-23", "2026-09-23", "L3");
  });

  it("uses the window that fixes a tier without sending one", () => {
    const onApply = renderSre();
    fireEvent.click(screen.getByRole("button", { name: "L1 for TZ1" }));
    expect(onApply).toHaveBeenCalledWith("SRE_TZ1_L1", "2026-09-23", "2026-09-23", undefined);
  });

  it("marks the tier already held", () => {
    renderSre({ shiftCode: "SRE_TZ1", tier: "L2" });
    expect(screen.getByRole("button", { name: "L2 for TZ1" })).toHaveClass("on");
    expect(screen.getByRole("button", { name: "L1 for TZ1" })).not.toHaveClass("on");
  });

  it("clears the turn, not the allocation beside it", () => {
    // A cell holding L1 and RnD: the allocation has its own Remove, so Clear
    // is about the turn.
    renderSre({ shiftCode: "SRE_TZ1", tier: "L1", absenceKindCode: "RND", baseShiftCode: REGULAR.code });
    const clear = screen.getByRole("button", { name: /Clear/ });
    expect(clear).toHaveTextContent(/back to regular hours/i);
    expect(clear).not.toHaveTextContent("back on the rota");
  });
});

describe("CellPicker: a rota of Day and Night (IaaS)", () => {
  // IaaS's two windows: one per zone, every day, the tier left to the turn.
  const IAAS_DAY = shift({
    code: "SRE_IAAS_DAY", shortCode: "Day", label: "IaaS day escalation", family: "SRE",
    zoneCode: "IAAS_D", dayScope: "ANY", isEscalation: true, sortOrder: 210,
  });
  const IAAS_NIGHT = shift({
    code: "SRE_IAAS_NIGHT", shortCode: "Night", label: "IaaS night escalation", family: "SRE",
    zoneCode: "IAAS_N", dayScope: "ANY", isEscalation: true, sortOrder: 220,
  });

  function renderIaas(
    shifts: ReturnType<typeof shift>[],
    onApply = vi.fn(),
    moveKinds: ScheduleAbsenceKind[] = [],
  ) {
    render(
      <CellPicker
        // A Saturday: the rota is worked every day.
        target={target({ teamKey: "iaas", zoneCode: "IAAS_D", rotaDate: "2026-09-26" })}
        shifts={shifts}
        awayKinds={[ANNUAL_LEAVE, RND]}
        moveKinds={moveKinds}
        onApply={onApply}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    return onApply;
  }

  it("offers L1, L2 and L3 on Day and on Night, weekends included", () => {
    const onApply = renderIaas([IAAS_DAY, IAAS_NIGHT]);
    for (const tier of ["L1", "L2", "L3"]) {
      const both = screen.getAllByRole("button", { name: new RegExp(`^${tier} for `) });
      expect(both).toHaveLength(2);
      for (const b of both) expect(b).toBeEnabled();
    }
    fireEvent.click(screen.getAllByRole("button", { name: /^L2 for / })[1]);
    expect(onApply).toHaveBeenCalledWith("SRE_IAAS_NIGHT", "2026-09-26", "2026-09-26", "L2");
    expect(screen.queryByText(/no rota windows set up/)).not.toBeInTheDocument();
  });

  it("says so when the rota has no windows, rather than leaving the side blank", () => {
    // What an IaaS lead got while the windows were switched off: leave on the
    // right and nothing at all on the left, which read as "not allowed".
    renderIaas([]);
    expect(screen.getByText(/no rota windows set up yet/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^L1 for / })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Annual leave/ })).toBeInTheDocument();
  });

  it("still says so when a move to another team is on offer", () => {
    // The move buttons alone would fill the side and leave the missing L1-L3
    // unexplained.
    const toPaas: ScheduleAbsenceKind = {
      id: "k-paas", code: "TO_PAAS", shortCode: "PaaS", label: "PaaS", bucket: "ALLOCATION",
      colourToken: "MIG", sortOrder: 50, movesToTeamKey: "paas",
    };
    renderIaas([], vi.fn(), [toPaas]);
    expect(screen.getByText(/no rota windows set up yet/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Move to paas team/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^L1 for / })).not.toBeInTheDocument();
  });
});

describe("CellPicker: deleting a tag a lead added", () => {
  const CUSTOM = { ...RND, id: "k9", code: "TRAINING", shortCode: "Trn", label: "Training", custom: true };

  function renderWithDelete(onDeleteKind: (code: string) => Promise<void>) {
    render(
      <CellPicker
        target={target()}
        shifts={[REGULAR]}
        awayKinds={[ANNUAL_LEAVE, RND, CUSTOM]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onDeleteKind={onDeleteKind}
        onClose={vi.fn()}
      />,
    );
  }

  it("offers delete on a custom tag only, and asks before deleting", async () => {
    const onDeleteKind = vi.fn(() => Promise.resolve());
    renderWithDelete(onDeleteKind);
    expect(screen.queryByRole("button", { name: /Delete the R&D tag/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete the Training tag" }));
    expect(onDeleteKind).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Confirm deleting the Training tag" }));
    expect(onDeleteKind).toHaveBeenCalledWith("TRAINING");
  });

  it("says why a tag was not deleted", async () => {
    renderWithDelete(() => Promise.reject(new Error("That tag is still used on the rota.")));
    fireEvent.click(screen.getByRole("button", { name: "Delete the Training tag" }));
    fireEvent.click(screen.getByRole("button", { name: "Confirm deleting the Training tag" }));
    expect(await screen.findByText(/still used on the rota/)).toBeInTheDocument();
  });
});

describe("CellPicker: moving somebody back", () => {
  const MIG = {
    id: "k-mig", code: "MIGRATION", shortCode: "Mig", label: "Migration", bucket: "ALLOCATION" as const,
    colourToken: "MIG", sortOrder: 90, movesToTeamKey: "bravo",
  };
  const onMoved = (onMarkAway = vi.fn()) => {
    render(
      <CellPicker
        target={target({
          rotaDate: "2026-09-25",
          absenceKindCode: "MIGRATION",
          absence: { id: "ab-1", kindCode: "MIGRATION", startsOn: "2026-09-01", endsOn: "2026-09-30", homeTeamKey: "alpha" },
        })}
        shifts={[REGULAR, EVENING, WEEKEND]}
        awayKinds={[ANNUAL_LEAVE]}
        allKinds={[ANNUAL_LEAVE, MIG]}
        onApply={vi.fn()}
        onMarkAway={onMarkAway}
        onClear={vi.fn()}
        onRemoveAbsence={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    return onMarkAway;
  };

  it("ends the move from the day picked to the move's own end", () => {
    const onMarkAway = onMoved();
    fireEvent.click(screen.getByRole("button", { name: /^Back to .* from 25 Sept?$/ }));
    expect(onMarkAway).toHaveBeenCalledWith("", "2026-09-25", "2026-09-30");
  });

  it("is not offered from a day after the move has ended", () => {
    onMoved();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2026-10-02" } });
    expect(screen.queryByRole("button", { name: /^Back to/ })).not.toBeInTheDocument();
  });
});

describe("CellPicker: a dialog in the middle of the view", () => {
  it("opens as a modal dialog and takes focus", () => {
    renderPicker();
    const dialog = screen.getByRole("dialog", { name: "Change Asela's rota" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveFocus();
  });

  it("grows out of the cell that was clicked", () => {
    renderPicker();
    // jsdom lays nothing out, so the dialog sits at 0,0 and the origin is the
    // cell's own middle.
    expect(screen.getByRole("dialog")).toHaveStyle({ transformOrigin: "122px 115px" });
  });

  it("says what the day holds now", () => {
    renderPicker({ target: target({ shiftCode: EVENING.code }) });
    const now = screen.getByTitle("What this day holds now");
    expect(now).toHaveTextContent(EVENING.shortCode);
    expect(now).toHaveTextContent(EVENING.label);
  });

  it("closes from the backdrop, after fading out", () => {
    vi.useFakeTimers();
    try {
      const { onClose } = renderPicker();
      fireEvent.mouseDown(screen.getByRole("dialog"));
      expect(screen.getByRole("dialog").parentElement).not.toHaveClass("closing");
      fireEvent.mouseDown(screen.getByRole("dialog").parentElement as HTMLElement);
      expect(screen.getByRole("dialog").parentElement).toHaveClass("closing");
      expect(onClose).not.toHaveBeenCalled();
      act(() => {
        vi.advanceTimersByTime(200);
      });
      expect(onClose).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("closes on Escape once, however often it is pressed", () => {
    vi.useFakeTimers();
    try {
      const { onClose } = renderPicker();
      fireEvent.keyDown(window, { key: "Escape" });
      fireEvent.keyDown(window, { key: "Escape" });
      act(() => {
        vi.advanceTimersByTime(200);
      });
      expect(onClose).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps Tab inside the dialog", () => {
    renderPicker();
    const dialog = screen.getByRole("dialog");
    const close = screen.getByRole("button", { name: "Close" });
    const clear = screen.getByRole("button", { name: /Clear/ });
    clear.focus();
    fireEvent.keyDown(window, { key: "Tab" });
    expect(close).toHaveFocus();
    fireEvent.keyDown(window, { key: "Tab", shiftKey: true });
    expect(clear).toHaveFocus();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
  });

  it("gives focus back to where it was on close", () => {
    const before = document.createElement("button");
    document.body.appendChild(before);
    before.focus();
    const { unmount } = render(
      <CellPicker
        target={target()}
        shifts={[REGULAR]}
        awayKinds={[ANNUAL_LEAVE]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(before).not.toHaveFocus();
    unmount();
    expect(before).toHaveFocus();
    before.remove();
  });

  it("puts the dates with leave and allocations", () => {
    renderPicker();
    const away = screen.getByRole("region", { name: "Leave & allocations" });
    expect(within(away).getByLabelText("Mark from")).toBeInTheDocument();
    expect(within(away).getByLabelText("Mark until")).toBeInTheDocument();
    expect(within(away).getByRole("button", { name: /Annual leave/ })).toBeInTheDocument();
    const rota = screen.getByRole("region", { name: /^Rota/ });
    expect(within(rota).queryByLabelText("Mark from")).not.toBeInTheDocument();
    expect(within(rota).getByRole("button", { name: /Evening 6-9pm/ })).toBeInTheDocument();
  });

  it("says on the rota side which days a pick will set", () => {
    renderPicker();
    const rota = screen.getByRole("region", { name: /^Rota/ });
    expect(within(rota).getByText(/^Wed 23 Sept?$/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Mark until"), { target: { value: "2026-09-25" } });
    expect(within(rota).getByText(/23 Sept? – 25 Sept? · 3 days/)).toBeInTheDocument();
  });
});

describe("CellPicker: moving somebody to another team", () => {
  const AMERICAS_COVER = shift({
    code: "CRE_AMERICAS",
    shortCode: "NLK",
    label: "Americas cover",
    isRotation: false,
    startMinute: 1260,
    endMinute: 1800,
    sortOrder: 95,
  });
  const BRAZIL: ScheduleAbsenceKind = {
    id: "k-br",
    code: "ALLO_BR",
    shortCode: "BR",
    label: "Brazil rotation",
    bucket: "ALLOCATION",
    colourToken: "BR",
    sortOrder: 40,
    movesToTeamKey: "americas",
    worksRotaThere: true,
    showsAsShiftCode: "CRE_AMERICAS",
  };
  const MIGRATION: ScheduleAbsenceKind = {
    id: "k-mig",
    code: "MIGRATION",
    shortCode: "Mig",
    label: "Migration",
    bucket: "ALLOCATION",
    colourToken: "MIG",
    sortOrder: 41,
    movesToTeamKey: "migration",
  };

  function renderMoves(teamKey: string, onApply = vi.fn(), onMarkAway = vi.fn()) {
    render(
      <CellPicker
        target={target({ teamKey })}
        shifts={[REGULAR, EVENING, AMERICAS_COVER]}
        awayKinds={[ANNUAL_LEAVE]}
        allKinds={[ANNUAL_LEAVE, BRAZIL, MIGRATION]}
        moveKinds={[MIGRATION]}
        onApply={onApply}
        onMarkAway={onMarkAway}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    return { onApply, onMarkAway };
  }

  it("offers Americas cover as a move to the Americas team, not as standing hours", () => {
    const { onApply } = renderMoves("draco");
    expect(screen.queryByRole("button", { name: /Americas cover/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Move to Americas Team/ }));
    // The same pick the window was: the server writes it as the move.
    expect(onApply).toHaveBeenCalledWith("CRE_AMERICAS", "2026-09-23", "2026-09-23");
  });

  it("sets a move's months in one click: 3 months from the 23rd runs to the 22nd", () => {
    const { onApply } = renderMoves("draco");
    fireEvent.click(screen.getByRole("button", { name: "3 months" }));
    expect(screen.getByLabelText("Mark until")).toHaveValue("2026-12-22");
    fireEvent.click(screen.getByRole("button", { name: /Move to Americas Team/ }));
    expect(onApply).toHaveBeenCalledWith("CRE_AMERICAS", "2026-09-23", "2026-12-22");
    fireEvent.click(screen.getByRole("button", { name: "6 months" }));
    expect(screen.getByLabelText("Mark until")).toHaveValue("2027-03-22");
  });

  it("puts both moves with leave and allocations, under the dates, not on the rota", () => {
    renderMoves("draco");
    const away = screen.getByRole("region", { name: "Leave & allocations" });
    expect(within(away).getByRole("button", { name: /Move to Americas Team/ })).toBeInTheDocument();
    expect(within(away).getByRole("button", { name: /Move to Migration Team/ })).toBeInTheDocument();
    expect(within(away).getByText(/For the dates above/)).toBeInTheDocument();
    const rota = screen.getByRole("region", { name: /^Rota/ });
    expect(within(rota).queryByRole("button", { name: /Move to/ })).not.toBeInTheDocument();
  });

  it("names the Migration move as a team too", () => {
    const { onMarkAway } = renderMoves("draco");
    fireEvent.click(screen.getByRole("button", { name: /Move to Migration Team/ }));
    expect(onMarkAway).toHaveBeenCalledWith("MIGRATION", "2026-09-23", "2026-09-23");
  });

  it("sets a span of leave in one click, counting the first day", () => {
    const { onMarkAway } = renderPicker();
    const until = screen.getByLabelText("Mark until");
    fireEvent.click(screen.getByRole("button", { name: "1 week" }));
    expect(until).toHaveValue("2026-09-29");
    fireEvent.click(screen.getByRole("button", { name: "2 weeks" }));
    expect(until).toHaveValue("2026-10-06");
    fireEvent.click(screen.getByRole("button", { name: "2 months" }));
    expect(until).toHaveValue("2026-11-22");
    fireEvent.click(screen.getByRole("button", { name: "1 month" }));
    expect(until).toHaveValue("2026-10-22");
    // A length only fills "To": nothing is marked until a tag is picked.
    expect(onMarkAway).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /Annual leave/ }));
    expect(onMarkAway).toHaveBeenCalledWith("ANNUAL_LEAVE", "2026-09-23", "2026-10-22", undefined);
  });

  it("runs a month from a day the later month lacks to that month's end", () => {
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2027-01-31" } });
    const until = screen.getByLabelText("Mark until");
    fireEvent.click(screen.getByRole("button", { name: "1 month" }));
    expect(until).toHaveValue("2027-02-28");
    fireEvent.click(screen.getByRole("button", { name: "3 months" }));
    expect(until).toHaveValue("2027-04-30");
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2028-01-31" } });
    fireEvent.click(screen.getByRole("button", { name: "1 month" }));
    expect(until).toHaveValue("2028-02-29");
    // A day the later month has still runs to the day before it.
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2027-01-15" } });
    fireEvent.click(screen.getByRole("button", { name: "1 month" }));
    expect(until).toHaveValue("2027-02-14");
  });

  it("keeps Americas cover as standing hours for somebody already on Americas", () => {
    renderMoves("americas");
    expect(screen.getByRole("button", { name: /Americas cover/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Move to Americas Team/ })).not.toBeInTheDocument();
  });
});

describe("CellPicker: moving somebody to the India team", () => {
  const AEST = shift({ code: "CRE_REGULAR_AEST", shortCode: "AEST", label: "AEST regular hours", isRotation: false, sortOrder: 92 });
  const INDIA_MOVE: ScheduleAbsenceKind = {
    id: "k-ind",
    code: "ALLO_IND",
    shortCode: "IND",
    label: "India team rotation",
    bucket: "ALLOCATION",
    colourToken: "IND",
    sortOrder: 72,
    movesToTeamKey: "phoenix",
    worksRotaThere: true,
    showsAsShiftCode: "CRE_REGULAR_IND",
  };

  function renderFor(teamKey: string, onApply = vi.fn()) {
    render(
      <CellPicker
        target={target({ teamKey })}
        shifts={[REGULAR, REGULAR_IND, AEST, EVENING]}
        awayKinds={[ANNUAL_LEAVE]}
        allKinds={[ANNUAL_LEAVE, INDIA_MOVE]}
        moveKinds={[]}
        onApply={onApply}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    return { onApply };
  }

  it("offers the India region shift as a move to the India team, not as standing hours", () => {
    const { onApply } = renderFor("draco");
    const rota = screen.getByRole("region", { name: /^Rota/ });
    expect(within(rota).queryByRole("button", { name: /India region shift/ })).not.toBeInTheDocument();
    expect(within(rota).getByRole("button", { name: /Regular hours/ })).toBeInTheDocument();
    expect(within(rota).getByRole("button", { name: /AEST regular hours/ })).toBeInTheDocument();
    const away = screen.getByRole("region", { name: "Leave & allocations" });
    fireEvent.click(within(away).getByRole("button", { name: /Move to India Team \(Phoenix\)/ }));
    expect(onApply).toHaveBeenCalledWith("CRE_REGULAR_IND", "2026-09-23", "2026-09-23");
  });

  it("gives somebody already on the India team Regular hours only, and no move to it", () => {
    renderFor("phoenix");
    const rota = screen.getByRole("region", { name: /^Rota/ });
    expect(within(rota).getByRole("button", { name: /Regular hours/ })).toBeInTheDocument();
    expect(within(rota).queryByRole("button", { name: /India region shift/ })).not.toBeInTheDocument();
    expect(within(rota).queryByRole("button", { name: /AEST regular hours/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Move to India Team/ })).not.toBeInTheDocument();
  });
});

describe("CellPicker: customer engagements from the allocation app", () => {
  const live: CellEngagement = { id: "al1", label: "Allo-E08110", title: "Quartz Robotics - E08110: Firefighting", dismissed: false };
  const gone: CellEngagement = { id: "al2", label: "Allo-E07202", title: "Lumen Partners - E07202: Consultancy", dismissed: true };

  function renderWith(engagements: CellEngagement[], onDismissEngagement = vi.fn()) {
    render(
      <CellPicker
        target={target({ engagements })}
        shifts={[REGULAR, EVENING, WEEKEND]}
        awayKinds={[ANNUAL_LEAVE, LIEU_LEAVE, RND]}
        onApply={vi.fn()}
        onMarkAway={vi.fn()}
        onClear={vi.fn()}
        onDismissEngagement={onDismissEngagement}
        onClose={vi.fn()}
      />,
    );
    return onDismissEngagement;
  }

  it("offers to remove one on the roster for the picked day", () => {
    const onDismiss = renderWith([live]);
    const group = screen.getByRole("group", { name: /customer engagements from the allocation app/i });
    expect(within(group).getByText("Allo-E08110")).toBeInTheDocument();
    fireEvent.click(within(group).getByRole("button", { name: /^Remove/ }));
    expect(onDismiss).toHaveBeenCalledWith(live, "2026-09-23", "2026-09-23", false);
  });

  it("offers to restore one that was removed", () => {
    const onDismiss = renderWith([gone]);
    fireEvent.click(screen.getByRole("button", { name: /^Restore/ }));
    expect(onDismiss).toHaveBeenCalledWith(gone, "2026-09-23", "2026-09-23", true);
  });

  it("shows nothing without engagements or without the handler", () => {
    renderWith([]);
    expect(screen.queryByRole("group", { name: /customer engagements/i })).toBeNull();
  });
});

describe("CellPicker: a day left on a team the person is not on", () => {
  it("offers the dates and Clear and nothing else", () => {
    const onClear = vi.fn();
    renderPicker({ target: target({ teamKey: "alpha", shiftCode: EVENING.code, clearOnly: true }), onClear });
    expect(screen.queryByText("Rotations")).not.toBeInTheDocument();
    expect(screen.queryByText("Leave")).not.toBeInTheDocument();
    expect(screen.getByText(/is not on/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^Clear — take off .* rota$/ }));
    // A plain clear: no standing window to fall back to on a team they are not on.
    expect(onClear).toHaveBeenCalledWith("", "2026-09-23", "2026-09-23");
  });
});
