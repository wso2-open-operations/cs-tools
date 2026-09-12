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

import type { ReactElement } from "react";
import { fireEvent, render as rtlRender, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";
import IncidentsTab from "@features/csm-operations/components/IncidentsTab";
import { useSearchIncidents } from "@features/csm-operations/api/useSearchIncidents";
import type { BeIncident } from "@api/backend/types";

// The backend client reads runtime config at module load, which isn't present
// under vitest (same approach as ChangeRequestsTab.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn() }),
}));

vi.mock("@features/csm-operations/api/useSearchIncidents", () => ({
  useSearchIncidents: vi.fn(),
}));

// Only the column picker's storage key derives from the signed-in user.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

// The real export button needs an `ErrorBannerProvider` ancestor this test
// doesn't set up (same approach as ChangeRequestsTab.test.tsx) — unrelated to
// the column-customization behavior under test here.
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));

const mockedUseSearch = vi.mocked(useSearchIncidents);
// The picker only lists the 7 optional columns (Number/Subject/Caller/State/
// Priority/Opened/Updated are fixed and never appear in it).
const INCIDENT_OPTIONAL_COLUMN_COUNT = 7;

function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const INCIDENT: BeIncident = {
  id: "inc-1",
  number: "INC0000001",
  openedOn: "2026-07-01T00:00:00Z",
  subject: "Cluster degraded",
  caller: { id: "user-3", name: "Caller Person" },
  priority: "HIGH",
  state: "IN_PROGRESS",
  category: "SERVICE_INTERRUPTION",
  parent: { id: "inc-0", name: "INC0000000" },
  assignmentGroup: { id: "team-1", name: "Platform CRE" },
  assignedTo: { id: "user-2", name: "Jane Doe" },
  createdOn: "2026-06-30T00:00:00Z",
  createdBy: "system",
  updatedOn: "2026-07-02T00:00:00Z",
  updatedBy: "jane.doe",
};

function mockResult(overrides: Partial<ReturnType<typeof useSearchIncidents>>): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchIncidents>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  window.localStorage.clear();
});

describe("IncidentsTab — list states", () => {
  it("renders a row from the search result with only the original fixed columns visible", () => {
    mockResult({ data: { incidents: [INCIDENT], total: 1, limit: 20, offset: 0 } });
    render(<IncidentsTab />);

    expect(screen.getByText("INC0000001")).toBeInTheDocument();
    expect(screen.getByText("Cluster degraded")).toBeInTheDocument();
    expect(screen.getByText("Caller Person")).toBeInTheDocument();
    // None of the new optional columns are visible by default.
    expect(screen.queryByRole("columnheader", { name: "Category" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Assignment group" })).not.toBeInTheDocument();
  });

  it("shows the empty state when there are no incidents", () => {
    mockResult({ data: { incidents: [], total: 0, limit: 20, offset: 0 } });
    render(<IncidentsTab />);
    expect(screen.getByText(/no incidents found/i)).toBeInTheDocument();
  });
});

describe("IncidentsTab — customise columns", () => {
  it("offers every new optional column in the picker", () => {
    mockResult({ data: { incidents: [INCIDENT], total: 1, limit: 20, offset: 0 } });
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("button", { name: "Customise incident columns" }));

    for (const label of [
      "Category",
      "Assignment group",
      "Assigned to",
      "Parent",
      "Created by",
      "Updated by",
      "Created",
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("adds the Category column when checked, and it renders the row's category", () => {
    mockResult({ data: { incidents: [INCIDENT], total: 1, limit: 20, offset: 0 } });
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("button", { name: "Customise incident columns" }));
    // Column order matches `INCIDENT_OPTIONAL_COLUMNS`: Category(0),
    // Assignment group(1), Assigned to(2), ...
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[0]);

    expect(screen.getByRole("columnheader", { name: "Category", hidden: true })).toBeInTheDocument();
    expect(screen.getByText("SERVICE_INTERRUPTION")).toBeInTheDocument();
  });

  it("adds the Assigned to column, falling back to Unassigned when no one is assigned", () => {
    mockResult({
      data: { incidents: [{ ...INCIDENT, assignedTo: null }], total: 1, limit: 20, offset: 0 },
    });
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("button", { name: "Customise incident columns" }));
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[2]); // Assigned to

    expect(screen.getByText("Unassigned")).toBeInTheDocument();
  });

  it("never lets every column be unchecked, once at least one is checked", () => {
    mockResult({ data: { incidents: [INCIDENT], total: 1, limit: 20, offset: 0 } });
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("button", { name: "Customise incident columns" }));
    // Check every column first (none are checked by default here, unlike
    // Change Requests), then try to uncheck them all.
    for (let i = 0; i < INCIDENT_OPTIONAL_COLUMN_COUNT; i++) {
      fireEvent.click(screen.getAllByRole("checkbox")[i]);
    }
    for (let i = 0; i < INCIDENT_OPTIONAL_COLUMN_COUNT; i++) {
      const checkbox = screen.getAllByRole("checkbox")[i];
      if (checkbox && !checkbox.hasAttribute("disabled") && (checkbox as HTMLInputElement).checked) {
        fireEvent.click(checkbox);
      }
    }

    const remainingChecked = screen
      .getAllByRole("checkbox")
      .filter((checkbox) => (checkbox as HTMLInputElement).checked);
    expect(remainingChecked.length).toBe(1);
    expect(remainingChecked[0]).toBeDisabled();
  });
});
