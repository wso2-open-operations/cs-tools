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

describe("EditIncidentDialog optional subcategory", () => {
  const CLASSIFIED: BeIncidentDetail = {
    ...BASE_INCIDENT,
    state: "IN_PROGRESS",
    category: "SECURITY",
    subcategory: null,
    contactType: "EMAIL",
    impact: "LOW",
    urgency: "LOW",
  };
  const saveButton = (): HTMLElement => screen.getByRole("button", { name: /^save$/i });
  const pick = (combobox: RegExp, option: string): void => {
    fireEvent.mouseDown(screen.getByRole("combobox", { name: combobox }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: option }));
  };

  it("lets an incident with no subcategory be edited and saved without picking one", () => {
    const onSave = vi.fn();
    render(<EditIncidentDialog incident={CLASSIFIED} isSaving={false} onClose={vi.fn()} onSave={onSave} />);

    pick(/^impact$/i, "High");
    expect(saveButton()).not.toBeDisabled();
    fireEvent.click(saveButton());

    expect(onSave).toHaveBeenCalledWith({ impact: "HIGH" });
  });

  it("lets a category be set on an incident that had no subcategory, without picking one", () => {
    const onSave = vi.fn();
    render(<EditIncidentDialog incident={CLASSIFIED} isSaving={false} onClose={vi.fn()} onSave={onSave} />);

    pick(/^category$/i, "Inquiry / Help");
    fireEvent.click(saveButton());

    expect(onSave).toHaveBeenCalledWith({ category: "INQUIRY" });
  });

  it("still requires a new subcategory when the category changes on an incident that has one", () => {
    // PATCH can't clear a subcategory, so dropping it silently would leave
    // PHISHING (a Security subcategory) paired with the new category.
    const onSave = vi.fn();
    render(
      <EditIncidentDialog
        incident={{ ...CLASSIFIED, subcategory: "PHISHING" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );

    pick(/^category$/i, "Service Interruption");
    expect(saveButton()).toBeDisabled();
    expect(screen.getByText("Required when the category changes.")).toBeInTheDocument();

    pick(/^subcategory$/i, "Slowness");
    expect(saveButton()).not.toBeDisabled();
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ category: "SERVICE_INTERRUPTION", subcategory: "SLOWNESS" });
  });
});

describe("EditIncidentDialog channel", () => {
  const WITH_CHANNEL: BeIncidentDetail = {
    ...BASE_INCIDENT,
    state: "IN_PROGRESS",
    category: "SECURITY",
    subcategory: "PHISHING",
    contactType: "PHONE",
    impact: "LOW",
    urgency: "LOW",
  };

  it("shows the incident's current value under a Channel label", () => {
    render(<EditIncidentDialog incident={WITH_CHANNEL} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);
    expect(screen.getByRole("combobox", { name: /^channel$/i })).toHaveTextContent("Phone");
    expect(screen.queryByText(/contact type/i)).not.toBeInTheDocument();
  });

  it("sends a changed channel as the wire field contactType, and only that", () => {
    const onSave = vi.fn();
    render(<EditIncidentDialog incident={WITH_CHANNEL} isSaving={false} onClose={vi.fn()} onSave={onSave} />);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^channel$/i }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "Chat" }));
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    expect(onSave).toHaveBeenCalledWith({ contactType: "CHAT" });
  });
});

