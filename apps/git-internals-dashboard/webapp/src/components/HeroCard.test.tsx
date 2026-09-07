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
import { CsHeroCard, HeroCard } from "./HeroCard";

describe("HeroCard", () => {
  it("renders the count, label, and an 'at target' badge when n is zero", () => {
    render(<HeroCard label="Violated" n={0} delta={0} spark={[1, 2, 3]} accent="var(--sla-violated)" />);
    expect(screen.getByText("Violated")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
    expect(screen.getByText("✓ at target")).toBeInTheDocument();
  });

  it("does not render the 'at target' badge when n is non-zero", () => {
    render(<HeroCard label="Violated" n={3} delta={1} spark={[1, 2, 3]} accent="var(--sla-violated)" />);
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.queryByText("at target")).not.toBeInTheDocument();
  });

  it("shows an up-arrow delta with a plus sign when the count increased", () => {
    render(<HeroCard label="Violated" n={3} delta={2} spark={[1, 2, 3]} accent="var(--sla-violated)" />);
    expect(screen.getByText("▲ +2")).toBeInTheDocument();
  });

  it("shows a down-arrow delta without a plus sign when the count decreased", () => {
    render(<HeroCard label="Violated" n={3} delta={-2} spark={[1, 2, 3]} accent="var(--sla-violated)" />);
    expect(screen.getByText("▼ -2")).toBeInTheDocument();
  });

  it("shows a flat delta when unchanged", () => {
    render(<HeroCard label="Violated" n={3} delta={0} spark={[1, 2, 3]} accent="var(--sla-violated)" />);
    expect(screen.getByText("• 0")).toBeInTheDocument();
  });

  it("invokes onClick when the count is clickable", () => {
    const onClick = vi.fn();
    render(<HeroCard label="Violated" n={3} delta={0} spark={[1, 2, 3]} accent="var(--sla-violated)" onClick={onClick} />);
    fireEvent.click(screen.getByText("3"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("renders the count as plain text (not a button) when onClick is omitted", () => {
    render(<HeroCard label="Violated" n={3} delta={0} spark={[1, 2, 3]} accent="var(--sla-violated)" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});

describe("CsHeroCard", () => {
  const byStatus = [
    { status: "WOC", n: 2 },
    { status: "PPQ", n: 1 },
  ];

  it("renders a count and label per status", () => {
    render(<CsHeroCard n={3} byStatus={byStatus} />);
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("WOC")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("PPQ")).toBeInTheDocument();
  });

  it("calls onDrill with the clicked status", () => {
    const onDrill = vi.fn();
    render(<CsHeroCard n={3} byStatus={byStatus} onDrill={onDrill} />);
    fireEvent.click(screen.getByText("WOC"));
    expect(onDrill).toHaveBeenCalledWith("WOC");
  });
});
