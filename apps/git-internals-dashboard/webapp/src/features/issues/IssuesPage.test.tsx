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
import { act, fireEvent, render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import IssuesPage from "./IssuesPage";

/** Builds a 200 OK Response with a JSON body, for mocking fetch. */
function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

const EMPTY_OVERVIEW = {
  refreshedAt: "2026-01-01T00:00:00Z",
  filters: { repo: null, priority: null },
  hero: {
    violated: { n: 0, delta: 0, spark: [] },
    atRisk: { n: 0, delta: 0, spark: [] },
    cs: { n: 0, byStatus: [] },
    productSide: { n: 0, delta: 0, spark: [] },
  },
  projects: [],
  priorities: [],
  matrix: { rows: [], totals: { violated: 0, atRisk: 0, onTrack: 0, cs: 0 }, grandTotal: 0 },
  volume: [],
  unknownStatuses: [],
};

/** Renders IssuesPage under a fresh QueryClient and a memory router at /issues. */
function renderIssuesPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/issues", element: <IssuesPage /> }], {
    initialEntries: ["/issues"],
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe("IssuesPage", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // @ts-expect-error -- partial config is fine for this test
    window.config = { GID_BACKEND_BASE_URL: "https://backend.example.test" };
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    // @ts-expect-error -- test-only cleanup of the global window.config
    delete window.config;
  });

  it("keeps a filter changed mid-debounce instead of the search box's stale snapshot clobbering it", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/metrics/overview")) return Promise.resolve(jsonResponse(EMPTY_OVERVIEW));
      if (url.includes("/issues")) return Promise.resolve(jsonResponse([]));
      if (url.includes("/taxonomy")) return Promise.resolve(jsonResponse({ statuses: [], csStatuses: [] }));
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });
    vi.stubGlobal("fetch", fetchMock);

    const router = renderIssuesPage();

    // Let the initial overview/issues/taxonomy queries settle.
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    const searchInput = screen.getByPlaceholderText("Search by issue #…");
    act(() => {
      fireEvent.change(searchInput, { target: { value: "42" } });
    });

    // Before the 300ms search debounce fires, apply a second, non-debounced
    // filter change (the "Violated" bucket chip) — this goes through the
    // same immediate setParams(...) path as the repo/priority <Select>s.
    act(() => {
      fireEvent.click(screen.getByText("Violated"));
    });

    await act(async () => {
      vi.advanceTimersByTime(300);
      await vi.runOnlyPendingTimersAsync();
    });

    const search = router.state.location.search;
    expect(search).toContain("q=42");
    expect(search).toContain("bucket=violated");
  });

  it("splits the CS-side chip into separate Waiting on CS Team / Pending Patch Queue tiles", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/metrics/overview")) return Promise.resolve(jsonResponse(EMPTY_OVERVIEW));
      if (url.includes("/issues")) return Promise.resolve(jsonResponse([]));
      if (url.includes("/taxonomy")) return Promise.resolve(jsonResponse({ statuses: [], csStatuses: ["WOC", "Pending Patch Queue"] }));
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });
    vi.stubGlobal("fetch", fetchMock);

    const router = renderIssuesPage();

    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    // There is no single combined "On CS Side" chip anymore.
    expect(screen.queryByText("On CS Side")).toBeNull();

    act(() => {
      fireEvent.click(screen.getByText("Waiting on CS Team"));
    });
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    let search = router.state.location.search;
    expect(search).toContain("bucket=cs");
    expect(search).toContain("status=WOC");
    expect(screen.getByText("Waiting on CS Team issues")).toBeTruthy();

    act(() => {
      fireEvent.click(screen.getByText("Pending Patch Queue"));
    });
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    search = router.state.location.search;
    expect(search).toContain("bucket=cs");
    expect(search).toContain("status=Pending+Patch+Queue");
    expect(screen.getByText("Pending Patch Queue issues")).toBeTruthy();
  });
});
