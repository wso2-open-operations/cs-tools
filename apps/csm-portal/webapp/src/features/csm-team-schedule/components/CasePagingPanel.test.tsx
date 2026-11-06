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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { PagingChainResponse } from "../types";
import { pagingMember, readinessChain, readinessGap } from "../test/fixtures";
import CasePagingPanel from "./CasePagingPanel";

const mutateAsync = vi.fn();
const chain: { data?: PagingChainResponse } = {};

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {
    status = 0;
  },
  useBackendApi: () => ({}),
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("../api/usePagingChain", () => ({
  useGetPagingChain: () => ({ data: chain.data, isLoading: false, isError: false }),
  usePatchPagingMember: () => ({ mutateAsync }),
}));

const abt = (key: string, name: string) => ({ teamKey: key, teamName: name, teamType: "cre-abt" });
const AMERICAS = { teamKey: "americas", teamName: "americas_team", teamType: "cre-americas" };
const LEADERSHIP = { teamKey: "leadership", teamName: "CRE Leadership", teamType: "leadership" };

/** Two ABTs and Americas -- served with Americas first, to show it still
 *  ends up last -- and the leadership team holding the heads. */
function matrixChain(): PagingChainResponse {
  return {
    family: "CRE",
    count: 0,
    canEdit: { responderTeams: ["vega"], teamLeadTeams: ["vega", "americas"], americasTeamLead: false, heads: false },
    members: [
      pagingMember({ membershipId: "am1", name: "Nia Night", role: "lead", ...AMERICAS }),
      pagingMember({ membershipId: "am2", name: "Oz Owl", role: "americas_team_lead", ...AMERICAS }),
      pagingMember({ membershipId: "am3", name: "Pat Moon", responderRank: 1, ...AMERICAS }),
      pagingMember({ membershipId: "v1", name: "Vic Lead", role: "lead", ...abt("vega", "Vega_abt_cre_team") }),
      pagingMember({ membershipId: "v2", name: "Val First", responderRank: 1, ...abt("vega", "Vega_abt_cre_team") }),
      pagingMember({ membershipId: "v3", name: "Viv Spare", ...abt("vega", "Vega_abt_cre_team") }),
      pagingMember({ membershipId: "l1", name: "Lou Lead", role: "lead", ...abt("lyra", "Lyra_abt_cre_team") }),
      pagingMember({ membershipId: "l2", name: "Lee First", responderRank: 1, ...abt("lyra", "Lyra_abt_cre_team") }),
      pagingMember({ membershipId: "h1", name: "Cara Head", role: "cre_head", ...LEADERSHIP }),
      pagingMember({ membershipId: "h2", name: "Sol Head", role: "cs_head", ...LEADERSHIP }),
    ],
  };
}

const readiness = { days: 7, isLoading: false, isError: false };

function renderPanel(chainOverride?: ReturnType<typeof readinessChain>) {
  return render(
    <CasePagingPanel readiness={{ ...readiness, chain: chainOverride ?? readinessChain({ chain: "CRE" }) }} />,
  );
}

const rowOf = (name: RegExp) => screen.getByRole("rowheader", { name }).closest("tr") as HTMLTableRowElement;

beforeEach(() => {
  mutateAsync.mockReset();
  mutateAsync.mockResolvedValue({});
  chain.data = matrixChain();
});

