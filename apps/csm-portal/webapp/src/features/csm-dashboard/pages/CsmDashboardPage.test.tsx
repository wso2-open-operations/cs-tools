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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import type { NavigateOptions, To } from "react-router";
import CsmDashboardPage from "@features/csm-dashboard/pages/CsmDashboardPage";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";

const navigateMock = vi.fn();

vi.mock("@hooks/useNavTransition", () => ({
  useNavTransition: () => navigateMock,
}));

/** Bridges the mocked `useNavTransition` to the real router's `useNavigate`
 * (same pattern as the already-migrated Case/Project detail page tests) so a
 * simulated dashboard/team switch actually drives the `MemoryRouter` instead
 * of being a no-op. */
function NavigateBridge(): null {
  const navigate = useNavigate();
  navigateMock.mockImplementation((to: To, options?: NavigateOptions) =>
    navigate(to, options),
  );
  return null;
}

/** Surfaces the router's current pathname for assertions — the
 * `MemoryRouter`'s history is in-memory, not reflected on `window.location`,
 * so this reads it via `useLocation` instead. */
function LocationDisplay(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-path">{location.pathname}</div>;
}

function renderAt(initialEntry: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <NavigateBridge />
      <LocationDisplay />
      <Routes>
        <Route path="/dashboard" element={<CsmDashboardPage />} />
        <Route path="/dashboard/:dashboardId" element={<CsmDashboardPage />} />
        <Route
          path="/dashboard/:dashboardId/:teamId"
          element={<CsmDashboardPage />}
        />
      </Routes>
    </MemoryRouter>,
  );
}

function currentPath(): string {
  return screen.getByTestId("location-path").textContent ?? "";
}

vi.mock("@features/csm-dashboard/api/useDashboardList", () => ({
  useDashboardList: vi.fn(),
}));

// None of these dashboards are team-based, so the header's team selector
// never renders/fetches here, but useTeams still needs mocking since
// AbtDashboardHeader (and now CsmDashboardPage itself, to resolve the
// selected team's groupId) call it unconditionally (the fetch itself is
// disabled via its `enabled` param) — without this, the real hook reaches
// the real API client, which throws under vitest (no runtime config).
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: vi.fn(() => ({ data: undefined })),
  abtFamilyForDashboardType: vi.fn(() => undefined),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: vi.fn(),
}));

// Keeps this test focused on dashboard selection + the header; the widget
// grid itself has its own tests (AgentsLandingPagePilot.test.tsx). Also
// surfaces `selectedTeamGroupId` so the wiring from CsmDashboardPage down
// can be asserted on without a real /teams/search round trip.
vi.mock("@features/csm-dashboard/components/AgentsLandingPagePilot", () => ({
  default: ({
    dashboardId,
    selectedTeamGroupId,
    selectedTeamLabel,
  }: {
    dashboardId: string;
    selectedTeamGroupId?: string | string[];
    selectedTeamLabel?: string;
  }) => (
    <div
      data-testid="agents-landing-pilot"
      data-team-group-id={
        Array.isArray(selectedTeamGroupId)
          ? selectedTeamGroupId.join(",")
          : (selectedTeamGroupId ?? "")
      }
      data-team-label={selectedTeamLabel ?? ""}
    >
      {dashboardId}
    </div>
  ),
}));

const mockedUseDashboardList = vi.mocked(useDashboardList);
const mockedUseCurrentUser = vi.mocked(useCurrentUser);

