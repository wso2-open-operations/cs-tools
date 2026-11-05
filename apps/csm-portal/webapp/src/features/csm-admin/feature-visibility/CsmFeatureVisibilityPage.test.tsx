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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CsmFeatureVisibilityPage from "@features/csm-admin/feature-visibility/CsmFeatureVisibilityPage";

// A tiny stand-in for entity-service: rows keyed by page id, a saved-once
// mark, upsert on write, never a delete.
const server = vi.hoisted(() => ({
  rows: {} as Record<string, { state: string; updatedBy: string; updatedOn: string }>,
  savedOnce: false,
  failIds: new Set<string>(),
  failLoad: false,
  put: vi.fn(),
}));

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({
    get: async () => {
      if (server.failLoad) throw new Error("boom");
      return {
        overrides: Object.entries(server.rows).map(([navNodeId, row]) => ({ navNodeId, ...row })),
        savedOnce: server.savedOnce,
      };
    },
    put: async (_path: string, body: { navNodeId: string; state: string; markSaved?: boolean }) => {
      server.put(body);
      if (server.failIds.has(body.navNodeId)) throw new Error("nope");
      server.rows[body.navNodeId] = { state: body.state, updatedBy: "admin@example.com", updatedOn: "2026-11-05T10:00:00Z" };
      if (body.markSaved) server.savedOnce = true;
      return { navNodeId: body.navNodeId, ...server.rows[body.navNodeId] };
    },
  }),
}));
function setConfig(value: unknown): void {
  window.config = { ...window.config, CSM_PORTAL_FEATURE_OVERRIDES: value } as Window["config"];
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <CsmFeatureVisibilityPage />
    </QueryClientProvider>,
  );
}

async function choose(name: string, option: "Available" | "Preview"): Promise<void> {
  fireEvent.click(screen.getByRole("button", { name: `Edit ${name}` }));
  const item = await screen.findByRole("menuitem", { name: new RegExp(`^${option}`) });
  fireEvent.click(item);
}

beforeEach(() => {
  server.rows = {};
  server.savedOnce = false;
  server.failIds = new Set();
  server.failLoad = false;
  server.put.mockClear();
  setConfig(undefined);
});

