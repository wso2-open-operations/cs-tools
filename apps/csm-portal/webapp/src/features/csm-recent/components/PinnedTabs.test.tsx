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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PinnedTabs from "@features/csm-recent/components/PinnedTabs";
import type { RecentView } from "@features/csm-recent/hooks/useRecentViews";

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true }),
}));

const toggleRecentViewPinMock = vi.fn();
const renameRecentViewMock = vi.fn();
let pinnedEntries: RecentView[] = [];

vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({
  useRecentViews: () => pinnedEntries,
  toggleRecentViewPin: (...args: unknown[]) => toggleRecentViewPinMock(...args),
  renameRecentView: (...args: unknown[]) => renameRecentViewMock(...args),
}));

function pinnedEntry(overrides: Partial<RecentView> = {}): RecentView {
  return {
    kind: "search",
    id: "/customers/projects/proj-1?tab=workItems&types=case",
    title: "Customers: 1 filter",
    href: "/customers/projects/proj-1?tab=workItems&types=case",
    visitedAt: "2026-08-18T00:00:00.000Z",
    pinned: true,
    ...overrides,
  };
}

beforeEach(() => {
  toggleRecentViewPinMock.mockReset();
  renameRecentViewMock.mockReset();
  pinnedEntries = [];
});

function renderPinnedTabs() {
  return render(
    <MemoryRouter initialEntries={["/somewhere-else"]}>
      <PinnedTabs />
    </MemoryRouter>,
  );
}

describe("PinnedTabs", () => {
  it("renders nothing but a spacer when there are no pinned entries", () => {
    const { container } = renderPinnedTabs();
    expect(container.querySelector(".MuiChip-root")).not.toBeInTheDocument();
  });

  it("renders a chip per pinned entry", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();
    expect(screen.getByText("My work items view")).toBeInTheDocument();
  });

  it("unpins via the chip's delete affordance", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();

    const chip = screen.getByText("My work items view").closest(".MuiChip-root")!;
    fireEvent.click(chip.querySelector(".MuiChip-deleteIcon")!);

    expect(toggleRecentViewPinMock).toHaveBeenCalledWith("search", "1");
  });

  it("right-click opens a context menu with a Rename item", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();

    fireEvent.contextMenu(screen.getByText("My work items view"));

    expect(screen.getByText("Rename")).toBeInTheDocument();
  });

  it("Rename opens a dialog pre-filled with the current title; saving calls renameRecentView", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();

    fireEvent.contextMenu(screen.getByText("My work items view"));
    fireEvent.click(screen.getByText("Rename"));

    const input = screen.getByRole("textbox", { name: "Tab name" }) as HTMLInputElement;
    expect(input.value).toBe("My work items view");

    fireEvent.change(input, { target: { value: "Project X work items" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(renameRecentViewMock).toHaveBeenCalledWith(
      "search",
      "1",
      "Project X work items",
    );
  });

  it("marks the rename field required", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();

    fireEvent.contextMenu(screen.getByText("My work items view"));
    fireEvent.click(screen.getByText("Rename"));

    expect(screen.getByRole("textbox", { name: "Tab name" })).toBeRequired();
  });

  it("disables Save while the rename field is blank", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();

    fireEvent.contextMenu(screen.getByText("My work items view"));
    fireEvent.click(screen.getByText("Rename"));

    const input = screen.getByRole("textbox", { name: "Tab name" }) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "   " } });

    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    expect(renameRecentViewMock).not.toHaveBeenCalled();
  });

  // Regression test: `renameRecentView` normalizes via `stripHtmlTags(...).trim()`
  // and no-ops on an empty result, but the dialog's own validation used to
  // check the raw field value with a plain `.trim()` -- so tag-only input
  // ("<b></b>", non-blank as raw text) passed that check, closed the dialog
  // via Enter, and still silently renamed nothing.
  it("does not close the dialog or call renameRecentView for tag-only input via Enter", () => {
    pinnedEntries = [pinnedEntry({ id: "1", title: "My work items view" })];
    renderPinnedTabs();

    fireEvent.contextMenu(screen.getByText("My work items view"));
    fireEvent.click(screen.getByText("Rename"));

    const input = screen.getByRole("textbox", { name: "Tab name" }) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "<b></b>" } });
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

    fireEvent.keyDown(input, { key: "Enter" });

    expect(renameRecentViewMock).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox", { name: "Tab name" })).toBeInTheDocument();
  });
});
