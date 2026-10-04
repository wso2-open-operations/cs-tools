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
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";
import CsmDashboardPage from "@features/csm-dashboard/pages/CsmDashboardPage";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import { useTeams } from "@features/csm-dashboard/api/useTeams";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import type { BeTeam } from "@api/backend/types";
import {
  DASHBOARD_SELECTION_STORAGE_KEY,
  readStoredDashboardSelection,
} from "@features/csm-dashboard/utils/dashboardSelectionStorage";

/** Surfaces the router's current pathname for assertions — the
 * `MemoryRouter`'s history is in-memory, not reflected on `window.location`,
 * so this reads it via `useLocation` instead. */
function LocationDisplay(): JSX.Element {
  const location = useLocation();
  return <div data-testid="location-path">{location.pathname}</div>;
}

// Real `<Routes>` (not a bare `<CsmDashboardPage />`) so `useParams` actually
// resolves `:dashboardId`/`:teamId` — matching the three sibling `dashboard`
// routes in App.tsx that all render this same page.
function renderAt(initialEntry: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route
          path="/dashboard"
          element={
            <>
              <CsmDashboardPage />
              <LocationDisplay />
            </>
          }
        />
        <Route
          path="/dashboard/:dashboardId"
          element={
            <>
              <CsmDashboardPage />
              <LocationDisplay />
            </>
          }
        />
        <Route
          path="/dashboard/:dashboardId/:teamId"
          element={
            <>
              <CsmDashboardPage />
              <LocationDisplay />
            </>
          }
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
// AbtDashboardHeader and CsmDashboardPage itself (to resolve the signed-in
// user's own team family, and the selected team's creGroupId/sreGroupId)
// call it unconditionally (the fetch itself is disabled via its `enabled`
// param where applicable) — without this, the real hook reaches the real
// API client, which throws under vitest (no runtime config).
// `dashboardTypeForTeamFamily` is reimplemented here rather than imported
// via `vi.importActual`: the real module transitively pulls in the
// backend API client, which throws under vitest without runtime config
// (see the real `useTeams` mock above for the same reason). It's a tiny,
// pure function, so duplicating it is cheap; keep this in sync with the
// real implementation in useTeams.ts if that one changes.
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: vi.fn(() => ({ data: undefined })),
  abtFamilyForDashboardType: vi.fn(() => undefined),
  dashboardTypeForTeamFamily: (family: string | undefined) => {
    if (!family) return undefined;
    if (family.startsWith("cre")) return "cre";
    if (family.startsWith("sre")) return "sre";
    return undefined;
  },
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: vi.fn(),
}));

// Keeps this test focused on dashboard selection + the header; the widget
// grid itself has its own tests (AgentsLandingPagePilot.test.tsx). Also
// surfaces `selectedTeamCreGroupId`/`selectedTeamSreGroupId` so the wiring
// from CsmDashboardPage down can be asserted on without a real
// /teams/search round trip.
vi.mock("@features/csm-dashboard/components/AgentsLandingPagePilot", () => ({
  default: ({
    dashboardId,
    selectedTeamCreGroupId,
    selectedTeamSreGroupId,
    selectedTeamLabel,
  }: {
    dashboardId: string;
    selectedTeamCreGroupId?: string | string[];
    selectedTeamSreGroupId?: string | string[];
    selectedTeamLabel?: string;
  }) => (
    <div
      data-testid="agents-landing-pilot"
      data-team-cre-group-id={
        Array.isArray(selectedTeamCreGroupId)
          ? selectedTeamCreGroupId.join(",")
          : (selectedTeamCreGroupId ?? "")
      }
      data-team-sre-group-id={
        Array.isArray(selectedTeamSreGroupId)
          ? selectedTeamSreGroupId.join(",")
          : (selectedTeamSreGroupId ?? "")
      }
      data-team-label={selectedTeamLabel ?? ""}
    >
      {dashboardId}
    </div>
  ),
}));

const mockedUseDashboardList = vi.mocked(useDashboardList);
const mockedUseCurrentUser = vi.mocked(useCurrentUser);
const mockedUseTeams = vi.mocked(useTeams);

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

/** Mocks the unscoped `useTeams` call CsmDashboardPage makes to resolve the
 * signed-in user's own team `family` (for type-based default selection) and
 * the selected team's creGroupId/sreGroupId. Each mocked team should carry
 * a `family` when the test cares about type/family-based default
 * selection. */
