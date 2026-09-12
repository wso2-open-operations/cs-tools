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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import { Children, cloneElement, isValidElement, type ReactElement, type ReactNode } from "react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// A `shape: "list"` tile now renders through widgetListConfig.tsx, which
// pulls in useTimeSheets.ts (time_card's mapper) — that module reads
// `window.config` at load via `@config/apiConfig`, unavailable under vitest.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
const SIGNED_IN_USER_ID = "11111111-aaaa-bbbb-cccc-000000000001";
// Mutable so a test can reproduce the window before `GET /users/me` resolves,
// which is a real window: CurrentUserProvider does not gate its children on
// that fetch. Reset to the signed-in user in `beforeEach`.
let mockCurrentUserId: string | undefined = SIGNED_IN_USER_ID;

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: mockCurrentUserId === undefined ? undefined : { id: mockCurrentUserId },
    isLoading: mockCurrentUserId === undefined,
    isError: false,
  }),
}));
// Recharts' ResponsiveContainer measures a real layout size, which jsdom
// always reports as 0 — nothing would render. Stubbed to a plain list of
// slice buttons (label + value), enough to assert on data/clicks without
// depending on actual SVG geometry (same approach the customer-portal app's
// own chart tests use for this same package).
vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  ResponsiveContainer: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PieChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Pie: ({
    data,
    onClick,
  }: {
    data: { name: string; value: number }[];
    onClick?: (item: unknown, index: number, event?: unknown) => void;
  }) => (
    <div>
      {data.map((item, i) => (
        // Forwards the real click event as recharts' own `Pie.onClick`
        // does (data, index, event) — DashboardPieChart's wedge onClick
        // relies on that third argument to stop the click from also
        // bubbling to the tile-level click-through.
        <button key={item.name} type="button" onClick={(e) => onClick?.(item, i, e)}>
          slice:{item.name}:{item.value}
        </button>
      ))}
    </div>
  ),
  Cell: () => null,
  // `data` is a BarChart-level prop (not Bar's own) in the real package —
  // clone it onto the Bar child so the mock below can read it.
  BarChart: ({
    children,
    data,
  }: {
    children: ReactNode;
    data?: { name: string; value: number }[];
  }) => (
    <div>
      {Children.map(children, (child) =>
        isValidElement(child)
          ? cloneElement(child as ReactElement<{ data?: typeof data }>, { data })
          : child,
      )}
    </div>
  ),
  Bar: ({
    data,
    onClick,
  }: {
    data?: { name: string; value: number }[];
    onClick?: (item: unknown, index: number, event?: unknown) => void;
  }) => (
    <div>
      {(data ?? []).map((item, i) => (
        // Forwards the real click event, same rationale as the Pie mock
        // above.
        <button key={item.name} type="button" onClick={(e) => onClick?.(item, i, e)}>
          bar:{item.name}:{item.value}
        </button>
      ))}
    </div>
  ),
}));

import type { BeDashboardPieSlice } from "@api/backend/types";
import DashboardWidgetTile from "@features/csm-dashboard/components/DashboardWidgetTile";
import { CURRENT_TEAM_PLACEHOLDER } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { CURRENT_USER_PLACEHOLDER } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import { __resetWidgetFetchConcurrencyForTests } from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return (
    <>
      <div data-testid="location-probe">{location.pathname + location.search}</div>
      <div data-testid="location-state-probe">{JSON.stringify(location.state ?? null)}</div>
    </>
  );
}

/** For tests that need to observe where a click actually navigated to —
 * `renderWithClient` has no destination route to land on. */
