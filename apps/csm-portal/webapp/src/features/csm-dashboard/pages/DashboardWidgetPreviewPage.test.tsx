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
          <Route
            path="/dashboard/preview/:previewSlug"
            element={<DashboardWidgetPreviewPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("DashboardWidgetPreviewPage", () => {
  beforeEach(() => {
    postMock.mockReset();
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

  it("renders the widget's table and paginates using the URL-provided widget id/filters", async () => {
    postMock.mockResolvedValue({
      total: 12,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 10,
      offset: 0,
      hasMore: true,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: { severities: ["critical"] },
      }),
    );

    expect(screen.getByText("My Critical & High Cases")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: { severities: ["critical"] },
        pagination: { offset: 0, limit: 10 },
      },
      { signal: expect.any(AbortSignal) },
    );

    // TablePagination's "next page" button.
    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/search",
        {
          filters: { severities: ["critical"] },
          pagination: { offset: 10, limit: 10 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("resolves the masked @me sentinel back to the signed-in user's own id before querying", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_cases",
        displayName: "My Cases",
        filters: { assignedUserIds: [CURRENT_USER_ID] },
        currentUserId: CURRENT_USER_ID,
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
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
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: { severities: ["critical"] },
      }),
    );
    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText("Search"), { target: { value: "disk" } });

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/search",
        {
          filters: { severities: ["critical"], searchQuery: "disk" },
          pagination: { offset: 0, limit: 10 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("renders a visible summary of the active filter criteria (flat filter shape)", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: { severities: ["critical", "high"] },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    const group = screen.getByRole("group", { name: "Active filters" });
    expect(group).toHaveTextContent("severities: critical, high");
  });

  it("renders a visible summary of the active filter criteria (case field/op/values DSL shape), including the resolved team filter", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "team_open_cases",
        displayName: "Team Open Cases",
        filters: {
          filters: [
            { field: "state", op: "in", values: ["open"] },
            { field: "tag", op: "notIn", values: ["s_dip"] },
            {
              field: "integrationCsTeam",
              op: "in",
              values: ["22222222-2222-2222-2222-222222222222"],
            },
          ],
        },
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    const group = screen.getByRole("group", { name: "Active filters" });
    expect(group).toHaveTextContent("state: open");
    expect(group).toHaveTextContent("tag (notIn): s_dip");
    expect(group).toHaveTextContent(
      "integrationCsTeam: 22222222-2222-2222-2222-222222222222",
    );
  });

  it("does not render an active-filters summary when the widget has no filters", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "11111111-1111-1111-1111-111111111111", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 10,
      offset: 0,
      hasMore: false,
    });

    renderAt(
      buildWidgetPreviewHref({
        previewSlug: "cases",
        widgetId: "my_critical_open",
        displayName: "My Critical & High Cases",
        filters: {},
      }),
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();
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
