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

import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router";
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
const renameMock = vi.fn();
let renamePending = false;
const showSuccess = vi.fn();
const showError = vi.fn();
// Advanced dashboards: off unless a test turns them on, so the rest of the
// page is tested as every other user sees it.
let canUseAdvanced = false;
let codeDashboards: { id: string; name: string; createdOn: string; updatedOn: string }[] = [];
const codeListEnabled: boolean[] = [];
let codeListExtra: { isLoading?: boolean; isFetching?: boolean; isError?: boolean } = {};
let codeDetailState: DetailState;
const codeDetailIds: (string | undefined)[] = [];
const deleteCodeMock = vi.fn();
let deleteCodePending = false;
const renameCodeMock = vi.fn();
let renameCodePending = false;

vi.mock("@features/csm-custom-dashboards/api/useCustomDashboards", () => ({
  useGetCustomDashboards: () => listState,
  useGetCustomDashboard: (id: string | undefined) => {
    detailIds.push(id);
    return detailState;
  },
  useDeleteCustomDashboard: () => ({ mutateAsync: deleteMock, isPending: deletePending }),
  useRenameCustomDashboard: () => ({ mutateAsync: renameMock, isPending: renamePending }),
}));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboards", () => ({
  // As the real hook: no data while it is turned off.
  useGetCodeDashboards: (enabled: boolean) => {
    codeListEnabled.push(enabled);
    if (!enabled) return { data: undefined };
    return { data: codeListExtra.isLoading || codeListExtra.isError ? undefined : codeDashboards, ...codeListExtra };
  },
  useGetCodeDashboard: (id: string | undefined) => {
    codeDetailIds.push(id);
    return codeDetailState;
  },
  savedCodeDashboardPath: (id: string) => `/dashboard/my/code/${encodeURIComponent(id)}`,
  useDeleteCodeDashboard: () => ({ mutateAsync: deleteCodeMock, isPending: deleteCodePending }),
  useRenameCodeDashboard: () => ({ mutateAsync: renameCodeMock, isPending: renameCodePending }),
}));
vi.mock("@features/csm-custom-dashboards/utils/advancedDashboardAccess", () => ({
  useCanGenerateAdvancedDashboards: () => canUseAdvanced,
}));
// The real client reads runtime config at module load; the rename error
// wording only needs the error class (same stand-in as saveErrorMessage.test.ts).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({ useSuccessBanner: () => ({ showSuccess }) }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError }) }));
// The widgets fetch their own data; the grid is not under test here.
vi.mock("@features/csm-dashboard/components/DashboardWidgetGrid", () => ({
  default: ({ widgets, linePointClickThrough }: { widgets: { widgetId: string }[]; linePointClickThrough?: boolean }) => (
    <div data-line-points={String(Boolean(linePointClickThrough))}>Grid: {widgets.map((w) => w.widgetId).join(", ")}</div>
  ),
}));

vi.mock("@hooks/useIdTokenClaims", () => ({ useIdTokenClaims: () => ({ email: "jane.doe@example.com" }) }));
// The frame runs the code against the portal; it is tested on its own.
vi.mock("@features/csm-custom-dashboards/components/CodeDashboardFrame", () => ({
  default: ({ source }: { source: string }) => <div>frame running {source}</div>,
}));
const { CODE_SHARING } = vi.hoisted(() => ({ CODE_SHARING: { kind: "advanced dashboard hooks" } }));
vi.mock("@features/csm-custom-dashboards/api/useCodeDashboardShares", () => ({ CODE_DASHBOARD_SHARING: CODE_SHARING }));
// The dialog has its own tests; here only whether, for what and with which
// hooks it opens. Like the real one, it keeps the hooks it was first given.
vi.mock("@features/csm-custom-dashboards/components/ShareCustomDashboardDialog", async () => {
  const { useState } = await import("react");
  return {
    default: function ShareDialog({
      open,
      dashboard,
      currentUserEmail,
      hooks,
    }: {
      open: boolean;
      dashboard: { name: string };
      currentUserEmail?: string;
      hooks?: unknown;
    }) {
      const [kind] = useState(hooks === CODE_SHARING ? "advanced" : hooks === undefined ? "custom" : "other");
      return open ? (
        <div>
          <div>
            Share dialog for {dashboard.name} as {currentUserEmail}
          </div>
          <div>Shares through the {kind} hooks</div>
        </div>
      ) : null;
    },
  };
});

import MyDashboardsPage from "@features/csm-custom-dashboards/pages/MyDashboardsPage";

