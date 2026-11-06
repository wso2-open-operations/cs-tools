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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import BulkEditIncidentsDialog from "@features/csm-operations/components/BulkEditIncidentsDialog";
import type { BeIncident } from "@api/backend/types";

vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: () => ({ data: [], isFetching: false, isError: false }),
}));
const { noResults } = vi.hoisted(() => ({ noResults: () => ({ data: [], isFetching: false, isError: false }) }));
vi.mock("@api/useSearchServiceOfferings", () => ({ useSearchServiceOfferings: noResults }));
vi.mock("@api/useSearchConfigurationItems", () => ({ useSearchConfigurationItems: noResults }));
const { searchParentIncidents } = vi.hoisted(() => ({ searchParentIncidents: vi.fn() }));
vi.mock("@features/csm-operations/api/useSearchIncidentsForSelect", () => ({
  useSearchIncidentsExcludingIds: (...args: unknown[]) => searchParentIncidents(...args),
}));
vi.mock("@features/csm-operations/api/useSearchChangeRequestsForSelect", () => ({ useSearchChangeRequestsForSelect: noResults }));
vi.mock("@features/csm-operations/api/useSearchProblemsForSelect", () => ({ useSearchProblemsForSelect: noResults }));

const onApply = vi.fn();
const incident = (id: string, state: BeIncident["state"], category: BeIncident["category"] = null): BeIncident =>
  ({ id, number: id, state, category }) as BeIncident;

function pick(dialog: HTMLElement, field: string, option: string): void {
  fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: field }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

beforeEach(() => {
  onApply.mockClear();
  searchParentIncidents.mockImplementation(noResults);
});

