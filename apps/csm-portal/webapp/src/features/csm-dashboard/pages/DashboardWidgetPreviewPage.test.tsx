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
import { MemoryRouter, Route, Routes } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// Pulls in widgetListConfig.tsx -> useTimeSheets.ts (time_card's mapper),
// which reads `window.config` at load via `@config/apiConfig` — same
// workaround as DashboardWidgetTile.test.tsx.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
const CURRENT_USER_ID = "11111111-aaaa-bbbb-cccc-000000000001";
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: CURRENT_USER_ID },
    isLoading: false,
    isError: false,
  }),
}));
// `useGetCsmCases` (pulled in transitively once a case-family widget renders
// the real CasesFilterBar/CasesList) reads `useLogger`, which needs a
// `LoggerProvider` this test doesn't otherwise set up.
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "agent@wso2.com" }),
}));

import DashboardWidgetPreviewPage from "@features/csm-dashboard/pages/DashboardWidgetPreviewPage";
import { buildWidgetPreviewHref } from "@features/csm-dashboard/utils/widgetPreviewUrl";

function renderAt(initialEntry: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/dashboard" element={<div>Dashboard landing</div>} />
          <Route path="/dashboard/preview/:previewSlug" element={<DashboardWidgetPreviewPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

/** Routes `postMock` by URL, since a case-family widget's preview page
 * fires up to three different POST endpoints at once: `/teams/search`
 * (`CasesFilterBar`'s "Team" control loads unconditionally on mount),
 * `/tags/search` (only when the widget's own filter needs the tag
 * complement — see `DashboardWidgetPreviewPage.tsx`), and `/cases/search`
 * itself. */
function mockPost(responses: {
  teams?: unknown;
  tags?: unknown;
  cases?: unknown;
}): void {
  postMock.mockImplementation((url: string) => {
    if (url === "/teams/search") return Promise.resolve(responses.teams ?? { teams: [] });
    if (url === "/tags/search") return Promise.resolve(responses.tags ?? { tags: [] });
    if (url === "/cases/search") {
      return Promise.resolve(
        responses.cases ?? { cases: [], total: 0, limit: 10, offset: 0 },
      );
    }
    return Promise.resolve({});
  });
}

/**
 * `case_feedback`'s own "View more" landing: a rating-pie slice click-through
 * must land the viewer on the exact rating it clicked (regression for the
 * numeric-`rating`-dropped-from-URL bug — see `widgetPreviewUrl.ts`'s own
 * regression test for the encoding half of this), and — reported live as "a
 * must" — the rating/date filters must be real, editable controls, not the
 * generic resourceTypes' read-only "Filtered by:" chip summary.
 */
describe("DashboardWidgetPreviewPage — case_feedback gets a real, editable rating/date filter bar", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({
      results: [
        {
          instanceId: "fb-1",
          caseId: "11111111-1111-1111-1111-111111111111",
          rating: 5,
          ratingLabel: "Very Satisfied",
          comment: "Great support",
          submittedAt: "2026-08-01T00:00:00Z",
        },
      ],
      totalRecords: 1,
    });
  });

  it("queries with the rating a pie slice click-through carried, not the unfiltered list", async () => {
    // Mirrors what DashboardWidgetTile actually builds for a rating-pie
    // slice click (see useCaseFeedbackTrendData's `{ rating: 5 }` query,
    // merged and passed through WIDGET_RESOURCE_CONFIG.case_feedback.buildHref).
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "case-feedback",
        widgetId: "feedback_rating_distribution",
        displayName: "Rating Distribution",
        filters: { rating: 5 },
      }),
    );

    await waitFor(() => expect(screen.getByText("Very Satisfied")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/feedback/search",
      { filters: { rating: 5 }, page: 1, pageSize: 10 },
      { signal: expect.any(AbortSignal) },
    );
    // The rating select shows the slice's own rating pre-selected, not "All
    // ratings" -- confirms the filter bar is seeded, not just the query.
    expect(screen.getByRole("combobox", { name: /^rating$/i })).toHaveTextContent(
      /very satisfied/i,
    );
  });

  it("re-queries when the rating filter is changed from the dropdown", async () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "case-feedback",
        widgetId: "feedback_list",
        displayName: "Feedback Records",
        filters: {},
      }),
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/feedback/search",
        { filters: {}, page: 1, pageSize: 10 },
        { signal: expect.any(AbortSignal) },
      ),
    );

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^rating$/i }));
    fireEvent.click(screen.getByRole("option", { name: /1 — very dissatisfied/i }));

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/feedback/search",
        { filters: { rating: 1 }, page: 1, pageSize: 10 },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("Reset restores the widget's own original rating, not 'All ratings'", async () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "case-feedback",
        widgetId: "feedback_rating_distribution",
        displayName: "Rating Distribution",
        filters: { rating: 5 },
      }),
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/feedback/search",
        { filters: { rating: 5 }, page: 1, pageSize: 10 },
        { signal: expect.any(AbortSignal) },
      ),
    );

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^rating$/i }));
    fireEvent.click(screen.getByRole("option", { name: /all ratings/i }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/feedback/search",
        { filters: {}, page: 1, pageSize: 10 },
        { signal: expect.any(AbortSignal) },
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: /^reset$/i }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/feedback/search",
        { filters: { rating: 5 }, page: 1, pageSize: 10 },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });
});

