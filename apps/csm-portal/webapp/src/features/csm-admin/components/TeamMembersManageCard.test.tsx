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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const api = { get: vi.fn(), put: vi.fn(), del: vi.fn(), patch: vi.fn(), post: vi.fn() };
vi.mock("@api/backend/client", () => ({ useBackendApi: () => api }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
// The real picker is a directory-search autocomplete; what matters here is
// which user id it hands back and that it is scoped to internal staff.
vi.mock("@features/csm-cases/components/AsyncUserIdSingleSelect", () => ({
  default: (p: { value: string; onChange: (v: string) => void; roleIds?: string[] }) => (
    <input
      aria-label="Add a member"
      data-internal-only={String(Array.isArray(p.roleIds) && p.roleIds.length > 0)}
      value={p.value}
      onChange={(e) => p.onChange(e.target.value)}
    />
  ),
}));

import TeamMembersManageCard from "@features/csm-admin/components/TeamMembersManageCard";

const JANE = "00000000-0000-0000-0000-000000000001";
const JOHN = "00000000-0000-0000-0000-000000000002";
const LEAD = "00000000-0000-0000-0000-000000000003";
const NEW = "00000000-0000-0000-0000-000000000004";

const catalogue = {
  zones: [],
  shifts: [],
  absenceKinds: [],
  teams: [
    {
      key: "vega",
      name: "Vega",
      family: "CRE",
      sortOrder: 1,
      members: [
        { userId: LEAD, name: "Lee Lead", email: "lee.lead@example.com", isLead: true, role: "lead" },
        { userId: JANE, name: "Jane Doe", email: "jane.doe@example.com", isLead: false, role: "engineer" },
        { userId: JOHN, name: "John Roe", email: "john.roe@example.com", isLead: false, role: "sub_lead" },
      ],
    },
    { key: "apollo", name: "Apollo", family: "CRE", sortOrder: 2, members: [] },
  ],
};
const chain = {
  family: "CRE",
  count: 1,
  canEdit: {},
  members: [{ teamKey: "vega", userId: JOHN, responderRank: 2, role: "sub_lead" }],
};

function renderCard(isAdmin = true) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const spy = vi.spyOn(qc, "invalidateQueries");
  render(
    <QueryClientProvider client={qc}>
      <TeamMembersManageCard teamKey="vega" isAdmin={isAdmin} />
    </QueryClientProvider>,
  );
  return { spy };
}

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
  api.get.mockImplementation(async (path: string) =>
    path.startsWith("/team-schedule/catalogue") ? catalogue : path.startsWith("/team-schedule/paging-chain") ? chain : null,
  );
});

