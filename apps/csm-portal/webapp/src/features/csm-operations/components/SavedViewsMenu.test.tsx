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

import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type * as React from "react";
import SavedViewsMenu from "@features/csm-operations/components/SavedViewsMenu";
import { createSavedFilterViewsStore } from "@features/csm-operations/utils/savedFilterViews";

const STORAGE_KEY = "csm.savedFilters.test.v1";

function renderMenu(overrides: Partial<React.ComponentProps<typeof SavedViewsMenu>> = {}) {
  const store = createSavedFilterViewsStore(STORAGE_KEY);
  const onApply = vi.fn();
  const canonicalizeQs = (qs: string) => qs;
  const utils = render(
    <SavedViewsMenu
      currentQs="q=hello"
      canonicalizeQs={canonicalizeQs}
      activeCount={1}
      hasSearch={false}
      onApply={onApply}
      store={store}
      {...overrides}
    />,
  );
  return { ...utils, store, onApply };
}

describe("SavedViewsMenu", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("saves the current view via the dialog, persisting it to the tab's store", () => {
    renderMenu();

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /save current view/i }));

    const nameField = screen.getByLabelText(/view name/i);
    fireEvent.change(nameField, { target: { value: "My open S1s" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    // The Dialog's own close transition means the "Saved views" button isn't
    // reliably re-queryable (aria-hidden while unmounting) right after this
    // synchronous click — assert against the store the menu itself reads
    // from instead of reopening the menu.
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]")).toContainEqual({
      name: "My open S1s",
      qs: "q=hello",
    });
  });

  it("applying a saved view calls onApply with its stored qs", () => {
    const store = createSavedFilterViewsStore(STORAGE_KEY);
    store.saveFilterView("Critical only", "severities=S1");
    const onApply = vi.fn();
    renderMenu({ store, onApply });

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /critical only/i }));

    expect(onApply).toHaveBeenCalledWith("severities=S1");
  });

  it("highlights the saved view matching the current qs as active", () => {
    const store = createSavedFilterViewsStore(STORAGE_KEY);
    store.saveFilterView("Matches current", "q=hello");
    store.saveFilterView("Different", "q=other");
    renderMenu({ store, currentQs: "q=hello" });

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    expect(screen.getByText("Matches current").closest('[role="menuitem"]')).toHaveClass(
      "Mui-selected",
    );
    expect(screen.getByText("Different").closest('[role="menuitem"]')).not.toHaveClass(
      "Mui-selected",
    );
  });

  it("reorders saved views with the up/down icon buttons", () => {
    const store = createSavedFilterViewsStore(STORAGE_KEY);
    store.saveFilterView("First", "q=1");
    store.saveFilterView("Second", "q=2");
    // Most-recently-saved first: ["Second", "First"].
    renderMenu({ store });

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByRole("button", { name: /move saved view second down/i }));

    // The menu itself stays open across a move (`store.moveFilterView` never
    // closes it) — re-query it in place rather than re-clicking "Saved
    // views" (which the modal's own aria-hidden handling would hide anyway
    // while it's still open).
    const items = screen
      .getAllByRole("menuitem")
      .filter((el) => el.textContent?.match(/First|Second/));
    expect(items[0]).toHaveTextContent("First");
    expect(items[1]).toHaveTextContent("Second");
  });

  it("does not claim 'all records' in the save dialog when only a search term is active", () => {
    // activeCount excludes search (see each tab's countActive*Filters), but a
    // search-only view still restores that search on apply — the helper text
    // must not tell the user it will show everything.
    renderMenu({ activeCount: 0, hasSearch: true });

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /save current view/i }));

    expect(screen.queryByText(/will show all records/i)).not.toBeInTheDocument();
    expect(screen.getByText(/captures the 0 active filters and the current search/i)).toBeInTheDocument();
  });

  it("shows the 'all records' tip only when there is neither an active filter nor a search", () => {
    renderMenu({ activeCount: 0, hasSearch: false });

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByRole("menuitem", { name: /save current view/i }));

    expect(screen.getByText(/will show all records/i)).toBeInTheDocument();
  });

  it("deletes a saved view via its delete icon button", () => {
    const store = createSavedFilterViewsStore(STORAGE_KEY);
    store.saveFilterView("Temp view", "q=temp");
    renderMenu({ store });

    fireEvent.click(screen.getByRole("button", { name: /saved views/i }));
    fireEvent.click(screen.getByRole("button", { name: /delete saved view temp view/i }));

    // Same reasoning as the reorder test above — the menu stays open across
    // a delete, so assert in place instead of reopening it.
    expect(screen.queryByText("Temp view")).not.toBeInTheDocument();
    expect(screen.getByText(/no saved views yet/i)).toBeInTheDocument();
  });
});
