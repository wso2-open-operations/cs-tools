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

import { render, screen, fireEvent, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import { ALL_TEAMS_SENTINEL } from "@features/csm-dashboard/utils/teamFilterPlaceholder";

const DASHBOARD_LIST = [
  { id: "agents_pilot", displayName: "Engineer overview", isDefault: true, isTeamBased: false },
  { id: "team_performance", displayName: "Team performance", isDefault: false, isTeamBased: true },
  { id: "abt", displayName: "ABT Dashboard", type: "cre" as const, isDefault: false, isTeamBased: true },
];

function renderWithClient(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

describe("AbtDashboardHeader", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("hides the dashboard switcher when only one dashboard is available", () => {
    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="agents_pilot"
        onDashboardChange={vi.fn()}
        dashboardList={[DASHBOARD_LIST[0]]}
        selectedTeamId={undefined}
        onTeamChange={vi.fn()}
      />,
    );

    expect(screen.queryByLabelText("Select dashboard")).not.toBeInTheDocument();
    // Still shows the current dashboard's name as plain text, just no
    // switcher control alongside it.
    expect(screen.getByText("Engineer overview")).toBeInTheDocument();
  });

  it("shows no team selector for a non-team-based dashboard", () => {
    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="agents_pilot"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={vi.fn()}
      />,
    );

    expect(postMock).not.toHaveBeenCalled();
    // Only the dashboard switcher combobox, no second (team) combobox.
    expect(screen.getAllByRole("combobox")).toHaveLength(1);
  });

  it("shows a populated team selector for a team-based dashboard", async () => {
    postMock.mockResolvedValue({
      teams: [
        { id: "cs_team_leads", name: "CS Team Leads" },
        { id: "cs_operations", name: "CS Operations" },
      ],
    });

    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="team_performance"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={vi.fn()}
      />,
    );

    expect(postMock).toHaveBeenCalledWith("/teams/search", {
      pagination: { offset: 0, limit: 100 },
    });

    // Dashboard order is: dashboard selector, then team selector — see
    // AbtDashboardHeader's layout.
    const [, teamSelect] = screen.getAllByRole("combobox");
    fireEvent.mouseDown(teamSelect);
    const listbox = await screen.findByRole("listbox");
    // The teams query resolves asynchronously; retry (findByText) rather
    // than asserting synchronously right after the menu opens.
    expect(
      await within(listbox).findByText("CS Team Leads"),
    ).toBeInTheDocument();
    expect(within(listbox).getByText("CS Operations")).toBeInTheDocument();
    // No "All teams" option any more — every team-based dashboard view must
    // have a real team selected.
    expect(within(listbox).queryByText("All teams")).not.toBeInTheDocument();
  });

  it("calls onTeamChange when a team is picked", async () => {
    postMock.mockResolvedValue({
      teams: [{ id: "cs_team_leads", name: "CS Team Leads" }],
    });
    const onTeamChange = vi.fn();

    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="team_performance"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={onTeamChange}
      />,
    );

    // Dashboard order is: dashboard selector, then team selector — see
    // AbtDashboardHeader's layout.
    const [, teamSelect] = screen.getAllByRole("combobox");
    fireEvent.mouseDown(teamSelect);
    const listbox = await screen.findByRole("listbox");
    fireEvent.click(await within(listbox).findByText("CS Team Leads"));

    expect(onTeamChange).toHaveBeenCalledWith("cs_team_leads");
  });

  it("scopes the team query to the dashboard's family for a typed dashboard", () => {
    postMock.mockResolvedValue({
      teams: [{ id: "castor", name: "Castor", family: "cre-abt" }],
    });

    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="abt"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={vi.fn()}
      />,
    );

    expect(postMock).toHaveBeenCalledWith("/teams/search", {
      filters: { family: "cre-abt" },
      pagination: { offset: 0, limit: 100 },
    });
  });

  it("does not scope the team query for an untyped team-based dashboard", () => {
    postMock.mockResolvedValue({ teams: [] });

    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="team_performance"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={vi.fn()}
      />,
    );

    expect(postMock).toHaveBeenCalledWith("/teams/search", {
      pagination: { offset: 0, limit: 100 },
    });
  });

  it("offers an 'All ABTs' option alongside the real teams for a team-based dashboard", async () => {
    postMock.mockResolvedValue({
      teams: [{ id: "castor", name: "Castor", family: "cre-abt" }],
    });

    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="abt"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={vi.fn()}
      />,
    );

    const [, teamSelect] = screen.getAllByRole("combobox");
    fireEvent.mouseDown(teamSelect);
    const listbox = await screen.findByRole("listbox");

    expect(within(listbox).getByText("All ABTs")).toBeInTheDocument();
    expect(await within(listbox).findByText("Castor")).toBeInTheDocument();
  });

  it("calls onTeamChange with the sentinel value when 'All ABTs' is selected", async () => {
    postMock.mockResolvedValue({
      teams: [{ id: "castor", name: "Castor", family: "cre-abt" }],
    });
    const onTeamChange = vi.fn();

    renderWithClient(
      <AbtDashboardHeader
        dashboardKey="abt"
        onDashboardChange={vi.fn()}
        dashboardList={DASHBOARD_LIST}
        selectedTeamId={undefined}
        onTeamChange={onTeamChange}
      />,
    );

    const [, teamSelect] = screen.getAllByRole("combobox");
    fireEvent.mouseDown(teamSelect);
    const listbox = await screen.findByRole("listbox");
    fireEvent.click(within(listbox).getByText("All ABTs"));

    expect(onTeamChange).toHaveBeenCalledWith(ALL_TEAMS_SENTINEL);
  });
});