describe("BulkEditIncidentsDialog", () => {
  it("offers only the states every selected incident can move to", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "NEW"), incident("b", "ON_HOLD")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["No change", "In Progress", "Cancelled"]);
  });

  it("can't apply with nothing changed", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    expect(screen.getByRole("button", { name: "Apply to 1 incident" })).toBeDisabled();
  });

  it("needs a resolution code and notes to resolve, and sends them with the state", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "IN_PROGRESS"), incident("b", "IN_PROGRESS")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    pick(dialog, "State", "Resolved");
    const apply = within(dialog).getByRole("button", { name: "Apply to 2 incidents" });
    expect(apply).toBeDisabled();
    expect(within(dialog).getByRole("status")).toHaveTextContent("Resolved needs a resolution code and notes.");

    pick(dialog, "Resolution code", "Solved (Permanently)");
    expect(apply).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Resolution notes/ }), {
      target: { value: " Patched the node pool. " },
    });
    expect(apply).toBeEnabled();
    fireEvent.click(apply);
    expect(onApply).toHaveBeenCalledWith({
      patch: { state: "RESOLVED", resolutionCode: "SOLVED_PERMANENTLY", resolutionNotes: "Patched the node pool." },
    });
  });

  it("takes a resolution code and notes on their own, without a state change", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "IN_PROGRESS")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    pick(dialog, "Resolution code", "False Alarm");
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 1 incident" }));
    expect(onApply).toHaveBeenCalledWith({ patch: { resolutionCode: "FALSE_ALARM" } });
  });

  it("sends every set field of the incident form, and only those", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "NEW"), incident("b", "NEW")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Short description" }), { target: { value: " Gateway latency " } });
    pick(dialog, "Channel", "Email");
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Environment" }), { target: { value: "Production" } });
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Additional comment (customer-visible)" }), {
      target: { value: "We are on it." },
    });
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Incident report" }), { target: { value: "Root cause: DNS." } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 incidents" }));
    expect(onApply).toHaveBeenCalledWith({
      patch: {
        subject: "Gateway latency",
        contactType: "EMAIL",
        environment: "Production",
        additionalComments: "We are on it.",
        incidentReport: "Root cause: DNS.",
      },
    });
  });

  it("offers subcategories of the category being set, or of the one every selected incident shares", () => {
    const { unmount } = render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "NEW", "SECURITY"), incident("b", "NEW", "INQUIRY")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    let dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    // Different categories: no subcategory until a category is set.
    expect(within(dialog).getByRole("combobox", { name: "Subcategory" })).toHaveAttribute("aria-disabled", "true");
    expect(within(dialog).getByText("Set a category first: the selected incidents have different categories.")).toBeInTheDocument();
    pick(dialog, "Category", "Security");
    expect(within(dialog).getByRole("combobox", { name: "Subcategory" })).not.toHaveAttribute("aria-disabled");
    unmount();

    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "NEW", "SECURITY"), incident("b", "NEW", "SECURITY")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    expect(within(dialog).getByRole("combobox", { name: "Subcategory" })).not.toHaveAttribute("aria-disabled");
  });

  it("says how many selected incidents were closed since they were picked, and leaves them out of the states", () => {
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "IN_PROGRESS"), incident("b", "CLOSED")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    expect(within(dialog).getByText(/1 of the selected incidents has been closed or cancelled/)).toBeInTheDocument();
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual(["No change", "On Hold", "Resolved", "Cancelled"]);
  });

  it("needs a subcategory with a new category: the old one can't be cleared otherwise", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW", "SECURITY")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    pick(dialog, "Category", "Inquiry / Help");
    const apply = within(dialog).getByRole("button", { name: "Apply to 1 incident" });
    expect(apply).toBeDisabled();
    expect(within(dialog).getByRole("status")).toHaveTextContent("A new category needs a subcategory.");
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: /Subcategory/ }));
    fireEvent.click(screen.getAllByRole("option")[1]);
    expect(apply).toBeEnabled();
    fireEvent.click(apply);
    expect(onApply.mock.calls[0][0].patch).toMatchObject({ category: "INQUIRY" });
    expect(onApply.mock.calls[0][0].patch.subcategory).toBeTruthy();
  });

  it("offers a service offering only once a service is set", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    expect(within(dialog).getByRole("combobox", { name: "Service offering" })).toBeDisabled();
    expect(within(dialog).getByText(/Set a Service first/)).toBeInTheDocument();
  });

  it("caps the environment at the 40 characters the backend takes", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Environment" }), { target: { value: "x".repeat(55) } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 1 incident" }));
    expect(onApply.mock.calls[0][0].patch.environment).toBe("x".repeat(40));
  });

  it("leaves the incident report out while resolving: resolving generates it", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "IN_PROGRESS")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Incident report" }), { target: { value: "Root cause: DNS." } });
    pick(dialog, "State", "Resolved");
    expect(within(dialog).getByRole("textbox", { name: "Incident report" })).toBeDisabled();
    expect(within(dialog).getByText(/Resolving generates each incident's report/)).toBeInTheDocument();
    pick(dialog, "Resolution code", "False Alarm");
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Resolution notes/ }), { target: { value: "Nothing wrong." } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 1 incident" }));
    expect(onApply.mock.calls[0][0].patch).not.toHaveProperty("incidentReport");
  });

  // The parent incident is what the child incidents list and the resolve
  // cascade follow; the generic parent is another link altogether.
  it("sets the parent incident (parentIncidentId), never the generic parentId", () => {
    searchParentIncidents.mockImplementation(() => ({
      data: [{ id: "inc-parent", number: "INC0011111", subject: "Gateway outage" }],
      isFetching: false,
      isError: false,
    }));
    render(
      <BulkEditIncidentsDialog
        incidents={[incident("a", "NEW"), incident("b", "IN_PROGRESS")]}
        isApplying={false}
        onClose={vi.fn()}
        onApply={onApply}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "Parent incident" }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "INC0011111 — Gateway outage" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 incidents" }));

    expect(onApply).toHaveBeenCalledWith({ patch: { parentIncidentId: "inc-parent" } });
    expect(onApply.mock.calls[0][0].patch).not.toHaveProperty("parentId");
  });

  it("opens wide, so its columns never squeeze a field", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying={false} onClose={vi.fn()} onApply={onApply} />,
    );
    expect(screen.getByRole("dialog", { name: "Edit 1 incident" })).toHaveClass("MuiDialog-paperWidthLg");
  });

  it("keeps everything still while applying", () => {
    render(
      <BulkEditIncidentsDialog incidents={[incident("a", "NEW")]} isApplying onClose={vi.fn()} onApply={onApply} />,
    );
    expect(screen.getByRole("button", { name: "Applying…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
  });
});
