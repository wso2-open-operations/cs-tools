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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

interface ListState {
  data?: { id: string; name: string; createdOn: string; updatedOn: string }[];
  isLoading?: boolean;
  isFetching?: boolean;
  isError?: boolean;
}
interface DetailState {
  data?: unknown;
  isLoading?: boolean;
  isError?: boolean;
  error?: Error;
}
let listState: ListState;
let detailState: DetailState;
const detailIds: (string | undefined)[] = [];
const deleteMock = vi.fn();
let deletePending = false;
const showSuccess = vi.fn();
const showError = vi.fn();

vi.mock("@features/csm-custom-dashboards/api/useCustomDashboards", () => ({
  useGetCustomDashboards: () => listState,
  useGetCustomDashboard: (id: string | undefined) => {
    detailIds.push(id);
    return detailState;
  },
  useDeleteCustomDashboard: () => ({ mutateAsync: deleteMock, isPending: deletePending }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess }) }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError }) }));
// The widgets fetch their own data; the grid is not under test here.
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets }: { widgets: { widgetId: string }[] }) => (
    <div>Grid: {widgets.map((w) => w.widgetId).join(", ")}</div>
  ),
}));

vi.mock("@hooks/useIdTokenClaims", () => ({ useIdTokenClaims: () => ({ email: "jane.doe@example.com" }) }));
// The dialog has its own tests; here only whether and for what it opens.
vi.mock("@features/csm-custom-dashboards/components/ShareCustomDashboardDialog", () => ({
  default: ({ open, dashboard, currentUserEmail }: { open: boolean; dashboard: { name: string }; currentUserEmail?: string }) =>
    open ? (
      <div>
        Share dialog for {dashboard.name} as {currentUserEmail}
      </div>
    ) : null,
}));

import MyDashboardsPage from "@features/csm-custom-dashboards/pages/MyDashboardsPage";

const A = { id: "5e636a12-0000-4000-8000-00000000000a", name: "Alpha", createdOn: "x", updatedOn: "y" };
const B = { id: "5e636a12-0000-4000-8000-00000000000b", name: "Beta", createdOn: "x", updatedOn: "y" };