describe("TeamMembersManageCard", () => {
  it("renders nothing for a non-admin and fetches nothing", () => {
    renderCard(false);
    expect(screen.queryByText("Manage members")).not.toBeInTheDocument();
    expect(api.get).not.toHaveBeenCalled();
  });

  it("shows an admin the members with their role and current tier", async () => {
    renderCard();
    expect(await screen.findByText("Manage members")).toBeInTheDocument();
    expect(await screen.findByLabelText("Role for Jane Doe")).toHaveValue("engineer");
    expect(screen.getByLabelText("Role for John Roe")).toHaveValue("sub_lead");
    await waitFor(() => expect(screen.getByLabelText("Alert tier for John Roe")).toHaveValue("T2"));
    expect(screen.getByLabelText("Add a member")).toHaveAttribute("data-internal-only", "true");
  });

  it("shows a message when the team is not on the schedule", async () => {
    api.get.mockImplementation(async () => ({ ...catalogue, teams: [] }));
    renderCard();
    expect(await screen.findByText(/not on the team schedule/i)).toBeInTheDocument();
  });

  it("shows an error when the schedule cannot be loaded", async () => {
    api.get.mockRejectedValue(new Error("boom"));
    renderCard();
    expect(await screen.findByText(/Couldn't load the team schedule/i)).toBeInTheDocument();
  });

  it("adds the picked person with PUT and refreshes the lists", async () => {
    api.put.mockResolvedValue({});
    const { spy } = renderCard();
    await screen.findByLabelText("Role for Jane Doe");
    await fireEvent.change(screen.getByLabelText("Add a member"), { target: { value: NEW } });
    await fireEvent.click(screen.getByRole("button", { name: "Add member" }));
    await waitFor(() => expect(api.put).toHaveBeenCalledWith(`/team-schedule/teams/vega/members/${NEW}`, {}));
    await waitFor(() => expect(spy).toHaveBeenCalledWith({ queryKey: ["csm-admin-team-members"] }));
    expect(spy).toHaveBeenCalledWith({ queryKey: ["csm-users-search"] });
    expect(spy).toHaveBeenCalledWith({ queryKey: ["team-schedule", "catalogue"] });
    expect(await screen.findByText("Member added as an engineer.")).toBeInTheDocument();
  });

  it("removes after confirmation with DELETE and refreshes", async () => {
    api.del.mockResolvedValue(null);
    const { spy } = renderCard();
    await fireEvent.click(await screen.findByRole("button", { name: "Remove Jane Doe from Vega" }));
    expect(api.del).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog");
    await fireEvent.click(within(dialog).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(api.del).toHaveBeenCalledWith(`/team-schedule/teams/vega/members/${JANE}`));
    await waitFor(() => expect(spy).toHaveBeenCalledWith({ queryKey: ["csm-admin-team-members"] }));
  });

  it("shows the server's reason when a removal is refused (409)", async () => {
    api.del.mockRejectedValue(new Error("Jane Doe has upcoming turns on Vega."));
    renderCard();
    await fireEvent.click(await screen.findByRole("button", { name: "Remove Jane Doe from Vega" }));
    await fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Remove" }));
    expect(await screen.findByText("Jane Doe has upcoming turns on Vega.")).toBeInTheDocument();
  });

  it("PATCHes only the role when only the role changed", async () => {
    api.patch.mockResolvedValue({ teamKey: "vega", userId: JANE, role: "sub_lead", alertTier: null });
    const { spy } = renderCard();
    await waitFor(() => expect(screen.getByLabelText("Alert tier for John Roe")).toHaveValue("T2"));
    await fireEvent.change(screen.getByLabelText("Role for Jane Doe"), { target: { value: "sub_lead" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save changes for Jane Doe" }));
    await waitFor(() => expect(api.patch).toHaveBeenCalledWith(`/team-schedule/teams/vega/members/${JANE}`, { role: "sub_lead" }));
    await waitFor(() => expect(spy).toHaveBeenCalledWith({ queryKey: ["csm-admin-team-members"] }));
  });

  it("PATCHes alertTier null to clear a tier, and a tier to set one", async () => {
    api.patch.mockResolvedValue({});
    renderCard();
    const johnTier = await screen.findByLabelText("Alert tier for John Roe");
    await waitFor(() => expect(johnTier).toHaveValue("T2"));
    await fireEvent.change(johnTier, { target: { value: "" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save changes for John Roe" }));
    await waitFor(() =>
      expect(api.patch).toHaveBeenCalledWith(`/team-schedule/teams/vega/members/${JOHN}`, { alertTier: null }),
    );
    await fireEvent.change(screen.getByLabelText("Alert tier for Jane Doe"), { target: { value: "T1" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save changes for Jane Doe" }));
    await waitFor(() =>
      expect(api.patch).toHaveBeenCalledWith(`/team-schedule/teams/vega/members/${JANE}`, { alertTier: "T1" }),
    );
  });

  it("shows the server's message for a taken tier (409) and a bad input (400)", async () => {
    api.patch.mockRejectedValueOnce(new Error("T1 is already held by John Roe."));
    renderCard();
    // Wait for the tiers: the rows re-seed their drafts once they load.
    await waitFor(() => expect(screen.getByLabelText("Alert tier for John Roe")).toHaveValue("T2"));
    await fireEvent.change(screen.getByLabelText("Alert tier for Jane Doe"), { target: { value: "T1" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save changes for Jane Doe" }));
    expect(await screen.findByText("T1 is already held by John Roe.")).toBeInTheDocument();
    api.patch.mockRejectedValueOnce(new Error("That person is not internal staff."));
    await waitFor(() => expect(screen.getByRole("button", { name: "Save changes for Jane Doe" })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: "Save changes for Jane Doe" }));
    expect(await screen.findByText("That person is not internal staff.")).toBeInTheDocument();
  });

  it("keeps Save disabled until something changes", async () => {
    renderCard();
    expect(await screen.findByRole("button", { name: "Save changes for Jane Doe" })).toBeDisabled();
  });

  it("moves to the chosen team with POST on the source team's path, after a confirm that states the consequence", async () => {
    api.post.mockResolvedValue({ teamKey: "apollo", userId: JANE, role: "engineer", alertTier: null });
    const { spy } = renderCard();
    await fireEvent.click(await screen.findByRole("button", { name: "Move Jane Doe to another team" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(/clears their alert tier and drops their rota manager grant/i)).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Move member" })).toBeDisabled();
    expect(within(dialog).queryByRole("option", { name: /Vega/ })).not.toBeInTheDocument();
    await fireEvent.change(within(dialog).getByLabelText("Move to"), { target: { value: "apollo" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "Move member" }));
    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith(`/team-schedule/teams/vega/members/${JANE}/move`, { toTeamKey: "apollo" }),
    );
    expect(await screen.findByText(/moved to Apollo. Their alert tier was cleared/i)).toBeInTheDocument();
    expect(spy).toHaveBeenCalledWith({ queryKey: ["csm-admin-team-members"] });
  });

  it("shows the server's reason when a move is refused (409)", async () => {
    api.post.mockRejectedValue(new Error("Jane Doe has upcoming turns on Vega."));
    renderCard();
    await fireEvent.click(await screen.findByRole("button", { name: "Move Jane Doe to another team" }));
    const dialog = await screen.findByRole("dialog");
    await fireEvent.change(within(dialog).getByLabelText("Move to"), { target: { value: "apollo" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "Move member" }));
    expect(await screen.findByText("Jane Doe has upcoming turns on Vega.")).toBeInTheDocument();
  });

  it("disables change and move for a lead, shows the role read-only, and still allows removal", async () => {
    renderCard();
    expect(await screen.findByText(/Role: Lee Lead|Role: lead \(lead\)/i)).toBeInTheDocument();
    expect(screen.queryByLabelText("Role for Lee Lead")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes for Lee Lead" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Move Lee Lead to another team" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove Lee Lead from Vega" })).toBeEnabled();
  });

  it("disables every write while one is in flight", async () => {
    api.del.mockReturnValue(new Promise(() => undefined));
    renderCard();
    await fireEvent.click(await screen.findByRole("button", { name: "Remove Jane Doe from Vega" }));
    await fireEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Remove" }));
    expect(await screen.findByRole("button", { name: "Removing…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Add member", hidden: true })).toBeDisabled();
  });
});
