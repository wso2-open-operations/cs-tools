/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */


import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { RotaAccess, ScheduleTeam } from "../types";
import type { RotaAccessChange } from "../api/useRotaAccess";
// vi.mock below is hoisted above this import, so the tab gets the mocks.
import RotaAccessTab from "./RotaAccessTab";

const access: { data?: RotaAccess } = {};
const mutate = vi.fn();
const found: { data: { id: string; firstName: string; lastName: string; email: string }[] } = { data: [] };

vi.mock("../api/useRotaAccess", () => ({
  useRotaAccess: () => ({ data: access.data, isLoading: false, isError: false }),
  useChangeRotaAccess: () => ({ mutate, isPending: false }),
  memberFamiliesOf: (a: RotaAccess | undefined) => a?.canManage.memberFamilies ?? [],
}));
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: (q: string, enabled: boolean) => ({ data: enabled && q ? found.data : [], isLoading: false }),
}));
// The search waits a moment for typing to stop; the tests do not.
vi.mock("@hooks/useDebouncedValue", () => ({ useDebouncedValue: <T,>(v: T) => v }));
// The reminder settings card has its own tests; here only whether it is shown.
vi.mock("./RotaReminderSettings", () => ({ default: () => <div>reminder settings card</div> }));

const ME = "11111111-1111-4111-8111-111111111111";
const ANA = "22222222-2222-4222-8222-222222222222";
const BEN = "33333333-3333-4333-8333-333333333333";
const LEO = "44444444-4444-4444-8444-444444444444";

const member = (userId: string, name: string, isLead = false) => ({
  userId,
  name,
  email: `${name.toLowerCase()}@example.test`,
  isLead,
  role: isLead ? "lead" : "engineer",
});

const TEAMS: ScheduleTeam[] = [
  { key: "apollo", name: "Apollo", family: "SRE", sortOrder: 1, members: [member(LEO, "Leo", true), member(ANA, "Ana"), member(BEN, "Ben")] },
  { key: "lyra", name: "Lyra", family: "CRE", sortOrder: 2, members: [member(ME, "Me")] },
];

const granted = { grantedBy: "head@example.test", grantedOn: "2026-10-01T00:00:00Z", active: true };

function lastChange(): RotaAccessChange {
  return mutate.mock.calls.at(-1)?.[0] as RotaAccessChange;
}

beforeEach(() => {
  mutate.mockReset();
  found.data = [];
  access.data = {
    canManage: { rotaAdmins: true, markerFamilies: ["CRE", "SRE", "SME"], memberFamilies: ["CRE", "SRE", "SME"] },
    rotaAdmins: [{ family: "SRE", userId: BEN, name: "Ben", email: "ben@example.test", ...granted }],
    markers: [{ teamKey: "apollo", family: "SRE", userId: ANA, name: "Ana", email: "ana@example.test", ...granted }],
  };
});