/**
 * `call_request`'s own "View more" landing: unlike the generic
 * "Filtered by:" chip summary + free-text search box (which
 * `/call-requests/search` doesn't even support — it has no `searchQuery`
 * field), this gives a real, editable Simple-only filter bar (call state,
 * case state, assignee, CRE team) seeded from the widget's own flat
 * filters and feeding the same `useWidgetData` + `CallRequestWidgetList`
 * the tile itself renders.
 */
describe("DashboardWidgetPreviewPage — call_request gets a real, editable filter bar", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockImplementation((url: string) => {
      if (url === "/teams/search") {
        return Promise.resolve({
          teams: [
            {
              id: "castor",
              name: "Team Castor",
              family: "cre-abt",
              creGroupId: "33333333-3333-3333-3333-333333333333",
            },
          ],
          total: 1,
          limit: 100,
          offset: 0,
        });
      }
      if (url === "/call-requests/search") {
        return Promise.resolve({
          callRequests: [
            {
              id: "cr-1",
              number: "CR-1",
              reason: "Kickoff call",
              state: { id: 3, label: "Scheduled" },
              case: { id: "case-1", number: "CS-1" },
            },
          ],
          total: 1,
          limit: 10,
          offset: 0,
        });
      }
      return Promise.resolve({});
    });
  });

  it("renders the real filter bar (not the read-only chip summary), seeded from the widget's own flat filters", async () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "call-requests",
        widgetId: "team_open_calls",
        displayName: "Team Open Calls",
        filters: { states: ["scheduled"], caseStates: ["open"] },
      }),
    );

    await waitFor(() => expect(screen.getByText("CR-1")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: "Call state" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Case state" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "CRE Team" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();

    expect(postMock).toHaveBeenCalledWith(
      "/call-requests/search",
      expect.objectContaining({
        filters: { states: ["scheduled"], caseStates: ["open"] },
        pagination: { offset: 0, limit: 10 },
      }),
      { signal: expect.any(AbortSignal) },
    );
  });

  it("re-queries /call-requests/search when the Call state filter is changed from the dropdown", async () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "call-requests",
        widgetId: "team_open_calls",
        displayName: "Team Open Calls",
        filters: {},
      }),
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/call-requests/search",
        expect.objectContaining({ filters: {} }),
        { signal: expect.any(AbortSignal) },
      ),
    );

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Call state" }));
    fireEvent.click(screen.getByRole("option", { name: "Concluded" }));

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/call-requests/search",
        expect.objectContaining({ filters: { states: ["concluded"] } }),
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("Reset restores the widget's own starting filters after an edit", async () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "call-requests",
        widgetId: "team_open_calls",
        displayName: "Team Open Calls",
        filters: { states: ["scheduled"] },
      }),
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/call-requests/search",
        expect.objectContaining({ filters: { states: ["scheduled"] } }),
        { signal: expect.any(AbortSignal) },
      ),
    );

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Call state" }));
    fireEvent.click(screen.getByRole("option", { name: "Scheduled" }));
    // Deselecting the only selected option clears the field.
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/call-requests/search",
        expect.objectContaining({ filters: {} }),
        { signal: expect.any(AbortSignal) },
      ),
    );
    // The multi-select's menu stays open after a selection (multi-select UX);
    // close it so the rest of the page isn't aria-hidden behind the popup.
    fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });

    fireEvent.click(screen.getByRole("button", { name: /^reset$/i }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/call-requests/search",
        expect.objectContaining({ filters: { states: ["scheduled"] } }),
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("offers the CRE-team-family teams loaded from /teams/search as CRE Team options", async () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "call-requests",
        widgetId: "team_open_calls",
        displayName: "Team Open Calls",
        filters: {},
      }),
    );

    await waitFor(() => expect(screen.getByText("CR-1")).toBeInTheDocument());
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "CRE Team" }));
    expect(screen.getByRole("option", { name: "Team Castor" })).toBeInTheDocument();
  });
});

