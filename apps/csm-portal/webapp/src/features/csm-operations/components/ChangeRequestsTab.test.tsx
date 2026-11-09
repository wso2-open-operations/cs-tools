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

import type { JSX, ReactElement } from "react";
import { fireEvent, render as rtlRender, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import "@testing-library/jest-dom/vitest";
import ChangeRequestsTab from "@features/csm-operations/components/ChangeRequestsTab";
import { useSearchChangeRequests } from "@features/csm-operations/api/useSearchChangeRequests";
import type { BeChangeRequestDetail, BeChangeRequestSearchView } from "@api/backend/types";

// The bulk actions' requests, and whether the signed-in user may write.
const { apiMock, access } = vi.hoisted(() => ({
  apiMock: { post: vi.fn(), get: vi.fn(), patch: vi.fn() },
  access: { canWrite: false },
}));

// The backend client reads runtime config at module load, which isn't present
// under vitest (same approach as CsmAnnouncementsPage.test.tsx).
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
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: () => ({ data: [], isFetching: false, isError: false }),
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

vi.mock("@features/csm-operations/api/useSearchChangeRequests", () => ({
  useSearchChangeRequests: vi.fn(),
}));

// The filter bar's SRE Team control goes through the shared team registry
// query — stub it out so this test doesn't need a QueryClientProvider,
// same rationale as the other mocks below.
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => ({ data: [], isLoading: false }),
}));

// The filter bar's Project control (`AsyncProjectMultiSelect`) goes through
// the same shared infinite-query hook the cases list uses — stub it out for
// the same "no QueryClientProvider" reason as the SRE Team mock above.
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
vi.mock("@features/csm-cases/api/useSearchTags", () => ({
  useSearchTags: () => ({ data: [], isFetching: false, isError: false }),
}));
vi.mock("@features/csm-cases/api/useProjectSearch", () => ({
  useInfiniteProjectSearch: () => ({
    projects: [],
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  }),
}));

// Only the column picker's storage key derives from the signed-in user.
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));

// The bulk edit dialog's other lookups: nothing to find, and no provider of theirs to set up.
const noResults = vi.hoisted(() => () => ({ data: [], isFetching: false, isError: false }));
vi.mock("@api/useSearchItServices", () => ({ useSearchItServices: noResults }));
vi.mock("@api/useSearchServiceOfferings", () => ({ useSearchServiceOfferings: noResults }));
vi.mock("@features/csm-operations/api/useSearchServiceRequestsForSelect", () => ({
  useSearchServiceRequestsForSelect: noResults,
}));
vi.mock("@features/csm-cases/components/AsyncProjectSelect", () => ({
  default: ({ label, disabled }: { label: string; disabled?: boolean }) => <input aria-label={label} disabled={disabled} />,
}));
vi.mock("@features/csm-operations/api/useChangeRequestScopeLookups", () => ({
  useChangeRequestScopeLookups: () => ({
    deployments: [],
    customerContacts: [],
    contactsReady: false,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));
// The plan panels' rich-text editor, as a plain text area (it has its own tests).
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ onChange, disabled }: { onChange?: (html: string) => void; disabled?: boolean }) => (
    <textarea disabled={disabled} onChange={(e) => onChange?.(e.target.value)} />
  ),
}));

// The real export menu needs an `ErrorBannerProvider` ancestor this test
// doesn't set up (same approach as CsmIssuesView.test.tsx); it has its own test.
vi.mock("@components/FilteredExportMenu", () => ({
  default: ({ title }: { title: string }) => <div>Export {title}</div>,
}));

const mockedUseSearch = vi.mocked(useSearchChangeRequests);
// The picker only lists the 10 optional columns (Number/Subject/State/Updated
// are fixed and never appear in it).
const CR_OPTIONAL_COLUMN_COUNT = 10;

let queryClient: QueryClient;

