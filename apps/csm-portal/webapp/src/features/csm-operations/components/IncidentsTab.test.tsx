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
import { fireEvent, render as rtlRender, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";
import IncidentsTab from "@features/csm-operations/components/IncidentsTab";
import { useSearchIncidents } from "@features/csm-operations/api/useSearchIncidents";
import type { BeIncident } from "@api/backend/types";

// The bulk actions' requests, and whether the signed-in user may write.
const { apiMock, access, currentUser } = vi.hoisted(() => ({
  apiMock: { post: vi.fn(), get: vi.fn(), patch: vi.fn() },
  access: { canWrite: false },
  // The signed-in user as `useCurrentUser` reports it; "Assigned to me" tests
  // replace it with a profile still loading or settled without an id.
  currentUser: {
    value: { user: { id: "user-1" } as { id?: string } | undefined, isLoading: false },
  },
}));

// The backend client reads runtime config at module load, which isn't present
// under vitest (same approach as ChangeRequestsTab.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
  useBackendApi: () => apiMock,
}));
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({ canWrite: access.canWrite }),
}));
// The bulk edit dialog's pickers.
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: () => ({
    data: [{ id: "user-7", name: "Wanda Watcher", email: "wanda@example.test" }],
    isFetching: false,
    isError: false,
  }),
}));
const { noResults } = vi.hoisted(() => ({ noResults: () => ({ data: [], isFetching: false, isError: false }) }));
vi.mock("@api/useSearchServiceOfferings", () => ({ useSearchServiceOfferings: noResults }));
vi.mock("@api/useSearchConfigurationItems", () => ({ useSearchConfigurationItems: noResults }));
vi.mock("@features/csm-operations/api/useSearchIncidentsForSelect", () => ({ useSearchIncidentsExcludingIds: noResults }));
vi.mock("@features/csm-operations/api/useSearchChangeRequestsForSelect", () => ({ useSearchChangeRequestsForSelect: noResults }));
vi.mock("@features/csm-operations/api/useSearchProblemsForSelect", () => ({ useSearchProblemsForSelect: noResults }));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@api/useSearchItServices", () => ({
  useSearchItServices: () => ({ data: [], isFetching: false, isError: false }),
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

vi.mock("@features/csm-operations/api/useSearchIncidents", () => ({
  useSearchIncidents: vi.fn(),
}));

// The column picker's storage key and "Assigned to me" derive from the signed-in user.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ ...currentUser.value, isError: false, error: null }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

// The real export menu needs an `ErrorBannerProvider` ancestor this test
// doesn't set up (same approach as ChangeRequestsTab.test.tsx); it has its own test.
vi.mock("@components/FilteredExportMenu", () => ({
  default: ({ title }: { title: string }) => <div>Export {title}</div>,
}));

const mockedUseSearch = vi.mocked(useSearchIncidents);
// The picker only lists the 7 optional columns (Number/Subject/Caller/State/
// Priority/Opened/Updated are fixed and never appear in it).
const INCIDENT_OPTIONAL_COLUMN_COUNT = 7;

let queryClient: QueryClient;

function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
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
  apiMock.patch.mockReset();
  apiMock.get.mockReset();
  access.canWrite = false;
  currentUser.value = { user: { id: "user-1" }, isLoading: false };
  window.localStorage?.clear();
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

