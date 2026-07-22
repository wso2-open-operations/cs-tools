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
// to, Watch list, ...) all go through `useSearch*` hooks that hit the
// backend client via react-query. Stub them out — this test only cares
// about the State select's transition-guard behavior.
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchServiceOfferings", () => ({
  useSearchServiceOfferings: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchConfigurationItems", () => ({
  useSearchConfigurationItems: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchUsersByName: () => ({ data: [], isFetching: false, isError: false }),
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
