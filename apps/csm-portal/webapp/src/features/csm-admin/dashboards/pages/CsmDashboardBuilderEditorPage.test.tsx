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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { MemoryRouter, Route, Routes } from "react-router";

const getMock = vi.fn();
const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));

// A window.prompt-driven flow ("Add section") — stubbed so its own tests can
// control what the admin "typed".
const promptMock = vi.fn();
vi.stubGlobal("prompt", promptMock);

import CsmDashboardBuilderEditorPage from "@features/csm-admin/dashboards/pages/CsmDashboardBuilderEditorPage";
import {
  getDashboardDraft,
  saveDashboardDraft,
} from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";

function renderEditor(initialPath: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/admin/dashboards/new" element={<CsmDashboardBuilderEditorPage />} />
          <Route path="/admin/dashboards/:draftId" element={<CsmDashboardBuilderEditorPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmDashboardBuilderEditorPage", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    // Every widget in these tests renders through the real
    // `DashboardWidgetTile` (see this page's own doc comment on reusing the
    // live render path) — give its own `/cases/search` call a real
    // response so a widget's tile doesn't sit in its own loading skeleton
    // forever (a count-shape tile only renders its `displayName` once
    // loaded — see `DashboardWidgetTile.tsx`).
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 1, offset: 0, hasMore: false });
    promptMock.mockReset();
    localStorage.clear();
  });

  it("creating a new dashboard: seeds an empty draft, lets the admin name it, add a widget, and persists to localStorage", async () => {
    getMock.mockResolvedValue(null); // GET /dashboards/{newId} 404s -> null
    renderEditor("/admin/dashboards/new");

    // Seeded and rendered (not stuck on the loading skeleton).
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());
    expect(
      screen.getByText(/this dashboard has never been deployed/i),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Dashboard display name"), {
      target: { value: "My new dashboard" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));
    fireEvent.change(screen.getByLabelText("Widget display name"), {
      target: { value: "Open cases" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add widget" }));

    await waitFor(() => expect(screen.getByText("Open cases")).toBeInTheDocument());

    // Debounced persistence — wait past the 300ms window.
    await waitFor(
      () => {
        const drafts = Object.values(localStorage)
          .map((v) => v)
          .join("");
        expect(drafts).toContain("My new dashboard");
      },
      { timeout: 1000 },
    );
  });

  it("marks Display name and Type required, and keeps Copy as JSON disabled until both are set", async () => {
    getMock.mockResolvedValue(null);
    const { container } = renderEditor("/admin/dashboards/new");

    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());
    // By text, not role name: the Tooltip around it names the button with
    // its own title.
    const copy = screen.getByText("Copy as JSON").closest("button")!;
    expect(copy).toHaveAccessibleName(/Add a display name and a type first/);
    expect(screen.getByLabelText("Dashboard display name")).toBeRequired();
    const typeLabel = Array.from(container.querySelectorAll("label")).find((l) =>
      l.textContent?.startsWith("Type"),
    );
    expect(typeLabel?.textContent).not.toContain("optional");
    expect(typeLabel?.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
    expect(copy).toBeDisabled();
    expect(screen.getAllByText("Required to copy as JSON.")).toHaveLength(2);

    fireEvent.change(screen.getByLabelText("Dashboard display name"), {
      target: { value: "My new dashboard" },
    });
    expect(copy).toBeDisabled();
    expect(screen.getAllByText("Required to copy as JSON.")).toHaveLength(1);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Type" }));
    // A required type offers no "None".
    expect(screen.queryByRole("option", { name: "None" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("option", { name: "CS" }));

    await waitFor(() => expect(copy).toBeEnabled());
    expect(screen.queryByText("Required to copy as JSON.")).not.toBeInTheDocument();
  });

  it("editing an existing dashboard: seeds from GET /dashboards/{id} and shows no drift banner when unchanged", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "w1",
          displayName: "My Patches",
          resourceType: "case",
          shape: "count",
          gridWidth: 3,
          query: {},
        },
      ],
    });

    renderEditor("/admin/dashboards/agents_pilot");

    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/differs from what/i)).not.toBeInTheDocument();
  });

  it("does not flash the 'not yet deployed' drift banner while GET /dashboards/{id} is still in flight, for an existing local draft", async () => {
    const draft = {
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    };
    // A local draft already exists (the admin opened this before) — so
    // `working` is populated synchronously on mount, while `GET
    // /dashboards/agents_pilot` is still in flight. Before this fix,
    // `isDraftDrifted` ran against `live.data` (`undefined` while loading)
    // and unconditionally reported drift the instant `working` existed.
    saveDashboardDraft(draft);
    let resolveGet!: (value: unknown) => void;
    getMock.mockReturnValue(
      new Promise((resolve) => {
        resolveGet = resolve;
      }),
    );

    renderEditor("/admin/dashboards/agents_pilot");

    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"));
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/differs from what/i)).not.toBeInTheDocument();

    // Resolve the fetch with content identical to the draft — still no
    // drift banner once it settles.
    resolveGet({ ...draft, sourceDashboardId: undefined });
    await waitFor(() => expect(getMock).toHaveBeenCalled());
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/differs from what/i)).not.toBeInTheDocument();
  });

  it("shows a distinct 'couldn't check' notice, not a drift warning, when GET /dashboards/{id} fails", async () => {
    const draft = {
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    };
    saveDashboardDraft(draft);
    getMock.mockRejectedValue(new Error("network error"));

    renderEditor("/admin/dashboards/agents_pilot");

    await waitFor(() => expect(screen.getByText(/couldn't check/i)).toBeInTheDocument());
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/differs from what/i)).not.toBeInTheDocument();
  });

  it("does not re-stamp updatedAt via the autosave effect just from opening an unmodified existing draft", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "w1",
          displayName: "My Patches",
          resourceType: "case",
          shape: "count",
          gridWidth: 3,
          query: {},
        },
      ],
    });

    renderEditor("/admin/dashboards/agents_pilot");
    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());

    const initialUpdatedAt = getDashboardDraft("agents_pilot")?.updatedAt;
    expect(initialUpdatedAt).toBeDefined();

    // Wait well past the 300ms autosave debounce with no user interaction —
    // the stored `updatedAt` must not have moved.
    await new Promise((resolve) => setTimeout(resolve, 500));
    expect(getDashboardDraft("agents_pilot")?.updatedAt).toBe(initialUpdatedAt);
  });

  it("flushes a still-pending autosave on unmount, rather than losing an edit made just before navigating away", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
    });

    const { unmount } = renderEditor("/admin/dashboards/agents_pilot");
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"));

    fireEvent.change(screen.getByLabelText("Dashboard display name"), {
      target: { value: "Engineer overview (renamed)" },
    });

    // Unmount immediately — well inside the 300ms debounce window, so the
    // scheduled autosave `setTimeout` never gets a chance to fire on its
    // own. Without a flush-on-unmount, this edit would be lost outright
    // (there is no backend copy — see this page's own doc comment).
    unmount();

    expect(getDashboardDraft("agents_pilot")?.displayName).toBe("Engineer overview (renamed)");
  });

  it("shows the drift warning once a deployed dashboard's draft is edited locally", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
    });

    renderEditor("/admin/dashboards/agents_pilot");
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"));
    expect(screen.queryByText(/not yet deployed/i)).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Dashboard display name"), {
      target: { value: "Engineer overview (renamed)" },
    });

    await waitFor(() => expect(screen.getByText(/not yet deployed/i)).toBeInTheDocument());
  });

  it("removing a widget asks for confirmation before it disappears from the draft", async () => {
    const draft = {
      id: "with-widget",
      sourceDashboardId: "with-widget",
      displayName: "Has a widget",
      isDefault: false,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "w1",
          displayName: "My Patches",
          resourceType: "case" as const,
          shape: "count" as const,
          gridWidth: 3,
          query: {},
        },
      ],
      emptySections: [],
    };
    saveDashboardDraft(draft);
    getMock.mockResolvedValue({ ...draft, id: "with-widget" });

    renderEditor("/admin/dashboards/with-widget");
    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());

    // Every button query below that follows a Dialog open/close is wrapped
    // in `waitFor`: MUI's Modal keeps the rest of the page `aria-hidden`
    // until its own exit transition finishes (a real, un-mocked ~225ms
    // Fade), and `getByRole` — unlike `getByText` — respects that, so a
    // synchronous re-query right after a close click can transiently miss
    // an element that is genuinely about to reappear.
    const removeButton = await waitFor(() =>
      screen.getByRole("button", { name: "Remove widget My Patches" }),
    );
    fireEvent.click(removeButton);
    expect(screen.getByText(/removes "My Patches" from this draft/i)).toBeInTheDocument();

    // Cancelling keeps the widget.
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByText("My Patches")).toBeInTheDocument();

    const removeButtonAgain = await waitFor(() =>
      screen.getByRole("button", { name: "Remove widget My Patches" }),
    );
    fireEvent.click(removeButtonAgain);
    const confirmButton = await waitFor(() => screen.getByRole("button", { name: "Remove" }));
    fireEvent.click(confirmButton);

    await waitFor(() => expect(screen.queryByText("My Patches")).not.toBeInTheDocument());
    await waitFor(() => expect(getDashboardDraft("with-widget")?.widgets).toHaveLength(0));
  });

  it("adds an empty section shell via 'Add section', offering its own 'Add widget' entry point", async () => {
    getMock.mockResolvedValue(null);
    promptMock.mockReturnValue("New section");

    renderEditor("/admin/dashboards/new");
    await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "Add section" }));

    expect(await screen.findByText("New section")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove section New section" })).toBeInTheDocument();
  });

  it("keeps a placeholder-named section's identity on its own RAW (unresolved) key, not the display-resolved title", async () => {
    const draft = {
      id: "team-dash",
      sourceDashboardId: "team-dash",
      displayName: "Team dashboard",
      isDefault: false,
      isTeamBased: true,
      widgets: [
        {
          widgetId: "w1",
          displayName: "My Patches",
          resourceType: "case" as const,
          shape: "count" as const,
          gridWidth: 3,
          query: {},
          section: "{{currentTeam}} Escalations",
        },
      ],
      emptySections: [],
    };
    saveDashboardDraft(draft);
    getMock.mockResolvedValue({ ...draft, sourceDashboardId: undefined });

    renderEditor("/admin/dashboards/team-dash");

    // No signed-in team in this test's mock -> defaults to "All ABTs", so
    // the section's raw `{{currentTeam}} Escalations` key resolves to the
    // very different-looking display title "All ABTs Escalations".
    const resolvedHeading = await screen.findByText("All ABTs Escalations");
    expect(resolvedHeading).toBeInTheDocument();

    // Two "Add widget" buttons exist (the card-level one, and this
    // section's own) — the section-scoped one is the one whose default
    // section must be the RAW key.
    const addWidgetButtons = screen.getAllByRole("button", { name: "Add widget" });
    fireEvent.click(addWidgetButtons[addWidgetButtons.length - 1]);

    await waitFor(() =>
      expect(screen.getByDisplayValue("{{currentTeam}} Escalations")).toBeInTheDocument(),
    );
    expect(screen.queryByDisplayValue("All ABTs Escalations")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    // Removing the section (identified via the resolved title in its own
    // button label, same as before) must actually match and drop the
    // widget whose RAW `section` it was keyed on internally — before this
    // fix, the removal filtered on the resolved title, which never equals
    // any real `widget.section` value for a placeholder-named section, so
    // nothing was ever removed.
    const removeSectionButton = await waitFor(() =>
      screen.getByRole("button", { name: "Remove section All ABTs Escalations" }),
    );
    fireEvent.click(removeSectionButton);
    const confirmButton = await waitFor(() => screen.getByRole("button", { name: "Remove" }));
    fireEvent.click(confirmButton);

    await waitFor(() => expect(screen.queryByText("My Patches")).not.toBeInTheDocument());
    await waitFor(() => expect(getDashboardDraft("team-dash")?.widgets).toHaveLength(0));
  });
  describe("Discard local draft", () => {
    const deployed = {
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
    };

    // Same routes as `renderEditor` plus a stand-in list page, so navigation
    // after a discard is observable.
    function renderEditorWithList(initialPath: string) {
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      return render(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter initialEntries={[initialPath]}>
            <Routes>
              <Route path="/admin/dashboards" element={<div>Dashboard list page</div>} />
              <Route path="/admin/dashboards/new" element={<CsmDashboardBuilderEditorPage />} />
              <Route path="/admin/dashboards/:draftId" element={<CsmDashboardBuilderEditorPage />} />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>,
      );
    }

    async function openDiscardDialog() {
      const button = await waitFor(() => screen.getByRole("button", { name: "Discard local draft" }));
      fireEvent.click(button);
      await waitFor(() => expect(screen.getByText("Discard local draft?")).toBeInTheDocument());
    }

    it("opens a confirm dialog, and Cancel keeps the draft and stays on the page", async () => {
      getMock.mockResolvedValue(deployed);
      renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() => expect(getDashboardDraft("agents_pilot")).toBeDefined());

      await openDiscardDialog();
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

      await waitFor(() => expect(screen.queryByText("Discard local draft?")).not.toBeInTheDocument());
      expect(getDashboardDraft("agents_pilot")).toBeDefined();
      expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument();
      expect(screen.queryByText("Dashboard list page")).not.toBeInTheDocument();
    });

    const sourcedDraft = {
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      widgets: [],
      emptySections: [],
    };

    it("promises a deployed reset only once the lookup confirms the dashboard exists", async () => {
      getMock.mockResolvedValue(deployed);
      renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() => expect(getDashboardDraft("agents_pilot")).toBeDefined());
      await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());

      await openDiscardDialog();
      expect(screen.getByText(/"Engineer overview"/)).toBeInTheDocument();
      await waitFor(() => expect(screen.getByText(/reset to the deployed version/i)).toBeInTheDocument());
    });

    it("does not promise a reset while the deployed lookup is still pending, and keeps Discard enabled", async () => {
      saveDashboardDraft(sourcedDraft);
      getMock.mockReturnValue(new Promise(() => {}));
      renderEditorWithList("/admin/dashboards/agents_pilot");

      await openDiscardDialog();
      expect(screen.getByText(/still being checked/i)).toBeInTheDocument();
      expect(screen.getByText(/cannot be undone/i)).toBeInTheDocument();
      expect(screen.queryByText(/reset to the deployed version/i)).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Discard" })).toBeEnabled();
    });

    it("says it could not check when the deployed lookup failed, despite a sourceDashboardId", async () => {
      saveDashboardDraft(sourcedDraft);
      getMock.mockRejectedValue(new Error("network error"));
      renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() => expect(screen.getByText(/couldn't check/i)).toBeInTheDocument());

      await openDiscardDialog();
      expect(screen.getByText(/could not check whether a deployed version exists/i)).toBeInTheDocument();
      expect(screen.queryByText(/reset to the deployed version/i)).not.toBeInTheDocument();
    });

    it("says the draft is the only copy when the source dashboard is no longer deployed", async () => {
      saveDashboardDraft(sourcedDraft);
      getMock.mockResolvedValue(null);
      renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() => expect(screen.getByText(/not yet deployed|never been deployed|differs/i)).toBeInTheDocument());

      await openDiscardDialog();
      expect(screen.getByText(/deletes it entirely/i)).toBeInTheDocument();
      expect(screen.queryByText(/reset to the deployed version/i)).not.toBeInTheDocument();
    });

    it("shows the delete-entirely copy for a never-deployed dashboard", async () => {
      getMock.mockResolvedValue(null);
      renderEditorWithList("/admin/dashboards/new");
      await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toBeInTheDocument());

      await openDiscardDialog();
      expect(screen.getByText(/deletes it entirely/i)).toBeInTheDocument();
    });

    it("Discard deletes the draft, goes to the list, and the still-pending autosave does not resurrect it", async () => {
      getMock.mockResolvedValue(deployed);
      renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"));

      // An edit inside the 300ms debounce window: its save timer is armed
      // and the unmount flush would write it if discarding did not stop both.
      fireEvent.change(screen.getByLabelText("Dashboard display name"), {
        target: { value: "Edited then discarded" },
      });
      await openDiscardDialog();
      fireEvent.click(screen.getByRole("button", { name: "Discard" }));

      await waitFor(() => expect(screen.getByText("Dashboard list page")).toBeInTheDocument());
      expect(getDashboardDraft("agents_pilot")).toBeUndefined();

      // Past the debounce window: the timer must not have re-saved it either.
      await new Promise((resolve) => setTimeout(resolve, 600));
      expect(getDashboardDraft("agents_pilot")).toBeUndefined();
      expect(localStorage.getItem("csm.dashboardBuilder.drafts.v1") ?? "").not.toContain(
        "Edited then discarded",
      );
    });

    it("re-opening a discarded deployed dashboard re-seeds from the deployed version", async () => {
      getMock.mockResolvedValue(deployed);
      const first = renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() => expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"));
      fireEvent.change(screen.getByLabelText("Dashboard display name"), {
        target: { value: "Edited then discarded" },
      });
      await openDiscardDialog();
      fireEvent.click(screen.getByRole("button", { name: "Discard" }));
      await waitFor(() => expect(screen.getByText("Dashboard list page")).toBeInTheDocument());
      first.unmount();

      renderEditorWithList("/admin/dashboards/agents_pilot");
      await waitFor(() =>
        expect(screen.getByLabelText("Dashboard display name")).toHaveValue("Engineer overview"),
      );
      expect(getDashboardDraft("agents_pilot")?.displayName).toBe("Engineer overview");
    });
  });
});
