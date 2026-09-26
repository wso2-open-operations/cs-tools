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
  filters: { repo: null, priority: null, abtTeam: null },
  abtTeams: ["Atlas"],
  hero: {
    violated: { n: 0, delta: 0, spark: [] },
    atRisk: { n: 0, delta: 0, spark: [] },
    cs: { n: 0, byStatus: [] },
    productSide: { n: 0, delta: 0, spark: [] },
  },
  projects: [
    { repoId: 1, name: "Alpha", repo: "org/alpha", violated: 0, atRisk: 0, cs: 0, onTrack: 0, openTracked: 0, untracked: 0, worst: false, allClear: true },
  ],
  priorities: [
    { key: "critical", code: "P1", label: "Critical", budgetHours: 24, violated: 0, atRisk: 0, cs: 0, onTrack: 0, total: 0 },
  ],
  matrix: { rows: [], totals: { violated: 0, atRisk: 0, onTrack: 0, cs: 0 }, grandTotal: 0 },
  volume: [],
  unknownStatuses: [],
};

const TAXONOMY = {
  statuses: [
    { name: "Open", category: "PRODUCT_SIDE", accruesSla: true, isTerminal: false, sortOrder: 10 },
    { name: "WOC", category: "CS_SIDE", accruesSla: false, isTerminal: false, sortOrder: 20 },
    { name: "Pending Patch Queue", category: "CS_SIDE", accruesSla: false, isTerminal: false, sortOrder: 30 },
    { name: "Resolved", category: "OTHER", accruesSla: false, isTerminal: true, sortOrder: 40 },
  ],
  csStatuses: ["WOC", "Pending Patch Queue"],
};

/** Two issues with title/abtTeam/openedBy set (one with openedBy: null), as the /issues envelope shape. */
const ISSUES_WITH_TITLES = {
  issues: [
    {
      id: 1,
      number: 101,
      state: "OPEN",
      url: "https://github.com/example/repo/issues/101",
      repo: "org/alpha",
      priority: "High(P2)",
      currentStatus: "Open",
      githubCreatedAt: "2026-01-01T00:00:00Z",
      githubUpdatedAt: "2026-01-01T00:00:00Z",
      sla: { budgetHours: 48, consumedHours: 10, remainingHours: 38, pctConsumed: 0.2, slaState: "OK", slaRunning: true },
      title: "Fix the widget",
      abtTeam: "Atlas",
      openedBy: "person@wso2.com",
    },
    {
      id: 2,
      number: 102,
      state: "OPEN",
      url: "https://github.com/example/repo/issues/102",
      repo: "org/alpha",
      priority: "High(P2)",
      currentStatus: "Open",
      githubCreatedAt: "2026-01-01T00:00:00Z",
      githubUpdatedAt: "2026-01-01T00:00:00Z",
      sla: { budgetHours: 48, consumedHours: 20, remainingHours: 28, pctConsumed: 0.42, slaState: "OK", slaRunning: true },
      title: "Untitled thing",
      abtTeam: null,
      openedBy: null,
    },
  ],
  total: 2,
  limit: 20,
  offset: 0,
  hasMore: false,
};

/** A fetch mock backing overview/taxonomy from the fixtures above, with issuesBody for /issues. */
function fetchMockFor(issuesBody: unknown = { issues: [], total: 0, limit: 20, offset: 0, hasMore: false }) {
  return vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url.includes("/metrics/overview")) return Promise.resolve(jsonResponse(EMPTY_OVERVIEW));
    if (url.includes("/issues")) return Promise.resolve(jsonResponse(issuesBody));
    if (url.includes("/taxonomy")) return Promise.resolve(jsonResponse(TAXONOMY));
    return Promise.reject(new Error(`unexpected fetch: ${url}`));
  });
}

