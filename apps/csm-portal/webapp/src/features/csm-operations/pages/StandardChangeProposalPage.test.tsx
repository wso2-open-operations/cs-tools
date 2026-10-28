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

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeStandardChangeProposal } from "@api/backend/types";

vi.hoisted(() => {
  window.config = { ...window.config, CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090" };
});

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  access: { canWrite: true },
  proposal: undefined as unknown,
  update: vi.fn(),
  updateAsync: vi.fn(),
  submitAsync: vi.fn(),
  withdraw: vi.fn(),
  decide: vi.fn(),
  create: vi.fn(),
  calls: [] as string[],
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
  useGetStandardChangeProposal: () => ({ data: mocks.proposal, isLoading: false, isError: false, refetch: vi.fn() }),
  useUpdateStandardChangeProposal: () => ({ mutate: mocks.update, mutateAsync: mocks.updateAsync, isPending: false }),
  useSubmitStandardChangeProposal: () => ({ mutateAsync: mocks.submitAsync, isPending: false }),
  useWithdrawStandardChangeProposal: () => ({ mutate: mocks.withdraw, isPending: false }),
  useDecideStandardChangeProposal: () => ({ mutate: mocks.decide, isPending: false, isError: false, reset: vi.fn() }),
  useCreateStandardChangeProposal: () => ({ mutate: mocks.create, isPending: false }),
  useSearchClosedChangeRequestsForSelect: () => ({ data: [], isFetching: false, isError: false }),
}));
// The Lexical editor does not run under jsdom: a textarea stands in, showing the
// value the field mounted with (as the real editor does) and reporting edits.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange, disabled }: { value: string; onChange: (v: string) => void; disabled?: boolean }) => (
    <textarea defaultValue={value} disabled={disabled} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// The evidence picker is a plain input reporting the picked change request.
const PICKED: Record<string, { id: string; number: string; subject: string }> = {
  "chg-7": { id: "chg-7", number: "CHG0000007", subject: "Rotated the certificate on gw-7" },
};
vi.mock("@components/AsyncEntitySelect", () => ({
  default: ({ label, onChange }: { label: string; onChange: (next: string, item?: unknown) => void }) => (
    <input aria-label={label} onChange={(e) => onChange(e.target.value, PICKED[e.target.value])} />
  ),
}));

import StandardChangeProposalPage from "@features/csm-operations/pages/StandardChangeProposalPage";

const DRAFT: BeStandardChangeProposal = {
  id: "scp-1",
  number: "SCP0000001",
  kind: "new",
  state: "draft",
  name: "Rotate a TLS certificate",
  rationale: "<p>Done every quarter without incident.</p>",
  template: null,
  baseVersion: null,
  sourceChangeRequest: { id: "chg-1", name: "CHG0000001" },
  subject: "Rotate the TLS certificate on {{host}}",
  description: "<p>Rotate the certificate on {{host}}.</p>",
  justification: null,
  implementationPlan: "<p>Steps.</p>",
  riskImpactAnalysis: null,
  backoutPlan: "<p>Restore.</p>",
  testPlan: "<p>Check.</p>",
  category: "software",
  impact: "low",
  risk: "low",
  rollbackDurationText: "15 mins",
  isPlanningVisibleToCustomers: false,
  customerApprovalRequired: false,
  customerReviewRequired: false,
  parameters: [{ key: "host", label: "Host", type: "text", required: true, helpText: null }],
  evidence: [{ id: "chg-1", number: "CHG0000001", subject: "Rotated the certificate on gw-1", state: "closed", closedOn: "2026-09-01T00:00:00Z" }],
  approvers: [],
  proposedBy: { id: "u-1", name: "Jane Doe", email: "jane@example.com" },
  createdOn: "2026-10-01T00:00:00Z",
  updatedOn: "2026-10-01T00:00:00Z",
  submittedOn: null,
  decidedOn: null,
  decidedBy: null,
  decisionComment: null,
  resultingVersion: null,
  canEdit: true,
  canSubmit: true,
  canWithdraw: true,
};

