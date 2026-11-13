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
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeStandardChangeSuggestion, BeStandardChangeSuggestionsResponse } from "@api/backend/types";

vi.hoisted(() => {
  window.config = { ...window.config, CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090" };
});

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  access: { canWrite: true },
  suggestions: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  dismiss: vi.fn(),
  restore: vi.fn(),
  showError: vi.fn(),
  customerSearch: vi.fn<(query: string, enabled: boolean) => unknown>(() => ({ data: [], isLoading: false, isError: false })),
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({ useErrorBanner: () => ({ showError: mocks.showError }) }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
  useStandardChangeSuggestions: (q: unknown) => mocks.suggestions(q),
  useCreateStandardChangeProposal: () => ({ mutate: mocks.create, isPending: false }),
  useUpdateStandardChangeProposal: () => ({ mutate: mocks.update, isPending: false }),
  useDismissStandardChangeSuggestion: () => ({ mutate: mocks.dismiss, isPending: false }),
  useRestoreStandardChangeSuggestion: () => ({ mutate: mocks.restore, isPending: false }),
  useSearchStandardChangeCustomersForSelect: (query: string, enabled: boolean) => mocks.customerSearch(query, enabled),
}));

import StandardChangeSuggestions from "@features/csm-operations/components/StandardChangeSuggestions";
import { SUGGESTION_SKIPPED_ERROR, SUGGESTION_SKIPPED_TOO_LARGE } from "@features/csm-operations/utils/standardChangeSuggestions";

function suggestion(over: Partial<BeStandardChangeSuggestion> = {}): BeStandardChangeSuggestion {
  return {
    key: "deploy capps pekin prod",
    customer: { id: "acct-1", name: "Acme" },
    subject: "Deploy CApps to Pekin prod",
    memberCount: 5,
    closedCount: 5,
    rollbackCount: 0,
    successRate: 1,
    incidents: [],
    risk: { low: 5, moderate: 0, high: 0, unknown: 0 },
    impact: { low: 4, moderate: 0, high: 0, unknown: 1 },
    firstCompletedOn: "2026-09-01T00:00:00Z",
    lastCompletedOn: "2026-10-30T00:00:00Z",
    eligible: true,
    unmet: [],
    proposeFrom: { id: "cr-1", name: "CHG0000001" },
    evidence: [
      { id: "cr-2", name: "CHG0000002" },
      { id: "cr-3", name: "CHG0000003" },
    ],
    openProposals: [],
    members: [
      { id: "cr-1", number: "CHG0000001", subject: "Deploy CApps to Pekin prod", state: "closed", completedOn: "2026-10-30T00:00:00Z", hasDeviation: false, joinedBy: "subject" },
      { id: "cr-9", number: "CHG0000009", subject: "Promote to DR", state: "closed", completedOn: "2026-10-20T00:00:00Z", hasDeviation: false, joinedBy: "clone" },
    ],
    dismissal: null,
    ...over,
  };
}

function serve(
  list: BeStandardChangeSuggestion[],
  computedOn: string | null = "2026-11-06T14:00:00Z",
  skippedCustomers: BeStandardChangeSuggestionsResponse["skippedCustomers"] = [],
): void {
  mocks.suggestions.mockReturnValue({
    data: {
      suggestions: list,
      criteria: { windowMonths: 12, minClosed: 5, minSuccessPercent: 90, planSimilarityPercent: 50 },
      computedOn,
      scanned: { customers: 2, changeRequests: 20, clusters: 3, eligible: list.length, dismissed: 0 },
      skippedCustomers,
    },
    isLoading: false,
    isError: false,
    isFetching: false,
    refetch: vi.fn(),
  });
}

function renderIt(): void {
  render(
    <MemoryRouter initialEntries={["/operations/change-requests?view=standard&show=suggested"]}>
      <StandardChangeSuggestions leading={null} trailing={null} />
    </MemoryRouter>,
  );
}

function row(subject: string): HTMLElement {
  return screen.getByText(subject).closest("tr") as HTMLElement;
}

