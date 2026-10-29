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
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
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
  createProposal: vi.fn(),
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
  useSearchStandardChangeTemplates: (payload: unknown) => mocks.templateSearch(payload),
  useSearchStandardChangeProposals: (payload: unknown) => mocks.proposalSearch(payload),
  useCreateStandardChangeProposal: () => ({ mutate: mocks.createProposal, isPending: false, isError: false, reset: vi.fn() }),
}));

import StandardChangesTab from "@features/csm-operations/components/StandardChangesTab";

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

const CATALOGUE = "/operations/change-requests?view=catalogue";
const PROPOSALS_VIEW = "/operations/change-requests?view=proposals";

/** The sub-view rendered as `ChangeRequestsArea` renders it: from the URL it is mounted at. */
function renderTab(entry = CATALOGUE): void {
  const view = new URLSearchParams(entry.split("?")[1] ?? "").get("view") === "proposals" ? "proposals" : "catalogue";
  render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route
          path="/operations/change-requests"
          element={
            <>
              <StandardChangesTab view={view} />
              <Probe />
            </>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

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

  it("introduces standard changes as pre-authorised, templated change requests", () => {
    renderTab();
    expect(screen.getByText(/standard changes are pre-authorised, templated change requests/i)).toBeInTheDocument();
    expect(screen.getByText(/the CAB approves a template once/i)).toBeInTheDocument();
    expect(screen.getByText(/any completed change request can be proposed as a template/i)).toBeInTheDocument();
    // The proposals live in their own sub-view, not in a second strip in this one.
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Standard change proposals" })).not.toBeInTheDocument();
  });

  it("shows an empty state", () => {
    mocks.templateSearch.mockReturnValue({ data: { templates: [], total: 0 }, isLoading: false, isError: false, refetch: vi.fn() });
    renderTab();
    expect(screen.getByText("No standard change templates found.")).toBeInTheDocument();
  });
});

describe("StandardChangesTab — proposals", () => {
  it("lists my proposals first", () => {
    renderTab(PROPOSALS_VIEW);
    expect(screen.queryByRole("table", { name: "Standard change templates" })).not.toBeInTheDocument();
    expect(screen.getByText(/standard changes are pre-authorised, templated change requests/i)).toBeInTheDocument();
    expect(lastProposalPayload()).toEqual({ proposedByMe: true, pagination: { offset: 0, limit: 20 } });
    const row = within(screen.getByRole("table", { name: "Standard change proposals" })).getAllByRole("row")[1]!;
    expect(row).toHaveTextContent("SCP0000001");
    expect(row).toHaveTextContent("New template");
    expect(row).toHaveTextContent("CAB review");
    expect(row).toHaveTextContent("Jane Doe");
  });

  it("lists the proposals awaiting my decision, or all, in a state", () => {
    renderTab(PROPOSALS_VIEW);
    choose("Show", "Awaiting my decision");
    expect(lastProposalPayload()).toEqual({ awaitingMyDecision: true, pagination: { offset: 0, limit: 20 } });
    choose("Show", "All proposals");
    choose("State", "Rejected");
    expect(lastProposalPayload()).toEqual({ states: ["rejected"], pagination: { offset: 0, limit: 20 } });
  });

  it("opens a proposal with Back returning to the proposals view", () => {
    renderTab(PROPOSALS_VIEW);
    fireEvent.click(screen.getByText("SCP0000001"));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-1", {
      state: { from: PROPOSALS_VIEW },
    });
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