function renderWithRoutes(ui: ReactNode, destinationPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route path="/" element={ui} />
          <Route path={destinationPath} element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DashboardWidgetTile", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockCurrentUserId = SIGNED_IN_USER_ID;
    // The concurrency semaphore (widgetFetchConcurrency.ts) is a
    // module-level singleton shared across this whole file — a test below
    // deliberately leaves its own fetch pending forever
    // (`new Promise(() => {})`) to assert a loading state, which would
    // otherwise permanently hold its slot and starve every later test in
    // this file (fatal at WIDGET_FETCH_CONCURRENCY_LIMIT === 1, since
    // there is then nothing left to acquire).
    __resetWidgetFetchConcurrencyForTests();
  });

  it("renders a skeleton while its own count is in flight", () => {
    postMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1);
  });

  it("renders the resolved count once its own /cases/search call succeeds", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(screen.getByText("My Patches")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {},
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("renders its own error state when its /cases/search call fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );

    await waitFor(() =>
      expect(screen.getByText("Could not load this widget.")).toBeInTheDocument(),
    );
  });

  it("issues no search at all while the signed-in user's profile is still loading, rather than one without the user filter", async () => {
    mockCurrentUserId = undefined;
    postMock.mockResolvedValue({ total: 999, cases: [], limit: 1, offset: 0, hasMore: false });

    const { container } = renderWithClient(
      <DashboardWidgetTile
        widgetId="my_cases_count"
        displayName="My Cases"
        resourceType="case"
        shape="count"
        filters={{
          filters: [{ field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] }],
        }}
      />,
    );

    // The regression this guards: the placeholder used to be dropped while
    // the profile loaded, leaving an unfiltered /cases/search that painted
    // every engineer's case count into a tile labelled "My Cases".
    await waitFor(() =>
      expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1),
    );
    expect(postMock).not.toHaveBeenCalled();
    expect(screen.queryByText("999")).not.toBeInTheDocument();
  });

  it("keeps a mixed user filter's literal values while the profile is still loading", async () => {
    mockCurrentUserId = undefined;
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_and_me"
        displayName="Team and me"
        resourceType="case"
        shape="count"
        filters={{
          filters: [
            {
              field: "assignedUserId",
              op: "in",
              values: ["22222222-bbbb-cccc-dddd-000000000002", CURRENT_USER_PLACEHOLDER],
            },
          ],
        }}
      />,
    );

    // Deferred, so nothing goes out; the point of the assertion is that the
    // literal co-value is not discarded on the way to that decision.
    expect(postMock).not.toHaveBeenCalled();
  });

  it("resolves the __current_user__ filter placeholder with the signed-in user's own id before firing its /cases/search call", async () => {
    postMock.mockResolvedValue({ total: 1, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_cases_count"
        displayName="My Cases"
        resourceType="case"
        shape="count"
        filters={{
          filters: [
            { field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] },
          ],
        }}
      />,
    );

    await waitFor(() => expect(screen.getByText("1")).toBeInTheDocument());
    // "11111111-aaaa-bbbb-cccc-000000000001" is the mocked signed-in user's
    // own id (see the CurrentUserContext mock above).
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {
          filters: [
            {
              field: "assignedUserId",
              op: "in",
              values: ["11111111-aaaa-bbbb-cccc-000000000001"],
            },
          ],
        },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("renders the same table the Cases tab uses for shape: list, capped at listLimit", async () => {
    postMock.mockResolvedValue({
      total: 2,
      cases: [
        { id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" },
        {
          id: "22222222-2222-2222-2222-222222222222",
          number: "CS-2",
          subject: "Auth failing",
          state: "work_in_progress",
        },
      ],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.getByText("Disk full")).toBeInTheDocument();
    expect(screen.getByText("CS-2")).toBeInTheDocument();
    expect(screen.getByText("Auth failing")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {},
        pagination: { offset: 0, limit: 5 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("shape list: shows the widget's own total count, not just the capped row count shown below it", async () => {
    postMock.mockResolvedValue({
      total: 42,
      cases: [
        { id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" },
      ],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("shows a 'View more' link through to the full tab only when more records exist than shown", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.queryByRole("link", { name: /view more/i })).not.toBeInTheDocument();

    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open_2"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    // Goes to the widget's own preview page (real, bookmarkable URL — see
    // widgetPreviewUrl.ts), not straight to the resource's own tab.
    expect(href.startsWith("/dashboard/preview/cases?")).toBe(true);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("w")).toBe("my_critical_open_2");
    expect(params.get("n")).toBe("My Critical & High Cases");
    expect(params.get("f")).toBeNull();
  });

  it("masks the signed-in user's own id in the 'View more' link's filter query params", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_cases"
        displayName="My Cases"
        resourceType="case"
        shape="list"
        // "11111111-aaaa-bbbb-cccc-000000000001" is the mocked signed-in
        // user's own id (see the CurrentUserContext mock above) — it must
        // never appear verbatim in the resulting URL. Matches the real
        // DASHBOARDS_CONFIG shape: the widget's opaque case filters are the
        // generic field/op/values DSL nested under `filters.filters`.
        filters={{
          filters: [
            {
              field: "assignedUserId",
              op: "in",
              values: ["11111111-aaaa-bbbb-cccc-000000000001"],
            },
          ],
        }}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    expect(href).not.toContain("11111111-aaaa-bbbb-cccc-000000000001");
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("assignedUserId")).toBe("@me");
  });

  it("resolves the __current_team__ placeholder before it reaches the 'View more' href (list-shape), so the drill-down page never falls back to querying every team", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_cases"
        displayName="Team Open Cases"
        resourceType="case"
        shape="list"
        filters={{
          filters: [
            { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
          ],
        }}
        listLimit={5}
        selectedTeamCreGroupId="22222222-2222-2222-2222-222222222222"
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    // The literal placeholder must never reach the URL — the destination
    // preview page has no team context of its own to resolve it with, so a
    // still-placeholder-carrying filter there silently gets DROPPED
    // (fail-open — see teamFilterPlaceholder.ts), widening the query to
    // every team's cases instead of just the viewer's own team's.
    expect(href).not.toContain(CURRENT_TEAM_PLACEHOLDER);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("creTeam")).toBe("22222222-2222-2222-2222-222222222222");
  });

  it("drops the creTeam filter from the 'View more' href (list-shape) rather than sending the literal placeholder when no team groupId is selected", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_cases"
        displayName="Team Open Cases"
        resourceType="case"
        shape="list"
        filters={{
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
          ],
        }}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    expect(href).not.toContain(CURRENT_TEAM_PLACEHOLDER);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("creTeam")).toBeNull();
    expect(params.get("state")).toBe("open");
  });

  it("renders through the existing hardcoded per-resourceType renderer (CasesList) when no columns are configured — byte-for-byte unaffected by the columns feature", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    // CasesList's own header markup ("Case ID") only appears via the
    // hardcoded renderer — GenericColumnList never renders it, since its
    // columns come entirely from the widget's own `columns` config.
    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.getByText("Case ID")).toBeInTheDocument();
  });

  it("resolves the __current_user__ placeholder (raw, as the backend now sends it unresolved) to the signed-in user's own id before it reaches the 'View more' href, where it's then masked to @me", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_cases"
        displayName="My Cases"
        resourceType="case"
        shape="list"
        filters={{
          filters: [
            { field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] },
          ],
        }}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    expect(href).not.toContain(CURRENT_USER_PLACEHOLDER);
    expect(href).not.toContain("11111111-aaaa-bbbb-cccc-000000000001");
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("assignedUserId")).toBe("@me");
  });

  it("renders through the generic column renderer, resolving a nested dot-path, when columns are configured", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [
        {
          id: "11111111-1111-1111-1111-111111111111",
          subject: "Disk full",
          bestCaseFixEta: "2026-08-01",
          project: { id: "p-1", name: "Alpha", key: "ALPHA" },
        },
      ],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
        columns={[
          { path: "subject", label: "Subject" },
          { path: "project.key", label: "Project key" },
          { path: "bestCaseFixEta", label: "Best case ETA", format: "date" },
        ]}
      />,
    );

    // Column headers come from the widget's own config, not a hardcoded
    // per-resourceType label set.
    await waitFor(() => expect(screen.getByText("Subject")).toBeInTheDocument());
    expect(screen.getByText("Project key")).toBeInTheDocument();
    expect(screen.getByText("Best case ETA")).toBeInTheDocument();

    // Row cells: a top-level field, a nested dot-path field, and a
    // date-formatted field.
    expect(screen.getByText("Disk full")).toBeInTheDocument();
    expect(screen.getByText("ALPHA")).toBeInTheDocument();
    expect(screen.getByText("Aug 1, 2026")).toBeInTheDocument();

    // The hardcoded CasesList renderer's own "Case ID" column header must
    // NOT appear — this widget rendered through GenericColumnList instead.
    expect(screen.queryByText("Case ID")).not.toBeInTheDocument();
  });

  it("forwards a widget's configured sortBy into the /search request, only for shape list", async () => {
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 5, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
        sortBy={{ field: "updatedOn", order: "asc" }}
      />,
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/search",
        {
          filters: {},
          pagination: { offset: 0, limit: 5 },
          sortBy: { field: "updatedOn", order: "asc" },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("does not forward sortBy for shape count (only meaningful for shape list)", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
        sortBy={{ field: "updatedOn", order: "asc" }}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {},
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("navigates to /cases with translated filters when a case-resource tile is clicked", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{
          filters: [
            { field: "severity", op: "in", values: ["critical"] },
            { field: "state", op: "in", values: ["open"] },
          ],
        }}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());

    const link = screen.getByRole("link");
    const href = link.getAttribute("href") ?? "";
    expect(href.startsWith("/cases?")).toBe(true);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("resolves a relative-date filter in the click-through href, so the destination list matches the number that was clicked", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="created_today"
        displayName="Created Today"
        resourceType="case"
        shape="count"
        filters={{
          filters: [{ field: "createdOn", op: "gte", values: ["__today__"] }],
        }}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());

    const params = new URLSearchParams(
      (screen.getByRole("link").getAttribute("href") ?? "").split("?")[1],
    );
    const createdFrom = params.get("createdFrom") ?? "";
    // An unresolved placeholder reaching the list page is forwarded to the
    // backing service, which resolves "today" against UTC rather than the
    // viewer's own local day — so the tile's count and the list behind it
    // disagreed by the offset between the two midnights. The href must
    // carry the same browser-local instant the tile itself counted with.
    expect(createdFrom).not.toContain("__today__");
    const now = new Date();
    expect(createdFrom).toBe(
      new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0).toISOString(),
    );
  });

  it("resolves a relative-date filter in the View more preview href too", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [
        {
          id: "11111111-1111-1111-1111-111111111111",
          number: "CS-1",
          subject: "Disk full",
          state: "open",
        },
      ],
      limit: 5,
      offset: 0,
      hasMore: true,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="created_today_list"
        displayName="Created Today"
        resourceType="case"
        shape="list"
        filters={{
          filters: [{ field: "createdOn", op: "gte", values: ["__today__"] }],
        }}
        listLimit={5}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    expect(viewMoreLink.getAttribute("href") ?? "").not.toContain("__today__");
  });

  it("shape count: click-through carries a `from` location.state pointing back to this dashboard page, so the destination list can offer a Back button", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("link"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    expect(screen.getByTestId("location-state-probe").textContent).toBe(
      JSON.stringify({ from: "/" }),
    );
  });

  it("resolves the __current_team__ placeholder with the selected team's groupId in both the /search request and the count tile's own click-through href", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_cases"
        displayName="Team Open Cases"
        resourceType="case"
        shape="count"
        filters={{
          filters: [
            { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
          ],
        }}
        selectedTeamCreGroupId="22222222-2222-2222-2222-222222222222"
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {
          filters: [
            {
              field: "creTeam",
              op: "in",
              values: ["22222222-2222-2222-2222-222222222222"],
            },
          ],
        },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("re-fetches with the new team's own filters when selectedTeamCreGroupId changes (team switch must not reuse a stale cached query)", async () => {
    // Regression guard: this widget's react-query queryKey must include the
    // RESOLVED filters (placeholder already substituted with the selected
    // team's groupId), not the raw `filters` prop (which still carries the
    // literal __current_team__ placeholder, identical for every team). A
    // queryKey built from the raw prop would hash the same regardless of
    // which team is selected, so switching teams would keep serving the
    // first team's cached response — the exact bug this guards against.
    const filters = {
      filters: [{ field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] }],
    };
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    postMock.mockResolvedValueOnce({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    const { rerender } = render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <DashboardWidgetTile
            widgetId="team_open_cases"
            displayName="Team Open Cases"
            resourceType="case"
            shape="count"
            filters={filters}
            selectedTeamCreGroupId="team-a-group-id"
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(postMock).toHaveBeenLastCalledWith(
      "/cases/search",
      {
        filters: { filters: [{ field: "creTeam", op: "in", values: ["team-a-group-id"] }] },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );

    postMock.mockResolvedValueOnce({ total: 9, cases: [], limit: 1, offset: 0, hasMore: false });

    rerender(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <DashboardWidgetTile
            widgetId="team_open_cases"
            displayName="Team Open Cases"
            resourceType="case"
            shape="count"
            filters={filters}
            selectedTeamCreGroupId="team-b-group-id"
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(screen.getByText("9")).toBeInTheDocument());
    expect(postMock).toHaveBeenLastCalledWith(
      "/cases/search",
      {
        filters: { filters: [{ field: "creTeam", op: "in", values: ["team-b-group-id"] }] },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("drops the creTeam filter (request and href) rather than sending the literal placeholder when no team groupId is selected", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_cases"
        displayName="Team Open Cases"
        resourceType="case"
        shape="count"
        filters={{
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
          ],
        }}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );

    const link = screen.getByRole("link");
    expect(link.getAttribute("href") ?? "").not.toContain(CURRENT_TEAM_PLACEHOLDER);
  });

  it("shape pie: resolves __current_team__ in a slice click-through href using the selected team's groupId", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-team"
        displayName="Cases by team"
        resourceType="case"
        shape="pie"
        filters={{}}
        slices={[
          {
            label: "My team",
            query: {
              filters: [
                { field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
              ],
            },
          },
        ]}
        selectedTeamCreGroupId="22222222-2222-2222-2222-222222222222"
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:My team:2")).toBeInTheDocument());
    fireEvent.click(screen.getByText("slice:My team:2"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(probeText).not.toContain(CURRENT_TEAM_PLACEHOLDER);
  });

  it("shape bar: issues one search per slice and renders a bar per slice, clickable the same way as a pie slice", async () => {
    postMock.mockImplementation(
      (_path: string, body: { filters: { filters: { field: string; values?: string[] }[] } }) => {
        const severity = body.filters.filters.find((f) => f.field === "severity")?.values;
        if (severity?.includes("critical")) return Promise.resolve({ total: 1 });
        if (severity?.includes("high")) return Promise.resolve({ total: 3 });
        return Promise.resolve({ total: 0 });
      },
    );

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases_by_severity"
        displayName="Open Cases by Severity"
        resourceType="case"
        shape="bar"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "S1 · Critical",
            color: "error",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
          {
            label: "S2 · High",
            color: "warning",
            query: { filters: [{ field: "severity", op: "in", values: ["high"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("bar:S1 · Critical:1")).toBeInTheDocument());
    expect(screen.getByText("bar:S2 · High:3")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "severity", op: "in", values: ["critical"] },
          ],
        },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );

    fireEvent.click(screen.getByText("bar:S1 · Critical:1"));
    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape bar: shows the widget's overall total (sum of every slice) next to its title", async () => {
    postMock.mockImplementation(
      (_path: string, body: { filters: { filters: { field: string; values?: string[] }[] } }) => {
        const severity = body.filters.filters.find((f) => f.field === "severity")?.values;
        if (severity?.includes("critical")) return Promise.resolve({ total: 1 });
        if (severity?.includes("high")) return Promise.resolve({ total: 3 });
        return Promise.resolve({ total: 0 });
      },
    );

    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases_by_severity"
        displayName="Open Cases by Severity"
        resourceType="case"
        shape="bar"
        filters={{}}
        slices={[
          {
            label: "S1 · Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
          {
            label: "S2 · High",
            query: { filters: [{ field: "severity", op: "in", values: ["high"] }] },
          },
        ]}
      />,
    );

    await waitFor(() => expect(screen.getByText("bar:S1 · Critical:1")).toBeInTheDocument());
    expect(screen.getByText("4")).toBeInTheDocument();
  });

  it("shape pie: issues one search per slice (own filters merged under the widget's base filters) and renders values + percentages", async () => {
    postMock.mockImplementation(
      (_path: string, body: { filters: { filters: { field: string; values?: string[] }[] } }) => {
        const severity = body.filters.filters.find((f) => f.field === "severity")?.values;
        if (severity?.includes("critical")) return Promise.resolve({ total: 1 });
        if (severity?.includes("high")) return Promise.resolve({ total: 3 });
        return Promise.resolve({ total: 0 });
      },
    );

    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        description="Share of active cases at each severity level."
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "S1 · Critical",
            color: "error",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
          {
            label: "S2 · High",
            color: "warning",
            query: { filters: [{ field: "severity", op: "in", values: ["high"] }] },
          },
        ]}
      />,
    );

    expect(screen.getByText("Share of active cases at each severity level.")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("slice:S1 · Critical:1")).toBeInTheDocument());
    expect(screen.getByText("slice:S2 · High:3")).toBeInTheDocument();
    expect(screen.getByText("1 (25%)")).toBeInTheDocument();
    expect(screen.getByText("3 (75%)")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "severity", op: "in", values: ["critical"] },
          ],
        },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: {
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "severity", op: "in", values: ["high"] },
          ],
        },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("shape pie: renders a groupBy-configured widget via a single group-by call, including a synthetic Others slice", async () => {
    postMock.mockResolvedValue({
      groups: [
        { key: "critical", label: "S1 · Critical", count: 1 },
        { key: "high", label: "S2 · High", count: 3 },
      ],
      othersCount: 4,
      totalRecords: 8,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        groupBy={{ field: "severity", maxGroups: 2 }}
      />,
    );

    await waitFor(() => expect(screen.getByText("slice:S1 · Critical:1")).toBeInTheDocument());
    expect(screen.getByText("slice:S2 · High:3")).toBeInTheDocument();
    expect(screen.getByText("slice:Others:4")).toBeInTheDocument();
    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/aggregate",
      {
        filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
        groupBy: "severity",
        maxGroups: 2,
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("shape pie: clicking a slice navigates to /cases with the widget's base filters merged under that slice's own filters", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    fireEvent.click(screen.getByText("slice:Critical:2"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(probeText.startsWith("/cases?")).toBe(true);
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape pie: clicking a slice carries a `from` location.state pointing back to this dashboard page", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    fireEvent.click(screen.getByText("slice:Critical:2"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    expect(screen.getByTestId("location-state-probe").textContent).toBe(
      JSON.stringify({ from: "/" }),
    );
  });

  it("shape pie: clicking a legend row navigates the same way as clicking the slice", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("Critical")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Critical"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(new URLSearchParams(probeText.split("?")[1]).get("severities")).toBe("S1");
  });

  it("shape pie: clicking the tile itself (not a slice/legend row) navigates to the widget's own base filters", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    // The accessible tile-level click target (role="button", not a slice or
    // legend row) — its own aria-label names the widget.
    fireEvent.click(screen.getByRole("button", { name: "View all cases for Cases by severity" }));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(probeText.startsWith("/cases?")).toBe(true);
    const params = new URLSearchParams(probeText.split("?")[1]);
    // The tile's own base filters (state:open), NOT the slice's severity —
    // that's what distinguishes this from a slice/legend click.
    expect(params.get("states")).toBe("open");
    expect(params.get("severities")).toBeNull();
  });

  it("shape pie: clicking the tile itself carries a `from` location.state pointing back to this dashboard page", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "View all cases for Cases by severity" }));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    expect(screen.getByTestId("location-state-probe").textContent).toBe(
      JSON.stringify({ from: "/" }),
    );
  });

  it("shape pie: Enter on the focused tile activates the same tile-level click-through as a click", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    const tile = screen.getByRole("button", { name: "View all cases for Cases by severity" });
    tile.focus();
    fireEvent.keyDown(tile, { key: "Enter" });

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    expect(new URLSearchParams(probeText.split("?")[1]).get("states")).toBe("open");
  });

  it("shape pie: Space on the focused tile activates the same tile-level click-through as a click", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    const tile = screen.getByRole("button", { name: "View all cases for Cases by severity" });
    tile.focus();
    fireEvent.keyDown(tile, { key: " " });

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    // Same destination filters as a click — states:open (tile-level), not
    // the slice's own severity.
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("states")).toBe("open");
    expect(params.get("severities")).toBeNull();
  });

  it("shape pie: the tile-level click target is a sibling of the legend rows, not their ancestor", async () => {
    // Regression guard for the ancestor role="button" issue: a role="button"
    // (or role="link") ancestor makes its descendants' own roles
    // presentational to assistive tech, so the legend's own role="button"
    // rows must never be nested inside the tile-level click target — only
    // ever a sibling of it.
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("Critical")).toBeInTheDocument());
    const tile = screen.getByRole("button", { name: "View all cases for Cases by severity" });
    const legendRow = screen.getByRole("button", { name: /Critical: 2 cases/ });

    expect(tile.contains(legendRow)).toBe(false);
  });

  it("shape count: renders a per-widget refresh button that invalidates only this widget's own queries without navigating", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <DashboardWidgetTile
            widgetId="my_patches"
            displayName="My Patches"
            resourceType="case"
            shape="count"
            filters={{}}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    const refreshButton = screen.getByRole("button", { name: "Refresh My Patches" });
    expect(refreshButton).toBeInTheDocument();

    fireEvent.click(refreshButton);

    expect(invalidateSpy).toHaveBeenCalled();
    const predicate = invalidateSpy.mock.calls[0]?.[0]?.predicate;
    expect(typeof predicate).toBe("function");
    // Matches only this widget's own count query key, not some other
    // widget's — the whole point of scoping the refresh to a single tile.
    expect(
      predicate?.({
        queryKey: ["csm-dashboard-widget-data", "my_patches", "case", {}, undefined, undefined],
      } as never),
    ).toBe(true);
    expect(
      predicate?.({
        queryKey: ["csm-dashboard-widget-data", "some_other_widget", "case", {}, undefined, undefined],
      } as never),
    ).toBe(false);

    // Must not also trigger the tile's own whole-card navigation.
    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("shape count: folds 'Last refreshed' into the refresh button's tooltip (no room for an inline label on this shape), absent before click and present after", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    const refreshButton = screen.getByRole("button", { name: "Refresh My Patches" });

    // Not shown from the tile's own initial data load — only after a manual
    // refresh click (matches the section-level RefreshButton's own gating).
    // Count-shape tiles are too narrow for an inline label, so this never
    // renders as visible text on the tile itself either before or after the
    // click — only ever inside the tooltip.
    expect(screen.queryByText(/Last refreshed/)).not.toBeInTheDocument();
    fireEvent.mouseOver(refreshButton);
    await waitFor(() => expect(screen.getByRole("tooltip")).toBeInTheDocument());
    // Deliberately generic — no widget name (it's already visible next to
    // the tile's own title; including it made the combined tooltip too
    // long). The `aria-label` above still carries the widget name for
    // screen-reader users who need to tell apart multiple refresh buttons.
    expect(screen.getByRole("tooltip")).toHaveTextContent("Refresh this widget");
    expect(screen.getByRole("tooltip")).not.toHaveTextContent(/Last refreshed/);
    fireEvent.mouseOut(refreshButton);
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());

    fireEvent.click(refreshButton);
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));

    expect(screen.queryByText(/Last refreshed/)).not.toBeInTheDocument();
    fireEvent.mouseOver(refreshButton);
    await waitFor(() =>
      expect(screen.getByRole("tooltip")).toHaveTextContent(/Last refreshed.*(just now|ago)/),
    );
  });

  it("shape list: renders a per-widget refresh button", async () => {
    postMock.mockResolvedValue({
      total: 2,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(
      screen.getByRole("button", { name: "Refresh My Critical & High Cases" }),
    ).toBeInTheDocument();
  });

  it("shape list: shows the 'Customise columns' button next to refresh (same corner), not in a separate row, and toggling a column there updates the table live", async () => {
    // Reported live: the column customizer (added to case-family list-shape
    // tiles by CaseWidgetList) rendered in its own row above the table,
    // separate from the tile's existing refresh button in the top-right
    // corner — read as two unrelated controls. CaseWidgetList now hands its
    // button up to the tile via `onColumnCustomizerChange` so both sit in
    // the same corner; this also guards the single-source-of-truth wiring
    // behind that (a second, independent `useColumnPreferences` instance
    // would let the header's button drift out of sync with the table).
    postMock.mockResolvedValue({
      total: 2,
      cases: [
        {
          id: "11111111-1111-1111-1111-111111111111",
          number: "CS-1",
          subject: "Disk full",
          state: "open",
          createdBy: { id: null, email: "jane.doe@example.test", name: "Jane Doe" },
        },
      ],
      limit: 5,
      offset: 0,
      hasMore: false,
    });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_critical_open"
        displayName="My Critical & High Cases"
        resourceType="case"
        shape="list"
        filters={{}}
        listLimit={5}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());

    const actionsCorner = screen.getByTestId("widget-tile-actions");
    const refreshButton = within(actionsCorner).getByRole("button", {
      name: "Refresh My Critical & High Cases",
    });
    const customizerButton = within(actionsCorner).getByRole("button", {
      name: "Customise columns",
    });
    expect(refreshButton).toBeInTheDocument();
    expect(customizerButton).toBeInTheDocument();

    // Toggling "Reporter" (off by default) from the hoisted header button
    // must update the actual table, proving both share one live
    // `useColumnPreferences` instance rather than two independently-seeded
    // ones (which would leave the header's button out of sync with what
    // the table actually shows).
    fireEvent.click(customizerButton);
    const picker = screen.getByRole("list", { name: "Customise columns" });
    expect(screen.queryByText("Jane Doe")).not.toBeInTheDocument();
    fireEvent.click(within(picker).getByText("Reporter"));
    await waitFor(() => expect(screen.getByText("Jane Doe")).toBeInTheDocument());
  });

  it("shape pie: renders a per-widget refresh button that does not trigger the tile's own click-through", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    const refreshButton = screen.getByRole("button", { name: "Refresh Cases by severity" });
    fireEvent.click(refreshButton);

    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
  });

  it("shape bar: renders a per-widget refresh button", async () => {
    postMock.mockResolvedValue({ total: 0 });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases_by_severity"
        displayName="Open Cases by Severity"
        resourceType="case"
        shape="bar"
        filters={{}}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Refresh Open Cases by Severity" }),
    ).toBeInTheDocument();
  });

  it("shape count: shows an info icon with the widget's description in an accessible tooltip, only when a description is set", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    const { rerender } = renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        description="Cases assigned to you that carry an open patch."
        resourceType="case"
        shape="count"
        filters={{}}
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    const infoButton = screen.getByRole("button", { name: "About My Patches" });
    expect(infoButton).toBeInTheDocument();

    // The button existing proves nothing on its own — the tooltip could render
    // empty and still pass. Open it and assert it actually surfaces the
    // description text.
    fireEvent.mouseOver(infoButton);
    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent("Cases assigned to you that carry an open patch.");

    // It closes again on mouse-out, so the assertion above is about this
    // tooltip and not some permanently-mounted node.
    // (The keyboard path is not asserted here: the Tooltip opens on
    // :focus-visible, which fireEvent.focus does not produce and which needs
    // @testing-library/user-event — not a dependency of this app.)
    fireEvent.mouseOut(infoButton);
    await waitFor(() => expect(screen.queryByRole("tooltip")).not.toBeInTheDocument());

    // Only `description` changes across the rerender — `widgetId` stays
    // `my_patches`, so the disappearing icon can only be attributed to the
    // missing description and not to a different widget being rendered.
    rerender(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <MemoryRouter>
          <DashboardWidgetTile
            widgetId="my_patches"
            displayName="My Patches"
            resourceType="case"
            shape="count"
            filters={{}}
          />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "About My Patches" })).not.toBeInTheDocument(),
    );
  });

  it("shape pie: clicking a slice does NOT also trigger the tile-level click-through (no double-navigation)", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:2")).toBeInTheDocument());
    fireEvent.click(screen.getByText("slice:Critical:2"));

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    const params = new URLSearchParams(probeText.split("?")[1]);
    // Slice's own severity survives — if the tile-level handler had also
    // fired (bubbling not stopped), this would have been overwritten by the
    // base-filters-only navigation and severities would be absent.
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape pie: legend row is keyboard-activatable (Enter) the same as a click, and doesn't also trigger the tile-level click-through", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("Critical")).toBeInTheDocument());
    const legendRow = screen.getByRole("button", { name: /Critical: 2 cases/ });
    legendRow.focus();
    fireEvent.keyDown(legendRow, { key: "Enter" });

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape pie: legend row is keyboard-activatable (Space) the same as a click, and doesn't also trigger the tile-level click-through", async () => {
    postMock.mockResolvedValue({ total: 2 });

    renderWithRoutes(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          {
            label: "Critical",
            query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
          },
        ]}
      />,
      "/cases",
    );

    await waitFor(() => expect(screen.getByText("Critical")).toBeInTheDocument());
    const legendRow = screen.getByRole("button", { name: /Critical: 2 cases/ });
    legendRow.focus();
    fireEvent.keyDown(legendRow, { key: " " });

    await waitFor(() => expect(screen.getByTestId("location-probe")).toBeInTheDocument());
    const probeText = screen.getByTestId("location-probe").textContent ?? "";
    const params = new URLSearchParams(probeText.split("?")[1]);
    expect(params.get("severities")).toBe("S1");
    expect(params.get("states")).toBe("open");
  });

  it("shape bar: still shows a '0' total next to the title when the widget has no slices configured yet", async () => {
    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="bar"
        filters={{}}
      />,
    );

    expect(screen.getByText("Cases by severity")).toBeInTheDocument();
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });

  it("shape pie: renders an empty state (no slices, zero total) rather than crashing when a widget has no slices configured yet", async () => {
    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{}}
      />,
    );

    expect(screen.getByText("Cases by severity")).toBeInTheDocument();
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });

  // Regression test: DASHBOARDS_CONFIG is a raw JSON env var, not
  // schema-validated beyond basic decoding — a slice entry missing its own
  // `filters` field entirely used to crash the whole tile with "Cannot read
  // properties of undefined (reading 'filters')" inside mergeWidgetFilters,
  // despite the wire type declaring it required.
  it("shape pie: renders a slice whose config is missing its own `filters` instead of crashing", async () => {
    postMock.mockResolvedValue({ total: 4 });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="cases-by-severity"
        displayName="Cases by severity"
        resourceType="case"
        shape="pie"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slices={[
          // filters intentionally omitted
          { label: "Critical" } as BeDashboardPieSlice,
        ]}
      />,
    );

    await waitFor(() => expect(screen.getByText("slice:Critical:4")).toBeInTheDocument());
  });

  it("resolves the {{currentTeam}} text token in displayName/description to the selected team's own label", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_incidents"
        displayName="Open Incidents — {{currentTeam}}"
        description="Open incidents for {{currentTeam}}."
        resourceType="case"
        shape="count"
        filters={{}}
        selectedTeamLabel="Castor"
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(screen.getByText("Open Incidents — Castor")).toBeInTheDocument();
    const infoButton = screen.getByRole("button", { name: "About Open Incidents — Castor" });
    fireEvent.mouseOver(infoButton);
    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent("Open incidents for Castor.");
  });

  it("resolves the {{currentTeam}} text token to the literal 'All ABTs' when that's the selected team label", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_incidents"
        displayName="Open Incidents — {{currentTeam}}"
        resourceType="case"
        shape="count"
        filters={{}}
        selectedTeamLabel="All ABTs"
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(screen.getByText("Open Incidents — All ABTs")).toBeInTheDocument();
  });

  it("strips the {{currentTeam}} text token cleanly (no literal token, no dangling separator) when unresolved", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="team_open_incidents"
        displayName="Open Incidents — {{currentTeam}}"
        resourceType="case"
        shape="count"
        filters={{}}
        // No selectedTeamLabel passed — the unresolved case (non-team-based
        // dashboard, or the team list/user profile still loading).
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(screen.getByText("Open Incidents")).toBeInTheDocument();
    expect(screen.queryByText(/\{\{currentTeam\}\}/)).not.toBeInTheDocument();
  });

  it("renders an unsupported-widget message instead of crashing for an unrecognized resourceType", () => {
    renderWithClient(
      <DashboardWidgetTile
        widgetId="mystery_widget"
        displayName="Mystery Widget"
        // Simulates a resourceType the backend registry knows about (now
        // runtime JSON config, not compile-time checked) but this frontend
        // build doesn't yet have an entry for in WIDGET_RESOURCE_CONFIG.
        resourceType={"future_resource" as unknown as never}
        shape="count"
        filters={{}}
      />,
    );

    expect(screen.getByText("Mystery Widget")).toBeInTheDocument();
    expect(screen.getByText("Unsupported widget type.")).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });

  it("shape count: hides its own refresh button when hideRefreshButton is set (avoids covering the dashboard builder's Edit/Remove overlay)", async () => {
    postMock.mockResolvedValue({ total: 3, cases: [], limit: 1, offset: 0, hasMore: false });

    renderWithClient(
      <DashboardWidgetTile
        widgetId="my_patches"
        displayName="My Patches"
        resourceType="case"
        shape="count"
        filters={{}}
        hideRefreshButton
      />,
    );

    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    expect(
      screen.queryByRole("button", { name: "Refresh My Patches" }),
    ).not.toBeInTheDocument();
  });
});