function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return rtlRender(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const CR: BeChangeRequestSearchView = {
  id: "cr-1",
  number: "CHG-1001",
  subject: "Upgrade cluster to WSO2 IS 7.1",
  project: { id: "proj-1", name: "Acme Project" },
  impact: "low",
  state: "new",
  plannedStartOn: "2026-08-01T00:00:00Z",
  plannedEndOn: "2026-08-02T00:00:00Z",
  product: { id: "prod-1", name: "WSO2 Identity Server" },
  assignedEngineer: { id: "user-2", name: "Jane Doe" },
  assignedTeam: { id: "team-1", name: "Platform CRE" },
  type: "Standard",
  case: { id: "case-1", name: "CS-2001" },
  createdOn: "2026-07-01T00:00:00Z",
  updatedOn: "2026-07-05T00:00:00Z",
};

function mockResult(
  overrides: Partial<ReturnType<typeof useSearchChangeRequests>>,
): void {
  mockedUseSearch.mockReturnValue({
    data: undefined,
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchChangeRequests>);
}

beforeEach(() => {
  mockedUseSearch.mockReset();
  apiMock.get.mockReset();
  apiMock.patch.mockReset();
  access.canWrite = false;
  window.localStorage?.clear();
});

describe("ChangeRequestsTab — list states", () => {
  it("renders a row from the search result with its default-visible columns", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    expect(screen.getByText("CHG-1001")).toBeInTheDocument();
    expect(screen.getByText("Upgrade cluster to WSO2 IS 7.1")).toBeInTheDocument();
    // Default-visible optional columns: Project, Impact, Planned start.
    expect(screen.getByText("Acme Project")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Planned start" })).toBeInTheDocument();
    // Not default-visible.
    expect(screen.queryByRole("columnheader", { name: "Product" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Assigned engineer" })).not.toBeInTheDocument();
  });

  it("shows the empty state when there are no change requests", () => {
    mockResult({ data: { changeRequests: [], total: 0, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);
    expect(screen.getByText(/no change requests found/i)).toBeInTheDocument();
  });
});

describe("ChangeRequestsTab — customise columns", () => {
  it("offers every widened optional column in the picker", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    fireEvent.click(
      screen.getByRole("button", { name: "Customise change request columns" }),
    );

    // Project/Impact/Planned start are already default-visible, so they
    // render twice (once as a column header, once in the picker) — assert
    // those via their column header instead of `getByText` to avoid an
    // ambiguous match.
    for (const label of ["Project", "Impact", "Planned start"]) {
      expect(screen.getByRole("columnheader", { name: label, hidden: true })).toBeInTheDocument();
    }
    // The rest are only offered, not shown yet — read from the picker itself,
    // since the filter bar has fields of the same names (Type, Assigned engineer).
    for (const label of [
      "Planned end",
      "Product",
      "Assigned engineer",
      "Assigned team",
      "Type",
      "Case",
      "Created",
    ]) {
      expect(
        within(screen.getByRole("list", { name: "Customise change request columns" })).getByText(label),
      ).toBeInTheDocument();
    }
  });

  it("adds the Product column when checked, and it renders the row's product name", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    fireEvent.click(
      screen.getByRole("button", { name: "Customise change request columns" }),
    );
    // Column order matches `CHANGE_REQUEST_OPTIONAL_COLUMNS`: Project(0),
    // Impact(1), Planned start(2), Planned end(3), Product(4), ...
    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[4]);

    expect(
      screen.getByRole("columnheader", { name: "Product", hidden: true }),
    ).toBeInTheDocument();
    expect(screen.getByText("WSO2 Identity Server")).toBeInTheDocument();
  });

  it("never lets every column be unchecked", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);

    fireEvent.click(
      screen.getByRole("button", { name: "Customise change request columns" }),
    );
    for (let i = 0; i < CR_OPTIONAL_COLUMN_COUNT; i++) {
      const checkbox = screen.getAllByRole("checkbox")[i];
      if (checkbox && !checkbox.hasAttribute("disabled") && (checkbox as HTMLInputElement).checked) {
        fireEvent.click(checkbox);
      }
    }

    // The invariant under test: the hook refuses to let the last visible
    // optional column be unchecked. Fixed columns (Number/Subject/State/
    // Updated) always render a header regardless, so assert the checkbox
    // state directly rather than counting headers.
    const remainingChecked = screen
      .getAllByRole("checkbox")
      .filter((checkbox) => (checkbox as HTMLInputElement).checked);
    expect(remainingChecked.length).toBe(1);
    expect(remainingChecked[0]).toBeDisabled();
  });
});

describe("ChangeRequestsTab — sorting by date columns", () => {
  it("sorts by a date column when its header is clicked, and flips on a second click", () => {
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 } });
    render(<ChangeRequestsTab />);
    const lastSortBy = (): unknown => mockedUseSearch.mock.calls.at(-1)?.[0]?.sortBy;
    expect(lastSortBy()).toEqual({ field: "createdOn", order: "desc" });

    fireEvent.click(screen.getByText("Planned start"));
    expect(lastSortBy()).toEqual({ field: "plannedStartOn", order: "desc" });
    fireEvent.click(screen.getByText("Planned start"));
    expect(lastSortBy()).toEqual({ field: "plannedStartOn", order: "asc" });

    fireEvent.click(screen.getByText("Updated"));
    expect(lastSortBy()).toEqual({ field: "updatedOn", order: "desc" });
  });
});

