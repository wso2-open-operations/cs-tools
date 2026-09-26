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
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FetchProgressBar, FetchProgressProvider } from "@components/FetchProgressBar";
import { useFetchProgressActive } from "@lib/fetchProgress";
import DashboardPage from "./DashboardPage";

// Mirrors how AppShell wires the page-reported placeholder-data state into
// the progress bar it renders above its own header — DashboardPage itself no
// longer renders a progress bar in its own DOM.
function ProgressBarHost() {
  const active = useFetchProgressActive();
  return <FetchProgressBar active={active} />;
}

/** Builds a 200 OK Response with a JSON body, for mocking fetch. */
function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

const OVERVIEW = {
  refreshedAt: "2026-01-01T00:00:00Z",
  filters: { repo: null, priority: null, abtTeam: null },
  abtTeams: [],
  hero: {
    violated: { n: 1, delta: 0, spark: [] },
    atRisk: { n: 0, delta: 0, spark: [] },
    cs: { n: 0, byStatus: [] },
    productSide: { n: 0, delta: 0, spark: [] },
  },
  projects: [
    {
      repoId: 1,
      name: "Alpha",
      repo: "org/alpha",
      violated: 1,
      atRisk: 0,
      cs: 0,
      onTrack: 3,
      openTracked: 4,
      untracked: 0,
      worst: true,
      allClear: false,
    },
    {
      repoId: 2,
      name: "Beta",
      repo: "org/beta",
      violated: 0,
      atRisk: 0,
      cs: 0,
      onTrack: 2,
      openTracked: 2,
      untracked: 0,
      worst: false,
      allClear: true,
    },
  ],
  priorities: [
    { key: "Critical(P1)", code: "P1", label: "Critical", budgetHours: 24, violated: 1, atRisk: 0, cs: 0, onTrack: 0, total: 1 },
    { key: "High(P2)", code: "P2", label: "High", budgetHours: 24, violated: 0, atRisk: 0, cs: 0, onTrack: 2, total: 2 },
    { key: "Medium(P3)", code: "P3", label: "Medium", budgetHours: 48, violated: 0, atRisk: 0, cs: 0, onTrack: 3, total: 3 },
  ],
  matrix: { rows: [], totals: { violated: 0, atRisk: 0, onTrack: 0, cs: 0 }, grandTotal: 0 },
  volume: [],
  unknownStatuses: [],
};

/** Renders DashboardPage under a fresh QueryClient and a memory router at /, with a stub /issues route so a drill navigation has somewhere to land. */
function renderDashboardPage(fetchMock: ReturnType<typeof vi.fn>, initialEntries: string[] = ["/"]) {
  vi.stubGlobal("fetch", fetchMock);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/", element: <DashboardPage /> },
      { path: "/issues", element: <div>Issues</div> },
    ],
    { initialEntries },
  );
  render(
    <QueryClientProvider client={queryClient}>
      <FetchProgressProvider>
        <ProgressBarHost />
        <RouterProvider router={router} />
      </FetchProgressProvider>
    </QueryClientProvider>,
  );
  return router;
}

describe("DashboardPage", () => {
  beforeEach(() => {
    // @ts-expect-error -- partial config is fine for this test
    window.config = { GID_BACKEND_BASE_URL: "https://backend.example.test" };
    // jsdom has no ResizeObserver; recharts' ResponsiveContainer (used by TimeseriesChart) needs one.
    vi.stubGlobal(
      "ResizeObserver",
      class {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    // @ts-expect-error -- test-only cleanup of the global window.config
    delete window.config;
  });

  it("shows the progress bar (not the full skeleton) while a filter change's fetch is in flight", async () => {
    let resolveFocusedOverview: ((r: Response) => void) | undefined;

    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/metrics/overview")) {
        if (url.includes("repo=")) {
          return new Promise<Response>((resolve) => {
            resolveFocusedOverview = resolve;
          });
        }
        return Promise.resolve(jsonResponse(OVERVIEW));
      }
      if (url.includes("/metrics/timeseries"))
        return Promise.resolve(jsonResponse({ window: 12, metric: "violated", groupBy: "priority", dates: [], series: [] }));
      if (url.includes("/taxonomy")) return Promise.resolve(jsonResponse({ statuses: [], csStatuses: [] }));
      if (url.includes("/issues")) return Promise.resolve(jsonResponse([]));
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });

    renderDashboardPage(fetchMock);

    // Cold load settles: the project card renders, no progress bar yet.
    const focusButton = await screen.findByRole("button", { name: "Focus project Alpha" });
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();

    // Focus that project — this changes the `repo` filter and the overview
    // query key, which the held promise above turns into a pending fetch.
    fireEvent.click(focusButton);

    await waitFor(() => expect(screen.getByRole("progressbar")).toBeInTheDocument());
    // The full-page skeleton must not reappear, and the previous data stays visible.
    expect(document.querySelectorAll(".MuiSkeleton-root")).toHaveLength(0);
    expect(screen.getByRole("button", { name: "Focus project Alpha" })).toBeInTheDocument();

    resolveFocusedOverview?.(jsonResponse(OVERVIEW));

    await waitFor(() => expect(screen.queryByRole("progressbar")).not.toBeInTheDocument());
  });

  it("carries the abtTeam filter through a drill link", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/metrics/overview")) return Promise.resolve(jsonResponse(OVERVIEW));
      if (url.includes("/metrics/timeseries"))
        return Promise.resolve(jsonResponse({ window: 12, metric: "violated", groupBy: "priority", dates: [], series: [] }));
      if (url.includes("/taxonomy")) return Promise.resolve(jsonResponse({ statuses: [], csStatuses: [] }));
      if (url.includes("/issues")) return Promise.resolve(jsonResponse([]));
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });

    const router = renderDashboardPage(fetchMock, ["/?abtTeam=Atlas"]);

    const violatedButton = await screen.findByTitle("View violated issues");
    fireEvent.click(violatedButton);

    await waitFor(() => expect(router.state.location.pathname).toBe("/issues"));
    expect(router.state.location.search).toContain("abtTeam=Atlas");
    expect(router.state.location.search).toContain("slaState=VIOLATED");
  });

  it("ticks every configured priority tier instead of a bucket=tracked scope chip when drilling 'Open tracked'", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.includes("/metrics/overview")) return Promise.resolve(jsonResponse(OVERVIEW));
      if (url.includes("/metrics/timeseries"))
        return Promise.resolve(jsonResponse({ window: 12, metric: "violated", groupBy: "priority", dates: [], series: [] }));
      if (url.includes("/taxonomy")) return Promise.resolve(jsonResponse({ statuses: [], csStatuses: [] }));
      if (url.includes("/issues")) return Promise.resolve(jsonResponse([]));
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });

    const router = renderDashboardPage(fetchMock);

    // Alpha's card renders first among the two project cards.
    const openTrackedButtons = await screen.findAllByText("Open tracked");
    fireEvent.click(openTrackedButtons[0]);

    await waitFor(() => expect(router.state.location.pathname).toBe("/issues"));
    const search = new URLSearchParams(router.state.location.search);
    expect(search.getAll("priority")).toEqual(["Critical(P1)", "High(P2)", "Medium(P3)"]);
    expect(search.get("bucket")).toBeNull();
  });
});
