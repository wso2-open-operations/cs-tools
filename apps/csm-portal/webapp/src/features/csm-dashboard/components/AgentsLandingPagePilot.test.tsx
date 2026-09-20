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
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router";

const getMock = vi.fn();
const postMock = vi.fn();

// A minimal stand-in for the real BackendApiError (see client.ts) — needed
// purely so DashboardWidgetTile's own `instanceof BackendApiError` status
// check (rendered underneath this page) has a real class to check against;
// this suite doesn't itself assert on status-specific behavior. Declared via
// `vi.hoisted` since `vi.mock`'s factory is itself hoisted above any
// top-level `const`/`class` in this file.
const { MockBackendApiError } = vi.hoisted(() => {
  class MockBackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.name = "BackendApiError";
      this.status = status;
    }
  }
  return { MockBackendApiError };
});

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
  BackendApiError: MockBackendApiError,
}));
// A `shape: "list"` tile renders through widgetListConfig.tsx, which pulls in
// useTimeSheets.ts (time_card's mapper) — that module reads `window.config`
// at load via `@config/apiConfig`, unavailable under vitest.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
// A mutable mock so individual tests can grant dashboard-builder access
// (roles) without affecting the rest of the suite's default (no roles).
const { currentUserMock } = vi.hoisted(() => ({ currentUserMock: vi.fn() }));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => currentUserMock(),
}));

import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";
import { saveDashboardDraft } from "@features/csm-admin/dashboards/utils/dashboardDraftsStorage";
import type { BeDashboardWidget } from "@api/backend/types";

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

const DASHBOARD_DETAIL = {
  id: "agents_pilot",
  displayName: "Engineer overview",
  isDefault: true,
  widgets: [
    {
      widgetId: "my_patches",
      displayName: "My Patches",
      resourceType: "case",
      shape: "count",
      gridWidth: 3,
      query: { assignedUserIds: ["user-1"], tags: ["patch"] },
    },
    {
      widgetId: "my_reminders",
      displayName: "My Reminders",
      resourceType: "case",
      shape: "count",
      gridWidth: 3,
      query: { assignedUserIds: ["user-1"], states: ["awaiting_info"] },
    },
    {
      widgetId: "open_incident_team",
      displayName: "Open Incidents (Team)",
      resourceType: "case",
      shape: "count",
      gridWidth: 3,
      query: { tags: ["s_dip"] },
    },
  ],
};

function searchResponseFor(total: number) {
  return { total, cases: [], limit: 1, offset: 0, hasMore: false };
}