describe("Feature visibility page", () => {
  it("is titled Feature Visibility, lists every section including Knowledge, and shows no save banner until something changes", async () => {
    renderPage();
    expect(await screen.findByText("Knowledge")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Feature Visibility" })).toBeInTheDocument();
    expect(screen.getByText("Operations")).toBeInTheDocument();
    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Discard" })).not.toBeInTheDocument();
  });

  it("keeps the table header and the filter plain: no counts, no Status or Expand all, no row descriptions", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    expect(screen.queryByText(/available · \d+ preview/)).not.toBeInTheDocument();
    expect(screen.queryByText("Status")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Expand all|Collapse all/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/pages? ·|Everyone can open this/)).not.toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Show" }));
    const options = (await screen.findAllByRole("option")).map((option) => option.textContent);
    expect(options).toEqual(["All", "Available", "Preview", "Unsaved changes"]);
  });

  it("says the list still comes from the config file until the first save", async () => {
    setConfig({ operations: "wip" });
    renderPage();
    expect(await screen.findByText(/Nothing has been saved here yet/)).toBeInTheDocument();
    // An older wip line shows as Preview.
    const operations = screen.getByRole("button", { name: "Edit Operations" }).parentElement!;
    expect(within(operations).getByText("Preview")).toBeInTheDocument();
  });

  it("marks a change as unsaved, offers no per-row Undo, and can discard everything", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    await choose("Knowledge", "Preview");
    expect(screen.getByText("1 unsaved change")).toBeInTheDocument();
    expect(screen.getByText(/Unsaved\. Was not set\./)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeEnabled();

    expect(screen.queryByRole("button", { name: /^Undo/ })).not.toBeInTheDocument();

    await choose("Support", "Preview");
    await choose("Dashboards", "Preview");
    expect(screen.getByText("3 unsaved changes")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(screen.queryByText(/^\d+ unsaved change/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
  });

  it("changes several ticked sections at once", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    fireEvent.click(screen.getByRole("checkbox", { name: "Select Support" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Select Dashboards" }));
    expect(screen.getByText("2 items selected")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Change status" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: /Set to Preview/ }));
    expect(screen.getByText("2 unsaved changes")).toBeInTheDocument();
    // Ticks stay after a bulk change, so the choice can be corrected.
    expect(screen.getByRole("checkbox", { name: "Select Support" })).toBeChecked();
  });

  it("lets sub-items be ticked and changed too, together with a section", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    fireEvent.click(screen.getByRole("button", { name: "Expand Operations" }));
    const incidents = await screen.findByRole("checkbox", { name: "Select Incidents" });
    fireEvent.click(incidents);
    fireEvent.click(screen.getByRole("checkbox", { name: "Select Support" }));
    expect(screen.getByText("2 items selected")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Change status" }));
    fireEvent.click(await screen.findByRole("menuitem", { name: /Set to Preview/ }));
    expect(screen.getByText("2 unsaved changes")).toBeInTheDocument();
    // Each ticked row got its own entry: the page is not just following its section.
    expect(screen.getAllByText(/Unsaved\. Was not set\./)).toHaveLength(2);
  });

  it("selects everything on screen with the header checkbox, and only what is on screen", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    fireEvent.click(screen.getByRole("checkbox", { name: "Select all" }));
    // Collapsed sections' pages are not on screen, so they are not ticked.
    expect(screen.queryByRole("checkbox", { name: "Select Incidents" })).not.toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: "Select Support" })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Expand Operations" }));
    expect(await screen.findByRole("checkbox", { name: "Select Incidents" })).not.toBeChecked();
  });

  it("opens the matching section when you search for one of its pages", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    expect(screen.queryByText("Incidents")).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: "Search sections and pages" }), {
      target: { value: "Incidents" },
    });
    expect(await screen.findByText("Incidents")).toBeInTheDocument();
    expect(screen.queryByText("Knowledge")).not.toBeInTheDocument();
  });

  it("opens and closes a section's pages from its arrow", async () => {
    renderPage();
    await screen.findByText("Knowledge");
    fireEvent.click(screen.getByRole("button", { name: "Expand Operations" }));
    expect(await screen.findByText("Incidents")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Collapse Operations" }));
    await waitFor(() => expect(screen.queryByText("Incidents")).not.toBeInTheDocument());
  });

  describe("saving", () => {
    it("keeps the config file's lines as rows on the first save, then marks it saved", async () => {
      setConfig({ operations: "preview", "operations.incidents": "enabled" });
      renderPage();
      await screen.findByText("Knowledge");
      await choose("Support", "Preview");
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
      const dialog = await screen.findByRole("dialog");
      // The popup lists only what the admin changed, not the config lines that were kept.
      expect(within(dialog).getByText("Changes saved")).toBeInTheDocument();
      expect(within(dialog).getAllByRole("listitem")).toHaveLength(1);
      expect(within(dialog).getByText("Support")).toBeInTheDocument();

      const sent = server.put.mock.calls.map(([body]) => body);
      expect(sent.map((body) => body.navNodeId).sort()).toEqual(["operations", "operations.incidents", "support"]);
      // An Available config line is kept as an enabled row, and only the last write carries the mark.
      expect(sent.find((body) => body.navNodeId === "operations.incidents")).toMatchObject({ state: "enabled" });
      expect(sent.filter((body) => body.markSaved)).toHaveLength(1);
      expect(sent[sent.length - 1].markSaved).toBe(true);
      expect(server.savedOnce).toBe(true);
    });

    it("writes Available as an enabled row after the first save, never as a delete", async () => {
      server.savedOnce = true;
      server.rows = {
        operations: { state: "preview", updatedBy: "admin@example.com", updatedOn: "2026-11-01T10:00:00Z" },
      };
      renderPage();
      await screen.findByText("Knowledge");
      await choose("Operations", "Available");
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
      // The popup says what changed: Operations went from Preview to Available.
      const dialog = await screen.findByRole("dialog");
      const item = within(dialog).getByRole("listitem");
      expect(item).toHaveTextContent("Operations");
      expect(item).toHaveTextContent("Preview");
      expect(item).toHaveTextContent("Available");
      fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

      expect(server.put).toHaveBeenCalledTimes(1);
      expect(server.put).toHaveBeenCalledWith({ navNodeId: "operations", state: "enabled" });
      // The row is still there, now Available. The page does not say who saved it.
      expect(server.rows.operations!.state).toBe("enabled");
      expect(screen.queryByText(/Saved by/)).not.toBeInTheDocument();
    });

    it("clears the ticks after a save", async () => {
      renderPage();
      await screen.findByText("Knowledge");
      fireEvent.click(screen.getByRole("checkbox", { name: "Select Support" }));
      await choose("Support", "Preview");
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
      const dialog = await screen.findByRole("dialog");
      // The popup covers the page, so close it before looking at the list again.
      fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
      await waitFor(() => expect(screen.getByRole("checkbox", { name: "Select Support" })).not.toBeChecked());
      expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
    });

    it("keeps a page that could not be saved marked as unsaved after the first save", async () => {
      server.savedOnce = true;
      server.failIds = new Set(["support"]);
      renderPage();
      await screen.findByText("Knowledge");
      await choose("Support", "Preview");
      await choose("Dashboards", "Preview");
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

      expect(await screen.findByText("1 of 2 could not be saved. They are still marked as unsaved, so you can try again.")).toBeInTheDocument();
      // The popup lists only the page that did save. The one that failed is still unsaved.
      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getAllByRole("listitem")).toHaveLength(1);
      expect(within(dialog).getByText("Dashboards")).toBeInTheDocument();
      expect(within(dialog).queryByText("Support")).not.toBeInTheDocument();
      await waitFor(() => expect(screen.getByText("1 unsaved change")).toBeInTheDocument());
    });

    it("leaves the first save unmarked when any write fails, so the config file still applies", async () => {
      server.failIds = new Set(["support"]);
      renderPage();
      await screen.findByText("Knowledge");
      await choose("Support", "Preview");
      await choose("Dashboards", "Preview");
      fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

      expect(await screen.findByText(/could not be saved/)).toBeInTheDocument();
      expect(server.savedOnce).toBe(false);
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(server.put).not.toHaveBeenCalledWith(expect.objectContaining({ markSaved: true }));
      // Nothing was switched over, so both pages are still unsaved changes.
      await waitFor(() => expect(screen.getByText("2 unsaved changes")).toBeInTheDocument());
    });
  });

  it("shows an error with Retry when the settings cannot be loaded", async () => {
    server.failLoad = true;
    renderPage();
    expect(await screen.findByText("Failed to load feature visibility.")).toBeInTheDocument();
    server.failLoad = false;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByText("Knowledge")).toBeInTheDocument();
  });
});
