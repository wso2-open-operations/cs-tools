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
import { MemoryRouter, Route, Routes, useLocation, useSearchParams } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { JSX } from "react";

// The import tree reads this at module load; vitest does not load public/config.js.
vi.hoisted(() => {
  window.config = { ...window.config, CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090" };
});

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  access: { canWrite: true },
  templateSearch: vi.fn(),
  proposalSearch: vi.fn(),
  proposalCount: vi.fn(),
  createProposal: vi.fn(),
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
  useSearchStandardChangeTemplates: (payload: unknown) => mocks.templateSearch(payload),
  useSearchStandardChangeProposals: (payload: unknown) => mocks.proposalSearch(payload),
  useCountStandardChangeProposals: (filter: unknown) => mocks.proposalCount(filter),
  useCreateStandardChangeProposal: () => ({ mutate: mocks.createProposal, isPending: false, isError: false, reset: vi.fn() }),
}));

import StandardChangesTab from "@features/csm-operations/components/StandardChangesTab";
import { standardChangesShow } from "@features/csm-operations/utils/standardChanges";

const TEMPLATES = [
  {
    id: "tpl-1",
    number: "STD0000001",
    name: "Rotate a TLS certificate",
    state: "active",
    currentVersion: { id: "ver-3", version: 3, risk: "low", impact: "medium" },
    reviewRequired: true,
    usage: { total: 12, open: 2, closed: 9, rolledBack: 1, canceled: 0, lastUsedOn: null },
  },
  {
    id: "tpl-2",
    number: "STD0000002",
    name: "Restart a gateway node",
    state: "suspended",
    currentVersion: { id: "ver-1", version: 1, risk: "moderate", impact: "low" },
    reviewRequired: false,
    usage: { total: 0, open: 0, closed: 0, rolledBack: 0, canceled: 0, lastUsedOn: null },
  },
];
const PROPOSALS = [
  {
    id: "scp-1",
    number: "SCP0000001",
    kind: "new",
    state: "cab_review",
    name: "Rotate a TLS certificate",
    template: null,
    proposedBy: { id: "u-1", name: "Jane Doe", email: "jane@example.com" },
    createdOn: "2026-10-01T00:00:00Z",
    updatedOn: "2026-10-02T00:00:00Z",
    submittedOn: null,
    decidedOn: null,
  },
];

function Probe(): JSX.Element {
  const l = useLocation();
  return <div data-testid="location">{`${l.pathname}${l.search}`}</div>;
}

const CATALOGUE = "/operations/change-requests?view=standard";
const PROPOSALS_VIEW = "/operations/change-requests?view=standard&show=proposals";

/** The tab as `ChangeRequestsArea` renders it: which part is shown comes from the URL. */
function TabAtUrl(): JSX.Element {
  const [params] = useSearchParams();
  return <StandardChangesTab show={standardChangesShow(params)} />;
}

function renderTab(entry = CATALOGUE): void {
  render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route
          path="/operations/change-requests"
          element={
            <>
              <TabAtUrl />
              <Probe />
            </>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

const currentUrl = (): string => screen.getByTestId("location").textContent ?? "";

type CountResult = { data?: number; isPending: boolean; isSuccess: boolean; isError: boolean };
const count = (n: number): CountResult => ({ data: n, isPending: false, isSuccess: true, isError: false });
const PENDING: CountResult = { isPending: true, isSuccess: false, isError: false };
const FAILED: CountResult = { isPending: false, isSuccess: false, isError: true };

/** What the two counts answer: proposals awaiting my CAB decision, and my drafts. */
function counts(awaiting: CountResult, drafts: CountResult): void {
  mocks.proposalCount.mockImplementation((filter: Record<string, unknown>) =>
    filter.awaitingMyDecision ? awaiting : drafts,
  );
}

const attention = (): HTMLElement | null => screen.queryByRole("status", { name: "Needs your attention" });

const lastTemplatePayload = (): Record<string, unknown> => mocks.templateSearch.mock.calls.at(-1)![0];
const lastProposalPayload = (): Record<string, unknown> => mocks.proposalSearch.mock.calls.at(-1)![0];

function choose(label: string, option: string): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: label }));
  fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: option }));
}