/** Renders IssuesPage under a fresh QueryClient and a memory router at /issues. */
function renderIssuesPage(initialEntries: string[] = ["/issues"]) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/issues", element: <IssuesPage /> }], { initialEntries });
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

  it("clicking a sortable column header sets sort/order URL params", async () => {
    vi.stubGlobal("fetch", fetchMockFor());

    const router = renderIssuesPage();
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    act(() => {
      fireEvent.click(screen.getByText("Created"));
    });

    expect(router.state.location.search).toContain("sort=created");
  });

  it("ticking two Status options sets repeated status= URL params, OR-ed together", async () => {
    vi.stubGlobal("fetch", fetchMockFor());

    const router = renderIssuesPage();
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    const statusSelect = screen.getByLabelText("Status");
    act(() => {
      fireEvent.mouseDown(statusSelect);
    });
    act(() => {
      fireEvent.click(screen.getByRole("option", { name: "WOC" }));
    });
    act(() => {
      fireEvent.click(screen.getByRole("option", { name: "Pending Patch Queue" }));
    });

    const statusValues = new URLSearchParams(router.state.location.search).getAll("status");
    expect(statusValues).toEqual(["WOC", "Pending Patch Queue"]);
  });

  it('shows "Clear filters (n)" once a filter is active, and clears the filter keys and bucket', async () => {
    vi.stubGlobal("fetch", fetchMockFor());

    const router = renderIssuesPage(["/issues?status=WOC&bucket=cs"]);
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    // 1 dropdown with a selection (status) + 1 for the bucket scope chip = 2.
    const clearButton = screen.getByText("Clear filters (2)");
    act(() => {
      fireEvent.click(clearButton);
    });

    const search = router.state.location.search;
    expect(search).not.toContain("status=");
    expect(search).not.toContain("bucket=");
  });

  it('renders a removable scope chip for a bucket with no exact dropdown equivalent, e.g. "on_track"', async () => {
    vi.stubGlobal("fetch", fetchMockFor());

    const router = renderIssuesPage(["/issues?bucket=on_track"]);
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    const chipLabel = screen.getByText("On track (excluding CS side)");
    expect(chipLabel).toBeTruthy();

    const chip = chipLabel.closest(".MuiChip-root");
    const deleteIcon = chip!.querySelector(".MuiChip-deleteIcon")!;
    act(() => {
      fireEvent.click(deleteIcon);
    });

    expect(router.state.location.search).not.toContain("bucket=");
  });

  it("renders titles inline and the Opened by column without a separate titles request", async () => {
    vi.stubGlobal("fetch", fetchMockFor(ISSUES_WITH_TITLES));

    renderIssuesPage();

    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    // The title renders directly from the /issues response, with no round trip
    // to a titles endpoint.
    expect(screen.getByText("Fix the widget")).toBeTruthy();
    expect(screen.getByText("Untitled thing")).toBeTruthy();
    expect(screen.getByText("person@wso2.com")).toBeTruthy();
    // The other issue's openedBy is null; both rows' sla is non-null so this
    // "—" can only be the empty Opened by cell.
    expect(screen.getByText("—")).toBeTruthy();

    // Only these three endpoints back the page; any other endpoint being
    // hit (including a round trip to fetch titles separately) would fail
    // this assertion.
    const calledEndpoints = new Set(
      (vi.mocked(fetch).mock.calls).map(([input]) => new URL(String(input)).pathname),
    );
    expect(calledEndpoints).toEqual(new Set(["/issues", "/metrics/overview", "/taxonomy"]));
  });

  it("renders exactly the five filter dropdowns, none of the app header's own selects", async () => {
    vi.stubGlobal("fetch", fetchMockFor());

    renderIssuesPage();

    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    const comboboxes = screen.getAllByRole("combobox");
    expect(comboboxes).toHaveLength(5);
    expect(screen.getByLabelText("Project")).toBeTruthy();
    expect(screen.getByLabelText("Priority")).toBeTruthy();
    expect(screen.getByLabelText("ABT Team")).toBeTruthy();
    expect(screen.getByLabelText("Status")).toBeTruthy();
    expect(screen.getByLabelText("SLA state")).toBeTruthy();
  });
});
