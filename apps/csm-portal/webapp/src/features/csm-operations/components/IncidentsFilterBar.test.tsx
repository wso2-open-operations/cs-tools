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
import { beforeEach, describe, expect, it, vi } from "vitest";
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

// The Assignment group control lists the SRE teams from the shared team
// registry query first — stub it out too, same rationale as the product options
// above.
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
vi.mock("@features/csm-users/api/useUserNamesById", () => ({
  useUserNamesById: () => new Map(),
}));
// The extra incident assignment groups (security operations) come from their
// own narrowed group search; tests steer its result per case.
const defaultExtraGroups = (): {
  data: Array<{ id: string; name: string; active: boolean }> | undefined;
  isError: boolean;
} => ({
  data: [{ id: "group-soc", name: "Security Operations", active: true }],
  isError: false,
});
const useSearchIncidentExtraGroupsMock = vi.fn(defaultExtraGroups);
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
// The selected groups' names are looked up by id (GET /groups/{id}); the
// option list itself is the real hook over the mocks above. Its module also
// imports the API client, which reads runtime config at load.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
}));
const useIncidentFilterGroupLabelsMock = vi.fn((ids: string[]): Record<string, string> => {
  void ids;
  return {};
});
vi.mock("@features/csm-operations/api/useSearchIncidentFilterGroups", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@features/csm-operations/api/useSearchIncidentFilterGroups")>()),
  useIncidentFilterGroupLabels: (ids: string[]) => useIncidentFilterGroupLabelsMock(ids),
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
  // The picker calls the extra-group search on every render, so a test steers
  // it with mockImplementation and this puts the default back.
  beforeEach(() => {
    useSearchIncidentExtraGroupsMock.mockImplementation(defaultExtraGroups);
    useIncidentFilterGroupLabelsMock.mockImplementation(() => ({}));
  });

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

  /** Open the Assignment group picker and return its options' labels, in order. */
  function openAssignmentGroups(): string[] {
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Assignment group" }));
    return screen.getAllByRole("option").map((o) => o.textContent ?? "");
  }

  it("lists the SRE teams, then the extra incident group, then any other group, keyed by group id", () => {
    const { onChange } = renderFilterBar();

    expect(openAssignmentGroups()).toEqual(["Apollo SRE Team", "Security Operations", "CAB Approval"]);
    // Only the sre-abt family's teams are pinned (the CRE teams are the cases list's).
    expect(screen.queryByRole("option", { name: "Atlas CRE Team" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("option", { name: "Apollo SRE Team" }));
    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_INCIDENT_FILTERS, sreTeamIds: ["team-apollo"] });
  });

  it("filters by a group that is neither an SRE team nor an extra incident group", () => {
    const { onChange } = renderFilterBar();

    openAssignmentGroups();
    fireEvent.click(screen.getByRole("option", { name: "CAB Approval" }));

    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_INCIDENT_FILTERS, sreTeamIds: ["group-cab"] });
  });

  it("does not list an extra group twice when a team already carries its id", () => {
    useSearchIncidentExtraGroupsMock.mockImplementation(() => ({
      data: [{ id: "team-apollo", name: "Apollo Group", active: true }],
      isError: false,
    }));
    renderFilterBar();

    expect(openAssignmentGroups()).toEqual(["Apollo SRE Team", "CAB Approval"]);
  });

  it("still lists the teams and other groups when the extra-group request fails", () => {
    useSearchIncidentExtraGroupsMock.mockImplementation(() => ({ data: undefined, isError: true }));
    renderFilterBar();

    expect(openAssignmentGroups()).toEqual(["Apollo SRE Team", "CAB Approval"]);
  });

  it("renders the label of a selected SRE team or extra group while the picker is closed", () => {
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, sreTeamIds: ["team-apollo", "group-soc"] } });
    expect(screen.getAllByText("Apollo SRE Team, Security Operations").length).toBeGreaterThan(0);
  });

  it("names a restored group that is not pinned by looking it up, not by its id", () => {
    useIncidentFilterGroupLabelsMock.mockImplementation(
      (ids): Record<string, string> => (ids.includes("group-choreo") ? { "group-choreo": "Choreo SRE Team" } : {}),
    );
    renderFilterBar({ filters: { ...DEFAULT_INCIDENT_FILTERS, sreTeamIds: ["group-choreo"] } });

    expect(useIncidentFilterGroupLabelsMock).toHaveBeenLastCalledWith(["group-choreo"]);
    expect(screen.getAllByText("Choreo SRE Team").length).toBeGreaterThan(0);
    expect(screen.queryByText("group-choreo")).not.toBeInTheDocument();
  });

  it("calls onReset when Clear filters is clicked", () => {
    const { onReset } = renderFilterBar({
      filters: { ...DEFAULT_INCIDENT_FILTERS, products: ["Choreo"] },
    });
    fireEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    expect(onReset).toHaveBeenCalledTimes(1);
  });

  it("keeps State, Assignment group and Service quick, with Category, Assignee and Updated only under Advanced", () => {
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
    for (const label of ["State", "Assignment group", "Service", "Priority"]) {
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