function renderPage(): void {
  render(
    <MemoryRouter initialEntries={["/operations/standard-change-proposals/scp-1"]}>
      <Routes>
        <Route path="/operations/standard-change-proposals/:id" element={<StandardChangeProposalPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

const richField = (name: RegExp): HTMLElement => within(screen.getByRole("group", { name })).getByRole("textbox");
const lastPatch = (): Record<string, unknown> => mocks.update.mock.calls.at(-1)![0].patch;
const saveButton = (): HTMLElement => screen.getByRole("button", { name: /^save$/i });
const submitButton = (): HTMLElement => screen.getByRole("button", { name: /submit to cab/i });

beforeEach(() => {
  mocks.navigate.mockReset();
  mocks.access = { canWrite: true };
  mocks.proposal = DRAFT;
  mocks.calls = [];
  mocks.update.mockReset();
  mocks.updateAsync.mockReset().mockImplementation(async () => {
    mocks.calls.push("save");
    return DRAFT;
  });
  mocks.submitAsync.mockReset().mockImplementation(async () => {
    mocks.calls.push("submit");
    return { ...DRAFT, state: "cab_review" };
  });
  mocks.withdraw.mockReset();
  mocks.decide.mockReset();
  mocks.create.mockReset();
});

describe("StandardChangeProposalPage — the proposer's editor", () => {
  it("saves the whole draft as one PATCH: content, parameters and evidence", () => {
    renderPage();
    fireEvent.change(screen.getByRole("textbox", { name: /^name/i }), { target: { value: "Rotate a certificate" } });
    fireEvent.click(saveButton());
    expect(mocks.update).toHaveBeenCalledWith({ id: "scp-1", patch: expect.any(Object) }, expect.anything());
    expect(lastPatch()).toEqual({
      name: "Rotate a certificate",
      rationale: "<p>Done every quarter without incident.</p>",
      evidenceChangeRequestIds: ["chg-1"],
      subject: "Rotate the TLS certificate on {{host}}",
      description: "<p>Rotate the certificate on {{host}}.</p>",
      justification: "",
      implementationPlan: "<p>Steps.</p>",
      riskImpactAnalysis: "",
      backoutPlan: "<p>Restore.</p>",
      testPlan: "<p>Check.</p>",
      category: "software",
      impact: "low",
      risk: "low",
      rollbackDurationText: "15 mins",
      isPlanningVisibleToCustomers: false,
      customerApprovalRequired: false,
      customerReviewRequired: false,
      parameters: [{ key: "host", label: "Host", type: "text", required: true }],
    });
    act(() => mocks.update.mock.calls[0]![1].onSuccess());
    expect(screen.getByText("Saved.")).toBeInTheDocument();
  });

  it("clears a category set back to -- Select -- (an empty value clears it on the server)", () => {
    renderPage();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^category/i }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "-- Select --" }));
    fireEvent.click(saveButton());
    expect(lastPatch()).toMatchObject({ category: "", impact: "low", risk: "low" });
  });

  it("shows a refused save in the backend's words", () => {
    renderPage();
    fireEvent.click(saveButton());
    act(() => mocks.update.mock.calls[0]![1].onError(new Error('parameter "host" is declared twice')));
    expect(screen.getByRole("alert")).toHaveTextContent('parameter "host" is declared twice');
  });

  it("checks parameters as the server does and holds Save back until they are fixed", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /add parameter/i }));
    const row = screen.getByRole("group", { name: "Parameter 2" });
    expect(within(row).getByRole("textbox", { name: "Key of parameter 2" })).toHaveValue("param_2");
    expect(within(row).getByText("A label is required.")).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
    expect(submitButton()).toBeDisabled();
    expect(screen.getByText(/fix the parameters above before saving/i)).toBeInTheDocument();

    fireEvent.change(within(row).getByRole("textbox", { name: "Key of parameter 2" }), { target: { value: "Host" } });
    expect(within(row).getByText(/start with a lower-case letter/i)).toBeInTheDocument();
    fireEvent.change(within(row).getByRole("textbox", { name: "Key of parameter 2" }), { target: { value: "host" } });
    expect(within(row).getByText(/declared more than once/i)).toBeInTheDocument();
    fireEvent.change(within(row).getByRole("textbox", { name: "Key of parameter 2" }), { target: { value: "env" } });
    fireEvent.change(within(row).getByRole("textbox", { name: "Label of parameter 2" }), { target: { value: "Environment" } });
    fireEvent.change(within(row).getByRole("combobox", { name: "Type of parameter 2" }), { target: { value: "select" } });
    expect(within(row).getByText(/a choice needs 1 to 50 options/i)).toBeInTheDocument();
    fireEvent.change(within(row).getByRole("textbox", { name: "Options of parameter 2" }), { target: { value: "staging\nproduction\n" } });
    expect(saveButton()).toBeEnabled();

    fireEvent.click(saveButton());
    expect(lastPatch().parameters).toEqual([
      { key: "host", label: "Host", type: "text", required: true },
      { key: "env", label: "Environment", type: "select", required: true, options: ["staging", "production"] },
    ]);
  });

  it("reorders and removes parameters", () => {
    mocks.proposal = {
      ...DRAFT,
      parameters: [
        { key: "host", label: "Host", type: "text", required: true },
        { key: "port", label: "Port", type: "number", required: false },
      ],
      subject: "{{host}}:{{port}}",
    };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Move Port up" }));
    fireEvent.click(saveButton());
    expect((lastPatch().parameters as Array<{ key: string }>).map((p) => p.key)).toEqual(["port", "host"]);
    fireEvent.click(screen.getByRole("button", { name: "Remove Port" }));
    fireEvent.click(saveButton());
    expect((lastPatch().parameters as Array<{ key: string }>).map((p) => p.key)).toEqual(["host"]);
    // The subject still refers to {{port}}: no parameter declares it any more.
    expect(screen.getByText(/refers to placeholders no parameter declares: \{\{port\}\}/i)).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it("inserts a {{key}} placeholder into a rich-text field and at the subject's caret", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Insert a placeholder into Implementation plan" }));
    fireEvent.click(screen.getByRole("menuitem", { name: /\{\{host\}\}/ }));
    expect(richField(/^implementation plan/i)).toHaveValue("<p>Steps. {{host}}</p>");

    const subject = screen.getByRole("textbox", { name: /^subject/i }) as HTMLInputElement;
    fireEvent.change(subject, { target: { value: "Rotate on " } });
    subject.setSelectionRange(7, 7);
    fireEvent.click(screen.getByRole("button", { name: "Insert a placeholder into Subject" }));
    fireEvent.click(screen.getByRole("menuitem", { name: /\{\{host\}\}/ }));
    expect(subject).toHaveValue("Rotate {{host}}on ");

    fireEvent.click(saveButton());
    expect(lastPatch()).toMatchObject({ implementationPlan: "<p>Steps. {{host}}</p>", subject: "Rotate {{host}}on" });
  });

  it("previews a change request raised from it with sample values", () => {
    renderPage();
    const preview = screen.getByRole("region", { name: /preview of a change request raised from this template/i });
    expect(preview.querySelector('mark[data-param="host"]')).toHaveTextContent("{{host}}");
    fireEvent.change(within(screen.getByRole("region", { name: "Preview" })).getByRole("textbox", { name: /^host/i }), {
      target: { value: "gw-5" },
    });
    expect(preview).toHaveTextContent("Rotate the TLS certificate on gw-5");
    expect(preview).toHaveTextContent("Rotate the certificate on gw-5.");
  });

  it("adds and removes evidence, sent as the whole list", () => {
    renderPage();
    fireEvent.change(screen.getByRole("textbox", { name: "Add a closed change request" }), { target: { value: "chg-7" } });
    const table = screen.getByRole("table", { name: "Evidence" });
    expect(within(table).getByText("CHG0000007")).toBeInTheDocument();
    fireEvent.click(within(table).getByRole("button", { name: "Remove CHG0000001 from the evidence" }));
    fireEvent.click(saveButton());
    expect(lastPatch().evidenceChangeRequestIds).toEqual(["chg-7"]);
  });

  it("saves, then sends it to the CAB", async () => {
    renderPage();
    fireEvent.click(submitButton());
    await waitFor(() => expect(mocks.submitAsync).toHaveBeenCalledWith("scp-1"));
    expect(mocks.calls).toEqual(["save", "submit"]);
    expect(mocks.updateAsync.mock.calls[0]![0].id).toBe("scp-1");
  });

  it("says when it saved but the CAB submission was refused", async () => {
    mocks.submitAsync.mockRejectedValue(new Error("the proposal is not complete; it still needs: testPlan"));
    renderPage();
    fireEvent.click(submitButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Saved, but not sent to the CAB: the proposal is not complete; it still needs: testPlan",
    );
  });

  it("does not submit when the save was refused", async () => {
    mocks.updateAsync.mockRejectedValue(new Error("refused"));
    renderPage();
    fireEvent.click(submitButton());
    expect(await screen.findByRole("alert")).toHaveTextContent("refused");
    expect(mocks.submitAsync).not.toHaveBeenCalled();
  });

  it("lists what a draft still needs and holds Submit back", () => {
    mocks.proposal = { ...DRAFT, rationale: null, testPlan: null, impact: null };
    renderPage();
    expect(submitButton()).toBeDisabled();
    expect(screen.getByText(/still needs a rationale, a test plan, an impact/i)).toBeInTheDocument();
    // Saving an incomplete draft is fine.
    expect(saveButton()).toBeEnabled();
  });

  it("never lets a standard change be high risk", () => {
    mocks.proposal = { ...DRAFT, risk: "high" };
    renderPage();
    expect(screen.getByText(/cannot be high risk/i)).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^risk/i }));
    const options = within(screen.getByRole("listbox")).getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual(["-- Select --", "Low", "Moderate", "High"]);
    expect(options[3]).toHaveAttribute("aria-disabled", "true");
  });

  it("asks a retirement only for its name, rationale and evidence", () => {
    mocks.proposal = { ...DRAFT, kind: "retire", template: { id: "tpl-1", name: "Rotate a TLS certificate" } };
    renderPage();
    expect(screen.queryByRole("region", { name: "Parameters (change specifics)" })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /^subject/i })).not.toBeInTheDocument();
    fireEvent.click(saveButton());
    expect(lastPatch()).toEqual({
      name: "Rotate a TLS certificate",
      rationale: "<p>Done every quarter without incident.</p>",
      evidenceChangeRequestIds: ["chg-1"],
    });
  });

  it("withdraws after a confirmation", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Withdraw" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: /withdraw SCP0000001/i })).getByRole("button", { name: "Withdraw" }));
    expect(mocks.withdraw).toHaveBeenCalledWith("scp-1", expect.anything());
  });
});

