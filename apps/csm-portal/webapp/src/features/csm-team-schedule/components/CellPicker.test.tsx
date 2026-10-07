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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CellPicker, { type CellPickerTarget } from "./CellPicker";
import type { CellAbsence } from "../types";
import { ANNUAL_LEAVE, EVENING, LIEU_LEAVE, REGULAR, RND, TZ1, TZ1_L1, TZ2, TZ3, WEEKEND } from "../test/fixtures";

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

  it("blocks leave from a weekend start, but not an allocation", () => {
    renderPicker();
    fireEvent.change(screen.getByLabelText("Mark from"), { target: { value: "2026-09-26" } });
    expect(screen.getByRole("button", { name: /Annual leave/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: new RegExp(RND.label) })).toBeEnabled();
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
    fireEvent.change(screen.getByLabelText("Tag short code"), { target: { value: " Trn " } });
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "External training" } });
    fireEvent.click(screen.getByRole("radio", { name: "Colour MIG" }));
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).toHaveBeenCalledWith({
      shortCode: "Trn",
      label: "External training",
      bucket: "ALLOCATION",
      colourToken: "MIG",
    });
    expect(await screen.findByRole("button", { name: "+ New tag" })).toBeInTheDocument();
  });

  it("refuses a short code another tag already draws, without asking the server", () => {
    const onCreateKind = vi.fn(() => Promise.resolve());
    renderWithCreate(onCreateKind);
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag short code"), { target: { value: "al" } });
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "Another leave" } });
    fireEvent.click(screen.getByRole("button", { name: "Add tag" }));
    expect(onCreateKind).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("already a tag");
  });

  it("shows why the server refused it", async () => {
    renderWithCreate(() => Promise.reject(new Error("A tag with that name or short code already exists.")));
    fireEvent.click(screen.getByRole("button", { name: "+ New tag" }));
    fireEvent.change(screen.getByLabelText("Tag short code"), { target: { value: "Xyz" } });
    fireEvent.change(screen.getByLabelText("Tag name"), { target: { value: "Something" } });
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