function mockTeams(
  teams: BeTeam[],
  overrides: Partial<ReturnType<typeof useTeams>> = {},
): void {
  mockedUseTeams.mockReturnValue({
    // Every mocked team gets group ids unless the test sets its own: a
    // selected team with no group id holds the widget grid back (see
    // teamScopeState), which most tests here aren't about.
    data: teams.map((t) => ({ creGroupId: `${t.id}-cre`, sreGroupId: `${t.id}-sre`, ...t })),
    isLoading: false,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useTeams>);
}

beforeEach(() => {
  window.localStorage.clear();
  mockedUseDashboardList.mockReset();
  mockedUseCurrentUser.mockReset();
  mockedUseTeams.mockReset();
  // Default: a resolved profile with no team — most tests aren't about the
  // team-default-dashboard behavior, so this keeps them on the old
  // BE-isDefault-only path.
  mockCurrentUser({});
  // Default: no teams resolved — most tests aren't about type/family-based
  // default selection, so this keeps them on the existing permissive
  // (family-blind) fallback path.
  mockedUseTeams.mockReturnValue({ data: undefined } as unknown as ReturnType<
    typeof useTeams
  >);
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

  it("selects the dashboard named by the URL's fragment, not the BE default", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/iam");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("iam");
  });

  it("falls back to the BE default when the fragment names a dashboard id that isn't in the list", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/does-not-exist");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );
  });

  it("writes the dashboard id to the URL fragment (via replace) when the switcher is used", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard");

    const select = screen.getByRole("combobox");
    fireEvent.mouseDown(select);
    fireEvent.click(within(screen.getByRole("listbox")).getByText("Operations"));

    expect(currentPath()).toBe("/dashboard/operations");
  });

  it("canonicalizes a bare /dashboard onto the resolved default dashboard's own path", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );
    expect(currentPath()).toBe("/dashboard/agents_pilot");
  });

  it("canonicalizes onto the BE default's own path when the URL names a dashboard id that isn't in the list", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/does-not-exist");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "agents_pilot",
    );
    expect(currentPath()).toBe("/dashboard/agents_pilot");
  });

  it("canonicalizes away a stale team suffix already in the URL for a non-team-based dashboard, on cold load (not just on switcher use)", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/iam/some-stale-team");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
      "iam",
    );
    expect(currentPath()).toBe("/dashboard/iam");
  });

  it("clears a stale team suffix from the fragment when switching to a non-team-based dashboard", () => {
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

  it("ignores a stray team suffix on a fragment naming a non-team-based dashboard, rather than crashing", () => {
    mockListResult({ data: DASHBOARD_LIST, isLoading: false });

    renderAt("/dashboard/iam/some-stale-team");

    expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("iam");
    // Only the dashboard switcher renders — no team selector for a
    // non-team-based dashboard, regardless of what the stale fragment says.
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
      mockTeams([{ id: "cs_team_leads", name: "CS Team Leads" }]);

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "team_performance",
      );
      // The team selector renders (this dashboard is isTeamBased) — the
      // resolved dashboard id IS canonicalized onto the URL (a bare
      // `/dashboard` cold load is one of the three cases the canonical
      // redirect covers), but the user's own derived team default is not:
      // it stays derived UI state until the user (or a shared URL) actually
      // names a team explicitly.
      expect(screen.getAllByRole("combobox")).toHaveLength(2);
      expect(currentPath()).toBe("/dashboard/team_performance");
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
      mockTeams([{ id: "cs_team_leads", name: "CS Team Leads" }]);

      renderAt("/dashboard/team_performance/cs_team_leads");

      expect(currentPath()).toBe("/dashboard/team_performance/cs_team_leads");
      // The point of this test is that the label is NOT "All ABTs" (the
      // URL-named real team id always wins over the no-home-team default,
      // per `selectedTeamId`'s own precedence).
      expect(screen.getByTestId("agents-landing-pilot")).toHaveAttribute(
        "data-team-label",
        "CS Team Leads",
      );
    });

    it("holds the widget grid while the teams list is still loading, instead of fetching org-wide", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });
      mockedUseTeams.mockReturnValue({
        data: undefined,
        isPending: true,
        isError: false,
      } as unknown as ReturnType<typeof useTeams>);

      renderAt("/dashboard/team_performance/cs_team_leads");

      expect(screen.queryByTestId("agents-landing-pilot")).not.toBeInTheDocument();
    });

    it("shows an explicit no-team-scope state for a team with no group id, instead of org-wide numbers", () => {
      mockListResult({ data: LIST_WITH_TEAM_DASHBOARD, isLoading: false });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });
      mockTeams([{ id: "cs_team_leads", name: "CS Team Leads", creGroupId: undefined, sreGroupId: undefined }]);

      renderAt("/dashboard/team_performance/cs_team_leads");

      expect(screen.queryByTestId("agents-landing-pilot")).not.toBeInTheDocument();
      expect(screen.getByText(/No team scope/i)).toBeInTheDocument();
    });

    it("still lets the URL's own fragment win over the team-based default", () => {
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

  describe("type/family-based default dashboard selection", () => {
    // Two isDefault+isTeamBased dashboards of different types — the case
    // the old (type-blind) predicate could never have handled, since it
    // matched the first one it found regardless of the user's own team.
    const LIST_WITH_TWO_TYPED_DEFAULTS = [
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
      {
        id: "cre-abt",
        displayName: "CRE ABT Dashboard",
        isDefault: true,
        isTeamBased: true,
        type: "cre" as const,
      },
      {
        id: "sre-abt",
        displayName: "SRE ABT Dashboard",
        isDefault: true,
        isTeamBased: true,
        type: "sre" as const,
      },
    ];

    it("lands a user whose team family is sre-abt on the sre-typed default, not the cre one", () => {
      mockListResult({ data: LIST_WITH_TWO_TYPED_DEFAULTS, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "artemis_sre_group", teamName: "Artemis SRE Group" } },
        isLoading: false,
      });
      mockTeams([{ id: "artemis_sre_group", name: "Artemis SRE Group", family: "sre-abt" }]);

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("sre-abt");
      expect(currentPath()).toBe("/dashboard/sre-abt");
    });

    it("lands a user whose team family is cre-abt on the cre-typed default, not the sre one", () => {
      mockListResult({ data: LIST_WITH_TWO_TYPED_DEFAULTS, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "castor_cre_group", teamName: "Castor CRE Group" } },
        isLoading: false,
      });
      mockTeams([{ id: "castor_cre_group", name: "Castor CRE Group", family: "cre-abt" }]);

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("cre-abt");
      expect(currentPath()).toBe("/dashboard/cre-abt");
    });

    it("falls back to the non-team-based default for a user with no team, even with two typed defaults present", () => {
      mockListResult({ data: LIST_WITH_TWO_TYPED_DEFAULTS, isLoading: false });
      mockCurrentUser({ user: { team: undefined }, isLoading: false });
      mockTeams([]);

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("agents_pilot");
    });

    it("still lands on SOME isDefault+isTeamBased entry when the user's team family is unresolved (permissive fallback)", () => {
      mockListResult({ data: LIST_WITH_TWO_TYPED_DEFAULTS, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "unknown_team", teamName: "Unknown Team" } },
        isLoading: false,
      });
      // The team is in the list but carries no `family`, so its family
      // can't resolve.
      mockTeams([{ id: "unknown_team", name: "Unknown Team" }]);

      renderAt("/dashboard");

      const rendered = screen.getByTestId("agents-landing-pilot").textContent;
      expect(["cre-abt", "sre-abt"]).toContain(rendered);
    });
  });

  describe("defaultForTeamKeys default dashboard selection", () => {
    const LIST_WITH_DEFAULT_FOR_TEAM_KEYS = [
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
      { id: "team_performance", displayName: "Team performance", isDefault: true, isTeamBased: true },
      {
        id: "onboarding-engineer",
        displayName: "Onboarding engineer",
        isDefault: false,
        isTeamBased: false,
        defaultForTeamKeys: ["customer_onboarding"],
      },
    ];

    it("wins over preferredEntry for a user whose team key is named in defaultForTeamKeys", () => {
      mockListResult({ data: LIST_WITH_DEFAULT_FOR_TEAM_KEYS, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "customer_onboarding", teamName: "Customer Onboarding" } },
        isLoading: false,
      });
      mockTeams([{ id: "customer_onboarding", name: "Customer Onboarding" }]);

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "onboarding-engineer",
      );
      expect(currentPath()).toBe("/dashboard/onboarding-engineer");
    });

    it("leaves an unmapped team's default-selection behavior unchanged", () => {
      mockListResult({ data: LIST_WITH_DEFAULT_FOR_TEAM_KEYS, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "cs_team_leads", teamName: "CS Team Leads" } },
        isLoading: false,
      });
      mockTeams([{ id: "cs_team_leads", name: "CS Team Leads" }]);

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "team_performance",
      );
    });

    it("falls through cleanly when defaultForTeamKeys names a dashboard id absent from the loaded list", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "customer_onboarding", teamName: "Customer Onboarding" } },
        isLoading: false,
      });
      mockTeams([]);

      renderAt("/dashboard");

      // Falls through to preferredEntry (no isDefault+isTeamBased entry in
      // DASHBOARD_LIST) -> anyDefaultEntry ("agents_pilot").
      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "agents_pilot",
      );
    });

    it("lets a URL naming a different valid dashboard win over the defaultForTeamKeys default", () => {
      mockListResult({ data: LIST_WITH_DEFAULT_FOR_TEAM_KEYS, isLoading: false });
      mockCurrentUser({
        user: { team: { teamKey: "customer_onboarding", teamName: "Customer Onboarding" } },
        isLoading: false,
      });
      mockTeams([{ id: "customer_onboarding", name: "Customer Onboarding" }]);

      renderAt("/dashboard/agents_pilot");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "agents_pilot",
      );
      expect(currentPath()).toBe("/dashboard/agents_pilot");
    });
  });

  describe("remembered dashboard and team", () => {
    const TEAM_LIST = [
      { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
      { id: "iam", displayName: "IAM CS", isDefault: false, isTeamBased: false },
      { id: "team_performance", displayName: "Team performance", isDefault: false, isTeamBased: true },
    ];
    const TEAMS = [
      { id: "team_a", name: "Team A", family: "cre-abt" },
      { id: "team_b", name: "Team B", family: "cre-abt" },
    ] as BeTeam[];

    function store(value: unknown): void {
      window.localStorage.setItem(
        DASHBOARD_SELECTION_STORAGE_KEY,
        typeof value === "string" ? value : JSON.stringify(value),
      );
    }

    it("persists the dashboard picked from the switcher", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });

      renderAt("/dashboard");
      fireEvent.mouseDown(screen.getByRole("combobox"));
      fireEvent.click(within(screen.getByRole("listbox")).getByText("IAM CS"));

      expect(readStoredDashboardSelection()).toEqual({
        dashboardId: "iam",
        teamId: undefined,
      });
    });

    it("persists the team picked from the team selector", () => {
      mockListResult({ data: TEAM_LIST, isLoading: false });
      mockTeams(TEAMS);

      renderAt("/dashboard/team_performance");
      fireEvent.mouseDown(screen.getAllByRole("combobox")[1]);
      fireEvent.click(within(screen.getByRole("listbox")).getByText("Team B"));

      expect(readStoredDashboardSelection()).toEqual({
        dashboardId: "team_performance",
        teamId: "team_b",
      });
    });

    it("restores the remembered dashboard on a bare /dashboard instead of the default", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });
      store({ dashboardId: "iam" });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("iam");
      expect(currentPath()).toBe("/dashboard/iam");
    });

    it("restores the remembered team for the remembered team-based dashboard", () => {
      mockListResult({ data: TEAM_LIST, isLoading: false });
      mockTeams(TEAMS);
      store({ dashboardId: "team_performance", teamId: "team_b" });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent(
        "team_performance",
      );
      expect(screen.getByTestId("agents-landing-pilot")).toHaveAttribute(
        "data-team-label",
        "Team B",
      );
    });

    it("does not override a dashboard named by the URL", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });
      store({ dashboardId: "iam" });

      renderAt("/dashboard/operations");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("operations");
    });

    it("does not override a team named by the URL", () => {
      mockListResult({ data: TEAM_LIST, isLoading: false });
      mockTeams(TEAMS);
      store({ dashboardId: "team_performance", teamId: "team_b" });

      renderAt("/dashboard/team_performance/team_a");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveAttribute(
        "data-team-label",
        "Team A",
      );
    });

    it("falls back to the default when the remembered dashboard no longer exists", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });
      store({ dashboardId: "retired_dashboard" });

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("agents_pilot");
    });

    it("falls back to the default team when the remembered team no longer exists", () => {
      mockListResult({ data: TEAM_LIST, isLoading: false });
      mockTeams(TEAMS);
      store({ dashboardId: "team_performance", teamId: "gone_team" });

      renderAt("/dashboard");

      // No home team, so the existing default is "All ABTs".
      expect(screen.getByTestId("agents-landing-pilot")).toHaveAttribute(
        "data-team-label",
        "All ABTs",
      );
    });

    it("falls back to the default when the stored value is malformed", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });
      store("{not json");

      renderAt("/dashboard");

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("agents_pilot");
    });

    it("still renders when localStorage throws", () => {
      mockListResult({ data: DASHBOARD_LIST, isLoading: false });
      const getItem = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
        throw new Error("blocked");
      });
      const setItem = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
        throw new Error("blocked");
      });

      renderAt("/dashboard");
      fireEvent.mouseDown(screen.getByRole("combobox"));
      fireEvent.click(within(screen.getByRole("listbox")).getByText("IAM CS"));

      expect(screen.getByTestId("agents-landing-pilot")).toHaveTextContent("iam");
      getItem.mockRestore();
      setItem.mockRestore();
    });
  });
});