describe("IncidentsTab — Assigned to me", () => {
  function renderAssignedToMe(): void {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    rtlRender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/?incMine=1"]}>
          <IncidentsTab />
        </MemoryRouter>
      </QueryClientProvider>,
    );
  }

  it("searches for the viewer's incidents once their id is known", () => {
    mockResult({ data: { incidents: [INCIDENT], total: 1, limit: 20, offset: 0 } });
    renderAssignedToMe();
    const [payload, options] = mockedUseSearch.mock.calls.at(-1)!;
    expect(options).toEqual({ enabled: true });
    expect(payload.filters?.filters).toContainEqual({
      field: "assignedUserId",
      op: "in",
      values: ["user-1"],
    });
  });

  it("holds the search back and shows the loading rows while the profile loads", () => {
    currentUser.value = { user: undefined, isLoading: true };
    mockResult({});
    renderAssignedToMe();
    expect(mockedUseSearch.mock.calls.at(-1)![1]).toEqual({ enabled: false });
    expect(screen.queryByText(/could not be loaded/i)).not.toBeInTheDocument();
  });

  it("explains, instead of loading forever, once the profile settles without an id", () => {
    currentUser.value = { user: {}, isLoading: false };
    mockResult({});
    renderAssignedToMe();
    expect(mockedUseSearch.mock.calls.at(-1)![1]).toEqual({ enabled: false });
    expect(screen.getByText(/your user record could not be loaded/i)).toBeInTheDocument();
  });

  it("does not let Refresh search without the viewer's id", () => {
    currentUser.value = { user: {}, isLoading: false };
    const refetch = vi.fn();
    mockResult({ refetch });
    renderAssignedToMe();
    fireEvent.click(screen.getByRole("button", { name: "Refresh incidents" }));
    expect(refetch).not.toHaveBeenCalled();
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
      // Scoped to the picker: the filter bar has a "Category" field too.
      expect(
        within(screen.getByRole("list", { name: "Customise incident columns" })).getByText(label),
      ).toBeInTheDocument();
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

// --- Bulk edit, Assign to me, export -----------------------------------------

const INCIDENT_NEW: BeIncident = { ...INCIDENT, id: "inc-2", number: "INC0000002", state: "NEW" };
const INCIDENT_CLOSED: BeIncident = { ...INCIDENT, id: "inc-3", number: "INC0000003", state: "CLOSED" };

function showRows(rows: BeIncident[]): void {
  mockResult({ data: { incidents: rows, total: rows.length, limit: 20, offset: 0 } });
}

describe("IncidentsTab — bulk selection", () => {
  it("offers no selection without write access", () => {
    showRows([INCIDENT]);
    render(<IncidentsTab />);
    expect(screen.queryByRole("checkbox", { name: "Select INC0000001" })).not.toBeInTheDocument();
  });

  it("selects the open incidents of the page, never a closed one", () => {
    access.canWrite = true;
    showRows([INCIDENT, INCIDENT_NEW, INCIDENT_CLOSED]);
    render(<IncidentsTab />);

    expect(screen.getByRole("checkbox", { name: "INC0000003 is closed and can't be selected" })).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    const bar = screen.getByRole("region", { name: "Selected rows" });
    expect(within(bar).getByText("2 incidents selected")).toBeInTheDocument();
    expect(within(bar).getByRole("button", { name: "Edit (2)" })).toBeEnabled();

    // A second click on the page's box clears it.
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    expect(screen.queryByRole("region", { name: "Selected rows" })).not.toBeInTheDocument();
  });
});

describe("IncidentsTab — Assign to me", () => {
  it("assigns every selected incident to the signed-in user", async () => {
    access.canWrite = true;
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([INCIDENT, INCIDENT_NEW]);
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));

    await waitFor(() => expect(screen.getByText("Assigned to you 2 of 2 incidents.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledWith("/incidents/inc-1", { assignedEngineerId: "user-1" });
    expect(apiMock.patch).toHaveBeenCalledWith("/incidents/inc-2", { assignedEngineerId: "user-1" });
  });

  it("reports an incident the backend refuses, in its words, and updates the rest", async () => {
    access.canWrite = true;
    const { BackendApiError } = await import("@api/backend/client");
    apiMock.patch.mockImplementation((path: string) =>
      path.endsWith("inc-2")
        ? Promise.reject(new BackendApiError(403, "only the assignee can close this incident"))
        : Promise.resolve({ id: "inc-1" }),
    );
    showRows([INCIDENT, INCIDENT_NEW]);
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));

    await waitFor(() =>
      expect(screen.getByText("Assigned to you 1 of 2 incidents. 1 could not be updated.")).toBeInTheDocument(),
    );
    const notUpdated = screen.getByRole("list", { name: "Not updated" });
    expect(within(notUpdated).getByText("INC0000002")).toBeInTheDocument();
    expect(within(notUpdated).getByText(/only the assignee can close this incident/)).toBeInTheDocument();
  });
});

// These drive the whole edit dialog, which takes a while to render under a loaded suite.
describe("IncidentsTab — bulk edit", { timeout: 20_000 }, () => {
  it("applies only the changed fields to every selected incident", async () => {
    access.canWrite = true;
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([INCIDENT, INCIDENT_NEW]);
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });

    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "Urgency" }));
    fireEvent.click(screen.getAllByRole("option").find((o) => o.textContent !== "No change")!);
    const urgency = within(dialog).getByRole("combobox", { name: "Urgency" }).textContent;
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Work note (internal)" }), {
      target: { value: "Bulk triage" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 incidents" }));

    await waitFor(() => expect(screen.getByText("Updated 2 of 2 incidents.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledTimes(2);
    const [, body] = apiMock.patch.mock.calls[0] as [string, Record<string, unknown>];
    expect(Object.keys(body).sort()).toEqual(["urgency", "workNotes"]);
    expect(body.workNotes).toBe("Bulk triage");
    expect(urgency).toBeTruthy();
  });
});

describe("IncidentsTab — export", () => {
  it("offers the export menu for the filtered list", () => {
    showRows([INCIDENT]);
    render(<IncidentsTab />);
    expect(screen.getByText("Export Incidents")).toBeInTheDocument();
  });
});

// These drive the whole edit dialog, which takes a while to render under a loaded suite.
describe("IncidentsTab — bulk runs follow the incident as it is now", { timeout: 20_000 }, () => {
  it("skips an incident already assigned to the user, and one closed since it was picked, saying why", async () => {
    access.canWrite = true;
    apiMock.patch.mockResolvedValue({ id: "x" });
    const mine = { ...INCIDENT_NEW, id: "inc-4", number: "INC0000004", assignedTo: { id: "user-1", name: "Me" } };
    showRows([INCIDENT, INCIDENT_NEW, mine]);
    const view = render(<IncidentsTab />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));

    // The list refetches: INC0000002 was closed meanwhile.
    showRows([INCIDENT, { ...INCIDENT_NEW, state: "CLOSED" }, mine]);
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <IncidentsTab />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByRole("checkbox", { name: "Deselect INC0000002, now closed" })).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "Assign to me (3)" }));
    await waitFor(() => expect(screen.getByText("Assigned to you 1 of 3 incidents. 2 skipped.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledTimes(1);
    expect(apiMock.patch).toHaveBeenCalledWith("/incidents/inc-1", { assignedEngineerId: "user-1" });
    const notUpdated = screen.getByRole("list", { name: "Not updated" });
    expect(within(notUpdated).getByText(/Closed or cancelled: it can't be edited/)).toBeInTheDocument();
    expect(within(notUpdated).getByText(/Already assigned to you/)).toBeInTheDocument();
  });

  it("leaves an incident closed since it was picked out of the states the dialog offers", () => {
    access.canWrite = true;
    showRows([INCIDENT, INCIDENT_NEW]);
    const view = render(<IncidentsTab />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    showRows([INCIDENT, { ...INCIDENT_NEW, state: "CANCELLED" }]);
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <IncidentsTab />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    expect(within(dialog).getByText(/1 of the selected incidents has been closed or cancelled/)).toBeInTheDocument();
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "No change",
      "On Hold",
      "Resolved",
      "Cancelled",
    ]);
  });

  it("refreshes each incident's detail and tasks the way a single edit does", async () => {
    access.canWrite = true;
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([INCIDENT, INCIDENT_NEW]);
    render(<IncidentsTab />);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));
    await waitFor(() => expect(screen.getByText("Assigned to you 2 of 2 incidents.")).toBeInTheDocument());

    const keys = invalidate.mock.calls.map(([arg]) => JSON.stringify((arg as { queryKey: unknown }).queryKey));
    for (const id of ["inc-1", "inc-2"]) {
      expect(keys).toContain(JSON.stringify(["incident-details", id]));
      expect(keys).toContain(JSON.stringify(["csm-incident-tasks", id]));
    }
    expect(keys).toContain(JSON.stringify(["incidents"]));
    // Every update writes a field-change entry, so the activity feed is refreshed too.
    expect(keys).toContain(JSON.stringify(["incident-activities", "inc-1"]));
    // The outcome takes focus: the button that started the run has gone.
    expect(screen.getByText("Assigned to you 2 of 2 incidents.").closest("[role=status]")).toHaveFocus();
  });

  it("refreshes each incident's comments and activity when the edit writes a note", async () => {
    access.canWrite = true;
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([INCIDENT, INCIDENT_NEW]);
    render(<IncidentsTab />);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    fireEvent.change(within(dialog).getByRole("textbox", { name: "Work note (internal)" }), { target: { value: "Investigating." } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 incidents" }));
    await waitFor(() => expect(screen.getByText("Updated 2 of 2 incidents.")).toBeInTheDocument());

    const keys = invalidate.mock.calls.map(([arg]) => JSON.stringify((arg as { queryKey: unknown }).queryKey));
    for (const id of ["inc-1", "inc-2"]) {
      expect(keys).toContain(JSON.stringify(["incident-comments", id]));
      expect(keys).toContain(JSON.stringify(["incident-activities", id]));
    }
  });
});

