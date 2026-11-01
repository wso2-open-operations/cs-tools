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
import { DEFAULT_INCIDENT_FILTERS } from "@features/csm-operations/utils/incidents";
import IncidentsFilterBar from "@features/csm-operations/components/IncidentsFilterBar";

// The product multi-select goes through a bulk catalogue fetch — stub it out
// so this test doesn't need a QueryClientProvider.
const useIncidentProductNameOptionsMock = vi.fn(() => ({
  data: ["Choreo", "Asgardeo"],
  isFetching: false,
  isError: false,
}));
vi.mock("@features/csm-operations/api/useIncidentProductNameOptions", () => ({
  useIncidentProductNameOptions: () => useIncidentProductNameOptionsMock(),
}));

// The SRE Team control's options come from the shared team registry query —
// stub it out too, same rationale as the product options above.
const useTeamsMock = vi.fn(() => ({
  data: [
    { id: "apollo", name: "Apollo SRE Team", family: "sre-abt", sreGroupId: "team-apollo" },
    // Wrong family — must not surface as an SRE Team option here (that's
    // the cases list's CRE Team control's job, not this one's).
    { id: "atlas", name: "Atlas CRE Team", family: "cre-abt", creGroupId: "team-atlas" },
  ],
  isLoading: false,
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@features/csm-cases/api/useUserSearch", () => ({
  useInfiniteUserSearch: () => ({
    users: [],
    isFetching: false,
    isError: false,
    hasNextPage: false,
    fetchNextPage: () => {},
    isFetchingNextPage: false,
  }),
}));
// The extra incident assignment groups (security operations) come from their
// own narrowed group search; tests steer its result per case.
const useSearchIncidentExtraGroupsMock = vi.fn(
  (): { data: Array<{ id: string; name: string; active: boolean }> | undefined; isError: boolean } => ({
    data: [{ id: "group-soc", name: "Security Operations", active: true }],
    isError: false,
  }),
);
vi.mock("@api/useSearchGroups", () => ({
  useSearchIncidentExtraGroups: () => useSearchIncidentExtraGroupsMock(),
  useSearchGroups: () => ({
    data: [{ id: "group-cab", name: "CAB Approval", active: true }],
    isFetching: false,
    isError: false,
  }),
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => useTeamsMock(),
}));
vi.mock("@features/saved-filter-views/useSavedFilterViews", () => ({
  useSavedFilterViews: () => ({
    views: [],
    isLoading: false,
    saveFilterView: vi.fn(),
    deleteFilterView: vi.fn(),
    moveFilterView: vi.fn(),
  }),
}));

/**
 * Render the filter bar over `DEFAULT_INCIDENT_FILTERS` with the given prop
 * overrides, returning the `onChange`/`onReset`/`onFiltersToggle` spies. The bar
 * is controlled, so a test asserts on the filter object handed to `onChange`
 * rather than on the input's own value.
 */
function renderFilterBar(
  overrides: Partial<React.ComponentProps<typeof IncidentsFilterBar>> = {},
) {
  const onChange = vi.fn();
  const onReset = vi.fn();
  const onFiltersToggle = vi.fn();
  render(
    <IncidentsFilterBar
      filters={DEFAULT_INCIDENT_FILTERS}
      onChange={onChange}
      onReset={onReset}
      isFiltersOpen
      onFiltersToggle={onFiltersToggle}
      {...overrides}
    />,
  );
  return { onChange, onReset, onFiltersToggle };
}