describe("EditIncidentDialog required-field markers", () => {
  // Every field isValid insists on carries the asterisk (MUI `required`, which
  // also sets aria-required on the select's combobox), exactly in the cases
  // isValid insists on it.
  const IN_PROGRESS: BeIncidentDetail = {
    ...BASE_INCIDENT,
    state: "IN_PROGRESS",
    category: "SECURITY",
    subcategory: "PHISHING",
    contactType: "EMAIL",
    impact: "LOW",
    urgency: "LOW",
  };
  const combobox = (name: RegExp): HTMLElement => screen.getByRole("combobox", { name });
  const pick = (name: RegExp, option: string): void => {
    fireEvent.mouseDown(combobox(name));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: option }));
  };

  it("marks Subject, Category, Channel, Impact, Urgency and a changeable State required on any save, and the optional fields not", () => {
    render(<EditIncidentDialog incident={IN_PROGRESS} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);

    expect(screen.getByRole("textbox", { name: /^subject$/i })).toBeRequired();
    for (const name of [/^category$/i, /^channel$/i, /^impact$/i, /^urgency$/i, /^state$/i]) {
      expect(combobox(name)).toBeRequired();
    }
    expect(screen.getByRole("textbox", { name: /^environment$/i })).not.toBeRequired();
    expect(combobox(/^service$/i)).not.toBeRequired();
    expect(combobox(/^assignment group$/i)).not.toBeRequired();
  });

  it("marks Subcategory required only once the category changes on an incident that has one", () => {
    render(<EditIncidentDialog incident={IN_PROGRESS} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);
    expect(combobox(/^subcategory$/i)).not.toBeRequired();

    pick(/^category$/i, "Service Interruption");
    expect(combobox(/^subcategory$/i)).toBeRequired();
  });

  it("does not mark Subcategory while Category is cleared, since it is disabled until one is picked", () => {
    render(<EditIncidentDialog incident={IN_PROGRESS} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);

    pick(/^category$/i, "-- Select --");
    expect(combobox(/^subcategory$/i)).toHaveAttribute("aria-disabled", "true");
    expect(combobox(/^subcategory$/i)).not.toBeRequired();
    expect(screen.getByText("Pick a category first.")).toBeInTheDocument();
    // The blank Category is what holds Save back.
    expect(combobox(/^category$/i)).toBeRequired();
    expect(screen.getByRole("button", { name: /^save$/i })).toBeDisabled();

    pick(/^category$/i, "Service Interruption");
    expect(combobox(/^subcategory$/i)).toBeRequired();
  });

  it("does not mark a terminal State, whose select is disabled", () => {
    render(
      <EditIncidentDialog
        incident={{ ...IN_PROGRESS, state: "CLOSED" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(combobox(/^state$/i)).toHaveAttribute("aria-disabled", "true");
    expect(combobox(/^state$/i)).not.toBeRequired();
    expect(combobox(/^category$/i)).toBeRequired();
  });

  it("leaves Subcategory optional when the category changes on an incident that had none", () => {
    render(
      <EditIncidentDialog
        incident={{ ...IN_PROGRESS, subcategory: null }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    pick(/^category$/i, "Service Interruption");
    expect(combobox(/^subcategory$/i)).not.toBeRequired();
  });

  it.each(["Resolved", "Closed"])("marks Resolution code and Resolution notes required when moving to %s", (target) => {
    const initialState = target === "Closed" ? "RESOLVED" : "IN_PROGRESS";
    render(
      <EditIncidentDialog
        incident={{ ...IN_PROGRESS, state: initialState }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    pick(/^state$/i, target);
    expect(screen.getByText(`A resolution is required to move this incident to ${target}.`)).toBeInTheDocument();
    expect(combobox(/^resolution code$/i)).toBeRequired();
    expect(screen.getByRole("textbox", { name: /^resolution notes$/i })).toBeRequired();
    expect(screen.getByRole("button", { name: /^save$/i })).toBeDisabled();
  });

  it("leaves the resolution fields unmarked on an already-Resolved incident whose State is left alone", () => {
    // They still show (the State value is Resolved), but nothing requires
    // them: isTransitioningToResolved is false, so Save accepts other edits.
    render(
      <EditIncidentDialog
        incident={{ ...IN_PROGRESS, state: "RESOLVED" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(combobox(/^resolution code$/i)).not.toBeRequired();
    expect(screen.getByRole("textbox", { name: /^resolution notes$/i })).not.toBeRequired();
    // Nor does the caption claim a resolution is needed: this save moves nothing.
    expect(screen.queryByText(/A resolution is required/)).not.toBeInTheDocument();
    pick(/^impact$/i, "High");
    expect(screen.getByRole("button", { name: /^save$/i })).not.toBeDisabled();
  });
});
