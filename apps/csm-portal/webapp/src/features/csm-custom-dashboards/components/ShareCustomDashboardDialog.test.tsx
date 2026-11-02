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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

interface User {
  id: string;
  userName: string;
  name: string;
  email: string;
  userType?: string;
  active?: boolean;
}
let searchResults: { data?: { users: User[] }; isFetching?: boolean; isError?: boolean };
let sharesState: { data?: unknown; isLoading?: boolean; isError?: boolean };
const shareMock = vi.fn();
const unshareMock = vi.fn();
interface Team {
  id: string;
  name: string;
  family?: string;
  creGroupId?: string;
  sreGroupId?: string;
}
let teamsState: { data?: Team[]; isLoading?: boolean; isError?: boolean };
let teamSharesState: { data?: unknown; isLoading?: boolean; isError?: boolean };
const shareTeamMock = vi.fn();
const unshareTeamMock = vi.fn();

vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: () => searchResults,
}));
vi.mock("@features/csm-custom-dashboards/api/useDashboardShares", () => ({
  useGetCustomDashboardShares: () => sharesState,
  usePutCustomDashboardShare: () => ({ mutateAsync: shareMock, isPending: false }),
  useDeleteCustomDashboardShare: () => ({ mutateAsync: unshareMock, isPending: false, variables: undefined }),
}));

vi.mock("@features/csm-custom-dashboards/api/useDashboardTeamShares", () => ({
  useGetCustomDashboardTeamShares: () => teamSharesState,
  usePutCustomDashboardTeamShare: () => ({ mutateAsync: shareTeamMock, isPending: false }),
  useDeleteCustomDashboardTeamShare: () => ({ mutateAsync: unshareTeamMock, isPending: false, variables: undefined }),
}));
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: () => teamsState,
}));

import { BackendApiError } from "@api/backend/client";
import ShareCustomDashboardDialog from "@features/csm-custom-dashboards/components/ShareCustomDashboardDialog";

const DASHBOARD = { id: "5e636a12-0000-4000-8000-000000000001", name: "Live Save Test" };
const JANE: User = { id: "jane", userName: "jane", name: "Jane Doe", email: "jane.doe@example.com", userType: "internal" };
const ALICE: User = { id: "alice", userName: "alice", name: "Alice Perera", email: "alice.perera@example.com", userType: "internal" };
const BEN: User = { id: "ben", userName: "ben", name: "Ben Bee", email: "ben@example.com", userType: "internal" };
const CUSTOMER: User = { id: "cust", userName: "cust", name: "John Smith", email: "john@example.com", userType: "customer" };

// The registry teams as POST /teams/search lists them: Orion has no group, so
// it can't be shared with; Specialists is not a registry team (no family).
const TEAMS: Team[] = [
  { id: "rigel", name: "Rigel", family: "cre-abt", creGroupId: "g-rigel" },
  { id: "apollo", name: "Apollo", family: "sre-abt", sreGroupId: "g-apollo" },
  { id: "americas", name: "Americas", family: "cre", creGroupId: "g-americas" },
  { id: "castor", name: "Castor", family: "cre-abt", creGroupId: "g-castor" },
  { id: "orion", name: "Orion", family: "cre-abt" },
  { id: "specialists", name: "Specialists" },
];

function showTeams(): void {
  fireEvent.click(screen.getByRole("button", { name: "Teams" }));
}

/** The Teams list as "family: team, team" lines. */
function teamGroups(): string[] {
  return within(screen.getByLabelText("Teams to share with"))
    .queryAllByRole("group")
    .map(
      (g) =>
        `${g.getAttribute("aria-label")}: ${within(g)
          .queryAllByRole("button")
          .map((b) => (b.getAttribute("aria-label") ?? "").replace("Share with team ", ""))
          .join(", ")}`,
    );
}

function renderDialog(onClose = vi.fn()) {
  render(<ShareCustomDashboardDialog open dashboard={DASHBOARD} currentUserEmail="Jane.Doe@example.com" onClose={onClose} />);
  return onClose;
}

function resultNames(): string[] {
  return within(screen.getByLabelText("People to share with"))
    .queryAllByRole("button")
    .map((b) => b.getAttribute("aria-label") ?? "");
}