describe("DashboardWidgetPreviewPage", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockPost({});
  });

  it("prompts to open from a widget's View more link when the URL carries no widget params", () => {
    renderAt("/dashboard/preview/cases");
    expect(
      screen.getByText(/open this page from a dashboard widget/i),
    ).toBeInTheDocument();
  });

  it("falls back to the prompt for an unrecognized previewSlug", () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "not-a-real-resource",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: {},
      }),
    );
    expect(
      screen.getByText(/open this page from a dashboard widget/i),
    ).toBeInTheDocument();
  });

  it("returns to the dashboard when Back is clicked", () => {
    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: {},
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: /back/i }));
    expect(screen.getByText("Dashboard landing")).toBeInTheDocument();
  });
});

/**
 * `resourceType: "incident"` never routes through `CaseFamilyWidgetPreview`
 * (see `CASE_FAMILY_RESOURCE_TYPES` in `DashboardWidgetPreviewPage.tsx`), so
 * this generic `useWidgetData` + `WIDGET_LIST_RENDERERS` + "Filtered by:"
 * chip-summary path stays exactly as it always has. These were originally
 * written against `previewSlug: "cases"` fixtures purely as a convenient
 * generic resourceType — moved to "incidents" once "cases" gained its own,
 * behaviorally different branch (see the describe block below).
 */
describe("DashboardWidgetPreviewPage — generic resourceTypes keep the read-only summary + search box", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("renders the widget's table and paginates using the URL-provided widget id/filters", async () => {
    postMock.mockResolvedValue({
      total: 12,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: true,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: { priorities: ["critical"] },
      }),
    );

    expect(screen.getByText("My Critical & High Incidents")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/incidents/search",
      {
        filters: { priorities: ["critical"] },
        pagination: { offset: 0, limit: 10 },
      },
      { signal: expect.any(AbortSignal) },
    );

    // TablePagination's "next page" button.
    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/incidents/search",
        {
          filters: { priorities: ["critical"] },
          pagination: { offset: 10, limit: 10 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("resolves the masked @me sentinel back to the signed-in user's own id before querying", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_incidents",
        displayName: "My Incidents",
        filters: { assignedUserIds: [CURRENT_USER_ID] },
        currentUserId: CURRENT_USER_ID,
      }),
    );

    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/incidents/search",
      {
        filters: { assignedUserIds: [CURRENT_USER_ID] },
        pagination: { offset: 0, limit: 10 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("merges a typed search term into the widget's own filters as searchQuery", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: { priorities: ["critical"] },
      }),
    );
    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Search"), { target: { value: "disk" } });

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/incidents/search",
        {
          filters: { priorities: ["critical"], searchQuery: "disk" },
          pagination: { offset: 0, limit: 10 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("renders a visible summary of the active filter criteria (flat filter shape)", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: { priorities: ["critical", "high"] },
      }),
    );

    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    const group = screen.getByRole("group", { name: "Active filters" });
    expect(group).toHaveTextContent("priorities: critical, high");
  });

  it("does not render an active-filters summary when the widget has no filters", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "11111111-1111-1111-1111-111111111111", number: "INC-1", subject: "Disk full" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "incidents",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Incidents",
        filters: {},
      }),
    );

    await waitFor(() => expect(screen.getByText("INC-1")).toBeInTheDocument());
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();
  });
});