/** Stands in for the advanced page editing a saved dashboard: shows the id it was opened with. */
function CodeEditPage() {
  const { dashboardId } = useParams();
  return <div>Advanced edit page {dashboardId}</div>;
}
import { BackendApiError } from "@api/backend/client";

const A = { id: "5e636a12-0000-4000-8000-00000000000a", name: "Alpha", createdOn: "x", updatedOn: "y" };
const B = { id: "5e636a12-0000-4000-8000-00000000000b", name: "Beta", createdOn: "x", updatedOn: "y" };
const SOURCE = "async function render(portal, root) { root.textContent = 'hi'; }";

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
        <Route path="/dashboard/my/code/:codeDashboardId" element={<MyDashboardsPage />} />
        <Route path="/dashboard/my/code/:dashboardId/edit" element={<CodeEditPage />} />
        <Route path="/404" element={<div>Not found</div>} />
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
    renameMock.mockReset();
    renamePending = false;
    deleteCodeMock.mockReset();
    deleteCodePending = false;
    renameCodeMock.mockReset();
    renameCodePending = false;
    showSuccess.mockReset();
    showError.mockReset();
    canUseAdvanced = false;
    codeDashboards = [];
    codeListEnabled.length = 0;
    codeListExtra = {};
    codeDetailState = { data: null };
    codeDetailIds.length = 0;
  });

  it("shows the empty state with the Generate Dashboard button when nothing is saved", () => {
    listState = { data: [] };
    renderAt("/dashboard/my");
    expect(screen.getByText("You have no dashboards yet")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Generate Dashboard" })).toBeInTheDocument();
  });

  it("offers only Generate Dashboard, even with the code approach on: the advanced page is opened from the generate page", () => {
    const original = window.config;
    try {
      window.config = { ...original, DASHBOARD_GENERATE_CODE_ENABLED: true };
      for (const data of [[], [A, B]]) {
        listState = { data };
        renderAt("/dashboard/my");
        expect(screen.getByRole("button", { name: "Generate Dashboard" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Generate Advanced Dashboard" })).not.toBeInTheDocument();
        cleanup();
      }
    } finally {
      window.config = original;
    }
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
    // A line widget's points open that day's cases on a saved dashboard.
    expect(screen.getByText("Grid: open-cases")).toHaveAttribute("data-line-points", "true");
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

  describe("Rename, from the switcher's entries", () => {
    // Opens the switcher and clicks the rename icon of the named entry.
    function renameFromMenu(name: string) {
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      fireEvent.click(screen.getByRole("button", { name: `Rename dashboard ${name}` }));
    }
    function nameField() {
      return within(screen.getByRole("dialog")).getByRole("textbox", { name: "Name" });
    }
    function renameButton() {
      return within(screen.getByRole("dialog")).getByRole("button", { name: "Rename" });
    }

    it("has a rename icon on each entry, beside the delete icon", () => {
      renderAt(`/dashboard/my/${A.id}`);
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      expect(screen.getByRole("button", { name: "Rename dashboard Alpha" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Rename dashboard Beta" })).toBeInTheDocument();
    });

    it("opens with the entry's name without switching, needs a change, and Cancel renames nothing", async () => {
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Beta");

      expect(screen.getByText("Rename dashboard")).toBeInTheDocument();
      expect(nameField()).toHaveValue("Beta");
      expect(screen.getByRole("combobox", { name: "Select dashboard", hidden: true })).toHaveTextContent("Alpha");
      // The same name, or only spaces added, is not a rename; an empty one is refused.
      expect(renameButton()).toBeDisabled();
      fireEvent.change(nameField(), { target: { value: "  Beta " } });
      expect(renameButton()).toBeDisabled();
      fireEvent.change(nameField(), { target: { value: "   " } });
      expect(renameButton()).toBeDisabled();

      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(renameMock).not.toHaveBeenCalled();
    });

    it("renames with the trimmed name, closes and shows the success banner", async () => {
      renameMock.mockResolvedValue({});
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Beta");
      fireEvent.change(nameField(), { target: { value: "  Gamma  " } });
      fireEvent.click(renameButton());

      await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Beta" was renamed to "Gamma".'));
      expect(renameMock).toHaveBeenCalledWith({ id: B.id, name: "Gamma" });
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    });

    it("Enter in the field renames", async () => {
      renameMock.mockResolvedValue({});
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Alpha");
      fireEvent.change(nameField(), { target: { value: "Gamma" } });
      fireEvent.submit(nameField());

      await waitFor(() => expect(renameMock).toHaveBeenCalledWith({ id: A.id, name: "Gamma" }));
    });

    it("shows a taken name in the dialog, which stays open, and hides it once the name changes", async () => {
      renameMock.mockRejectedValue(new BackendApiError(409, "you already have a dashboard named: Beta"));
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Alpha");
      fireEvent.change(nameField(), { target: { value: "Beta" } });
      fireEvent.click(renameButton());

      const taken = 'You already have a dashboard named "Beta". Choose a different name.';
      expect(await within(screen.getByRole("dialog")).findByText(taken)).toBeInTheDocument();
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showError).not.toHaveBeenCalled();

      fireEvent.change(nameField(), { target: { value: "Beta 2" } });
      expect(screen.queryByText(taken)).not.toBeInTheDocument();
    });

    it("shows the backend's message in the dialog for a dashboard that can't be saved again", async () => {
      renameMock.mockRejectedValue(
        new BackendApiError(422, "This saved dashboard can't be shown any more. Generate it again."),
      );
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Alpha");
      fireEvent.change(nameField(), { target: { value: "Gamma" } });
      fireEvent.click(renameButton());

      expect(
        await within(screen.getByRole("dialog")).findByText(
          "This saved dashboard can't be shown any more. Generate it again.",
        ),
      ).toBeInTheDocument();
    });

    it("starts from the entry's name again the next time it opens", async () => {
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Alpha");
      fireEvent.change(nameField(), { target: { value: "Half typed" } });
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

      renameFromMenu("Alpha");
      expect(nameField()).toHaveValue("Alpha");
    });

    it("refuses a name over 255 characters", () => {
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Alpha");
      fireEvent.change(nameField(), { target: { value: "x".repeat(256) } });
      expect(renameButton()).toBeDisabled();
      expect(screen.getByText("Use at most 255 characters.")).toBeInTheDocument();

      fireEvent.change(nameField(), { target: { value: "x".repeat(255) } });
      expect(renameButton()).toBeEnabled();
    });

    it("disables the field and the dialog's buttons while the rename is in flight", () => {
      renamePending = true;
      renderAt(`/dashboard/my/${A.id}`);
      renameFromMenu("Alpha");
      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByRole("button", { name: "Renaming…" })).toBeDisabled();
      expect(within(dialog).getByRole("button", { name: "Cancel" })).toBeDisabled();
      expect(nameField()).toBeDisabled();
    });
  });

  describe("Advanced dashboards", () => {
    const X = { id: "c0de0000-0000-4000-8000-00000000000x", name: "Severity Trend", createdOn: "x", updatedOn: "y" };
    const Y = { id: "c0de0000-0000-4000-8000-00000000000y", name: "Open by Region", createdOn: "x", updatedOn: "y" };

    function openSwitcher() {
      fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
      return screen.getByRole("textbox", { name: "Search dashboards" });
    }

    /** The switcher's visible entries and the Advanced header, in order (the search row left out). */
    function visibleRows(): string[] {
      const box = screen.getByRole("textbox", { name: "Search dashboards" });
      return screen
        .getAllByRole("option")
        .filter((o) => !o.contains(box) && o.getAttribute("aria-disabled") !== "true")
        .map((o) => o.textContent ?? "");
    }

    beforeEach(() => {
      codeDashboards = [X, Y];
    });

    it("reads no advanced list, and lists none, for a user who cannot use them", () => {
      renderAt(`/dashboard/my/${A.id}`);
      expect(codeListEnabled.every((e) => e === false)).toBe(true);
      openSwitcher();
      expect(visibleRows()).toEqual(["Alpha", "Beta"]);
      expect(screen.queryByText("Advanced")).not.toBeInTheDocument();
    });

    it("lists them after the user's dashboards, under Advanced", () => {
      canUseAdvanced = true;
      renderAt(`/dashboard/my/${A.id}`);
      expect(codeListEnabled).toContain(true);
      openSwitcher();
      expect(visibleRows()).toEqual(["Alpha", "Beta", "Advanced", "Severity Trend", "Open by Region"]);
    });

    it("opens the one picked here, and a custom one again", () => {
      canUseAdvanced = true;
      codeDetailState = { data: { id: Y.id, name: Y.name, source: SOURCE } };
      renderAt(`/dashboard/my/${A.id}`);
      openSwitcher();
      fireEvent.click(screen.getByRole("option", { name: /Open by Region/ }));
      expect(screen.getByText("Open by Region", { selector: "p" })).toBeInTheDocument();
      expect(codeDetailIds).toContain(Y.id);
      openSwitcher();
      fireEvent.click(screen.getByRole("option", { name: /Alpha/ }));
      expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
      expect(screen.getByText("Grid: w1")).toBeInTheDocument();
    });

    it("filters them with the same search, hiding Advanced when none match", () => {
      canUseAdvanced = true;
      renderAt(`/dashboard/my/${A.id}`);
      const box = openSwitcher();
      fireEvent.change(box, { target: { value: "REGION" } });
      expect(visibleRows()).toEqual(["Advanced", "Open by Region"]);
      expect(screen.queryByText("No dashboards match", { exact: false })).not.toBeInTheDocument();
      fireEvent.change(box, { target: { value: "alp" } });
      expect(visibleRows()).toEqual(["Alpha"]);
      expect(screen.queryByText("Advanced")).not.toBeInTheDocument();
      fireEvent.change(box, { target: { value: "nothing" } });
      expect(screen.getByText("No dashboards match", { exact: false })).toBeInTheDocument();
    });

    describe("Rename and delete, from their entries", () => {
      function dialog() {
        return screen.getByRole("dialog");
      }
      function iconFromMenu(action: "Rename" | "Delete", name: string) {
        openSwitcher();
        fireEvent.click(screen.getByRole("button", { name: `${action} dashboard ${name}` }));
      }

      beforeEach(() => {
        canUseAdvanced = true;
      });

      it("has a rename and a delete icon on each advanced entry", () => {
        renderAt(`/dashboard/my/${A.id}`);
        openSwitcher();
        for (const name of [X.name, Y.name]) {
          expect(screen.getByRole("button", { name: `Rename dashboard ${name}` })).toBeInTheDocument();
          expect(screen.getByRole("button", { name: `Delete dashboard ${name}` })).toBeInTheDocument();
        }
      });

      it("deletes through the advanced call after asking, without switching, and keeps the open dashboard", async () => {
        deleteCodeMock.mockResolvedValue(null);
        // Beta, not the first one, so going back to My Dashboards would show.
        renderAt(`/dashboard/my/${B.id}`);
        iconFromMenu("Delete", Y.name);
        expect(within(dialog()).getByText(/“Open by Region” will be removed from My Dashboards/)).toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Select dashboard", hidden: true })).toHaveTextContent("Beta");
        fireEvent.click(within(dialog()).getByRole("button", { name: "Delete" }));

        await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Open by Region" was deleted.'));
        expect(deleteCodeMock).toHaveBeenCalledWith(Y.id);
        expect(deleteMock).not.toHaveBeenCalled();
        await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
        expect(screen.getByText("Beta", { selector: "p" })).toBeInTheDocument();
      });

      it("shows the error banner when the delete fails", async () => {
        const err = new Error("boom");
        deleteCodeMock.mockRejectedValue(err);
        renderAt(`/dashboard/my/${A.id}`);
        iconFromMenu("Delete", X.name);
        fireEvent.click(within(dialog()).getByRole("button", { name: "Delete" }));

        await waitFor(() =>
          expect(showError).toHaveBeenCalledWith("The dashboard couldn't be deleted. Try again in a moment.", err),
        );
        expect(showSuccess).not.toHaveBeenCalled();
      });

      it("disables the delete dialog while the advanced delete is in flight", () => {
        deleteCodePending = true;
        renderAt(`/dashboard/my/${A.id}`);
        iconFromMenu("Delete", X.name);
        expect(within(dialog()).getByRole("button", { name: "Deleting…" })).toBeDisabled();
      });

      it("renames through the advanced call, starting from its name, and shows the success banner", async () => {
        renameCodeMock.mockResolvedValue({});
        renderAt(`/dashboard/my/${A.id}`);
        iconFromMenu("Rename", X.name);
        const field = within(dialog()).getByRole("textbox", { name: "Name" });
        expect(field).toHaveValue("Severity Trend");
        fireEvent.change(field, { target: { value: " Severity by Week " } });
        fireEvent.click(within(dialog()).getByRole("button", { name: "Rename" }));

        await waitFor(() =>
          expect(showSuccess).toHaveBeenCalledWith('"Severity Trend" was renamed to "Severity by Week".'),
        );
        expect(renameCodeMock).toHaveBeenCalledWith({ id: X.id, name: "Severity by Week" });
        expect(renameMock).not.toHaveBeenCalled();
        await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      });

      it("shows a taken name in the dialog, which stays open", async () => {
        renameCodeMock.mockRejectedValue(new BackendApiError(409, "you already have an advanced dashboard named: Open by Region"));
        renderAt(`/dashboard/my/${A.id}`);
        iconFromMenu("Rename", X.name);
        fireEvent.change(within(dialog()).getByRole("textbox", { name: "Name" }), { target: { value: "Open by Region" } });
        fireEvent.click(within(dialog()).getByRole("button", { name: "Rename" }));

        expect(
          await within(dialog()).findByText('You already have a dashboard named "Open by Region". Choose a different name.'),
        ).toBeInTheDocument();
        expect(showSuccess).not.toHaveBeenCalled();
      });

      it("disables the rename dialog while the advanced rename is in flight", () => {
        renameCodePending = true;
        renderAt(`/dashboard/my/${A.id}`);
        iconFromMenu("Rename", X.name);
        expect(within(dialog()).getByRole("button", { name: "Renaming…" })).toBeDisabled();
      });
    });

    it("keeps the empty state as it was without any", () => {
      canUseAdvanced = true;
      codeDashboards = [];
      listState = { data: [] };
      renderAt("/dashboard/my");
      expect(screen.getByText("You have no dashboards yet")).toBeInTheDocument();
      expect(screen.queryByText(/Could not load/)).not.toBeInTheDocument();
    });

    describe("Open in My Dashboards", () => {
      beforeEach(() => {
        canUseAdvanced = true;
        codeDetailState = { data: { id: X.id, name: X.name, source: SOURCE } };
      });

      /** The header's buttons, in order, by name. */
      function headerButtons(): string[] {
        return screen
          .getAllByRole("button")
          .map((b) => b.textContent ?? "")
          .filter((n) => ["Share", "Edit with AI", "Generate Dashboard", "Edit", "Delete"].includes(n));
      }

      it("shows it under My Dashboards with the custom ones' header and buttons, and runs its code", () => {
        renderAt(`/dashboard/my/code/${X.id}`);
        expect(screen.getByRole("heading", { name: "My Dashboards" })).toBeInTheDocument();
        expect(screen.getByText("Severity Trend", { selector: "p" })).toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Select dashboard" })).toHaveTextContent("Severity Trend");
        expect(headerButtons()).toEqual(["Share", "Edit with AI", "Generate Dashboard"]);
        openSwitcher();
        expect(screen.getByRole("option", { name: /Severity Trend/ })).toHaveAttribute("aria-selected", "true");
        fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });
        expect(screen.queryByRole("button", { name: /Back to My Dashboards/ })).not.toBeInTheDocument();
        expect(codeDetailIds).toContain(X.id);
        const panel = screen.getByTestId("saved-code-dashboard-panel");
        expect(panel).toHaveStyle({ height: "calc(100dvh - 240px)", minHeight: "520px", display: "flex", flexDirection: "column" });
        expect(panel).toContainElement(screen.getByText(`frame running ${SOURCE}`));
        expect(screen.queryByText(/^Grid:/)).not.toBeInTheDocument();
        expect(detailIds.every((id) => id === undefined)).toBe(true);
      });

      it("shows the same buttons, in the same order, for a custom one", () => {
        renderAt(`/dashboard/my/${A.id}`);
        expect(headerButtons()).toEqual(["Share", "Edit with AI", "Generate Dashboard"]);
      });

      it("Share opens the share dialog for it, with the advanced dashboards' hooks", () => {
        renderAt(`/dashboard/my/code/${X.id}`);
        fireEvent.click(screen.getByRole("button", { name: "Share" }));
        expect(screen.getByText("Share dialog for Severity Trend as jane.doe@example.com")).toBeInTheDocument();
        expect(screen.getByText("Shares through the advanced hooks")).toBeInTheDocument();
      });

      it("Share uses the custom hooks for a custom one, and the advanced ones after switching to an advanced one", () => {
        renderAt(`/dashboard/my/${A.id}`);
        fireEvent.click(screen.getByRole("button", { name: "Share" }));
        expect(screen.getByText("Shares through the custom hooks")).toBeInTheDocument();
        openSwitcher();
        fireEvent.click(screen.getByRole("option", { name: /Severity Trend/ }));
        fireEvent.click(screen.getByRole("button", { name: "Share" }));
        expect(screen.getByText("Shares through the advanced hooks")).toBeInTheDocument();
      });

      it("Edit with AI opens it on the advanced page", () => {
        renderAt(`/dashboard/my/code/${X.id}`);
        fireEvent.click(screen.getByRole("button", { name: "Edit with AI" }));
        expect(screen.getByText(`Advanced edit page ${X.id}`)).toBeInTheDocument();
      });

      it("Generate Dashboard opens the generate page", () => {
        renderAt(`/dashboard/my/code/${X.id}`);
        fireEvent.click(screen.getByRole("button", { name: "Generate Dashboard" }));
        expect(screen.getByText("Generate page")).toBeInTheDocument();
      });

      it("says when it no longer exists, or why it can't be shown", () => {
        codeDetailState = { data: null };
        renderAt(`/dashboard/my/code/${X.id}`);
        expect(screen.getByText("This dashboard no longer exists.")).toBeInTheDocument();
        cleanup();
        codeDetailState = { isError: true, error: new Error("The dashboard couldn't be loaded.") };
        renderAt(`/dashboard/my/code/${X.id}`);
        expect(screen.getByText("The dashboard couldn't be loaded.")).toBeInTheDocument();
      });

      it("deleting it goes back to My Dashboards", async () => {
        deleteCodeMock.mockResolvedValue(null);
        renderAt(`/dashboard/my/code/${X.id}`);
        openSwitcher();
        fireEvent.click(screen.getByRole("button", { name: "Delete dashboard Severity Trend" }));
        fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
        await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Severity Trend" was deleted.'));
        await waitFor(() => expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument());
      });

      it("deleting another one keeps it open", async () => {
        deleteCodeMock.mockResolvedValue(null);
        renderAt(`/dashboard/my/code/${X.id}`);
        openSwitcher();
        fireEvent.click(screen.getByRole("button", { name: "Delete dashboard Open by Region" }));
        fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
        await waitFor(() => expect(showSuccess).toHaveBeenCalledWith('"Open by Region" was deleted.'));
        expect(screen.getByText("Severity Trend", { selector: "p" })).toBeInTheDocument();
      });

      it("opens it for its id in upper case", () => {
        renderAt(`/dashboard/my/code/${X.id.toUpperCase()}`);
        expect(screen.getByText("Severity Trend", { selector: "p" })).toBeInTheDocument();
      });

      it("opens the first custom one for an advanced id that is not in the list", () => {
        renderAt("/dashboard/my/code/c0de0000-0000-4000-8000-0000000000ff");
        expect(screen.getByText("Alpha", { selector: "p" })).toBeInTheDocument();
      });

      it("waits for the advanced list while it is re-read, for an advanced id not in it", () => {
        codeListExtra = { isFetching: true };
        renderAt("/dashboard/my/code/c0de0000-0000-4000-8000-0000000000ff");
        expect(screen.queryByText("Alpha", { selector: "p" })).not.toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "My Dashboards" })).not.toBeInTheDocument();
      });

      it("waits while the advanced list first loads", () => {
        listState = { data: [] };
        codeListExtra = { isLoading: true };
        renderAt("/dashboard/my");
        expect(screen.queryByText("You have no dashboards yet")).not.toBeInTheDocument();
      });

      it("opens the first advanced one when the user has no custom one", () => {
        listState = { data: [] };
        renderAt("/dashboard/my");
        expect(screen.queryByText("You have no dashboards yet")).not.toBeInTheDocument();
        expect(screen.getByText("Severity Trend", { selector: "p" })).toBeInTheDocument();
        expect(codeDetailIds).toContain(X.id);
      });

      it("keeps the custom ones when the advanced list fails, and says so", () => {
        codeListExtra = { isError: true };
        renderAt(`/dashboard/my/${A.id}`);
        expect(screen.getByText("Grid: w1")).toBeInTheDocument();
        expect(screen.getByText("Could not load your advanced dashboards.")).toBeInTheDocument();
      });

      it("says so under the empty state when the user has no custom one and the advanced list fails", () => {
        listState = { data: [] };
        codeListExtra = { isError: true };
        renderAt("/dashboard/my");
        expect(screen.getByText("You have no dashboards yet")).toBeInTheDocument();
        expect(screen.getByText("Could not load your advanced dashboards.")).toBeInTheDocument();
      });

      it("is the 404 page, reading nothing advanced, for a user who cannot use them", () => {
        canUseAdvanced = false;
        renderAt(`/dashboard/my/code/${X.id}`);
        expect(screen.getByText("Not found")).toBeInTheDocument();
        expect(codeListEnabled.every((e) => e === false)).toBe(true);
        expect(codeDetailIds).toEqual([]);
      });
    });
  });
});