describe("ShareCustomDashboardDialog", () => {
  beforeEach(() => {
    searchResults = { data: { users: [JANE, ALICE, BEN, CUSTOMER] } };
    sharesState = { data: [] };
    shareMock.mockReset();
    unshareMock.mockReset();
    teamsState = { data: TEAMS };
    teamSharesState = { data: [] };
    shareTeamMock.mockReset();
    unshareTeamMock.mockReset();
  });

  it("names the dashboard and says nobody has it yet", () => {
    renderDialog();
    expect(screen.getByText("Share “Live Save Test”")).toBeInTheDocument();
    expect(screen.getByText("Not shared with anyone yet.")).toBeInTheDocument();
    expect(screen.getByText(/can't edit it/)).toBeInTheDocument();
  });

  it("offers internal staff only, without the caller or people who already have it", () => {
    sharesState = { data: [{ user: { id: "ben", email: "ben@example.com", name: "Ben Bee" }, sharedOn: "a" }] };
    renderDialog();
    expect(resultNames()).toEqual(["Share with Alice Perera"]);
  });

  it("shares with the person clicked", async () => {
    shareMock.mockResolvedValue([]);
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    await waitFor(() => expect(shareMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, userId: "alice" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the backend's refusal as a sentence", async () => {
    shareMock.mockRejectedValue(new BackendApiError(400, "dashboards can only be shared with internal staff"));
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Dashboards can only be shared with internal staff.");
  });

  it("is generic when sharing fails for another reason", async () => {
    shareMock.mockRejectedValue(new Error("network down"));
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Share with Alice Perera" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("The dashboard couldn't be shared. Try again in a moment.");
  });

  it("lists who has it, and stops sharing with one of them", async () => {
    unshareMock.mockResolvedValue(null);
    sharesState = { data: [{ user: { id: "alice", email: "alice.perera@example.com", name: "Alice Perera" }, sharedOn: "a" }] };
    renderDialog();
    const shared = screen.getByLabelText("Shared with");
    expect(within(shared).getByText("Alice Perera")).toBeInTheDocument();
    expect(within(shared).getByText("alice.perera@example.com")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Stop sharing with Alice Perera" }));
    await waitFor(() => expect(unshareMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, userId: "alice" }));
  });

  it("says so when neither share list can be loaded", () => {
    sharesState = { isError: true };
    teamSharesState = { isError: true };
    renderDialog();
    expect(screen.getByText("Could not load who this dashboard is shared with.")).toBeInTheDocument();
  });

  it("keeps the teams, and their stop buttons, when only the people list fails", () => {
    sharesState = { isError: true };
    teamSharesState = { data: [{ team: { key: "rigel", name: "Rigel", family: "cre-abt" }, sharedOn: "a" }] };
    renderDialog();
    const shared = screen.getByLabelText("Shared with");
    expect(within(shared).getByText("Could not load the people this dashboard is shared with.")).toBeInTheDocument();
    expect(within(shared).getByText("Rigel")).toBeInTheDocument();
    expect(within(shared).getByRole("button", { name: "Stop sharing with team Rigel" })).toBeInTheDocument();
    expect(within(shared).queryByText("Could not load who this dashboard is shared with.")).not.toBeInTheDocument();
    expect(within(shared).queryByText("Not shared with anyone yet.")).not.toBeInTheDocument();
  });

  it("keeps the people, and their stop buttons, when only the team list fails", () => {
    sharesState = { data: [{ user: { id: "alice", email: "alice.perera@example.com", name: "Alice Perera" }, sharedOn: "a" }] };
    teamSharesState = { isError: true };
    renderDialog();
    const shared = screen.getByLabelText("Shared with");
    expect(within(shared).getByText("Could not load the teams this dashboard is shared with.")).toBeInTheDocument();
    expect(within(shared).getByText("Alice Perera")).toBeInTheDocument();
    expect(within(shared).getByRole("button", { name: "Stop sharing with Alice Perera" })).toBeInTheDocument();
  });

  it("does not claim the dashboard is unshared when one list failed and the other is empty", () => {
    sharesState = { data: [] };
    teamSharesState = { isError: true };
    renderDialog();
    const shared = screen.getByLabelText("Shared with");
    expect(within(shared).getByText("Could not load the teams this dashboard is shared with.")).toBeInTheDocument();
    expect(within(shared).queryByText("Not shared with anyone yet.")).not.toBeInTheDocument();
  });

  it("says so when the search fails or finds nobody", () => {
    searchResults = { isError: true };
    renderDialog();
    expect(screen.getByText("Could not search people. Try again.")).toBeInTheDocument();
  });

  it("closes with Done", () => {
    const onClose = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(onClose).toHaveBeenCalled();
  });

  describe("teams", () => {
    it("offers registry teams with a group, by family, each on its own", () => {
      renderDialog();
      showTeams();
      expect(screen.getByLabelText("Filter teams")).toBeInTheDocument();
      expect(teamGroups()).toEqual(["CRE ABT: Castor, Rigel", "CRE: Americas", "SRE ABT: Apollo"]);
      expect(screen.queryByLabelText("People to share with")).not.toBeInTheDocument();
    });

    it("filters by name and leaves out teams it is already shared with", () => {
      teamSharesState = { data: [{ team: { key: "castor", name: "Castor", family: "cre-abt" }, sharedOn: "a" }] };
      renderDialog();
      showTeams();
      expect(teamGroups()).toEqual(["CRE ABT: Rigel", "CRE: Americas", "SRE ABT: Apollo"]);
      fireEvent.change(screen.getByLabelText("Filter teams"), { target: { value: "AMER" } });
      expect(teamGroups()).toEqual(["CRE: Americas"]);
      fireEvent.change(screen.getByLabelText("Filter teams"), { target: { value: "zzz" } });
      expect(screen.getByText("No matching teams.")).toBeInTheDocument();
    });

    it("shares with the team clicked, by its key", async () => {
      shareTeamMock.mockResolvedValue([]);
      renderDialog();
      showTeams();
      fireEvent.click(screen.getByRole("button", { name: "Share with team Rigel" }));
      await waitFor(() => expect(shareTeamMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, teamKey: "rigel" }));
      expect(shareMock).not.toHaveBeenCalled();
    });

    it("shows the backend's refusal", async () => {
      shareTeamMock.mockRejectedValue(new BackendApiError(404, "That team can't be found, so the dashboard can't be shared with it."));
      renderDialog();
      showTeams();
      fireEvent.click(screen.getByRole("button", { name: "Share with team Apollo" }));
      expect(await screen.findByRole("alert")).toHaveTextContent("That team can't be found, so the dashboard can't be shared with it.");
    });

    it("says so when the teams can't be loaded", () => {
      teamsState = { isError: true };
      renderDialog();
      showTeams();
      expect(screen.getByText("Could not load the teams. Try again.")).toBeInTheDocument();
    });

    it("lists teams before people under Shared with, and stops sharing with a team", async () => {
      unshareTeamMock.mockResolvedValue(null);
      teamSharesState = { data: [{ team: { key: "rigel", name: "Rigel", family: "cre-abt" }, sharedOn: "a" }] };
      sharesState = { data: [{ user: { id: "alice", email: "alice.perera@example.com", name: "Alice Perera" }, sharedOn: "a" }] };
      renderDialog();
      const shared = screen.getByLabelText("Shared with");
      expect(within(shared).getByText("Team · CRE ABT")).toBeInTheDocument();
      const names = within(shared)
        .getAllByRole("button")
        .map((b) => b.getAttribute("aria-label"));
      expect(names).toEqual(["Stop sharing with team Rigel", "Stop sharing with Alice Perera"]);

      fireEvent.click(screen.getByRole("button", { name: "Stop sharing with team Rigel" }));
      await waitFor(() => expect(unshareTeamMock).toHaveBeenCalledWith({ dashboardId: DASHBOARD.id, teamKey: "rigel" }));
    });

    it("lists a team no longer in the registry without a remove button", () => {
      teamSharesState = { data: [{ team: { key: "", name: "Old Team", family: "cre" }, sharedOn: "a" }] };
      renderDialog();
      const shared = screen.getByLabelText("Shared with");
      expect(within(shared).getByText("Old Team")).toBeInTheDocument();
      expect(within(shared).queryByRole("button")).not.toBeInTheDocument();
    });

    it("says nothing is shared only when neither people nor teams have it", () => {
      teamSharesState = { data: [{ team: { key: "rigel", name: "Rigel", family: "cre-abt" }, sharedOn: "a" }] };
      renderDialog();
      expect(screen.queryByText("Not shared with anyone yet.")).not.toBeInTheDocument();
    });
  });
});