describe("StandardChangeProposalPage — read-only and the CAB's decision", () => {
  const IN_REVIEW: BeStandardChangeProposal = {
    ...DRAFT,
    state: "cab_review",
    canEdit: false,
    canSubmit: false,
    canWithdraw: false,
    submittedOn: "2026-10-02T00:00:00Z",
    approvers: [
      { id: "a-1", approver: { id: "u-2", name: "Carol Cab", email: "carol@example.com" }, state: "REQUESTED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: true },
      { id: "a-2", approver: { id: "u-3", name: "Dan Cab", email: "dan@example.com" }, state: "REQUESTED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: false },
    ],
  };

  it("shows the proposal read-only, with the approvers", () => {
    mocks.proposal = IN_REVIEW;
    renderPage();
    expect(screen.queryByRole("button", { name: /^save$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /^name/i })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Implementation plan" })).toHaveTextContent("Steps.");
    const approvers = screen.getByRole("table", { name: "CAB approvers" });
    expect(within(approvers).getByText("Carol Cab")).toBeInTheDocument();
    expect(within(approvers).getByText("(you)")).toBeInTheDocument();
  });

  it("needs a comment to reject", () => {
    mocks.proposal = IN_REVIEW;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    const dialog = screen.getByRole("dialog", { name: /reject the proposal/i });
    const reject = within(dialog).getByRole("button", { name: "Reject" });
    expect(reject).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /comment/i }), { target: { value: "   " } });
    expect(reject).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /comment/i }), { target: { value: " Too broad. " } });
    fireEvent.click(reject);
    expect(mocks.decide).toHaveBeenCalledWith({ id: "scp-1", decision: "rejected", comment: "Too broad." }, expect.anything());
  });

  it("approves with no comment needed", () => {
    mocks.proposal = IN_REVIEW;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: /approve the proposal/i })).getByRole("button", { name: "Approve" }));
    expect(mocks.decide).toHaveBeenCalledWith({ id: "scp-1", decision: "approved" }, expect.anything());
  });

  it("offers no decision to a viewer without a pending row of their own", () => {
    mocks.proposal = { ...IN_REVIEW, approvers: IN_REVIEW.approvers!.map((a) => ({ ...a, canDecide: false })) };
    renderPage();
    expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Reject" })).not.toBeInTheDocument();
  });

  it("lets the proposer withdraw it while the CAB reviews it", () => {
    mocks.proposal = { ...IN_REVIEW, canWithdraw: true };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Withdraw" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Withdraw" }));
    expect(mocks.withdraw).toHaveBeenCalledWith("scp-1", expect.anything());
  });

  it("keeps a rejected proposal as the record, with the comment, and copies it into a new one", () => {
    mocks.proposal = {
      ...IN_REVIEW,
      state: "rejected",
      approvers: [],
      decidedOn: "2026-10-03T00:00:00Z",
      decidedBy: { id: "u-2", name: "Carol Cab", email: "carol@example.com" },
      decisionComment: "Too broad: split it per gateway.",
    };
    renderPage();
    const decision = screen.getByRole("region", { name: "Decision" });
    expect(decision).toHaveTextContent("Rejected by the CAB");
    expect(decision).toHaveTextContent("Too broad: split it per gateway.");
    expect(decision).toHaveTextContent(/catalogue was not changed/i);
    fireEvent.click(screen.getByRole("button", { name: /copy into a new proposal/i }));
    expect(mocks.create).toHaveBeenCalledWith({ kind: "new", copyFromProposalId: "scp-1" }, expect.anything());
    act(() => mocks.create.mock.calls[0]![1].onSuccess({ id: "scp-2" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-2", {
      state: { from: "/operations/standard-change-proposals/scp-1" },
    });
  });

  it("offers no copy of a rejected revision (a copy can only start a new template)", () => {
    mocks.proposal = { ...IN_REVIEW, kind: "modify", state: "rejected", approvers: [], template: { id: "tpl-1", name: "Rotate" } };
    renderPage();
    expect(screen.queryByRole("button", { name: /copy into a new proposal/i })).not.toBeInTheDocument();
  });

  it("links an approved proposal to the template and version it wrote", () => {
    mocks.proposal = {
      ...IN_REVIEW,
      state: "approved",
      approvers: [],
      template: { id: "tpl-1", name: "Rotate a TLS certificate" },
      resultingVersion: 1,
    };
    renderPage();
    const decision = screen.getByRole("region", { name: "Decision" });
    expect(decision).toHaveTextContent("Approved by the CAB");
    expect(decision).toHaveTextContent(/resulting template: rotate a tls certificate, version 1/i);
    fireEvent.click(within(decision).getByRole("button", { name: "Rotate a TLS certificate" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-changes/tpl-1", {
      state: { from: "/operations/standard-change-proposals/scp-1" },
    });
  });
});