beforeEach(() => {
  mocks.navigate.mockReset();
  mocks.access = { canWrite: true };
  mocks.createProposal.mockReset();
  mocks.templateSearch.mockReset().mockReturnValue({
    data: { templates: TEMPLATES, total: 2, limit: 20, offset: 0 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
  mocks.proposalSearch.mockReset().mockReturnValue({
    data: { proposals: PROPOSALS, total: 1, limit: 20, offset: 0 },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  });
  mocks.proposalCount.mockReset();
  counts(count(0), count(0));
});

describe("StandardChangesTab — catalogue", () => {
  it("lists the templates with number, name, version, risk, impact, usage, state and the review chip", () => {
    renderTab();
    const table = screen.getByRole("table", { name: "Standard change templates" });
    const [first, second] = within(table).getAllByRole("row").slice(1);
    expect(first).toHaveTextContent("STD0000001");
    expect(first).toHaveTextContent("Rotate a TLS certificate");
    expect(first).toHaveTextContent("v3");
    expect(first).toHaveTextContent("Low");
    expect(first).toHaveTextContent("Medium");
    expect(first).toHaveTextContent("12 total, 2 open");
    expect(first).toHaveTextContent("1 rolled back");
    expect(within(first!).getByText("Review needed")).toBeInTheDocument();
    expect(within(first!).getByText("Active")).toBeInTheDocument();
    expect(within(second!).queryByText("Review needed")).not.toBeInTheDocument();
    expect(within(second!).getByText("Suspended")).toBeInTheDocument();
  });

  it("asks for active and suspended templates by default (the server's default), and the chosen state otherwise", () => {
    renderTab();
    expect(lastTemplatePayload()).toEqual({ pagination: { offset: 0, limit: 20 } });
    choose("State", "Retired");
    expect(lastTemplatePayload()).toEqual({ states: ["retired"], pagination: { offset: 0, limit: 20 } });
    choose("State", "All");
    expect(lastTemplatePayload()).toMatchObject({ states: ["active", "suspended", "retired"] });
  });

  it("searches on the server once typing pauses", async () => {
    renderTab();
    fireEvent.change(screen.getByRole("textbox", { name: /search templates/i }), { target: { value: " tls " } });
    await waitFor(() => expect(lastTemplatePayload()).toMatchObject({ query: "tls" }));
  });

  it("opens a template with Back returning to the catalogue inside Change requests", () => {
    renderTab();
    fireEvent.click(screen.getByText("Restart a gateway node"));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-changes/tpl-2", {
      state: { from: CATALOGUE },
    });
  });

  it("has Templates | Proposals beside the search, on Templates, and nothing else in a strip of its own", () => {
    renderTab();
    const show = screen.getByRole("group", { name: "Templates or proposals" });
    expect(within(show).getAllByRole("button").map((b) => b.textContent)).toEqual(["Templates", "Proposals"]);
    expect(within(show).getByRole("button", { name: "Templates" })).toHaveAttribute("aria-pressed", "true");
    expect(within(show).getByRole("button", { name: "Proposals" })).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Standard change proposals" })).not.toBeInTheDocument();
    // The proposals' scope select keeps "Show"; the toggle has a name of its own.
    expect(screen.queryByRole("group", { name: "Show" })).not.toBeInTheDocument();
  });

  it("opens the old ?view=catalogue link on the catalogue", () => {
    renderTab("/operations/change-requests?view=catalogue");
    expect(screen.getByRole("table", { name: "Standard change templates" })).toBeInTheDocument();
  });

  it("shows an empty state", () => {
    mocks.templateSearch.mockReturnValue({ data: { templates: [], total: 0 }, isLoading: false, isError: false, refetch: vi.fn() });
    renderTab();
    expect(screen.getByText("No standard change templates found.")).toBeInTheDocument();
  });
});

describe("StandardChangesTab — proposals", () => {
  it("is the secondary view, reached from Proposals beside the search and linkable", () => {
    renderTab();
    fireEvent.click(screen.getByRole("button", { name: "Proposals" }));
    expect(currentUrl()).toBe(PROPOSALS_VIEW);
    expect(screen.getByRole("table", { name: "Standard change proposals" })).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Standard change templates" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Proposals" })).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(screen.getByRole("button", { name: "Templates" }));
    expect(currentUrl()).toBe(CATALOGUE);
    expect(screen.getByRole("table", { name: "Standard change templates" })).toBeInTheDocument();
  });

  it("lists all proposals first", () => {
    renderTab(PROPOSALS_VIEW);
    expect(screen.queryByRole("table", { name: "Standard change templates" })).not.toBeInTheDocument();
    expect(lastProposalPayload()).toEqual({ pagination: { offset: 0, limit: 20 } });
    expect(screen.getByRole("combobox", { name: "Show" })).toHaveTextContent("All proposals");
    const row = within(screen.getByRole("table", { name: "Standard change proposals" })).getAllByRole("row")[1]!;
    expect(row).toHaveTextContent("SCP0000001");
    expect(row).toHaveTextContent("New template");
    expect(row).toHaveTextContent("CAB review");
    expect(row).toHaveTextContent("Jane Doe");
  });

  it("opens the old ?view=proposals link on all proposals", () => {
    renderTab("/operations/change-requests?view=proposals");
    expect(screen.getByRole("table", { name: "Standard change proposals" })).toBeInTheDocument();
    expect(lastProposalPayload()).toEqual({ pagination: { offset: 0, limit: 20 } });
  });

  it("offers All proposals first, then the two filters", () => {
    renderTab(PROPOSALS_VIEW);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Show" }));
    expect(within(screen.getByRole("listbox")).getAllByRole("option").map((o) => o.textContent)).toEqual([
      "All proposals",
      "Proposed by me",
      "Awaiting my decision",
    ]);
  });

  it("lists the proposals awaiting my decision, or mine, in a state, keeping the filters in the URL", () => {
    renderTab(PROPOSALS_VIEW);
    choose("Show", "Awaiting my decision");
    expect(lastProposalPayload()).toEqual({ awaitingMyDecision: true, pagination: { offset: 0, limit: 20 } });
    expect(currentUrl()).toBe(`${PROPOSALS_VIEW}&scope=awaiting`);
    choose("Show", "Proposed by me");
    choose("State", "Rejected");
    expect(lastProposalPayload()).toEqual({ proposedByMe: true, states: ["rejected"], pagination: { offset: 0, limit: 20 } });
    expect(currentUrl()).toBe(`${PROPOSALS_VIEW}&scope=mine&state=rejected`);
    choose("Show", "All proposals");
    choose("State", "Any state");
    expect(lastProposalPayload()).toEqual({ pagination: { offset: 0, limit: 20 } });
    expect(currentUrl()).toBe(PROPOSALS_VIEW);
  });

  it.each([
    ["&scope=awaiting", { awaitingMyDecision: true }, "Awaiting my decision", "Any state"],
    ["&scope=mine&state=draft", { proposedByMe: true, states: ["draft"] }, "Proposed by me", "Draft"],
    ["&scope=all&state=cab_review", { states: ["cab_review"] }, "All proposals", "CAB review"],
    ["&scope=nonsense&state=open", {}, "All proposals", "Any state"],
  ])("reads its filters from a link (%s)", (query, payload, scopeLabel, stateLabel) => {
    renderTab(`${PROPOSALS_VIEW}${query}`);
    expect(lastProposalPayload()).toEqual({ ...payload, pagination: { offset: 0, limit: 20 } });
    expect(screen.getByRole("combobox", { name: "Show" })).toHaveTextContent(scopeLabel);
    expect(screen.getByRole("combobox", { name: "State" })).toHaveTextContent(stateLabel);
  });

  it("drops its filters on the way back to the templates", () => {
    renderTab(`${PROPOSALS_VIEW}&scope=mine&state=rejected`);
    fireEvent.click(screen.getByRole("button", { name: "Templates" }));
    expect(currentUrl()).toBe(CATALOGUE);
  });

  it("keeps the change request list's filters in the URL, unread, through every switch and filter", () => {
    renderTab(`${CATALOGUE}&crQ=gateway&crStates=new`);
    fireEvent.click(screen.getByRole("button", { name: "Proposals" }));
    expect(currentUrl()).toBe(`${PROPOSALS_VIEW}&crQ=gateway&crStates=new`);
    choose("Show", "Awaiting my decision");
    expect(currentUrl()).toBe(`${PROPOSALS_VIEW}&scope=awaiting&crQ=gateway&crStates=new`);
    expect(lastProposalPayload()).toEqual({ awaitingMyDecision: true, pagination: { offset: 0, limit: 20 } });
    fireEvent.click(screen.getByRole("button", { name: "Templates" }));
    expect(currentUrl()).toBe(`${CATALOGUE}&crQ=gateway&crStates=new`);
  });

  it("opens a proposal with Back returning to the proposals view, filters and all", () => {
    renderTab(`${PROPOSALS_VIEW}&scope=awaiting`);
    fireEvent.click(screen.getByText("SCP0000001"));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-1", {
      state: { from: `${PROPOSALS_VIEW}&scope=awaiting` },
    });
  });
});