describe("StandardChangeSuggestions", () => {
  beforeEach(() => {
    Object.values(mocks).forEach((m) => typeof m === "function" && "mockReset" in m && (m as ReturnType<typeof vi.fn>).mockReset());
    mocks.customerSearch.mockReturnValue({ data: [], isLoading: false, isError: false });
    mocks.access = { canWrite: true };
  });

  it("lists each suggestion with its counts and the thresholds, and relates none to a template", () => {
    serve([suggestion(), suggestion({ key: "k2", subject: "Rotate keys" })]);
    renderIt();
    expect(screen.getByRole("status")).toHaveTextContent("2 suggestions from 20 change requests completed in the last 12 months");
    expect(within(row("Deploy CApps to Pekin prod")).getByText("5 closed")).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "Templates" })).not.toBeInTheDocument();
    expect(screen.queryByText(/Covered by|Close to|^New$/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^(Revise|Open) / })).not.toBeInTheDocument();
    expect(within(row("Rotate keys")).getByRole("button", { name: "Propose" })).toBeEnabled();
    expect(mocks.suggestions).toHaveBeenLastCalledWith({ accountId: undefined, includeIneligible: false, includeDismissed: false });
  });

  it("says when the snapshot was computed", () => {
    serve([suggestion()]);
    renderIt();
    expect(screen.getByRole("status")).toHaveTextContent(/^Updated Nov 6, \d\d:\d\d (AM|PM)\. 1 suggestion from 20/);
  });

  it("says suggestions are computed once a day when none have been yet", () => {
    serve([], null);
    renderIt();
    expect(screen.getByRole("status")).toHaveTextContent("Suggestions are computed once a day from completed change requests. None have been computed yet.");
    expect(within(screen.getByRole("table", { name: "Suggested standard changes" })).getByText(/None have been computed yet/)).toBeInTheDocument();
    expect(screen.queryByText(/No change is done often enough/)).not.toBeInTheDocument();
  });

  it("says which customers the latest refresh could not compute, and why", () => {
    serve([suggestion()], "2026-11-06T14:00:00Z", [
      { customer: { id: "acct-9", name: "BigCo" }, reason: SUGGESTION_SKIPPED_TOO_LARGE },
      { customer: { id: "acct-8", name: "OddCo" }, reason: SUGGESTION_SKIPPED_ERROR },
    ]);
    renderIt();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "BigCo's suggestions were not computed in the latest refresh: it has too many change requests to compute in time. " +
        "OddCo's suggestions were not computed in the latest refresh: an error stopped it, and the next refresh tries again.",
    );
  });

  it("no skipped notice when every customer was computed", () => {
    serve([suggestion()]);
    renderIt();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("marks a suggestion built from the newest of a large subject, and says so in the details", () => {
    serve([suggestion({ sample: { compared: 500, total: 3000 } })]);
    renderIt();
    expect(within(row("Deploy CApps to Pekin prod")).getByText("Newest 500 of 3,000")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show the details of Deploy CApps to Pekin prod" }));
    expect(screen.getByText(/Based on the newest 500 of 3,000 change requests with this subject/)).toBeInTheDocument();
  });

  it("tells apart two suggestions of one customer that share a subject", () => {
    const subject = "Update the firewall rules for partner integrations";
    const sg = suggestion({
      key: "update firewall rules partner integrations",
      subjectKey: "update firewall rules partner integrations",
      subject,
      members: [{ id: "cr-sg", number: "CHG0000101", subject, state: "closed", completedOn: "2026-10-30T00:00:00Z", hasDeviation: false, joinedBy: "subject" }],
    });
    const waf = suggestion({
      key: "update firewall rules partner integrations#0a1b2c3d4e5f",
      subjectKey: "update firewall rules partner integrations",
      subject,
      members: [{ id: "cr-waf", number: "CHG0000201", subject, state: "closed", completedOn: "2026-10-29T00:00:00Z", hasDeviation: false, joinedBy: "subject" }],
    });
    serve([sg, waf]);
    const { rerender } = render(
      <MemoryRouter initialEntries={["/operations/change-requests?view=standard&show=suggested"]}>
        <StandardChangeSuggestions leading={null} trailing={null} />
      </MemoryRouter>,
    );
    const table = screen.getByRole("table", { name: "Suggested standard changes" });
    expect(within(table).getAllByText(subject)).toHaveLength(2);

    // Opening one opens only that one.
    const [first, second] = screen.getAllByRole("button", { name: `Show the details of ${subject}` });
    fireEvent.click(first);
    expect(first).toHaveAttribute("aria-expanded", "true");
    expect(second).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("link", { name: "CHG0000101" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "CHG0000201" })).not.toBeInTheDocument();

    // Dismissing the second sends its own key.
    fireEvent.click(screen.getAllByRole("button", { name: "Dismiss" })[1]);
    const dialog = screen.getByRole("dialog", { name: "Dismiss this suggestion" });
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Why is this not a standard change/ }), { target: { value: "WAF is one-off" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Dismiss" }));
    expect(mocks.dismiss.mock.calls[0][0]).toEqual({ accountId: "acct-1", key: "update firewall rules partner integrations#0a1b2c3d4e5f", reason: "WAF is one-off" });

    // A narrower list leaves no row behind.
    serve([waf]);
    rerender(
      <MemoryRouter initialEntries={["/operations/change-requests?view=standard&show=suggested"]}>
        <StandardChangeSuggestions leading={null} trailing={null} />
      </MemoryRouter>,
    );
    expect(within(table).getAllByText(subject)).toHaveLength(1);
  });

  it("asks for near misses and dismissed ones when ticked", () => {
    serve([]);
    renderIt();
    expect(screen.getByText(/No change is done often enough/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox", { name: "Show near misses" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Show dismissed" }));
    expect(mocks.suggestions).toHaveBeenLastCalledWith({ accountId: undefined, includeIneligible: true, includeDismissed: true });
  });

  it("Propose starts a new proposal from the newest clean change request, cites the others, and opens it", () => {
    serve([suggestion()]);
    mocks.create.mockImplementation((_body, opts) => opts.onSuccess({ id: "scp-1" }));
    mocks.update.mockImplementation((_vars, opts) => opts.onSettled());
    renderIt();
    fireEvent.click(within(row("Deploy CApps to Pekin prod")).getByRole("button", { name: "Propose" }));
    expect(mocks.create.mock.calls[0][0]).toEqual({ kind: "new", sourceChangeRequestId: "cr-1" });
    expect(mocks.update.mock.calls[0][0]).toEqual({ id: "scp-1", patch: { evidenceChangeRequestIds: ["cr-1", "cr-2", "cr-3"] } });
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-1", {
      state: { from: "/operations/change-requests?view=standard&show=suggested" },
    });
  });

  it("a proposal whose evidence cannot be cited still opens, saying why", () => {
    serve([suggestion()]);
    mocks.create.mockImplementation((_body, opts) => opts.onSuccess({ id: "scp-2" }));
    mocks.update.mockImplementation((_vars, opts) => {
      opts.onError(new Error("CHG0000002 is another customer's"));
      opts.onSettled();
    });
    renderIt();
    fireEvent.click(screen.getByRole("button", { name: "Propose" }));
    expect(mocks.showError).toHaveBeenCalledWith("CHG0000002 is another customer's", expect.any(Error));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-2", expect.anything());
  });

  it("shows Propose disabled, with why, for a near miss and for one already being proposed", () => {
    serve([
      suggestion({ eligible: false, unmet: ["1 incident linked to a member"] }),
      suggestion({ key: "k2", subject: "Rotate keys", openProposals: [{ id: "p1", name: "SCP0000007" }] }),
    ]);
    renderIt();
    expect(within(row("Deploy CApps to Pekin prod")).getByRole("button", { name: /Propose: Not yet a candidate: 1 incident/ })).toBeDisabled();
    expect(within(row("Rotate keys")).getByRole("button", { name: "Propose: Already being proposed in SCP0000007." })).toBeDisabled();
  });

  it("Dismiss needs a reason and sends the customer and key", () => {
    serve([suggestion()]);
    renderIt();
    fireEvent.click(within(row("Deploy CApps to Pekin prod")).getByRole("button", { name: "Dismiss" }));
    const dialog = screen.getByRole("dialog", { name: "Dismiss this suggestion" });
    const confirm = within(dialog).getByRole("button", { name: "Dismiss" });
    expect(confirm).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /Why is this not a standard change/ }), { target: { value: " one-off migration " } });
    fireEvent.click(confirm);
    expect(mocks.dismiss.mock.calls[0][0]).toEqual({ accountId: "acct-1", key: "deploy capps pekin prod", reason: "one-off migration" });
  });

  it("a dismissed one is restored, and a reader gets no actions", () => {
    serve([suggestion({ dismissal: { id: "d1", reason: "one-off", dismissedOn: "2026-10-01T00:00:00Z", dismissedBy: { id: "u", name: "Jane" } } })]);
    renderIt();
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    expect(mocks.restore.mock.calls[0][0]).toBe("d1");
    expect(screen.queryByRole("button", { name: "Propose" })).not.toBeInTheDocument();
  });

  it("shows the change requests, and which joined as a clone, in the details", () => {
    serve([suggestion()]);
    mocks.access = { canWrite: false };
    renderIt();
    expect(screen.queryByRole("button", { name: "Dismiss" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show the details of Deploy CApps to Pekin prod" }));
    expect(screen.getByRole("link", { name: "CHG0000009" })).toHaveAttribute("href", "/operations/change-requests/cr-9");
    expect(screen.getByText(/Promote to DR · closed .* · a clone/)).toBeInTheDocument();
    expect(screen.getByText("4 low, 1 unknown")).toBeInTheDocument();
  });

  describe("the CAB's pushback and the deviations", () => {
    const pushback = {
      cabRejectionCount: 2,
      cabRejections: [
        {
          changeRequest: { id: "cr-20", number: "CHG0000020", subject: "Deploy CApps to Pekin prod", state: "canceled" },
          member: false,
          rejectedOn: "2026-10-25T00:00:00Z",
          rejectedBy: "Alice CAB",
          comment: "Not during the freeze",
        },
        {
          changeRequest: { id: "cr-1", number: "CHG0000001", subject: "Deploy CApps to Pekin prod", state: "closed" },
          member: true,
          rejectedOn: "2026-10-01T00:00:00Z",
          rejectedBy: "Bob CAB",
          comment: "",
        },
      ],
      cabCommentCount: 3,
      cabComments: [
        {
          changeRequest: { id: "cr-1", number: "CHG0000001", subject: "Deploy CApps to Pekin prod", state: "closed" },
          member: true,
          approverName: "Carol CAB",
          state: "APPROVED",
          comment: "Add the verification steps",
          decidedOn: "2026-10-10T00:00:00Z",
        },
      ],
      deviationCount: 1,
    };

    it("shows the counts as chips on the row: rejections in the error colour, comments and deviations as warnings", () => {
      serve([suggestion(pushback), suggestion({ key: "k2", subject: "Rotate keys" })]);
      renderIt();
      const deploy = row("Deploy CApps to Pekin prod");
      const rejected = within(deploy).getByText("2 CAB rejections").closest(".MuiChip-root");
      expect(rejected).toHaveClass("MuiChip-colorError");
      expect(within(deploy).getByText("3 CAB comments").closest(".MuiChip-root")).toHaveClass("MuiChip-colorWarning");
      expect(within(deploy).getByText("1 deviated from plan").closest(".MuiChip-root")).toHaveClass("MuiChip-colorWarning");
      const rotate = row("Rotate keys");
      expect(within(rotate).queryByText(/CAB rejection|CAB comment|deviated from plan/)).not.toBeInTheDocument();
      // Shown only: the suggestion can still be proposed.
      expect(within(deploy).getByRole("button", { name: "Propose" })).toBeEnabled();
    });

    it("counts the listed members' deviations for a snapshot that sends no count", () => {
      const members = suggestion().members.map((m, i) => ({ ...m, hasDeviation: i === 0 }));
      serve([suggestion({ members })]);
      renderIt();
      expect(within(row("Deploy CApps to Pekin prod")).getByText("1 deviated from plan")).toBeInTheDocument();
    });

    it("lists the rejections and comments in the details, each linked, and marks a member the CAB rejected", () => {
      const members = suggestion().members.map((m, i) => ({ ...m, cabRejected: i === 0 }));
      serve([suggestion({ ...pushback, members })]);
      renderIt();
      fireEvent.click(screen.getByRole("button", { name: "Show the details of Deploy CApps to Pekin prod" }));
      const rejections = screen.getByRole("list", { name: "CAB rejections" });
      const [look, own] = within(rejections).getAllByRole("listitem");
      expect(look).toHaveTextContent("by Alice CAB");
      expect(look).toHaveTextContent("a look-alike, not a member");
      expect(look).toHaveTextContent('"Not during the freeze"');
      expect(within(look).getByRole("link", { name: "CHG0000020" })).toHaveAttribute("href", "/operations/change-requests/cr-20");
      expect(own).not.toHaveTextContent("look-alike");
      const comments = screen.getByRole("list", { name: "CAB comments" });
      expect(comments).toHaveTextContent("Carol CAB approved");
      expect(comments).toHaveTextContent('"Add the verification steps"');
      expect(comments.parentElement).toHaveTextContent("The newest 1 are listed.");
      // The member list (CHG0000009 is in no CAB list).
      const membersList = screen.getByRole("link", { name: "CHG0000009" }).closest("ul") as HTMLElement;
      const [first, clone] = within(membersList).getAllByRole("listitem");
      expect(first).toHaveTextContent("· rejected by the CAB");
      expect(clone).not.toHaveTextContent("rejected by the CAB");
    });

    it("shows no CAB pushback block for a suggestion (or an older snapshot) without any", () => {
      serve([suggestion()]);
      renderIt();
      fireEvent.click(screen.getByRole("button", { name: "Show the details of Deploy CApps to Pekin prod" }));
      expect(screen.queryByText("CAB pushback")).not.toBeInTheDocument();
      expect(screen.queryByText(/rejected by the CAB/)).not.toBeInTheDocument();
    });
  });
});
