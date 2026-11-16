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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { HolidayAvailability } from "../types";
// vi.mock below is hoisted above this import, so the panel gets the mocks.
import HolidayAvailabilityPanel from "./HolidayAvailabilityPanel";

const confirmMutate = vi.fn();
const deleteMutate = vi.fn();
const read: { data?: { holidays: HolidayAvailability[]; count: number }; from?: string; to?: string } = {};
const confirmState: { error: Error | null } = { error: null };

vi.mock("../api/useHolidayAvailability", () => ({
  useHolidayAvailability: (from: string, to: string) => {
    read.from = from;
    read.to = to;
    return { data: read.data };
  },
  useHolidayConfirmation: () => ({ mutate: confirmMutate, isPending: false, error: confirmState.error }),
}));
vi.mock("../api/useTeamSchedule", () => ({
  useDeleteAbsence: () => ({ mutate: deleteMutate, isPending: false, error: null }),
}));

function holiday(over: Partial<HolidayAvailability> = {}): HolidayAvailability {
  return {
    teamKey: "atlas",
    teamName: "Atlas",
    date: "2026-11-30",
    titles: ["Il Full Moon Poya Day"],
    countries: ["Sri Lanka"],
    status: "NEEDS_MARKING",
    fullyMarked: false,
    members: [
      { userId: "u1", name: "Ann Poe", status: "TAGGED", absenceId: "abs-1" },
      { userId: "u2", name: "Ben Roe", status: "AVAILABLE" },
      { userId: "u3", name: "Cal Doe", status: "AWAY", awayKind: "AL" },
    ],
    turns: 2,
    uncovered: [{ shiftCode: "CRE_MORNING", shiftLabel: "Morning", userId: "u1", name: "Ann Poe" }],
    canConfirm: true,
    ...over,
  };
}

const NOW = new Date(2026, 10, 16, 9, 0);

describe("HolidayAvailabilityPanel", () => {
  beforeEach(() => {
    confirmMutate.mockReset();
    deleteMutate.mockReset();
    confirmState.error = null;
    read.data = undefined;
  });

  it("shows nothing when no holiday is coming", () => {
    read.data = { holidays: [], count: 0 };
    const { container } = render(<HolidayAvailabilityPanel now={NOW} />);
    expect(container).toBeEmptyDOMElement();
    expect(read.from).toBe("2026-11-16");
    expect(read.to).toBe("2026-12-07");
  });

  it("lists who is off, who works, the uncovered turn, and lets the lead make somebody available", () => {
    read.data = { holidays: [holiday()], count: 1 };
    render(<HolidayAvailabilityPanel now={NOW} />);
    const region = screen.getByRole("region", { name: "Public holidays coming up" });
    expect(within(region).getByText("Needs marking")).toBeInTheDocument();
    expect(within(region).getByText(/Nobody available for: Morning \(Ann Poe\)/)).toBeInTheDocument();
    expect(within(region).getByText("Working")).toBeInTheDocument();
    expect(within(region).getByText("Away (AL)")).toBeInTheDocument();
    // Only the tagged member can be made available.
    const buttons = within(region).getAllByRole("button", { name: "Make available" });
    expect(buttons).toHaveLength(1);
    fireEvent.click(buttons[0]);
    expect(deleteMutate).toHaveBeenCalledWith({ id: "abs-1", note: "Works on the public holiday" });
    // Confirm waits until every turn has somebody.
    expect(within(region).getByRole("button", { name: "Confirm team availability" })).toBeDisabled();
  });

  it("confirms when every turn is covered, withdraws once confirmed, and shows the server's refusal", () => {
    read.data = { holidays: [holiday({ status: "AWAITING_CONFIRMATION", uncovered: [] })], count: 1 };
    const { rerender } = render(<HolidayAvailabilityPanel now={NOW} />);
    fireEvent.click(screen.getByRole("button", { name: "Confirm team availability" }));
    expect(confirmMutate).toHaveBeenCalledWith({ teamKey: "atlas", date: "2026-11-30", confirm: true });

    read.data = {
      holidays: [
        holiday({ status: "NEEDS_ATTENTION", confirmedBy: "lead@example.com", confirmedOn: "2026-11-17T04:00:00Z" }),
      ],
      count: 1,
    };
    confirmState.error = new Error("1 rota turn(s) on 2026-11-30 have nobody available: Morning (Ann Poe).");
    rerender(<HolidayAvailabilityPanel now={NOW} />);
    fireEvent.click(screen.getByRole("button", { name: "Withdraw confirmation" }));
    expect(confirmMutate).toHaveBeenLastCalledWith({ teamKey: "atlas", date: "2026-11-30", confirm: false });
    expect(screen.getByRole("alert")).toHaveTextContent("have nobody available");
  });

  it("shows the reader's own teams first, and everyone's on request, without controls", () => {
    read.data = {
      holidays: [holiday(), holiday({ teamKey: "vega", teamName: "Vega", canConfirm: false })],
      count: 2,
    };
    render(<HolidayAvailabilityPanel now={NOW} />);
    expect(screen.queryByText("Vega")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show all teams (2)" }));
    fireEvent.click(screen.getByRole("button", { name: /Vega/ }));
    const rows = screen.getAllByRole("listitem").filter((li) => li.classList.contains("hol-row"));
    const vega = rows.find((r) => r.textContent?.includes("Vega")) as HTMLElement;
    expect(within(vega).queryByRole("button", { name: "Make available" })).not.toBeInTheDocument();
    expect(within(vega).queryByRole("button", { name: /Confirm/ })).not.toBeInTheDocument();
  });
});