describe("CasePagingPanel: the CRE chain as a team-by-level matrix", () => {
  it("is a captioned table with a column per team, the ABTs in order and Americas last, marked night", () => {
    renderPanel();
    const table = screen.getByRole("table", { name: "CRE alert chain by team and level" });
    const headers = within(table).getAllByRole("columnheader");
    expect(headers.map((h) => h.textContent?.replace(/(Ready|\d+ gaps?).*$/, "").trim())).toEqual([
      "Level",
      "Vega",
      "Lyra",
      "Americasnight",
    ]);
    expect(headers[3]).toHaveTextContent("night");
    for (const h of headers) expect(h).toHaveAttribute("scope", "col");
  });

  it("has a row per level, L0 to L4, with the optional responders marked in words", () => {
    renderPanel();
    const rows = screen.getAllByRole("rowheader");
    expect(rows.map((r) => r.textContent)).toEqual([
      "Level 0 (L0)1st, 2nd, 3rd responders · 2nd and 3rd optional",
      "Level 1 (L1)Team lead",
      "Level 2 (L2)Team leads pool · America lead at night",
      "Level 3 (L3)CRE head",
      "Level 4 (L4)CS head",
    ]);
    for (const r of rows) expect(r).toHaveAttribute("scope", "row");
  });

  it("merges the pool across the ABT columns and the heads across every column", () => {
    renderPanel();
    const tier3 = within(rowOf(/^Level 2/)).getAllByRole("cell");
    expect(tier3).toHaveLength(2);
    expect(tier3[0]).toHaveAttribute("colspan", "2");
    expect(tier3[0]).toHaveTextContent("The ABT Team leads above, longest since called first");
    expect(tier3[1]).toHaveTextContent("Oz Owl");
    for (const tier of [/^Level 3/, /^Level 4/]) {
      const cells = within(rowOf(tier)).getAllByRole("cell");
      expect(cells).toHaveLength(1);
      expect(cells[0]).toHaveAttribute("colspan", "3");
    }
    expect(within(rowOf(/^Level 3/)).getByRole("cell")).toHaveTextContent("Cara Head");
    expect(within(rowOf(/^Level 4/)).getByRole("cell")).toHaveTextContent("Sol Head");
  });

  it("offers no Edit alert chain to a reader who may change nothing, and still shows the chain", () => {
    chain.data = {
      ...matrixChain(),
      canEdit: { responderTeams: [], teamLeadTeams: [], americasTeamLead: false, heads: false },
    };
    renderPanel();
    expect(screen.queryByRole("button", { name: /Edit alert chain/ })).not.toBeInTheDocument();
    expect(screen.getByRole("table", { name: "CRE alert chain by team and level" })).toBeInTheDocument();
    expect(screen.getByText("Val First")).toBeInTheDocument();
  });

  it("offers Edit alert chain to a reader who may change only the heads", () => {
    chain.data = {
      ...matrixChain(),
      canEdit: { responderTeams: [], teamLeadTeams: [], americasTeamLead: false, heads: true },
    };
    renderPanel();
    expect(screen.getByRole("button", { name: /Edit alert chain/ })).toBeInTheDocument();
  });

  it("shows names, not pickers, until editing, and an empty slot in words", () => {
    renderPanel();
    const table = screen.getByRole("table", { name: "CRE alert chain by team and level" });
    expect(within(table).queryByRole("combobox")).not.toBeInTheDocument();
    const slots = within(screen.getByRole("list", { name: "Vega responders" })).getAllByRole("listitem");
    expect(slots[0]).toHaveTextContent("1st");
    expect(slots[0]).toHaveTextContent("Val First");
    expect(slots[1]).toHaveTextContent("2nd · optional");
    expect(slots[1]).toHaveTextContent("— not set —");
  });

  it("puts L0 in one row, each team's cell split into its 1st, 2nd and 3rd responder", () => {
    renderPanel();
    const cells = within(rowOf(/^Level 0/)).getAllByRole("cell");
    expect(cells).toHaveLength(3);
    for (const cell of cells) {
      const slots = within(within(cell).getByRole("list")).getAllByRole("listitem");
      expect(slots.map((s) => s.textContent?.slice(0, 3))).toEqual(["1st", "2nd", "3rd"]);
    }
  });

  it("gives Americas its Team leads list at L1 and its America lead at L2", () => {
    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: /Edit alert chain/ }));
    const tier2 = within(rowOf(/^Level 1/)).getAllByRole("cell");
    expect(within(tier2[0]).getByLabelText("Vega Team lead")).toBeInTheDocument();
    expect(tier2[2]).toHaveTextContent("Nia Night");
    expect(within(within(tier2[2]).getByRole("list", { name: "Americas Team leads" })).getAllByRole("listitem")).toHaveLength(1);
    expect(within(tier2[2]).getByRole("button", { name: "Remove Nia Night as a Team lead" })).toBeInTheDocument();
    // A list with add/remove, not ABTs' single Team lead picker.
    expect(within(tier2[2]).queryByRole("combobox", { name: "Americas Team lead" })).not.toBeInTheDocument();
    expect(within(tier2[2]).getByRole("combobox", { name: "Add a Americas Team lead" })).toBeInTheDocument();
    const tier3 = within(rowOf(/^Level 2/)).getAllByRole("cell");
    expect(within(tier3[1]).getByLabelText("Americas America lead")).toBeDisabled();
  });

  it("offers Test call on each Americas Team lead as on any other cell, editing or not", () => {
    const testCall = { masked: "+94•••••001", source: "profile" as const };
    const base = matrixChain();
    chain.data = {
      ...base,
      members: base.members.map((m) =>
        m.membershipId === "am1" || m.membershipId === "v1"
          ? { ...m, hasProfilePhone: true, canTestCall: true, testCall }
          : m,
      ),
    };
    const phone = { edit: vi.fn(), remove: vi.fn(), test: vi.fn(), calling: () => false };
    render(<CasePagingPanel readiness={{ ...readiness, chain: readinessChain({ chain: "CRE" }) }} phone={phone} />);
    const tier2 = () => within(rowOf(/^Level 1/)).getAllByRole("cell");
    const nightTest = () =>
      within(tier2()[2]).getByRole("button", { name: "Test call Nia Night's profile number +94•••••001" });

    fireEvent.click(nightTest());
    expect(phone.test).toHaveBeenCalledWith(expect.objectContaining({ membershipId: "am1" }));
    expect(within(tier2()[0]).getByRole("button", { name: /^Test call Vic Lead/ })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Edit alert chain/ }));
    expect(nightTest()).toBeInTheDocument();
  });

  it("edits a cell where the reader may, and locks the rest saying who may", async () => {
    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: /Edit alert chain/ }));
    expect(screen.getByLabelText("Lyra 1st responder")).toBeDisabled();
    expect(screen.getAllByText("this team's lead, rota admin").length).toBeGreaterThan(0);

    const pick = screen.getByLabelText("Vega 2nd responder");
    expect(pick).toBeEnabled();
    fireEvent.change(pick, { target: { value: "v3" } });
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({ membershipId: "v3", responderRank: 2 }));
  });

  it("Set responders opens editing and focuses that team's 1st responder in its column", async () => {
    renderPanel(
      readinessChain({
        chain: "CRE",
        ready: false,
        gaps: [readinessGap({ code: "ON_LEAVE", fix: "responders", teamKey: "Vega", date: "2026-10-10" })],
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Set responders" }));
    await waitFor(() => expect(screen.getByLabelText("Vega 1st responder")).toHaveFocus());
    expect(screen.getByRole("button", { name: /Done editing/ })).toBeInTheDocument();
  });

  it("Set responders on a team the reader may not edit still lands on its column's cell", async () => {
    renderPanel(
      readinessChain({
        chain: "CRE",
        ready: false,
        gaps: [readinessGap({ code: "NO_RESPONDER", fix: "responders", teamKey: "lyra" })],
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Set responders" }));
    // Lyra is not the reader's, so editing stays off and its cell takes focus.
    await waitFor(() => expect(document.activeElement?.id).toBe("cp-r1-lyra"));
  });
});