/**
 * Regression (digiops-cs#2880): a case-family widget carrying `anyOf`
 * (cross-field OR branches) must NOT fall into `CaseFamilyWidgetPreview` —
 * `CasesFilters`/`CasesFilterBar` have no OR construct, so seeding them from
 * `translateCaseDashboardFilters` would silently drop `anyOf` and land on a
 * broader, unfiltered-by-`anyOf` result set than the tile it was reached
 * from actually counted. It falls through to the generic, filter-faithful
 * `useWidgetData`-backed content instead (same path `resourceType: "incident"`
 * always used), which posts the widget's raw filters — `anyOf` included —
 * straight to `/cases/search`.
 */
describe("DashboardWidgetPreviewPage — a case-family widget with anyOf skips the editable filter bar", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("posts the widget's anyOf branches verbatim to /cases/search instead of seeding CasesFilterBar", async () => {
    postMock.mockResolvedValue({
      cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
      total: 1,
      limit: 10,
      offset: 0,
    });

    const anyOf = [
      { filters: [{ field: "severity", op: "in", values: ["catastrophic", "critical"] }] },
      { filters: [{ field: "type", op: "in", values: ["security_report_analysis"] }] },
    ];

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "wow_p0p1",
        displayName: "WOW P0/P1",
        filters: {
          filters: [{ field: "state", op: "in", values: ["open"] }],
          anyOf,
        },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    // The real, editable Cases filter bar never mounts for this widget.
    expect(screen.queryByRole("combobox", { name: "Severity" })).not.toBeInTheDocument();

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          filters: [{ field: "state", op: "in", values: ["open"] }],
          anyOf,
        }),
      }),
      { signal: expect.any(AbortSignal) },
    );
  });
});

/**
 * Reported live: a case-family widget's "View more" landed on a static
 * "Filtered by:" chip summary, unlike every other list page in the app,
 * which has a real, editable filter bar. `CaseFamilyWidgetPreview` (in
 * `DashboardWidgetPreviewPage.tsx`) fixes this by seeding the actual
 * `CasesFilterBar` + `useGetCsmCases` + `CasesList` from the widget's own
 * filters instead.
 */