describe("RotaAccessTab", () => {
  it("lists each group's rota admins, and removes one", () => {
    render(<RotaAccessTab teams={TEAMS} />);
    const sre = screen.getByRole("list", { name: "SRE rota admins" });
    expect(within(sre).getByText("Ben")).toBeInTheDocument();
    expect(screen.getByText("No CRE rota admins")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Remove Ben as SRE rota admin" }));
    expect(lastChange()).toEqual({ kind: "admin", family: "SRE", userId: BEN, grant: false });
  });

  it("adds a rota admin found by name among internal staff", () => {
    found.data = [{ id: ME, firstName: "Mina", lastName: "Silva", email: "mina@example.test" }];
    render(<RotaAccessTab teams={TEAMS} />);
    fireEvent.change(screen.getByRole("searchbox", { name: "Add a CRE rota admin" }), { target: { value: "min" } });
    fireEvent.click(screen.getByRole("button", { name: /Mina Silva/ }));
    expect(lastChange()).toEqual({ kind: "admin", family: "CRE", userId: ME, grant: true });
  });

  it("names a team's Rota Manager from its members, leaving out the lead and anyone already one", () => {
    render(<RotaAccessTab teams={TEAMS} />);
    fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
    const pick = screen.getByRole("combobox", { name: "Add Rota Manager for Apollo" });
    const offered = within(pick)
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(offered).toEqual(["+ Add Rota Manager", "Ben"]);

    fireEvent.change(pick, { target: { value: BEN } });
    expect(lastChange()).toEqual({ kind: "marker", teamKey: "apollo", userId: BEN, grant: true });

    fireEvent.click(screen.getByRole("button", { name: "Remove Ana as Apollo Rota Manager" }));
    expect(lastChange()).toEqual({ kind: "marker", teamKey: "apollo", userId: ANA, grant: false });
  });

  it("ties each group tab to the markers panel it shows", () => {
    render(<RotaAccessTab teams={TEAMS} />);
    const sre = screen.getByRole("tab", { name: "SRE" });
    fireEvent.click(sre);
    const panel = screen.getByRole("tabpanel", { name: "SRE" });
    expect(sre).toHaveAttribute("aria-controls", panel.id);
    expect(within(panel).getByRole("rowheader", { name: "Apollo" })).toBeInTheDocument();
  });

  it("names a team's head as its Lead and lists its other leads with the Rota Managers", () => {
    const head = { ...member(LEO, "Leo", true), role: "americas_team_lead" };
    const teams: ScheduleTeam[] = [
      { key: "westwind", name: "Westwind", family: "CRE", sortOrder: 1, heads: [head], members: [member(ANA, "Ana", true), member(BEN, "Ben")] },
      { key: "lyra", name: "Lyra", family: "CRE", sortOrder: 2, members: [member(ME, "Me", true), member(BEN, "Ben")] },
    ];
    render(<RotaAccessTab teams={teams} />);
    const row = screen.getByRole("rowheader", { name: "Westwind" }).closest("tr") as HTMLElement;
    expect(within(row).getByText("Leo")).toHaveClass("ra-leads");
    const managers = within(row).getByRole("list", { name: "Westwind Rota Managers" });
    expect(within(managers).getByText("Ana")).toBeInTheDocument();
    // A lead cannot be taken off here, and is not offered again.
    expect(within(row).queryByRole("button", { name: /Remove Ana/ })).not.toBeInTheDocument();
    const offered = within(within(row).getByRole("combobox")).getAllByRole("option").map((o) => o.textContent);
    expect(offered).toEqual(["+ Add Rota Manager", "Ben"]);
    // A team with no head keeps all its leads in the Lead column.
    const lyra = screen.getByRole("rowheader", { name: "Lyra" }).closest("tr") as HTMLElement;
    expect(within(lyra).getByText("Me")).toHaveClass("ra-leads");
  });

  it("shows a team synced as an identifier by its display name", () => {
    const teams: ScheduleTeam[] = [
      { key: "northwind_cre_team", name: "northwind_cre_team", family: "CRE", sortOrder: 1, members: [member(ME, "Me", true)] },
      { key: "onboarding_team", name: "onboarding_team", family: "CRE", sortOrder: 2, members: [member(ANA, "Ana")] },
      { key: "orion", name: "orion", family: "CRE", sortOrder: 3, members: [member(BEN, "Ben")] },
    ];
    render(<RotaAccessTab teams={teams} />);
    const markers = screen.getByRole("tabpanel", { name: "CRE" });
    for (const name of ["Northwind", "Onboarding", "Orion"]) {
      expect(within(markers).getByRole("rowheader", { name })).toBeInTheDocument();
    }
    expect(screen.queryByText("northwind_cre_team")).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /^Onboarding/ })).toBeInTheDocument();
  });

  it("shows a rota admin only their own group's markers, and no rota admin section", () => {
    access.data = { ...access.data!, canManage: { rotaAdmins: false, markerFamilies: ["SRE"] } };
    render(<RotaAccessTab teams={TEAMS} />);
    expect(screen.queryByRole("heading", { name: "Rota admins" })).not.toBeInTheDocument();
    // One group needs no group switch, so no tab panel either.
    expect(screen.queryByRole("tab", { name: "CRE" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tabpanel")).not.toBeInTheDocument();
    expect(screen.getByRole("rowheader", { name: "Apollo" })).toBeInTheDocument();
    expect(screen.queryByRole("rowheader", { name: "Lyra" })).not.toBeInTheDocument();
  });

  it("shows a team's lead only the teams they lead, and lets them name a Rota Manager", () => {
    access.data = {
      canManage: { rotaAdmins: false, markerFamilies: [], markerTeams: ["APOLLO"] },
      rotaAdmins: [],
      markers: [{ teamKey: "apollo", family: "SRE", userId: ANA, name: "Ana", email: "ana@example.test", ...granted }],
    };
    render(<RotaAccessTab teams={TEAMS} />);
    expect(screen.queryByRole("heading", { name: "Rota admins" })).not.toBeInTheDocument();
    expect(screen.queryByRole("tablist", { name: "Group" })).not.toBeInTheDocument();
    expect(screen.getByRole("rowheader", { name: "Apollo" })).toBeInTheDocument();
    expect(screen.queryByRole("rowheader", { name: "Lyra" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByRole("combobox", { name: "Add Rota Manager for Apollo" }), { target: { value: BEN } });
    expect(lastChange()).toEqual({ kind: "marker", teamKey: "apollo", userId: BEN, grant: true });
    expect(screen.queryByText("reminder settings card")).not.toBeInTheDocument();
  });

  it("adds the groups of the teams a rota admin also leads", () => {
    access.data = {
      canManage: { rotaAdmins: false, markerFamilies: ["CRE"], markerTeams: ["apollo"] },
      rotaAdmins: [],
      markers: [],
    };
    render(<RotaAccessTab teams={TEAMS} />);
    const tabs = within(screen.getByRole("tablist", { name: "Group" })).getAllByRole("tab");
    expect(tabs.map((t) => t.textContent)).toEqual(["CRE", "SRE"]);
  });

  it("shows the CRE rota reminder settings to a CSM admin", () => {
    render(<RotaAccessTab teams={TEAMS} />);
    expect(screen.getByText("reminder settings card")).toBeInTheDocument();
  });

  it("shows the CRE rota reminder settings to the CRE rota admin", () => {
    access.data = { ...access.data!, canManage: { rotaAdmins: false, markerFamilies: ["CRE"] } };
    render(<RotaAccessTab teams={TEAMS} />);
    expect(screen.getByText("reminder settings card")).toBeInTheDocument();
  });

  it("leaves the CRE rota reminder settings out for a rota admin of another group", () => {
    access.data = { ...access.data!, canManage: { rotaAdmins: false, markerFamilies: ["SRE"] } };
    render(<RotaAccessTab teams={TEAMS} />);
    expect(screen.queryByText("reminder settings card")).not.toBeInTheDocument();
  });

  it("marks a grant that does nothing any more, so it can be removed", () => {
    access.data!.markers[0] = { ...access.data!.markers[0], active: false };
    render(<RotaAccessTab teams={TEAMS} />);
    fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
    const list = screen.getByRole("list", { name: "Apollo Rota Managers" });
    expect(within(list).getByText("inactive")).toBeInTheDocument();
  });

  it("says why a change did not save", () => {
    mutate.mockImplementation((_c: RotaAccessChange, opts?: { onError?: (e: Error) => void }) =>
      opts?.onError?.(new Error("only a CSM admin can grant or remove rota admin")),
    );
    render(<RotaAccessTab teams={TEAMS} />);
    fireEvent.click(screen.getByRole("button", { name: "Remove Ben as SRE rota admin" }));
    expect(screen.getByRole("alert")).toHaveTextContent("only a CSM admin can grant or remove rota admin");
  });

  describe("team members", () => {
    it("lists the chosen team's members, lead first, and removes one only after asking", () => {
      render(<RotaAccessTab teams={TEAMS} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      const list = screen.getByRole("list", { name: "Apollo members" });
      expect(within(list).getAllByRole("listitem").map((li) => li.textContent?.replace("×", "").trim())).toEqual([
        "Leoleo@example.testlead",
        "Anaana@example.test",
        "Benben@example.test",
      ]);
      expect(screen.getByText(/3 members · Lead Leo/)).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Remove Ana from Apollo" }));
      expect(mutate).not.toHaveBeenCalled();
      expect(screen.getByText("Remove Ana from Apollo?")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Remove" }));
      expect(lastChange()).toEqual({ kind: "member", teamKey: "apollo", userId: ANA, grant: false });
    });

    it("cancels a removal without changing anything", () => {
      render(<RotaAccessTab teams={TEAMS} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      fireEvent.click(screen.getByRole("button", { name: "Remove Ben from Apollo" }));
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      expect(mutate).not.toHaveBeenCalled();
      expect(screen.getByRole("button", { name: "Remove Ben from Apollo" })).toBeInTheDocument();
    });

    it("shows one team at a time, picked from the group's teams with their sizes", () => {
      const twoTeams: ScheduleTeam[] = [
        ...TEAMS,
        { key: "iaas", name: "IaaS", family: "SRE", sortOrder: 3, members: [member(ME, "Mina")] },
      ];
      render(<RotaAccessTab teams={twoTeams} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      const teams = screen.getByRole("tablist", { name: "Team" });
      expect(within(teams).getAllByRole("tab").map((t) => t.textContent)).toEqual(["Apollo3", "IaaS1"]);
      expect(screen.getByRole("list", { name: "Apollo members" })).toBeInTheDocument();
      fireEvent.click(within(teams).getByRole("tab", { name: /IaaS/ }));
      expect(screen.queryByRole("list", { name: "Apollo members" })).toBeNull();
      expect(within(screen.getByRole("list", { name: "IaaS members" })).getByText("Mina")).toBeInTheDocument();
    });

    it("filters a big team's members by name or email", () => {
      const big = Array.from({ length: 12 }, (_, i) =>
        member(`aaaaaaaa-aaaa-4aaa-8aaa-${String(i).padStart(12, "0")}`, `Engineer ${String(i).padStart(2, "0")}`),
      );
      render(<RotaAccessTab teams={[{ key: "apollo", name: "Apollo", family: "SRE", sortOrder: 1, members: big }]} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      fireEvent.change(screen.getByRole("searchbox", { name: "Filter Apollo members" }), { target: { value: "engineer 1" } });
      expect(within(screen.getByRole("list", { name: "Apollo members" })).getAllByRole("listitem")).toHaveLength(2);
      fireEvent.change(screen.getByRole("searchbox", { name: "Filter Apollo members" }), { target: { value: "nobody" } });
      expect(screen.getByText(/No member matches/)).toBeInTheDocument();
    });

    it("adds a member found among internal staff, leaving out who is already on the team", () => {
      found.data = [
        { id: ME, firstName: "Mina", lastName: "Silva", email: "mina@example.test" },
        { id: ANA, firstName: "Ana", lastName: "", email: "ana@example.test" },
      ];
      render(<RotaAccessTab teams={TEAMS} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      fireEvent.change(screen.getByRole("searchbox", { name: "Add a member to Apollo" }), { target: { value: "a" } });
      fireEvent.change(screen.getByRole("searchbox", { name: "Add a member to Apollo" }), { target: { value: "an" } });
      const matches = screen.getByRole("list", { name: "Add a member to Apollo: matches" });
      expect(within(matches).queryByRole("button", { name: /^Ana/ })).toBeNull();
      fireEvent.click(within(matches).getByRole("button", { name: /Mina Silva/ }));
      expect(lastChange()).toEqual({ kind: "member", teamKey: "apollo", userId: ME, grant: true });
    });

    it("offers a CSM admin the lead's removal, and a rota admin not", () => {
      render(<RotaAccessTab teams={TEAMS} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      expect(screen.getByRole("button", { name: "Remove Leo from Apollo" })).toBeInTheDocument();
    });

    it("shows a rota admin only their own group's teams, without the lead's removal", () => {
      access.data = {
        canManage: { rotaAdmins: false, markerFamilies: ["SRE"], memberFamilies: ["SRE"] },
        rotaAdmins: [],
        markers: [],
      };
      render(<RotaAccessTab teams={TEAMS} />);
      expect(screen.getByRole("heading", { name: "Team members" })).toBeInTheDocument();
      expect(screen.getByRole("list", { name: "Apollo members" })).toBeInTheDocument();
      expect(screen.queryByRole("list", { name: "Lyra members" })).toBeNull();
      expect(screen.queryByRole("button", { name: "Remove Leo from Apollo" })).toBeNull();
      expect(screen.getByRole("button", { name: "Remove Ben from Apollo" })).toBeInTheDocument();
      expect(screen.getByText(/a team lead only by a CSM admin/)).toBeInTheDocument();
    });

    it("brings a refused removal's reason into view, however far down the list it was clicked", () => {
      const scrolled = vi.fn();
      Element.prototype.scrollIntoView = scrolled;
      mutate.mockImplementation((_c: RotaAccessChange, opts?: { onError?: (e: Error) => void }) =>
        opts?.onError?.(new Error("this person still has 1 upcoming turn on apollo: remove or reassign them on the roster first")),
      );
      render(<RotaAccessTab teams={TEAMS} />);
      fireEvent.click(screen.getByRole("tab", { name: "SRE" }));
      fireEvent.click(screen.getByRole("button", { name: "Remove Ben from Apollo" }));
      fireEvent.click(screen.getByRole("button", { name: "Remove" }));
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("1 upcoming turn on apollo");
      expect(scrolled).toHaveBeenCalled();
      expect(scrolled.mock.contexts.at(-1)).toBe(alert);
    });

    it("offers no member changes when the server says the reader may make none", () => {
      access.data = {
        canManage: { rotaAdmins: false, markerFamilies: ["SRE"] },
        rotaAdmins: [],
        markers: [],
      };
      render(<RotaAccessTab teams={TEAMS} />);
      expect(screen.queryByRole("heading", { name: "Team members" })).toBeNull();
      expect(screen.queryByRole("searchbox", { name: /Add a member/ })).toBeNull();
    });
  });
});