const DASHBOARD_LIST = [
  { id: "operations", displayName: "Operations", isDefault: false, isTeamBased: false },
  { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
  { id: "iam", displayName: "IAM CS", isDefault: false, isTeamBased: false },
];

function mockListResult(
  overrides: Partial<ReturnType<typeof useDashboardList>>,
): void {
  mockedUseDashboardList.mockReturnValue({
    data: undefined,
    isLoading: true,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useDashboardList>);
}

function mockCurrentUser(
  overrides: Partial<ReturnType<typeof useCurrentUser>>,
): void {
  mockedUseCurrentUser.mockReturnValue({
    user: undefined,
    isLoading: false,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useCurrentUser>);
}

beforeEach(() => {
  mockedUseDashboardList.mockReset();
  mockedUseCurrentUser.mockReset();
  navigateMock.mockReset();
  // Default: a resolved profile with no team — most tests aren't about the
  // team-default-dashboard behavior, so this keeps them on the old
  // BE-isDefault-only path.
  mockCurrentUser({});
});

describe("CsmDashboardPage", () => {
  it("shows a loading skeleton before the dashboard list resolves", () => {
    mockListResult({ data: undefined, isLoading: true });

    const { container } = renderAt("/dashboard");

    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("agents-landing-pilot")).not.toBeInTheDocument();
  });

  it("selects the isDefault dashboard once the list loads and renders the enabled, populated switcher", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard");

    // The isDefault entry ("agents_pilot") is selected on load.
    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );

    // The switcher is populated from the BE list and enabled (no
    // disabled-state tooltip gate any more): open it and check every
    // dashboard from the list appears as an option.
    const select = screen.getByRole("combobox");
    expect(select).not.toHaveAttribute("aria-disabled", "true");

    fireEvent.mouseDown(select);
    const listbox = screen.getByRole("listbox");
    expect(within(listbox).getByText("Operations")).toBeInTheDocument();
    expect(within(listbox).getByText("IAM CS")).toBeInTheDocument();
    expect(within(listbox).getByText("Engineer overview")).toBeInTheDocument();
  });

  it("renders the real widget grid for every dashboard, not only agents_pilot", () => {
    // "operations" is the default entry here — every dashboard now has real
    // widgets, so the grid renders regardless of which one is selected.
    mockListResult({
      data: [
        { id: "agents_pilot", displayName: "Engineer overview", isDefault: false, isTeamBased: false },
        { id: "operations", displayName: "Operations", isDefault: true, isTeamBased: false },
      ],
      isLoading: false,
    });

    renderAt("/dashboard");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "operations",
    );
  });

  it("switches to another dashboard when picked from the switcher", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );

    const select = screen.getByRole("combobox");
    fireEvent.mouseDown(select);
    fireEvent.click(within(screen.getByRole("listbox")).getByText("Operations"));

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "operations",
    );
  });

  it("shows an error state rather than an infinite skeleton when the list fails to load", () => {
    mockListResult({ data: undefined, isLoading: false, isError: true });

    renderAt("/dashboard");

    expect(
      screen.getByText("Could not load the dashboard list."),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("agents-landing-pilot")).not.toBeInTheDocument();
  });

  it("selects the dashboard named by the URL's path segment, not the BE default", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/iam");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("iam");
  });

  it("falls back to the BE default when the path segment names a dashboard id that isn't in the list", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/does-not-exist");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );
  });

  it("writes the dashboard id to the URL path (via replace) when the switcher is used", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard");

    const select = screen.getByRole("combobox");
    fireEvent.mouseDown(select);
    fireEvent.click(within(screen.getByRole("listbox")).getByText("Operations"));

    expect(currentPath()).toBe("/dashboard/operations");
    expect(navigateMock).toHaveBeenCalledWith("/dashboard/operations", {
      replace: true,
    });
  });

  it("clears a stale team segment from the URL when switching to a non-team-based dashboard", () => {
    mockListResult({
      data: [
        ...DASHBOARD_LIST,
        { id: "team_performance", displayName: "Team performance", isDefault: false, isTeamBased: true },
      ],
      isLoading: false,
    });

    renderAt("/dashboard/team_performance/cs_team_leads");

    // Two comboboxes render for a team-based dashboard (dashboard + team);
    // the dashboard switcher is always the first — see AbtDashboardHeader's
    // layout (dashboard selector, then team selector).
    const selects = screen.getAllByRole("combobox");
    const select = selects[0];
    fireEvent.mouseDown(select);
    fireEvent.click(within(screen.getByRole("listbox")).getByText("Operations"));

    expect(currentPath()).toBe("/dashboard/operations");
  });

  it("ignores a stray team segment on a URL naming a non-team-based dashboard, rather than crashing", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/iam/some-stale-team");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("iam");
    // Only the dashboard switcher renders — no team selector for a
    // non-team-based dashboard, regardless of what the stale segment says.
    expect(screen.getAllByRole("combobox")).toHaveLength(1);
  });

  describe("team-based default dashboard", () => {
    // The preferred predicate requires BOTH isDefault AND isTeamBased on the
    // SAME entry — team_performance carries both here, so a user with a
    // resolved team matches it directly (not merely the first isTeamBased
    // entry regardless of isDefault, which was the earlier, incorrect rule).
    const LIST_WITH_TEAM_DASHBOARD = [
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
      { id: "team_performance", displayName: "Team performance", isDefault: true, isTeamBased: true },
    ];

    it("defaults to the dashboard with isDefault AND isTeamBased both true, with the user's own team auto-selected, when the user has a resolved team", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "cs_team_leads", teamName: "CS Team Leads" } },
        isLoading: false,
      });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "team_performance",
      );
      // The team selector renders (this dashboard is isTeamBased) — both
      // the dashboard AND team default are derived UI state, never
      // auto-written to the URL, so the path is untouched until the
      // user (or a shared URL) actually names something explicitly.
      expect(screen.getAllByRole("combobox")).toHaveLength(2);
      expect(currentPath()).toBe("/dashboard");
    });

    it("keeps the isDefault+!isTeamBased entry when the user has no resolved team, even though an isDefault+isTeamBased dashboard exists", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "agents_pilot",
      );
    });

    it("falls back to the BE's own (any) isDefault entry when the user has a team but no dashboard has BOTH isDefault and isTeamBased set", () => {
      // team_performance is isTeamBased but NOT isDefault here — the
      // preferred predicate (isDefault && isTeamBased) matches nothing, so
      // this must fall back to the BE's own isDefault entry (agents_pilot),
      // not to team_performance just because it's isTeamBased.
      mockListResult({
        data: [
          { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
          { id: "team_performance", displayName: "Team performance", isDefault: false, isTeamBased: true },
        ],
        isLoading: false,
      });
      mockCurrentUser({
        user: { team: { teamKey: "cs_team_leads", teamName: "CS Team Leads" } },
        isLoading: false,
      });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "agents_pilot",
      );
    });

    it("falls back to the first dashboard in the list when the registry has no isDefault entry at all", () => {
      mockListResult({
        data: [
          { id: "team_performance", displayName: "Team performance", isDefault: false, isTeamBased: true },
          { id: "operations", displayName: "Operations", isDefault: false, isTeamBased: false },
        ],
        isLoading: false,
      });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "team_performance",
      );
    });

    it("holds the skeleton while the user profile is still loading, rather than flashing the BE default", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: undefined, isLoading: true, isError: false });

      const { container } = renderAt("/dashboard");

      expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
      expect(screen.queryByTestId("agents-landing-pilot")).not.toBeInTheDocument();
    });

    it("falls back to the BE default when the user profile fails to load, rather than hanging on the skeleton", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: undefined, isLoading: false, isError: true });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "agents_pilot",
      );
    });

    it("defaults the team selector to 'All ABTs' when the user has no home team, for a directly-viewed team-based dashboard", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });

      renderAt("/dashboard/team_performance");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "team_performance",
      );
      expect(screen.getByTestId("agents-landing-pilot")).toHaveAttribute(
        "data-team-label",
        "All ABTs",
      );
      // The team selector is the second of the two comboboxes for a
      // team-based dashboard (the dashboard selector renders first), and
      // shows its currently selected value even while closed.
      const selects = screen.getAllByRole("combobox");
      expect(within(selects[1]).getByText("All ABTs")).toBeInTheDocument();
    });

    it("does not default to 'All ABTs' when the URL already names a real team", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });

      renderAt("/dashboard/team_performance/cs_team_leads");

      expect(currentPath()).toBe("/dashboard/team_performance/cs_team_leads");
      // teams.data is undefined in this mock, so the real team's name can't
      // resolve to a label either — the point of this test is that it's
      // NOT "All ABTs" (the URL-named real team id always wins over the
      // no-home-team default, per `selectedTeamId`'s own precedence).
      expect(screen.getByTestId("agents-landing-pilot")).toHaveAttribute(
        "data-team-label",
        "",
      );
    });

    it("still lets the URL's own path segment win over the team-based default", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "cs_team_leads", teamName: "CS Team Leads" } },
        isLoading: false,
      });

      renderAt("/dashboard/agents_pilot");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "agents_pilot",
      );
    });
  });
});