describe("DashboardWidgetPreviewPage — case-family widgets get the real, editable Cases filter bar", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("renders the real CasesFilterBar (not the read-only chip summary), seeded from the widget's own field/op/values filters", async () => {
    mockPost({
      cases: {
        cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
        total: 1,
        limit: 10,
        offset: 0,
      },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "team_open_cases",
        displayName: "Team Open Cases",
        filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.getByRole("combobox", { name: "Severity" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "State" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();

    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      expect.objectContaining({
        filters: expect.objectContaining({
          filters: expect.arrayContaining([
            { field: "state", op: "in", values: ["open"] },
          ]),
        }),
      }),
    );
  });

  it("shows a 'Customise columns' picker next to the table, unlike the old chip-only page", async () => {
    // Reported live: the main Cases tab has a table column selector, but the
    // dashboard "View more" landing didn't show it at all -- because it only
    // ever went through `CsmIssuesView`, which this preview never rendered.
    // `CaseFamilyWidgetPreview` now wires its own `useColumnPreferences` +
    // `ColumnCustomizerButton` directly into `CasesList`.
    mockPost({
      cases: {
        cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
        total: 1,
        limit: 10,
        offset: 0,
      },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "team_open_cases",
        displayName: "Team Open Cases",
        filters: { filters: [{ field: "state", op: "in", values: ["open"] }] },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    fireEvent.click(
      screen.getByRole("button", { name: "Customise Team Open Cases columns" }),
    );

    // "case" is the resourceType behind a "cases" preview slug, so Severity
    // is offered alongside every other optional column.
    const picker = screen.getByRole("list", { name: "Customise Team Open Cases columns" });
    for (const label of ["Product", "Type", "Severity", "Assignee", "Customer", "Created"]) {
      expect(within(picker).getByText(label)).toBeInTheDocument();
    }
  });

  it("re-queries /cases/search when a filter is edited in the real filter bar", async () => {
    mockPost({
      cases: {
        cases: [{ id: "c1", number: "CS-1", subject: "Disk full", state: "open" }],
        total: 1,
        limit: 10,
        offset: 0,
      },
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "team_open_cases",
        displayName: "Team Open Cases",
        filters: {},
      }),
    );
    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    const callsBefore = postMock.mock.calls.filter((c) => c[0] === "/cases/search").length;

    fireEvent.change(screen.getByPlaceholderText(/search by case #/i), {
      target: { value: "disk" },
    });

    await waitFor(() => {
      const callsAfter = postMock.mock.calls.filter((c) => c[0] === "/cases/search").length;
      expect(callsAfter).toBeGreaterThan(callsBefore);
    });
    const lastCall = postMock.mock.calls
      .filter((c) => c[0] === "/cases/search")
      .at(-1);
    expect(lastCall?.[1]).toMatchObject({ filters: { searchQuery: "disk" } });
  });

  // `tag`/`excludeTags` is Advanced-mode-only now (see `CasesFilterBar.tsx`'s
  // mode toggle): a widget's `tag notIn [...]` seeds `excludeTags` directly
  // (no catalog fetch, no display/query divergence), which also means
  // `isSimpleRepresentable` is false, so the preview mounts straight into
  // Advanced mode with the `tag`/`notIn` row already showing that value --
  // what's shown and what's queried are the same `CasesFilters` value from
  // the start, same invariant as before, just via the unified builder now.
  it("seeds a widget's tag notIn directly into excludeTags -- shown in the Advanced-mode tag row and queried as the same notIn condition", async () => {
    mockPost({ cases: { cases: [], total: 0, limit: 10, offset: 0 } });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "excl_tag_widget",
        displayName: "Discussions on Going",
        filters: { filters: [{ field: "tag", op: "notIn", values: ["s_dip"] }] },
      }),
    );

    expect(postMock).not.toHaveBeenCalledWith("/tags/search", expect.anything());

    await waitFor(() => {
      expect(screen.getByText("Advanced filters")).toBeInTheDocument();
      expect(screen.getByText("s_dip")).toBeInTheDocument();
    });

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls
        .filter((c) => c[0] === "/cases/search")
        .at(-1);
      expect(lastCasesCall).toBeDefined();
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([
            { field: "tag", op: "notIn", values: ["s_dip"] },
          ]),
        },
      });
    });
  });

  it("Reset restores the widget's own starting tag notIn after an edit", async () => {
    mockPost({ cases: { cases: [], total: 0, limit: 10, offset: 0 } });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "excl_tag_widget",
        displayName: "Discussions on Going",
        filters: { filters: [{ field: "tag", op: "notIn", values: ["s_dip"] }] },
      }),
    );

    await waitFor(() => expect(screen.getByText("s_dip")).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText(/search by case #/i), {
      target: { value: "disk" },
    });
    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: expect.objectContaining({ searchQuery: "disk" }),
      });
    });

    fireEvent.click(screen.getByRole("button", { name: /clear filters/i }));

    await waitFor(() => {
      const lastCasesCall = postMock.mock.calls.filter((c) => c[0] === "/cases/search").at(-1);
      expect(lastCasesCall?.[1]).toMatchObject({
        filters: {
          filters: expect.arrayContaining([
            { field: "tag", op: "notIn", values: ["s_dip"] },
          ]),
        },
      });
    });
  });
});