describe("IncidentsFilterBar", () => {
  it("labels both created-date bounds as UTC, so a picked day isn't mistaken for local time", () => {
    renderFilterBar();
    expect(screen.getAllByText(/created from \(utc\)/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/created to \(utc\)/i).length).toBeGreaterThan(0);
  });

  it("renders the SLA-violated checkbox unchecked by default and toggles it on", () => {
    const { onChange } = renderFilterBar();
    const checkbox = screen.getByRole("checkbox", { name: /sla violated/i });
    expect(checkbox).not.toBeChecked();

    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_INCIDENT_FILTERS,
      slaViolated: true,
    });
  });

  it("reflects an already-active SLA-violated filter as checked", () => {
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, slaViolated: true } });
    expect(screen.getByRole("checkbox", { name: /sla violated/i })).toBeChecked();
  });

  // Regression: this control used to be a raw FormControl/InputLabel/Select
  // with no `shrink`/`notched` override, which read wrong against
  // oxygen-ui's own theme (its `MuiInputLabel` styleOverrides shift an
  // unshrunk label up by `top: -7px` for any Select-backed field) --
  // reported live as looking broken compared to the Cases tab's filter bar,
  // which already routes through the shared, fixed `MultiSelectField`. This
  // exercises the swap functionally: selecting a priority still updates the
  // filter correctly.
  it("selecting a priority sets it on the filter object", () => {
    const { onChange } = renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Priority" }));
    fireEvent.click(screen.getByRole("option", { name: /critical/i }));

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_INCIDENT_FILTERS,
      priorities: ["CRITICAL"],
    });
  });

  it("surfaces the product filter's coverage caveat as visible helper text", () => {
    renderFilterBar();
    expect(
      screen.getByText(/only incidents with a recorded service can match/i),
    ).toBeInTheDocument();
  });

  it("shows the active-filter count and a Clear filters action once a filter is set", () => {
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, slaViolated: true } });
    expect(screen.getByRole("button", { name: /filters \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /clear filters/i })).toBeInTheDocument();
  });

  it("shows no active-filter badge and no Clear filters action for the defaults", () => {
    renderFilterBar();
    expect(screen.getByRole("button", { name: /^filters$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /clear filters/i })).not.toBeInTheDocument();
  });

  it("offers only sre-abt-family teams as SRE Team options, keyed by sreGroupId", () => {
    const { onChange } = renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "SRE Team" }));
    expect(screen.getByRole("option", { name: "Apollo SRE Team" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Atlas CRE Team" })).not.toBeInTheDocument();
    // Not every group: the list is the SRE teams, the way the cases list lists its CRE teams.
    expect(screen.queryByRole("option", { name: "CAB Approval" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("option", { name: "Apollo SRE Team" }));

    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_INCIDENT_FILTERS,
      sreTeamIds: ["team-apollo"],
    });
  });

  it("appends the extra incident group after the teams as an SRE Team option", () => {
    const { onChange } = renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "SRE Team" }));
    const names = screen.getAllByRole("option").map((o) => o.textContent);
    expect(names.indexOf("Security Operations")).toBe(names.indexOf("Apollo SRE Team") + 1);

    fireEvent.click(screen.getByRole("option", { name: "Security Operations" }));
    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_INCIDENT_FILTERS,
      sreTeamIds: ["group-soc"],
    });
  });

  it("does not list an extra group twice when a team already carries its id", () => {
    useSearchIncidentExtraGroupsMock.mockReturnValueOnce({
      data: [{ id: "team-apollo", name: "Apollo Group", active: true }],
      isError: false,
    });
    renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "SRE Team" }));
    expect(screen.getAllByRole("option", { name: /apollo/i })).toHaveLength(1);
    expect(screen.getByRole("option", { name: "Apollo SRE Team" })).toBeInTheDocument();
  });

  it("shows only the teams when the extra-group request returns nothing", () => {
    useSearchIncidentExtraGroupsMock.mockReturnValueOnce({ data: [], isError: false });
    renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "SRE Team" }));
    expect(screen.getByRole("option", { name: "Apollo SRE Team" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Security Operations" })).not.toBeInTheDocument();
  });

  it("still shows the teams when the extra-group request fails", () => {
    useSearchIncidentExtraGroupsMock.mockReturnValueOnce({ data: undefined, isError: true });
    renderFilterBar();

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "SRE Team" }));
    expect(screen.getByRole("option", { name: "Apollo SRE Team" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Security Operations" })).not.toBeInTheDocument();
  });

  it("renders the label of a selected extra group while the picker is closed", () => {
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, sreTeamIds: ["group-soc"] } });
    expect(screen.getAllByText("Security Operations").length).toBeGreaterThan(0);
  });

  it("calls onReset when Clear filters is clicked", () => {
    const { onReset } = renderFilterBar({
      filters: { ...DEFAULT_INCIDENT_FILTERS, products: ["Choreo"] },
    });
    fireEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    expect(onReset).toHaveBeenCalledTimes(1);
  });

  it("keeps State, SRE Team and Service quick, with Category, Assignee and Updated only under Advanced", () => {
    const onChange = vi.fn();
    render(
      <IncidentsFilterBar
        filters={DEFAULT_INCIDENT_FILTERS}
        onChange={onChange}
        onReset={vi.fn()}
        isFiltersOpen
        onFiltersToggle={vi.fn()}
      />,
    );
    for (const label of ["State", "SRE Team", "Service", "Priority"]) {
      expect(screen.getAllByLabelText(label).length).toBeGreaterThan(0);
    }
    expect(screen.queryByLabelText("Category")).not.toBeInTheDocument();
    expect(screen.queryByText("Updated from (UTC)")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(screen.getByText("Advanced filters")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Add filter" }));
    // A new row starts on the first Advanced field, State ("is one of"); every
    // filter is offered there, the quick ones included.
    expect(screen.getByRole("combobox", { name: "Field" })).toHaveTextContent("State");
    expect(screen.getByRole("combobox", { name: "Operator" })).toHaveTextContent("is one of");
  });

  it("shows an Advanced filter that is already set as a row, and counts it on the toggle", () => {
    const onChange = vi.fn();
    render(
      <IncidentsFilterBar
        filters={{ ...DEFAULT_INCIDENT_FILTERS, updatedStartDate: "2026-01-01" }}
        onChange={onChange}
        onReset={vi.fn()}
        isFiltersOpen
        onFiltersToggle={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Advanced (1)" }));
    expect(screen.getByRole("combobox", { name: "Field" })).toHaveTextContent("Updated");
    expect(screen.getByRole("combobox", { name: "Operator" })).toHaveTextContent("on or after");

    fireEvent.click(screen.getByRole("button", { name: "Remove filter row" }));
    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_INCIDENT_FILTERS, updatedStartDate: "" });
  });

  it("adds an OR group under Advanced and keeps it in the filters", () => {
    const onChange = vi.fn();
    render(
      <IncidentsFilterBar
        filters={DEFAULT_INCIDENT_FILTERS}
        onChange={onChange}
        onReset={vi.fn()}
        isFiltersOpen
        onFiltersToggle={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    fireEvent.click(screen.getByRole("button", { name: "Add group" }));
    expect(onChange).toHaveBeenCalledWith({
      ...DEFAULT_INCIDENT_FILTERS,
      anyOf: [{ conditions: [{ field: "state", values: [] }] }],
    });
  });

  it("shows OR groups already set, and counts complete ones on the toggle", () => {
    render(
      <IncidentsFilterBar
        filters={{
          ...DEFAULT_INCIDENT_FILTERS,
          anyOf: [
            { conditions: [{ field: "state", values: ["NEW"] }] },
            { conditions: [{ field: "priority", values: ["CRITICAL"] }] },
          ],
        }}
        onChange={vi.fn()}
        onReset={vi.fn()}
        isFiltersOpen
        onFiltersToggle={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Advanced (2)" }));
    expect(screen.getByRole("group", { name: "OR group 1" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "OR group 2" })).toBeInTheDocument();
    expect(screen.getByText("OR")).toBeInTheDocument();
  });
});