describe("AgentsLandingPagePilot", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
    localStorage.clear();
    currentUserMock.mockReturnValue({
      user: { id: "11111111-aaaa-bbbb-cccc-000000000001" },
      isLoading: false,
      isError: false,
    });
  });

  it("renders skeleton tiles while the template list is in flight", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    const { container } = renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(3);
  });

  it("renders one tile per widget, each resolving its own count independently", async () => {
    getMock.mockResolvedValue(DASHBOARD_DETAIL);
    postMock.mockImplementation((_path: string, body: { filters: Record<string, unknown> }) => {
      if (body.filters.tags && (body.filters.tags as string[]).includes("patch")) {
        return Promise.resolve(searchResponseFor(3));
      }
      if (body.filters.states) {
        return Promise.resolve(searchResponseFor(5));
      }
      return Promise.resolve(searchResponseFor(12));
    });

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("3")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("5")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("12")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledTimes(3);
  });

  it("shows an error state when the template list itself fails to load", async () => {
    getMock.mockRejectedValue(new Error("boom"));

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() =>
      expect(screen.getByText("Could not load the widget pilot.")).toBeInTheDocument(),
    );
    expect(postMock).not.toHaveBeenCalled();
  });

  it("isolates one widget's failed count fetch to its own tile while siblings render their real counts", async () => {
    getMock.mockResolvedValue(DASHBOARD_DETAIL);
    postMock.mockImplementation((_path: string, body: { filters: Record<string, unknown> }) => {
      if (body.filters.tags && (body.filters.tags as string[]).includes("patch")) {
        return Promise.reject(new Error("boom"));
      }
      if (body.filters.states) {
        return Promise.resolve(searchResponseFor(5));
      }
      return Promise.resolve(searchResponseFor(12));
    });

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() =>
      expect(screen.getByText("Could not load this widget.")).toBeInTheDocument(),
    );
    expect(screen.getByText("My Reminders")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("Open Incidents (Team)")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.queryByText("3")).not.toBeInTheDocument();
  });

  it("re-fetches only that section's own widgets when its refresh button is clicked, without re-pulling the dashboard config", async () => {
    getMock.mockResolvedValue(DASHBOARD_DETAIL);
    postMock.mockResolvedValue(searchResponseFor(3));

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);
    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    expect(postMock).toHaveBeenCalledTimes(3);
    expect(getMock).toHaveBeenCalledTimes(1);

    // All three DASHBOARD_DETAIL widgets are unsectioned, so they share the
    // one untitled default group — its refresh button carries a plain
    // "Refresh section" label (no section name to fold into it).
    fireEvent.click(screen.getByRole("button", { name: "Refresh section" }));

    // Every widget in that section re-runs its own /cases/search — 3 more
    // calls on top of the initial 3 — but the dashboard's own metadata GET
    // is not re-pulled (a section refresh is data-only).
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(6));
    expect(getMock).toHaveBeenCalledTimes(1);
    expect(screen.getByText("My Patches")).toBeInTheDocument();
  });

  // Regression test: DASHBOARDS_CONFIG is a raw JSON env var, not
  // schema-validated beyond basic decoding — a widget entry missing
  // `filters` entirely (not `{}`, genuinely absent) used to crash the whole
  // dashboard with "Cannot read properties of undefined (reading 'filters')"
  // deep in resolveTeamPlaceholder, three calls below this component.
  it("renders a widget whose config is missing `filters` instead of crashing", async () => {
    getMock.mockResolvedValue({
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      widgets: [
        {
          widgetId: "malformed_widget",
          displayName: "Malformed Widget",
          resourceType: "case",
          shape: "count",
          gridWidth: 3,
          // filters intentionally omitted
        },
      ],
    });
    postMock.mockResolvedValue(searchResponseFor(7));

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() => expect(screen.getByText("Malformed Widget")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("7")).toBeInTheDocument());
  });

  it("groups widgets sharing a `section` under a titled heading, separate from unsectioned widgets", async () => {
    getMock.mockResolvedValue({
      id: "team_performance",
      displayName: "Team performance",
      isDefault: false,
      widgets: [
        {
          widgetId: "team_open_cases",
          displayName: "Team Open P0/P1",
          resourceType: "case",
          shape: "count",
          gridWidth: 6,
          query: {},
        },
        {
          widgetId: "incident_wow",
          displayName: "Incident WOW",
          section: "SLA Violation",
          resourceType: "case",
          shape: "count",
          gridWidth: 6,
          query: {},
        },
        {
          widgetId: "query_wow",
          displayName: "Query WOW",
          section: "SLA Violation",
          resourceType: "case",
          shape: "count",
          gridWidth: 6,
          query: {},
        },
      ],
    });
    postMock.mockResolvedValue(searchResponseFor(3));

    renderWithClient(<AgentsLandingPagePilot dashboardId="team_performance" />);

    await waitFor(() => expect(screen.getByText("Team Open P0/P1")).toBeInTheDocument());
    expect(screen.getByText("Incident WOW")).toBeInTheDocument();
    expect(screen.getByText("Query WOW")).toBeInTheDocument();

    // Exactly one "SLA Violation" heading — both of its widgets share the
    // section, they don't each get their own repeated heading.
    expect(screen.getAllByText("SLA Violation")).toHaveLength(1);
  });

  it("resolves the {{currentTeam}} text token in a section heading, same as a widget's own displayName", async () => {
    getMock.mockResolvedValue({
      id: "team_performance",
      displayName: "Team performance",
      isDefault: false,
      widgets: [
        {
          widgetId: "team_open_cases",
          displayName: "Team Open P0/P1",
          section: "Overall - {{currentTeam}}",
          resourceType: "case",
          shape: "count",
          gridWidth: 6,
          query: {},
        },
      ],
    });
    postMock.mockResolvedValue(searchResponseFor(1));

    renderWithClient(
      <AgentsLandingPagePilot dashboardId="team_performance" selectedTeamLabel="Castor" />,
    );

    await waitFor(() => expect(screen.getByText("Team Open P0/P1")).toBeInTheDocument());
    expect(screen.getByText("Overall - Castor")).toBeInTheDocument();
    expect(screen.queryByText(/{{currentTeam}}/)).not.toBeInTheDocument();
  });

  it("shows a local-draft note, and renders the DRAFT's widgets (not the deployed ones), when a dashboard_designer has an unsaved draft for this dashboard", async () => {
    currentUserMock.mockReturnValue({
      user: { id: "u-1", roles: ["dashboard_designer"] },
      isLoading: false,
      isError: false,
    });
    getMock.mockResolvedValue(DASHBOARD_DETAIL);
    postMock.mockResolvedValue(searchResponseFor(9));
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview (draft)",
      isDefault: true,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "draft_only_widget",
          displayName: "Draft Only Widget",
          resourceType: "case",
          shape: "count",
          gridWidth: 3,
          query: {},
        },
      ] as BeDashboardWidget[],
      emptySections: [],
    });

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() =>
      expect(
        screen.getByText(/You're viewing your unsaved local draft of this dashboard/),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("open it in the dashboard builder")).toBeInTheDocument();
    // The draft's own widget renders...
    await waitFor(() => expect(screen.getByText("Draft Only Widget")).toBeInTheDocument());
    // ...and the deployed dashboard's widgets do NOT — the draft fully
    // replaces them here, it isn't merged with what's deployed.
    expect(screen.queryByText("My Patches")).not.toBeInTheDocument();
  });

  it("shows the DEPLOYED widgets (not the draft's), with no note, for a viewer without dashboard-builder access, even with a stored draft", async () => {
    getMock.mockResolvedValue(DASHBOARD_DETAIL);
    postMock.mockResolvedValue(searchResponseFor(3));
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: "Engineer overview (draft)",
      isDefault: true,
      isTeamBased: false,
      widgets: [
        {
          widgetId: "draft_only_widget",
          displayName: "Draft Only Widget",
          resourceType: "case",
          shape: "count",
          gridWidth: 3,
          query: {},
        },
      ] as BeDashboardWidget[],
      emptySections: [],
    });

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    expect(screen.queryByText("Draft Only Widget")).not.toBeInTheDocument();
    expect(screen.queryByText(/unsaved local draft/)).not.toBeInTheDocument();
  });

  it("does not show the local-draft note when the draft matches what's deployed", async () => {
    currentUserMock.mockReturnValue({
      user: { id: "u-1", roles: ["dashboard_designer"] },
      isLoading: false,
      isError: false,
    });
    // Deliberately sets every field `isDraftDrifted`'s comparison reads
    // (including `isTeamBased`/`targetTeam`, which `DASHBOARD_DETAIL` above
    // leaves entirely unset) so both sides of the comparison see the exact
    // same keys — an explicit `false`/`undefined` mismatched against an
    // absent key would register as drifted even though the "true" content is
    // identical, which isn't what this test is checking.
    const matchingDetail = {
      id: "agents_pilot",
      displayName: "Engineer overview",
      isDefault: true,
      isTeamBased: false,
      targetTeam: undefined,
      widgets: DASHBOARD_DETAIL.widgets as BeDashboardWidget[],
    };
    getMock.mockResolvedValue(matchingDetail);
    postMock.mockResolvedValue(searchResponseFor(3));
    saveDashboardDraft({
      id: "agents_pilot",
      sourceDashboardId: "agents_pilot",
      displayName: matchingDetail.displayName,
      isDefault: matchingDetail.isDefault,
      isTeamBased: matchingDetail.isTeamBased,
      targetTeam: matchingDetail.targetTeam,
      widgets: matchingDetail.widgets,
      emptySections: [],
    });

    renderWithClient(<AgentsLandingPagePilot dashboardId="agents_pilot" />);

    await waitFor(() => expect(screen.getByText("My Patches")).toBeInTheDocument());
    expect(screen.queryByText(/unsaved local draft/)).not.toBeInTheDocument();
  });
});