function savedWith(widgetId: string) {
  return { dashboard: { id: "any", displayName: "any", isDefault: false, isTeamBased: false, widgets: [{ widgetId }] }, createdOn: "x", updatedOn: "y" };
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/dashboard/my" element={<MyDashboardsPage />} />
        <Route path="/dashboard/my/:dashboardId" element={<MyDashboardsPage />} />
        <Route path="/dashboard/my/new" element={<div>Generate page</div>} />
        <Route path="/dashboard/my/:dashboardId/edit" element={<div>Edit page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("MyDashboardsPage", () => {
  beforeEach(() => {
    listState = { data: [A, B] };
    detailState = { data: savedWith("w1") };
    detailIds.length = 0;
    deleteMock.mockReset();
    deletePending = false;
    showSuccess.mockReset();
    showError.mockReset();
  });

  it("shows the empty state with the Generate Dashboard button when nothing is saved", () => {
    listState = { data: [] };
    renderAt("/dashboard/my");
    expect(screen.getByText("You have no dashboards yet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Generate Dashboard" })).toBeInTheDocument();
  });

  it("opens the first saved dashboard when the URL names none", () => {
    renderAt("/dashboard/my");
    expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
    expect(detailIds).toContain(A.id);
  });

  it("draws the dashboard the URL names, with its widgets", () => {
    detailState = { data: savedWith("open-cases") };
    renderAt(`/dashboard/my/${B.id}`);
    expect(screen.getByText("Beta", { selector: "p" })).toBeInTheDocument();
    expect(screen.getByText("Grid: open-cases")).toBeInTheDocument();
    expect(detailIds).toContain(B.id);
  });

  it("waits for the list to be re-read before giving up on an unknown id", () => {
    listState = { data: [A], isFetching: true };
    renderAt(`/dashboard/my/${B.id}`);
    expect(detailIds).not.toContain(A.id);
    expect(screen.queryByText("Alpha", { selector: "p" })).not.toBeInTheDocument();
  });

  it("opens the first dashboard for an id that is not in the list", () => {
    listState = { data: [A] };
    renderAt(`/dashboard/my/${B.id}`);
    expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
  });

  it("shows the backend's message when a dashboard can't be shown", () => {
    detailState = { isError: true, error: new Error("This saved dashboard can't be shown any more. Generate it again.") };
    renderAt(`/dashboard/my/${A.id}`);
    expect(screen.getByText("This saved dashboard can't be shown any more. Generate it again.")).toBeInTheDocument();
  });

  it("says so when the dashboard no longer exists", () => {
    detailState = { data: null };
    renderAt(`/dashboard/my/${A.id}`);
    expect(screen.getByText("This dashboard no longer exists.")).toBeInTheDocument();
  });

  it("says so when the list can't be loaded", () => {
    listState = { isError: true };
    renderAt("/dashboard/my");
    expect(screen.getByText("Could not load the dashboard list.")).toBeInTheDocument();
  });

  it("Share opens the share dialog for the open dashboard", () => {
    renderAt(`/dashboard/my/${B.id}`);
    expect(screen.queryByText(/Share dialog for/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Share" }));
    expect(screen.getByText("Share dialog for Beta as jane.doe@example.com")).toBeInTheDocument();
  });

  it("Edit with AI opens the open dashboard's edit page", () => {
    renderAt(`/dashboard/my/${B.id}`);
    fireEvent.click(screen.getByRole("button", { name: "Edit with AI" }));
    expect(screen.getByText("Edit page")).toBeInTheDocument();
  });

  describe("Search in the switcher", () => {
    const C = { id: "5e636a12-0000-4000-8000-00000000000c", name: "Customer SLA Breaches", createdOn: "x", updatedOn: "y" };

    function openSwitcher() {
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      return screen.getByRole("textbox", { name: "Search dashboards" });
    }

    // The entries a user can see and pick: hidden, non-matching ones are not.
    // MUI's Select gives every child role="option", the search row's header
    // too (it ignores clicks on it: a header has no tabindex), so that row is
    // left out here.
    function visibleEntries(): string[] {
      const box = screen.getByRole("textbox", { name: "Search dashboards" });
      return screen
        .getAllByRole("option")
        .filter((o) => !o.contains(box) && o.getAttribute("aria-disabled") !== "true")
        .map((o) => o.textContent ?? "");
    }

    beforeEach(() => {
      listState = { data: [A, B, C] };
    });

    it("focuses the search box when the menu opens, with every dashboard listed", () => {
      renderAt(`/dashboard/my/${A.id}`);
      const box = openSwitcher();
      expect(box).toHaveFocus();
      expect(visibleEntries()).toEqual(["Alpha", "Beta", "Customer SLA Breaches"]);
    });

    it("filters by name as you type, ignoring case", () => {
      renderAt(`/dashboard/my/${A.id}`);
      fireEvent.change(openSwitcher(), { target: { value: "sla" } });
      expect(visibleEntries()).toEqual(["Customer SLA Breaches"]);
    });

    it("says so when nothing matches", () => {
      renderAt(`/dashboard/my/${A.id}`);
      fireEvent.change(openSwitcher(), { target: { value: "zzz" } });
      expect(visibleEntries()).toEqual([]);
      expect(screen.getByText("No dashboards match “zzz”")).toBeInTheDocument();
    });

    it("opens a dashboard picked from the filtered list", () => {
      renderAt(`/dashboard/my/${A.id}`);
      fireEvent.change(openSwitcher(), { target: { value: "beta" } });
      fireEvent.click(screen.getByRole("option", { name: /Beta/ }));
      expect(screen.getByText("Beta", { selector: "p" })).toBeInTheDocument();
    });

    it("is not a selection: clicking it keeps the open dashboard and the menu", () => {
      renderAt(`/dashboard/my/${A.id}`);
      fireEvent.click(openSwitcher());
      expect(screen.getByRole("listbox")).toBeInTheDocument();
      expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
    });

    it("keeps typed keys in the box instead of jumping between entries", () => {
      renderAt(`/dashboard/my/${A.id}`);
      const box = openSwitcher();
      fireEvent.keyDown(box, { key: "b" });
      expect(box).toHaveFocus();
    });

    it("starts empty again the next time the menu opens", async () => {
      renderAt(`/dashboard/my/${A.id}`);
      fireEvent.change(openSwitcher(), { target: { value: "sla" } });
      fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });
      await waitFor(() => expect(screen.queryByRole("listbox")).not.toBeInTheDocument());

      expect(openSwitcher()).toHaveValue("");
      expect(visibleEntries()).toEqual(["Alpha", "Beta", "Customer SLA Breaches"]);
    });
  });

  describe("Delete, from the switcher's entries", () => {
    // Opens the switcher and clicks the delete icon of the named entry.
    function deleteFromMenu(name: string) {
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      fireEvent.click(screen.getByRole("button", { name: `Delete dashboard ${name}` }));
    }

    it("has a delete icon on each entry, and none in the header", () => {
      renderAt(`/dashboard/my/${A.id}`);
      expect(screen.queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      expect(screen.getByRole("button", { name: "Delete dashboard Alpha" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Delete dashboard Beta" })).toBeInTheDocument();
    });

    it("asks first without switching, and Cancel deletes nothing", async () => {
      renderAt(`/dashboard/my/${A.id}`);
      deleteFromMenu("Beta");

      expect(screen.getByText("Delete this dashboard?")).toBeInTheDocument();
      expect(screen.getByText(/“Beta” will be removed from My Dashboards/)).toBeInTheDocument();
      // Still on Alpha: the icon is not a selection. (The open dialog hides the
      // page from the accessibility tree, hence `hidden`.)
      expect(screen.getByRole("combobox", { name: "Select dashboard", hidden: true })).toHaveTextContent("Alpha");

      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      await waitFor(() => expect(screen.queryByText("Delete this dashboard?")).not.toBeInTheDocument());
      expect(deleteMock).not.toHaveBeenCalled();
    });

    it("deleting the open dashboard goes back to My Dashboards", async () => {
      deleteMock.mockResolvedValue(null);
      renderAt(`/dashboard/my/${B.id}`);
      deleteFromMenu("Beta");
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));

      await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Beta" was deleted.'));
      expect(deleteMock).toHaveBeenCalledWith(B.id);
      // Back at /dashboard/my, which opens the first one in the list.
      await waitFor(() => expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument());
    });

    it("deleting another dashboard keeps the open one", async () => {
      deleteMock.mockResolvedValue(null);
      renderAt(`/dashboard/my/${B.id}`);
      deleteFromMenu("Alpha");
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));

      await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Alpha" was deleted.'));
      expect(deleteMock).toHaveBeenCalledWith(A.id);
      expect(screen.getByText("Beta", { selector: "p" })).toBeInTheDocument();
    });

    it("shows the error banner when the delete fails", async () => {
      const err = new Error("boom");
      deleteMock.mockRejectedValue(err);
      renderAt(`/dashboard/my/${A.id}`);
      deleteFromMenu("Alpha");
      fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));

      await waitFor(() =>
        expect(showError).toHaveBeenCalledWith("The dashboard couldn't be deleted. Try again in a moment.", err),
      );
      expect(showSuccess).not.toHaveBeenCalled();
    });

    it("disables the dialog's buttons while the delete is in flight", () => {
      deletePending = true;
      renderAt(`/dashboard/my/${A.id}`);
      deleteFromMenu("Alpha");
      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByRole("button", { name: "Deleting…" })).toBeDisabled();
      expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
    });
  });
});
