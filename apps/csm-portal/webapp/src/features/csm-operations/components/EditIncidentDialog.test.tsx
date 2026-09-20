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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeIncidentDetail } from "@api/backend/types";

// The dialog's async reference pickers (Service, Assignment group, Assigned
// to, Parent incident, Change request, Problem, ...) all go through
// `useSearch*` hooks that hit the backend client via react-query. Stub them
// out and track calls so tests can assert each picker fires an eager,
// empty-query search as soon as it opens.
const useSearchGroupsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchItServicesMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchServiceOfferingsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchConfigurationItemsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchUsersByNameMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchIncidentsExcludingSelfMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchChangeRequestsForSelectMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchProblemsForSelectMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));

vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (...args: unknown[]) => useSearchGroupsMock(...(args as [])),
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: (...args: unknown[]) => useSearchItServicesMock(...(args as [])),
}));
vi.mock("@api/useSearchServiceOfferings", () => ({
  useSearchServiceOfferings: (...args: unknown[]) => useSearchServiceOfferingsMock(...(args as [])),
}));
vi.mock("@api/useSearchConfigurationItems", () => ({
  useSearchConfigurationItems: (...args: unknown[]) => useSearchConfigurationItemsMock(...(args as [])),
}));
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: (...args: unknown[]) => useSearchUsersByNameMock(...(args as [])),
}));
vi.mock("@features/csm-operations/api/useSearchIncidentsForSelect", () => ({
  useSearchIncidentsExcludingSelf: (...args: unknown[]) => useSearchIncidentsExcludingSelfMock(...(args as [])),
}));
vi.mock("@features/csm-operations/api/useSearchChangeRequestsForSelect", () => ({
  useSearchChangeRequestsForSelect: (...args: unknown[]) =>
    useSearchChangeRequestsForSelectMock(...(args as [])),
}));
vi.mock("@features/csm-operations/api/useSearchProblemsForSelect", () => ({
  useSearchProblemsForSelect: (...args: unknown[]) => useSearchProblemsForSelectMock(...(args as [])),
}));

import EditIncidentDialog from "@features/csm-operations/components/EditIncidentDialog";

const BASE_INCIDENT: BeIncidentDetail = {
  id: "inc-1",
  number: "INC0012345",
  openedOn: "2026-01-01T00:00:00Z",
  subject: "Gateway 502s",
  priority: null,
  state: "RESOLVED",
  category: null,
};

// MUI Select renders its current value in a `role="combobox"` element,
// labelled via the InputLabel it's paired with (same pattern as
// EditDeploymentDialog.test.tsx's `typeSelect`).
const stateSelect = (): HTMLElement => screen.getByRole("combobox", { name: /state/i });

describe("EditIncidentDialog state transition guard", () => {
  it("from RESOLVED, offers only RESOLVED, CLOSED, and IN_PROGRESS (not NEW/ON_HOLD/CANCELLED)", () => {
    render(
      <EditIncidentDialog
        incident={BASE_INCIDENT}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    fireEvent.mouseDown(stateSelect());
    const listbox = screen.getByRole("listbox");
    const options = within(listbox)
      .getAllByRole("option")
      .map((o) => o.textContent);

    expect(options).toEqual(expect.arrayContaining(["Resolved", "Closed", "In Progress"]));
    expect(options).not.toEqual(expect.arrayContaining(["New"]));
    expect(options).not.toEqual(expect.arrayContaining(["On Hold"]));
    expect(options).not.toEqual(expect.arrayContaining(["Cancelled"]));
  });

  it("from CLOSED, the State select is disabled (terminal state)", () => {
    render(
      <EditIncidentDialog
        incident={{ ...BASE_INCIDENT, state: "CLOSED" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(stateSelect()).toHaveAttribute("aria-disabled", "true");
  });
});

describe("EditIncidentDialog redundant/legacy UI removal", () => {
  it("no longer renders a Notes section or a Watch list field", () => {
    render(
      <EditIncidentDialog
        incident={BASE_INCIDENT}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(screen.queryByText("Notes")).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/additional comments/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/internal work note/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/watch list/i)).not.toBeInTheDocument();
  });
});

describe("EditIncidentDialog advanced-linking pickers", () => {
  it("renders Parent incident, Change request, and Problem as searchable comboboxes, not raw text fields", () => {
    render(
      <EditIncidentDialog
        incident={BASE_INCIDENT}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByRole("combobox", { name: /parent incident/i })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /change request/i })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /^problem$/i })).toBeInTheDocument();
    // "Caused by" is left as a plain text field — no confirmed target entity
    // type to point a search at.
    expect(screen.getByLabelText(/caused by id/i)).toBeInTheDocument();
  });

  it("fires an eager, empty-query search for each linking picker as soon as the dialog mounts (dropdown-open behavior)", () => {
    render(
      <EditIncidentDialog
        incident={BASE_INCIDENT}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    // AsyncEntitySelect calls its useSearch hook on every render with
    // `enabled` following the dropdown's open state; what matters here is
    // that the hook is wired at all (not gated out by the component itself)
    // and that opening the combobox flips `enabled` to true with an empty
    // query, verified by asserting the mock was invoked and the picker
    // renders as a live combobox above. Open the Parent incident combobox
    // explicitly to confirm the eager empty-query call. The third argument is
    // `searchExtra` — `BASE_INCIDENT.id`, since the Parent incident picker
    // excludes the incident being edited from its own results.
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /parent incident/i }));
    expect(useSearchIncidentsExcludingSelfMock).toHaveBeenCalledWith("", true, BASE_INCIDENT.id);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /change request/i }));
    expect(useSearchChangeRequestsForSelectMock).toHaveBeenCalledWith("", true, undefined);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^problem$/i }));
    expect(useSearchProblemsForSelectMock).toHaveBeenCalledWith("", true, undefined);
  });
});
