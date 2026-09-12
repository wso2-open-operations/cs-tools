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
import "@testing-library/jest-dom/vitest";
import type * as React from "react";
import { DEFAULT_PROBLEM_FILTERS } from "@features/csm-operations/utils/problems";
import ProblemsFilterBar from "@features/csm-operations/components/ProblemsFilterBar";

// The SRE Team control's options come from the shared team registry query —
// stub it out, same as IncidentsFilterBar.test.tsx's own mock.
const useTeamsMock = vi.fn(() => ({
  data: [
    { id: "apollo", name: "Apollo SRE Team", family: "sre-abt", sreGroupId: "team-apollo" },
    // Wrong family — must not surface as an SRE Team option here.
    { id: "atlas", name: "Atlas CRE Team", family: "cre-abt", creGroupId: "team-atlas" },
  ],
  isLoading: false,
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => useTeamsMock(),
}));

function renderFilterBar(
  overrides: Partial<React.ComponentProps<typeof ProblemsFilterBar>> = {},
) {
  const onChange = vi.fn();
  const onReset = vi.fn();
  const onFiltersToggle = vi.fn();
  render(
    <ProblemsFilterBar
      filters={DEFAULT_PROBLEM_FILTERS}
      onChange={onChange}
      onReset={onReset}
      isFiltersOpen
      onFiltersToggle={onFiltersToggle}
      {...overrides}
    />,
  );
  return { onChange, onReset, onFiltersToggle };
}

describe("ProblemsFilterBar", () => {
  it("selecting a state sets it on the filter object", () => {
    const { onChange } = renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: /^new$/i }));

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_PROBLEM_FILTERS,
      states: ["NEW"],
    });
  });

  it("offers only sre-abt-family teams as SRE Team options, keyed by sreGroupId", () => {
    const { onChange } = renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "SRE Team" }));
    expect(screen.getByRole("option", { name: "Apollo SRE Team" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Atlas CRE Team" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("option", { name: "Apollo SRE Team" }));

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_PROBLEM_FILTERS,
      sreTeamIds: ["team-apollo"],
    });
  });

  it("shows the active-filter count and a Clear filters action once a filter is set", () => {
    renderFilterBar({ filters: { ...DEFAULT_PROBLEM_FILTERS, sreTeamIds: ["team-apollo"] } });
    expect(screen.getByRole("button", { name: /filters \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /clear filters/i })).toBeInTheDocument();
  });

  it("shows no active-filter badge and no Clear filters action for the defaults", () => {
    renderFilterBar();
    expect(screen.getByRole("button", { name: /^filters$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clear filters/i })).not.toBeInTheDocument();
  });

  it("calls onReset when Clear filters is clicked", () => {
    const { onReset } = renderFilterBar({
      filters: { ...DEFAULT_PROBLEM_FILTERS, states: ["NEW"] },
    });
    fireEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    expect(onReset).toHaveBeenCalledTimes(1);
  });
});