describe("StandardChangesTab — what needs your attention", () => {
  it("asks for the proposals awaiting my CAB decision and my own drafts", () => {
    renderTab();
    expect(mocks.proposalCount).toHaveBeenCalledWith({ awaitingMyDecision: true });
    expect(mocks.proposalCount).toHaveBeenCalledWith({ proposedByMe: true, states: ["draft"] });
  });

  it("says nothing when nothing applies to me", () => {
    counts(count(0), count(0));
    renderTab();
    expect(attention()).not.toBeInTheDocument();
    expect(screen.queryByText(/your CAB decision|of yours/)).not.toBeInTheDocument();
  });

  it("says both, each a link to the proposals filtered to it", () => {
    counts(count(2), count(1));
    renderTab();
    const line = attention()!;
    expect(line).toHaveTextContent("2 proposals await your CAB decision · 1 draft of yours");
    expect(within(line).getByRole("link", { name: "2 proposals await your CAB decision" })).toHaveAttribute(
      "href",
      `${PROPOSALS_VIEW}&scope=awaiting`,
    );
    expect(within(line).getByRole("link", { name: "1 draft of yours" })).toHaveAttribute(
      "href",
      `${PROPOSALS_VIEW}&scope=mine&state=draft`,
    );
  });

  it("keeps the change request list's filters in its links", () => {
    counts(count(2), count(1));
    renderTab(`${CATALOGUE}&crQ=gateway`);
    expect(screen.getByRole("link", { name: "2 proposals await your CAB decision" })).toHaveAttribute(
      "href",
      `${PROPOSALS_VIEW}&scope=awaiting&crQ=gateway`,
    );
    expect(screen.getByRole("link", { name: "1 draft of yours" })).toHaveAttribute(
      "href",
      `${PROPOSALS_VIEW}&scope=mine&state=draft&crQ=gateway`,
    );
  });

  it("says only what applies, in the singular or plural", () => {
    counts(count(1), count(0));
    renderTab();
    expect(attention()).toHaveTextContent(/^1 proposal awaits your CAB decision$/);
  });

  it("says only my drafts when nothing awaits my decision", () => {
    counts(count(0), count(3));
    renderTab();
    expect(attention()).toHaveTextContent(/^3 drafts of yours$/);
    expect(within(attention()!).getAllByRole("link")).toHaveLength(1);
  });

  it("opens the filtered proposals from a link", () => {
    counts(count(2), count(1));
    renderTab();
    fireEvent.click(screen.getByRole("link", { name: "2 proposals await your CAB decision" }));
    expect(currentUrl()).toBe(`${PROPOSALS_VIEW}&scope=awaiting`);
    expect(lastProposalPayload()).toEqual({ awaitingMyDecision: true, pagination: { offset: 0, limit: 20 } });
    expect(screen.getByRole("button", { name: "Proposals" })).toHaveAttribute("aria-pressed", "true");
  });

  it.each([
    ["the first count", PENDING, count(1)],
    ["the second count", count(2), PENDING],
    ["both counts", PENDING, PENDING],
  ])("shows nothing while %s is loading, so it does not appear piece by piece", (_, awaiting, drafts) => {
    counts(awaiting, drafts);
    renderTab();
    expect(attention()).not.toBeInTheDocument();
  });

  it("leaves out a count that failed, without a word about it", () => {
    counts(FAILED, count(1));
    renderTab();
    expect(attention()).toHaveTextContent(/^1 draft of yours$/);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("says nothing at all when both counts fail", () => {
    counts(FAILED, FAILED);
    renderTab();
    expect(attention()).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    // The catalogue itself is unaffected.
    expect(screen.getByRole("table", { name: "Standard change templates" })).toBeInTheDocument();
  });

  it("is above the catalogue only, not on the proposals view", () => {
    counts(count(2), count(1));
    renderTab(PROPOSALS_VIEW);
    expect(attention()).not.toBeInTheDocument();
  });
});

describe("StandardChangesTab — Propose a standard change", () => {
  it("starts an empty new proposal and opens it", () => {
    renderTab();
    fireEvent.click(screen.getByRole("button", { name: /propose a standard change/i }));
    expect(mocks.createProposal).toHaveBeenCalledWith({ kind: "new" }, expect.anything());
    mocks.createProposal.mock.calls[0]![1].onSuccess({ id: "scp-9" });
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-9", {
      state: { from: CATALOGUE },
    });
  });

  it("is offered from the proposals view too, with Back returning there", () => {
    renderTab(PROPOSALS_VIEW);
    fireEvent.click(screen.getByRole("button", { name: /propose a standard change/i }));
    mocks.createProposal.mock.calls[0]![1].onSuccess({ id: "scp-9" });
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-9", {
      state: { from: PROPOSALS_VIEW },
    });
  });

  it("is not offered without write access", () => {
    mocks.access = { canWrite: false };
    renderTab();
    expect(screen.queryByRole("button", { name: /propose a standard change/i })).not.toBeInTheDocument();
  });
});