// --- Bulk edit, Assign to me, export -----------------------------------------

/** An approved change request (Scheduled): its fields are read-only. */
const CR_APPROVED: BeChangeRequestSearchView = { ...CR, id: "cr-2", number: "CHG-1002", state: "scheduled" };
/** A closed change request: it can't be selected. */
const CR_CLOSED: BeChangeRequestSearchView = { ...CR, id: "cr-3", number: "CHG-1003", state: "closed" };

function showRows(rows: BeChangeRequestSearchView[]): void {
  mockResult({ data: { changeRequests: rows, total: rows.length, limit: 20, offset: 0 } });
}

describe("ChangeRequestsTab — bulk selection", () => {
  it("offers no selection without write access", () => {
    showRows([CR, CR_APPROVED]);
    render(<ChangeRequestsTab />);
    expect(screen.queryByRole("checkbox", { name: "Select CHG-1001" })).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Selected rows" })).not.toBeInTheDocument();
  });

  it("selects the open change requests of the page, never a closed one, and counts them", () => {
    access.canWrite = true;
    showRows([CR, CR_APPROVED, CR_CLOSED]);
    render(<ChangeRequestsTab />);

    expect(screen.getByRole("checkbox", { name: "CHG-1003 is closed and can't be selected" })).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));

    const bar = screen.getByRole("region", { name: "Selected rows" });
    expect(within(bar).getByText("2 change requests selected")).toBeInTheDocument();
    expect(within(bar).getByRole("button", { name: "Edit (2)" })).toBeEnabled();
    expect(within(bar).getByRole("button", { name: "Assign to me (2)" })).toBeEnabled();
    expect(screen.getByRole("checkbox", { name: "Select CHG-1001" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Select CHG-1002" })).toBeChecked();

    fireEvent.click(within(bar).getByRole("button", { name: "Clear selection" }));
    expect(screen.queryByRole("region", { name: "Selected rows" })).not.toBeInTheDocument();
  });

  it("does not open a change request when its checkbox is clicked", () => {
    access.canWrite = true;
    showRows([CR]);
    render(<ChangeRequestsTab />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select CHG-1001" }));
    // Still on the list: the row's own click (navigation) did not fire.
    expect(within(screen.getByRole("region", { name: "Selected rows" })).getByText("1 change request selected")).toBeInTheDocument();
  });
});

describe("ChangeRequestsTab — Assign to me", () => {
  it("assigns the open change request to the signed-in user and skips the approved one, saying why", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) => Promise.resolve(path.endsWith("cr-1") ? CR : CR_APPROVED));
    apiMock.patch.mockResolvedValue({ id: "cr-1" });
    showRows([CR, CR_APPROVED]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));

    await waitFor(() => expect(screen.getByText("Assigned to you 1 of 2 change requests. 1 skipped.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledTimes(1);
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-1", { assignedEngineerId: "user-1" });
    const notUpdated = screen.getByRole("list", { name: "Not updated" });
    expect(within(notUpdated).getByText("CHG-1002")).toBeInTheDocument();
    expect(within(notUpdated).getByText(/Approved, so its assignee can no longer be changed\./)).toBeInTheDocument();
    // The selection is spent.
    expect(screen.queryByRole("region", { name: "Selected rows" })).not.toBeInTheDocument();
  });

  it("reports a change request the backend refuses, in the backend's words, and keeps going", async () => {
    access.canWrite = true;
    const { BackendApiError } = await import("@api/backend/client");
    const other = { ...CR, id: "cr-4", number: "CHG-1004" };
    apiMock.get.mockImplementation((path: string) => Promise.resolve(path.endsWith("cr-1") ? CR : other));
    apiMock.patch.mockImplementation((path: string) =>
      path.endsWith("cr-1")
        ? Promise.reject(new BackendApiError(400, "change request is on hold; take it off hold first"))
        : Promise.resolve({ id: "cr-4" }),
    );
    showRows([CR, other]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));

    await waitFor(() =>
      expect(screen.getByText("Assigned to you 1 of 2 change requests. 1 could not be updated.")).toBeInTheDocument(),
    );
    expect(apiMock.patch).toHaveBeenCalledTimes(2);
    expect(screen.getByText(/change request is on hold; take it off hold first/)).toBeInTheDocument();
  });

  it("skips a change request whose detail can't be read just then, rather than sending it blind", async () => {
    access.canWrite = true;
    const { BackendApiError } = await import("@api/backend/client");
    const other = { ...CR, id: "cr-4", number: "CHG-1004" };
    apiMock.get.mockImplementation((path: string) =>
      path.endsWith("cr-1") ? Promise.reject(new BackendApiError(503, "unavailable")) : Promise.resolve(other),
    );
    apiMock.patch.mockResolvedValue({ id: "cr-4" });
    showRows([CR, other]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));

    await waitFor(() => expect(screen.getByText(/Assigned to you 1 of 2 change requests/)).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledTimes(1);
    expect(apiMock.patch.mock.calls[0][0]).toBe("/change-requests/cr-4");
    expect(screen.getByText(/Couldn't read it just now, so it was not changed. Try again./)).toBeInTheDocument();
  });
});

// These drive the whole edit dialog, which takes a while to render under a loaded suite.
describe("ChangeRequestsTab — bulk edit", { timeout: 20_000 }, () => {
  const detail = (cr: BeChangeRequestSearchView, legalNextStates: string[]): BeChangeRequestDetail => ({
    ...cr,
    legalNextStates,
  });

  it("offers only the moves every selected change request can make, needs a reason to cancel, and sends an approved one only the state and the note", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(
        path.endsWith("cr-1")
          ? detail(CR, ["assess", "canceled"])
          : detail(CR_APPROVED, ["implement", "canceled"]),
      ),
    );
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([CR, CR_APPROVED]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));

    const dialog = await screen.findByRole("dialog", { name: "Edit 2 change requests" });
    expect(within(dialog).getByText(/1 of the selected change requests has been approved/)).toBeInTheDocument();
    await waitFor(() => expect(within(dialog).getByText("Only the moves every selected change request can make.")).toBeInTheDocument());

    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["No change", "Cancel change (Canceled)"]);
    fireEvent.click(screen.getByRole("option", { name: "Cancel change (Canceled)" }));

    // Cancel change needs a reason before anything can be applied.
    const apply = within(dialog).getByRole("button", { name: "Apply to 2 change requests" });
    expect(apply).toBeDisabled();
    expect(within(dialog).getByRole("status")).toHaveTextContent(/A reason is required for this state change/);

    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "Impact" }));
    fireEvent.click(screen.getByRole("option", { name: "High" }));
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Reason \(internal work note\)/ }), {
      target: { value: "  Superseded by a newer change.  " },
    });
    expect(apply).toBeEnabled();
    fireEvent.click(apply);

    await waitFor(() => expect(screen.getByText("Updated 2 of 2 change requests.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-1", {
      state: "canceled",
      impact: "high",
      workNote: "Superseded by a newer change.",
    });
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-2", {
      state: "canceled",
      workNote: "Superseded by a newer change.",
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("says when the selected change requests share no move", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(path.endsWith("cr-1") ? detail(CR, ["assess"]) : detail(CR_APPROVED, ["implement"])),
    );
    showRows([CR, CR_APPROVED]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit 2 change requests" });
    expect(
      await within(dialog).findByText("The selected change requests have no state they can all move to."),
    ).toBeInTheDocument();
    expect(within(dialog).getByRole("combobox", { name: "State" })).toHaveAttribute("aria-disabled", "true");
  });

  it("skips an approved change request when only its fields were asked to change", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(path.endsWith("cr-1") ? detail(CR, ["assess", "canceled"]) : detail(CR_APPROVED, ["implement"])),
    );
    apiMock.patch.mockResolvedValue({ id: "cr-1" });
    showRows([CR, CR_APPROVED]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit 2 change requests" });
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "Priority" }));
    fireEvent.click(screen.getByRole("option", { name: "Critical" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 change requests" }));

    await waitFor(() => expect(screen.getByText("Updated 1 of 2 change requests. 1 skipped.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledTimes(1);
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-1", { priority: "critical" });
    expect(screen.getByText(/Approved, so its fields are read-only/)).toBeInTheDocument();
  });
});

describe("ChangeRequestsTab — export", () => {
  it("offers the export menu for the filtered list", () => {
    showRows([CR]);
    render(<ChangeRequestsTab />);
    expect(screen.getByText("Export Change requests")).toBeInTheDocument();
  });
});

// These drive the whole edit dialog, which takes a while to render under a loaded suite.
describe("ChangeRequestsTab — bulk runs follow the change request as it is now", { timeout: 20_000 }, () => {
  it("plans from the current record: one approved since it was picked is skipped by Assign to me", async () => {
    access.canWrite = true;
    const picked = { ...CR, id: "cr-5", number: "CHG-1005", state: "authorize" };
    // CAB approved it after it was picked: its detail now reads Customer Approval.
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(path.endsWith("cr-5") ? { ...picked, state: "customer_approval" } : CR),
    );
    apiMock.patch.mockResolvedValue({ id: "cr-1" });
    showRows([CR, picked]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (2)" }));

    await waitFor(() => expect(screen.getByText("Assigned to you 1 of 2 change requests. 1 skipped.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledTimes(1);
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-1", { assignedEngineerId: "user-1" });
    expect(screen.getByText(/Approved, so its assignee can no longer be changed/)).toBeInTheDocument();
  });

  it("sends a change request approved since it was picked only its state and note, and counts it as approved", async () => {
    access.canWrite = true;
    const picked = { ...CR, id: "cr-5", number: "CHG-1005", state: "authorize" };
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(
        path.endsWith("cr-5")
          ? { ...picked, state: "customer_approval", legalNextStates: ["authorize", "canceled"] }
          : { ...CR, legalNextStates: ["assess", "canceled"] },
      ),
    );
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([CR, picked]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit 2 change requests" });
    expect(await within(dialog).findByText(/1 of the selected change requests has been approved/)).toBeInTheDocument();
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: "Cancel change (Canceled)" }));
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "Impact" }));
    fireEvent.click(screen.getByRole("option", { name: "Low" }));
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Reason/ }), { target: { value: "Not needed." } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 change requests" }));

    await waitFor(() => expect(screen.getByText("Updated 2 of 2 change requests.")).toBeInTheDocument());
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-1", { state: "canceled", impact: "low", workNote: "Not needed." });
    expect(apiMock.patch).toHaveBeenCalledWith("/change-requests/cr-5", { state: "canceled", workNote: "Not needed." });
  });

  it("lets a change request closed since it was picked be taken out, and leaves it out of the moves", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(
        path.endsWith("cr-1")
          ? { ...CR, legalNextStates: ["assess", "canceled"] }
          : { ...CR_APPROVED, state: "canceled", legalNextStates: [] },
      ),
    );
    showRows([CR, CR_APPROVED]);
    const view = render(<ChangeRequestsTab />);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));

    // The list refetches: CHG-1002 has been canceled meanwhile.
    showRows([CR, { ...CR_APPROVED, state: "canceled" }]);
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ChangeRequestsTab />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit 2 change requests" });
    expect(await within(dialog).findByText(/1 of the selected change requests has been closed, canceled or rolled back/)).toBeInTheDocument();
    // The canceled one no longer narrows the moves to nothing.
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "No change",
      "Request Approval (Assess)",
      "Cancel change (Canceled)",
    ]);
    fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));

    // Its box is still checked and can be unticked.
    const box = screen.getByRole("checkbox", { name: "Deselect CHG-1002, now canceled" });
    expect(box).toBeChecked();
    fireEvent.click(box);
    expect(within(screen.getByRole("region", { name: "Selected rows" })).getByText("1 change request selected")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "CHG-1002 is canceled and can't be selected" })).toBeDisabled();
  });

  it("refreshes each change request the way a single edit does, after the dialog has closed", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation((path: string) =>
      Promise.resolve(path.endsWith("cr-1") ? { ...CR, legalNextStates: ["canceled"] } : { ...CR, id: "cr-4", legalNextStates: ["canceled"] }),
    );
    apiMock.patch.mockResolvedValue({ id: "x" });
    showRows([CR, { ...CR, id: "cr-4", number: "CHG-1004" }]);
    render(<ChangeRequestsTab />);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    fireEvent.click(screen.getByRole("checkbox", { name: "Select all change requests on this page" }));
    fireEvent.click(screen.getByRole("button", { name: "Edit (2)" }));
    const dialog = await screen.findByRole("dialog", { name: "Edit 2 change requests" });
    await within(dialog).findByText("Only the moves every selected change request can make.");
    fireEvent.mouseDown(within(dialog).getByRole("combobox", { name: "State" }));
    fireEvent.click(screen.getByRole("option", { name: "Cancel change (Canceled)" }));
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Reason/ }), { target: { value: "Duplicate." } });
    const getsBefore = apiMock.get.mock.calls.length;
    fireEvent.click(within(dialog).getByRole("button", { name: "Apply to 2 change requests" }));

    await waitFor(() => expect(screen.getByText("Updated 2 of 2 change requests.")).toBeInTheDocument());
    const keys = invalidate.mock.calls.map(([arg]) => JSON.stringify((arg as { queryKey: unknown }).queryKey));
    for (const id of ["cr-1", "cr-4"]) {
      expect(keys).toContain(JSON.stringify(["change-request-details", id]));
      expect(keys).toContain(JSON.stringify(["change-request-approvals", id]));
      expect(keys).toContain(JSON.stringify(["change-request-comments", id]));
    }
    expect(keys).toContain(JSON.stringify(["change-requests"]));
    // Each one is read once, just before it is planned (never from the dialog's cached read);
    // the dialog was gone before the refresh, so its detail reads did not run again after.
    expect(apiMock.get.mock.calls.slice(getsBefore).map(([path]) => path).sort()).toEqual([
      "/change-requests/cr-1",
      "/change-requests/cr-4",
    ]);
  });

  it("moves focus to the outcome after a run, and to the page's select-all box after Clear selection", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation(() => Promise.resolve(CR));
    apiMock.patch.mockResolvedValue({ id: "cr-1" });
    showRows([CR]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select CHG-1001" }));
    fireEvent.click(screen.getByRole("button", { name: "Clear selection" }));
    expect(screen.getByRole("checkbox", { name: "Select all change requests on this page" })).toHaveFocus();

    fireEvent.click(screen.getByRole("checkbox", { name: "Select CHG-1001" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (1)" }));
    await waitFor(() => expect(screen.getByText("Assigned to you 1 of 1 change requests.")).toBeInTheDocument());
    expect(screen.getByText("Assigned to you 1 of 1 change requests.").closest("[role=status]")).toHaveFocus();
  });

  it("keeps the boxes still while a run is in flight, so nothing picked meanwhile is lost", async () => {
    access.canWrite = true;
    apiMock.get.mockImplementation(() => Promise.resolve(CR));
    let release: () => void = () => {};
    apiMock.patch.mockImplementation(() => new Promise((resolve) => (release = () => resolve({ id: "cr-1" }))));
    const other = { ...CR, id: "cr-4", number: "CHG-1004" };
    showRows([CR, other]);
    render(<ChangeRequestsTab />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Select CHG-1001" }));
    fireEvent.click(screen.getByRole("button", { name: "Assign to me (1)" }));
    await waitFor(() => expect(apiMock.patch).toHaveBeenCalled());
    expect(screen.getByRole("checkbox", { name: "Select CHG-1004" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Select all change requests on this page" })).toBeDisabled();
    release();
    await waitFor(() => expect(screen.getByText("Assigned to you 1 of 1 change requests.")).toBeInTheDocument());
    expect(screen.getByRole("checkbox", { name: "Select CHG-1004" })).toBeEnabled();
  });

  it("does not let the select-all box act while a page is loading", () => {
    access.canWrite = true;
    mockResult({ data: { changeRequests: [CR], total: 1, limit: 20, offset: 0 }, isFetching: true });
    render(<ChangeRequestsTab />);
    expect(screen.getByRole("checkbox", { name: "Select all change requests on this page" })).toBeDisabled();
  });
});

describe("ChangeRequestsTab — rows keep the page's own way back", () => {
  // Operations reached from a dashboard is `{ from: <dashboard> }`; a row
  // nests that as `parentState`, so the record's Back can hand it back and
  // the Operations page keeps its own Back button.
  function StateProbe(): JSX.Element {
    const location = useLocation();
    return <div data-testid="state-probe">{JSON.stringify(location.state)}</div>;
  }

  it("passes the page's own state as parentState", async () => {
    showRows([CR]);
    rtlRender(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter initialEntries={[{ pathname: "/operations", search: "?tab=change-requests", state: { from: "/dashboard/my/d-1" } }]}>
          <Routes>
            <Route path="/operations" element={<ChangeRequestsTab />} />
            <Route path="/operations/change-requests/:id" element={<StateProbe />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    fireEvent.click(screen.getByText("CHG-1001"));
    await waitFor(() =>
      expect(JSON.parse(screen.getByTestId("state-probe").textContent ?? "")).toEqual({
        from: "/operations?tab=change-requests",
        parentState: { from: "/dashboard/my/d-1" },
      }),
    );
  });
});
