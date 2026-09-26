// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Overview } from "@api/types";
import { PriorityStateMatrix } from "./PriorityStateMatrix";

// Every cell holds a distinct value so getByText can target one cell
// unambiguously; the numbers don't need to sum consistently for this
// rendering/interaction test (that invariant is covered on the backend).
const MATRIX: Overview["matrix"] = {
  rows: [
    { key: "Critical(P1)", code: "P1", cells: { violated: 11, atRisk: 12, onTrack: 13, cs: 14 }, total: 50 },
    { key: "High(P2)", code: "P2", cells: { violated: 21, atRisk: 22, onTrack: 23, cs: 24 }, total: 90 },
  ],
  totals: { violated: 32, atRisk: 34, onTrack: 36, cs: 38 },
  grandTotal: 140,
};

describe("PriorityStateMatrix", () => {
  it("renders the product-side group title and every column label", () => {
    render(<PriorityStateMatrix matrix={MATRIX} onDrill={vi.fn()} />);

    expect(screen.getByText("On Product Team Side")).toBeTruthy();
    expect(screen.getByText("Violated")).toBeTruthy();
    expect(screen.getByText("At risk")).toBeTruthy();
    expect(screen.getByText("On track")).toBeTruthy();
    expect(screen.getByText("CS side")).toBeTruthy();
    expect(screen.getByText("Total")).toBeTruthy();
  });

  it("renders each row's cells and its own total", () => {
    render(<PriorityStateMatrix matrix={MATRIX} onDrill={vi.fn()} />);

    expect(screen.getByText("11")).toBeTruthy(); // P1 violated
    expect(screen.getByText("24")).toBeTruthy(); // P2 cs
    expect(screen.getByText("50")).toBeTruthy(); // P1 total
    expect(screen.getByText("90")).toBeTruthy(); // P2 total
  });

  it("clicking a priority row's cell drills with that cell's bucket and the row's priority key", () => {
    const onDrill = vi.fn();
    render(<PriorityStateMatrix matrix={MATRIX} onDrill={onDrill} />);

    fireEvent.click(screen.getByText("22")); // P2's atRisk cell

    expect(onDrill).toHaveBeenCalledWith("at_risk", "High(P2)");
  });

  it("clicking the totals row's cs cell drills into the cs bucket with no priority", () => {
    const onDrill = vi.fn();
    render(<PriorityStateMatrix matrix={MATRIX} onDrill={onDrill} />);

    fireEvent.click(screen.getByText("38")); // totals.cs

    expect(onDrill).toHaveBeenCalledWith("cs");
  });

  it("clicking the All row's Total cell drills into the tracked bucket with no priority", () => {
    const onDrill = vi.fn();
    render(<PriorityStateMatrix matrix={MATRIX} onDrill={onDrill} />);

    fireEvent.click(screen.getByText("140")); // grandTotal

    expect(onDrill).toHaveBeenCalledWith("tracked");
  });

  it("renders with no priority rows (grandTotal=0) without throwing", () => {
    render(<PriorityStateMatrix matrix={{ rows: [], totals: { violated: 0, atRisk: 0, onTrack: 0, cs: 0 }, grandTotal: 0 }} onDrill={vi.fn()} />);
    expect(screen.getByText("On Product Team Side")).toBeTruthy();
  });
});