// These drive the whole edit dialog, which takes a while to render under a loaded suite.
describe("IncidentsTab — bulk watch list", { timeout: 20_000 }, () => {
  it("adds a person to each selected incident's own watch list, reading each list first", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(
        path.endsWith("inc-1")
          ? { id: "inc-1", watchList: [{ id: "user-3", name: "Caller Person", email: "c@example.test" }] }
          : { id: "inc-2", watchList: [] },
      ),
    );
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([INCIDENT, INCIDENT_NEW]);
    render(<IncidentsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = screen.getByRole("dialog", { name: "Edit 2 incidents" });
    const watchInput = within(dialog).getByRole("combobox", { name: "Watch list" });
    watchInput.focus();
    fireEvent.keyDown(watchInput, { key: "ArrowDown" });
    fireEvent.click(await screen.findByRole("option", { name: /wanda@example.test/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 incidents" }));

    await waitFor(() => expect(screen.getByText("Updated 2 of 2 incidents.")).toBeInTheDocument());
    expect(apiMock.get).toHaveBeenCalledWith("/incidents/inc-1");
    expect(apiMock.patch).toHaveBeenCalledWith("/incidents/inc-1", { watchList: ["user-3", "user-7"] });
    expect(apiMock.patch).toHaveBeenCalledWith("/incidents/inc-2", { watchList: ["user-7"] });
  });

  it("reads each watch list afresh, never from a cached detail: what it sends replaces the list", async () => {
    access.canWrite = true;
    apiMock.get.mockResolvedValue({ id: "inc-1", watchList: [{ id: "user-9", name: "Dave", email: "d@example.test" }] });
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([INCIDENT]);
    render(<IncidentsTab />);
    // A detail opened moments ago, before Dave was added: still fresh by its own staleTime.
    queryClient.setQueryData(["incident-details", "inc-1"], { id: "inc-1", watchList: [] });

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all incidents on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (1)" }));
    const dialog = screen.getByRole("dialog", { name: "Edit 1 incident" });
    const watchInput = within(dialog).getByRole("combobox", { name: "Watch list" });
    watchInput.focus();
    fireEvent.keyDown(watchInput, { key: "ArrowDown" });
    fireEvent.click(await screen.findByRole("option", { name: /wanda@example.test/ }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 1 incident" }));

    await waitFor(() => expect(screen.getByText("Updated 1 of 1 incidents.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledWith("/incidents/inc-1", { watchList: ["user-9", "user-7"] });
  });
});
