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
import { MemoryRouter } from "react-router";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
// A `shape: "list"` renderer now renders through widgetListConfig.tsx, which
// pulls in useTimeSheets.ts (time_card's mapper) — that module reads
// `window.config` at load via `@config/apiConfig`, unavailable under vitest.
// Same mock `DashboardWidgetTile.test.tsx` already carries.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
const SIGNED_IN_USER_ID = "11111111-aaaa-bbbb-cccc-000000000001";
let mockCurrentUserId: string | undefined = SIGNED_IN_USER_ID;

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: mockCurrentUserId === undefined ? undefined : { id: mockCurrentUserId },
    isLoading: mockCurrentUserId === undefined,
    isError: false,
  }),
}));

import WidgetInlineDrilldownPanel from "@features/csm-dashboard/components/WidgetInlineDrilldownPanel";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";
import { __resetWidgetFetchConcurrencyForTests } from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function renderPanel(ui: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

const CRITICAL_SLICE: PieSliceResult = {
  label: "S1 · Critical",
  value: 1,
  query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
};

describe("WidgetInlineDrilldownPanel", () => {
  beforeEach(() => {
    postMock.mockReset();
    mockCurrentUserId = SIGNED_IN_USER_ID;
    // Shared module-level singleton (widgetFetchConcurrency.ts) — a test
    // below deliberately leaves its own fetch pending forever, which would
    // otherwise permanently hold its slot and starve every later test in
    // this file. Same reset `DashboardWidgetTile.test.tsx` performs.
    __resetWidgetFetchConcurrencyForTests();
  });

  it("names both the widget and the expanded slice in its own header", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "1", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Cases by severity — S1 · Critical")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
  });

  it("fetches the slice's own query merged under the widget's base filters", async () => {
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 4, offset: 0, hasMore: false });

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{ filters: [{ field: "state", op: "in", values: ["open"] }] }}
        slice={CRITICAL_SLICE}
        listLimit={4}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/search",
        {
          filters: {
            filters: [
              { field: "state", op: "in", values: ["open"] },
              { field: "severity", op: "in", values: ["critical"] },
            ],
          },
          pagination: { offset: 0, limit: 4 },
        },
        { signal: expect.any(AbortSignal) },
      ),
    );
  });

  it("renders a loading skeleton while the fetch is in flight", () => {
    postMock.mockReturnValue(new Promise(() => {}));

    const { container } = renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        onClose={vi.fn()}
      />,
    );

    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("renders an error state when the fetch fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() =>
      expect(screen.getByText("Could not load this widget.")).toBeInTheDocument(),
    );
  });

  it("shows a 'View more' link only once more records exist than listLimit shows", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "1", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        listLimit={4}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(screen.queryByRole("link", { name: /view more/i })).not.toBeInTheDocument();
  });

  it("shows a 'View more' link to the widget's own preview page when more records exist than listLimit shows", async () => {
    postMock.mockResolvedValue({
      total: 6,
      cases: [{ id: "1", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 4,
      offset: 0,
      hasMore: true,
    });

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        listLimit={4}
        onClose={vi.fn()}
      />,
    );

    const viewMoreLink = await screen.findByRole("link", { name: /view more/i });
    const href = viewMoreLink.getAttribute("href") ?? "";
    expect(href.startsWith("/dashboard/preview/cases?")).toBe(true);
    const params = new URLSearchParams(href.split("?")[1]);
    expect(params.get("w")).toBe("cases_by_severity");
    expect(params.get("severity")).toBe("critical");
  });

  it("calls onClose when the close control is activated", async () => {
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 4, offset: 0, hasMore: false });
    const onClose = vi.fn();

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        onClose={onClose}
      />,
    );

    const closeButton = await screen.findByRole("button", {
      name: "Close Cases by severity — S1 · Critical",
    });
    fireEvent.click(closeButton);

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("resolves the {{currentTeam}} display name text token in its own header", async () => {
    postMock.mockResolvedValue({ total: 0, cases: [], limit: 4, offset: 0, hasMore: false });

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="team_open_cases"
        displayName="Open Cases — {{currentTeam}}"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        selectedTeamLabel="Castor"
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Open Cases — Castor — S1 · Critical")).toBeInTheDocument();
  });

  it("renders a 'Customise columns' button in its own header next to Close, for a case-shaped slice", async () => {
    postMock.mockResolvedValue({
      total: 1,
      cases: [{ id: "1", number: "CS-1", subject: "Disk full", state: "open" }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });
    window.localStorage.clear();

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="cases_by_severity"
        displayName="Cases by severity"
        resourceType="case"
        filters={{}}
        slice={CRITICAL_SLICE}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText("CS-1")).toBeInTheDocument());
    expect(
      screen.getByRole("button", { name: /customise columns/i }),
    ).toBeInTheDocument();
  });

  it("renders a 'Customise columns' button for a columns-configured (GenericColumnList) slice too", async () => {
    postMock.mockResolvedValue({
      total: 1,
      incidents: [{ id: "inc-1", number: "INC0000001", subject: "Down", state: "new" }],
      limit: 4,
      offset: 0,
      hasMore: false,
    });
    window.localStorage.clear();

    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="incidents_by_priority"
        displayName="Incidents by priority"
        resourceType="incident"
        filters={{}}
        slice={CRITICAL_SLICE}
        columns={[
          { path: "number", label: "Number" },
          { path: "subject", label: "Subject" },
        ]}
        onClose={vi.fn()}
      />,
    );

    await waitFor(() => expect(screen.getByText("Down")).toBeInTheDocument());
    const customizerButton = screen.getByRole("button", { name: /customise columns/i });
    expect(customizerButton).toBeInTheDocument();

    fireEvent.click(customizerButton);
    expect(screen.getAllByText("Subject")).toHaveLength(2); // header cell + popover row
    fireEvent.click(screen.getAllByText("Subject")[1]);
    expect(screen.getAllByText("Subject")).toHaveLength(1); // popover row only -- header cell gone

    const key = Object.keys(window.localStorage).find((k) =>
      k.includes("dashboard-generic-list:incidents_by_priority"),
    );
    expect(key).toBeDefined();
  });

  it("renders an unsupported-widget message instead of crashing for an unrecognized resourceType", () => {
    renderPanel(
      <WidgetInlineDrilldownPanel
        widgetId="mystery_widget"
        displayName="Mystery Widget"
        resourceType={"future_resource" as unknown as never}
        filters={{}}
        slice={CRITICAL_SLICE}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Unsupported widget type.")).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });
});
