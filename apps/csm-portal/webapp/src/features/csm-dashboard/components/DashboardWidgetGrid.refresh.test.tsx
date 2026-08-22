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

/**
 * Regression test: a section's `RefreshButton` never showed a "Last
 * refreshed" hint no matter how many times it was clicked, because
 * `DashboardWidgetGrid` never tracked/passed an `updatedAt` for a section
 * (unlike single-widget call sites, which hand a query's own
 * `dataUpdatedAt` straight through) — see `sectionLastRefreshedAt` in
 * `DashboardWidgetGrid.tsx`.
 */

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import type { BeDashboardWidget } from "@api/backend/types";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "11111111-aaaa-bbbb-cccc-000000000001" },
    isLoading: false,
    isError: false,
  }),
}));

import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";

// Every tile reports itself as already on screen, so it fetches on mount —
// this test cares about the refresh flow after that, not lazy-loading.
class AlwaysIntersectingObserver {
  callback: IntersectionObserverCallback;

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback;
  }

  observe(node: Element): void {
    this.callback(
      [{ isIntersecting: true, target: node } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    );
  }

  unobserve(): void {}
  disconnect(): void {}
}

function widget(id: string, section: string): BeDashboardWidget {
  return {
    widgetId: id,
    displayName: id,
    resourceType: "case",
    shape: "count",
    gridWidth: 3,
    query: {},
    section,
  };
}

function renderGrid() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const widgets = [widget("w1", "My Section"), widget("w2", "My Section")];
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DashboardWidgetGrid widgets={widgets} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DashboardWidgetGrid section refresh 'Last refreshed' hint", () => {
  const originalIntersectionObserver = globalThis.IntersectionObserver;

  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ total: 1, cases: [], limit: 1, offset: 0, hasMore: false });
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      AlwaysIntersectingObserver;
  });

  afterEach(() => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      originalIntersectionObserver;
  });

  it("shows no 'Last refreshed' hint before any click, and shows it once the section's refresh resolves", async () => {
    renderGrid();

    // Two widgets share one section, so there's a single refresh button for
    // both.
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));

    const refreshButton = screen.getByRole("button", { name: "Refresh My Section" });
    expect(screen.queryByText(/Last refreshed/)).not.toBeInTheDocument();

    fireEvent.click(refreshButton);

    await waitFor(() => expect(screen.getByText(/Last refreshed/)).toBeInTheDocument());
    expect(screen.getByText(/Last refreshed\s+just now/)).toBeInTheDocument();
  });

  it("keeps the section refresh button and its 'Last refreshed' label in the DOM (hover/focus-reveal is opacity-only, not display:none) and reachable by keyboard", async () => {
    // jsdom does not compute CSS (:hover/:focus-within styling isn't
    // something Testing Library can assert on directly) — what IS
    // assertable here is that the control is structurally present and
    // still focusable/clickable at all times, rather than being removed
    // from the DOM (or the tab order) until hovered.
    renderGrid();

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));

    const refreshButton = screen.getByRole("button", { name: "Refresh My Section" });
    // Present and focusable (and thus clickable) even though it's visually
    // hidden by default via opacity, not display:none.
    refreshButton.focus();
    expect(refreshButton).toHaveFocus();

    fireEvent.click(refreshButton);
    await waitFor(() => expect(screen.getByText(/Last refreshed/)).toBeInTheDocument());

    // The label sits in the DOM right alongside the button once it exists,
    // both governed by the same hover/focus-within reveal wrapper.
    expect(screen.getByText(/Last refreshed\s+just now/)).toBeInTheDocument();
  });
});
