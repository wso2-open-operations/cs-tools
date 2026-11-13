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
import type { JSX } from "react";
import type { BeStandardChangeProposal } from "@api/backend/types";
import { BackendApiError } from "@api/backend/client";

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
  /** The decision mutation's error as the hook reports it (react-query keeps the last one). */
  decideError: null as unknown,
  create: vi.fn(),
  cancel: vi.fn(),
  remove: vi.fn(),
  recordView: vi.fn(),
  refetch: vi.fn(),
  calls: [] as string[],
}));

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({ useRecordRecentView: () => mocks.recordView }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
  useGetStandardChangeProposal: () => ({ data: mocks.proposal, isLoading: false, isError: false, refetch: mocks.refetch }),
  useUpdateStandardChangeProposal: () => ({ mutate: mocks.update, mutateAsync: mocks.updateAsync, isPending: false }),
  useSubmitStandardChangeProposal: () => ({ mutateAsync: mocks.submitAsync, isPending: false }),
  useWithdrawStandardChangeProposal: () => ({ mutate: mocks.withdraw, isPending: false }),
  useDecideStandardChangeProposal: () => ({
    mutate: mocks.decide,
    isPending: false,
    isError: mocks.decideError != null,
    error: mocks.decideError,
    reset: vi.fn(),
  }),
  useCreateStandardChangeProposal: () => ({ mutate: mocks.create, isPending: false }),
  useCancelStandardChangeProposal: () => ({ mutate: mocks.cancel, isPending: false, isError: false, reset: vi.fn() }),
  useDeleteStandardChangeProposal: () => ({ mutate: mocks.remove, isPending: false }),
  useSearchClosedChangeRequestsForSelect: () => ({ data: [], isFetching: false, isError: false }),
  useSearchStandardChangeCustomersForSelect: () => ({ data: [], isFetching: false, isError: false }),
}));
// The Lexical editor does not run under jsdom: a textarea stands in, showing the
// value the field mounted with (as the real editor does) and reporting edits.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange, disabled }: { value: string; onChange: (v: string) => void; disabled?: boolean }) => (
    <textarea defaultValue={value} disabled={disabled} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// The evidence and customer pickers are plain inputs reporting the picked record.
const PICKED: Record<string, unknown> = {
  "chg-7": { id: "chg-7", number: "CHG0000007", subject: "Rotated the certificate on gw-7" },
  "acct-2": { id: "acct-2", name: "Other Corp" },
};
vi.mock("@components/AsyncEntitySelect", () => ({
  default: ({
    id,
    label,
    value,
    disabled,
    helperText,
    searchExtra,
    onChange,
  }: {
    id: string;
    label: string;
    value: string;
    disabled?: boolean;
    helperText?: string;
    searchExtra?: string;
    onChange: (next: string, item?: unknown) => void;
  }) => (
    <>
      <input
        aria-label={label}
        aria-describedby={helperText ? `${id}-helper` : undefined}
        data-value={value}
        data-search-extra={searchExtra ?? ""}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value, PICKED[e.target.value])}
      />
      {helperText && <span id={`${id}-helper`}>{helperText}</span>}
    </>
  ),
}));

import StandardChangeProposalPage from "@features/csm-operations/pages/StandardChangeProposalPage";

const DRAFT: BeStandardChangeProposal = {
  id: "scp-1",
  number: "SCP0000001",
  kind: "new",
  state: "draft",
  name: "Rotate a TLS certificate",
  customer: { id: "acct-1", name: "Example Corp" },
  rationale: "<p>Done every quarter without incident.</p>",
  template: null,
  baseVersion: null,
  sourceChangeRequest: { id: "chg-1", name: "CHG0000001" },
  summary: "Rotates a gateway's TLS certificate.",
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
  priority: null,
  deploymentTypes: null,
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
  canDelete: false,
  canCancel: false,
};

/** Renders the page; the returned function renders it again, as a refetch would, with whatever `mocks.proposal` now holds. */
function renderPage(state?: { from: string }): () => void {
  const tree = (): JSX.Element => (
    <MemoryRouter initialEntries={[{ pathname: "/operations/standard-change-proposals/scp-1", state }]}>
      <Routes>
        <Route path="/operations/standard-change-proposals/:id" element={<StandardChangeProposalPage />} />
      </Routes>
    </MemoryRouter>
  );
  const { rerender } = render(tree());
  return () => rerender(tree());
}

const richField = (name: RegExp): HTMLElement => within(screen.getByRole("group", { name })).getByRole("textbox");
const lastPatch = (): Record<string, unknown> => mocks.update.mock.calls.at(-1)![0].patch;
const saveButton = (): HTMLElement => screen.getByRole("button", { name: /^save$/i });
const submitButton = (): HTMLElement => screen.getByRole("button", { name: /submit for approval/i });

beforeEach(() => {
  mocks.navigate.mockReset();
  mocks.recordView.mockReset();
  mocks.access = { canWrite: true };
  mocks.proposal = DRAFT;
  mocks.decideError = null;
  mocks.calls = [];
  mocks.update.mockReset();
  mocks.updateAsync.mockReset().mockImplementation(async () => {
    mocks.calls.push("save");
    return DRAFT;
  });
  mocks.submitAsync.mockReset().mockImplementation(async () => {
    mocks.calls.push("submit");
    return { ...DRAFT, state: "authority_review" };
  });
  mocks.withdraw.mockReset();
  mocks.decide.mockReset();
  mocks.refetch.mockReset();
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
      summary: "Rotates a gateway's TLS certificate.",
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
      priority: "",
      deploymentTypes: [],
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

  it("saves, then submits it to the CAB", async () => {
    renderPage();
    fireEvent.click(submitButton());
    await waitFor(() => expect(mocks.submitAsync).toHaveBeenCalledWith("scp-1"));
    expect(mocks.calls).toEqual(["save", "submit"]);
    expect(mocks.updateAsync.mock.calls[0]![0].id).toBe("scp-1");
  });

  it("says when it saved but the submission was refused", async () => {
    mocks.submitAsync.mockRejectedValue(new Error("the proposal is not complete; it still needs: testPlan"));
    renderPage();
    fireEvent.click(submitButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Saved, but not submitted: the proposal is not complete; it still needs: testPlan",
    );
  });

  it("does not submit when the save was refused", async () => {
    mocks.updateAsync.mockRejectedValue(new Error("refused"));
    renderPage();
    fireEvent.click(submitButton());
    expect(await screen.findByRole("alert")).toHaveTextContent("refused");
    expect(mocks.submitAsync).not.toHaveBeenCalled();
  });

  it("says a complete draft goes to two CAB members, and who judges the evidence", () => {
    renderPage();
    expect(submitButton()).toBeEnabled();
    expect(
      screen.getByText("Ready to submit: two CAB members other than you must authorise it. Submitting saves first."),
    ).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Evidence" })).toHaveTextContent("The CAB judges them.");
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
    // Named after its template, and approving it renames nothing: the name is shown, not edited.
    expect(screen.getByRole("textbox", { name: /^name/i })).toBeDisabled();
    expect(screen.getByRole("textbox", { name: /^name/i })).not.toBeRequired();
    fireEvent.click(saveButton());
    expect(lastPatch()).toEqual({
      name: "Rotate a TLS certificate",
      rationale: "<p>Done every quarter without incident.</p>",
      evidenceChangeRequestIds: ["chg-1"],
    });
  });

  it("warns a retirement of the change requests its template still has in New", () => {
    mocks.proposal = {
      ...DRAFT,
      kind: "retire",
      template: { id: "tpl-1", name: "Rotate a TLS certificate" },
      changeRequestsInNew: [{ id: "chg-7", name: "CHG0000007" }],
      changeRequestsInNewTotal: 3,
    };
    renderPage();
    const warning = screen.getByRole("region", { name: "Change requests still New" });
    expect(warning).toHaveTextContent("3 change requests raised from this template are still New");
    expect(warning).toHaveTextContent("CHG0000007 and 2 more");
    fireEvent.click(within(warning).getByRole("button", { name: "CHG0000007" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-7", expect.anything());
  });

  it("does not warn a retirement whose template has no change request in New", () => {
    mocks.proposal = { ...DRAFT, kind: "retire", template: { id: "tpl-1", name: "Rotate a TLS certificate" }, changeRequestsInNewTotal: 0 };
    renderPage();
    expect(screen.queryByRole("region", { name: "Change requests still New" })).not.toBeInTheDocument();
  });

  describe("its customer: a standard change is one customer's", () => {
    /** A new proposal started from nothing (the catalogue's "Propose a standard change"). */
    const BLANK_NEW: BeStandardChangeProposal = { ...DRAFT, sourceChangeRequest: null, customer: null, evidence: [] };
    const customerField = (): HTMLElement => screen.getByRole("textbox", { name: /^customer/i });
    const evidencePicker = (): HTMLElement => screen.getByRole("textbox", { name: "Add a closed change request" });

    it("shows the customer of a proposal started from a change request read-only, with why, and never sends it", () => {
      renderPage();
      expect(customerField()).toHaveValue("Example Corp");
      expect(customerField()).toHaveAttribute("readonly");
      expect(screen.getByText("The customer of the change request it started from.")).toBeInTheDocument();
      fireEvent.click(saveButton());
      expect(lastPatch()).not.toHaveProperty("accountId");
    });

    it("searches only that customer's closed change requests for evidence", () => {
      renderPage();
      expect(evidencePicker()).toBeEnabled();
      expect(evidencePicker()).toHaveAttribute("data-search-extra", "acct-1");
    });

    it("asks a new proposal for its customer before evidence and before the CAB", () => {
      mocks.proposal = BLANK_NEW;
      renderPage();
      expect(customerField()).not.toHaveAttribute("readonly");
      expect(customerField()).toHaveAttribute("data-value", "");
      expect(customerField()).toHaveAccessibleDescription(/required\. the customer the template is for/i);
      expect(evidencePicker()).toBeDisabled();
      expect(evidencePicker()).toHaveAccessibleDescription(/name the customer first/i);
      expect(screen.getByText(/still needs a customer/i)).toBeInTheDocument();
      expect(submitButton()).toBeDisabled();
      // Saving a draft with no customer yet is fine, and sends none.
      expect(saveButton()).toBeEnabled();
      fireEvent.click(saveButton());
      expect(lastPatch()).not.toHaveProperty("accountId");
    });

    it("names the customer of a new proposal, which then decides the evidence offered and is sent", () => {
      mocks.proposal = BLANK_NEW;
      renderPage();
      fireEvent.change(customerField(), { target: { value: "acct-2" } });
      expect(customerField()).toHaveAttribute("data-value", "acct-2");
      expect(evidencePicker()).toBeEnabled();
      expect(evidencePicker()).toHaveAttribute("data-search-extra", "acct-2");
      expect(screen.queryByText(/still needs a customer/i)).not.toBeInTheDocument();
      expect(submitButton()).toBeEnabled();
      fireEvent.click(saveButton());
      expect(lastPatch()).toMatchObject({ accountId: "acct-2", evidenceChangeRequestIds: [] });
    });

    it("drops the evidence of the previous customer, and says so, when another customer is named", () => {
      mocks.proposal = { ...DRAFT, sourceChangeRequest: null };
      renderPage();
      expect(within(screen.getByRole("table", { name: "Evidence" })).getByText("CHG0000001")).toBeInTheDocument();
      fireEvent.change(customerField(), { target: { value: "acct-2" } });
      expect(screen.queryByRole("table", { name: "Evidence" })).not.toBeInTheDocument();
      expect(screen.getByText("No evidence cited.")).toBeInTheDocument();
      expect(screen.getByRole("status")).toHaveTextContent(
        /the evidence was removed \(1 change request\): it must be closed change requests of the proposal's customer\. add other corp's/i,
      );
      fireEvent.click(saveButton());
      expect(lastPatch()).toMatchObject({ accountId: "acct-2", evidenceChangeRequestIds: [] });
    });

    it("keeps the evidence when the same customer is picked again", () => {
      mocks.proposal = { ...DRAFT, sourceChangeRequest: null };
      renderPage();
      fireEvent.change(customerField(), { target: { value: "acct-1" } });
      expect(within(screen.getByRole("table", { name: "Evidence" })).getByText("CHG0000001")).toBeInTheDocument();
      expect(screen.queryByRole("status")).not.toBeInTheDocument();
    });

    it.each(["modify", "retire"] as const)("shows a %s proposal's customer (its template's) read-only", (kind) => {
      mocks.proposal = { ...DRAFT, kind, sourceChangeRequest: null, template: { id: "tpl-1", name: "Rotate a TLS certificate" } };
      renderPage();
      expect(customerField()).toHaveValue("Example Corp");
      expect(customerField()).toHaveAttribute("readonly");
      expect(screen.getByText(/the template's customer/i)).toBeInTheDocument();
      expect(screen.queryByText(/still needs a customer/i)).not.toBeInTheDocument();
      fireEvent.click(saveButton());
      expect(lastPatch()).not.toHaveProperty("accountId");
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
    state: "authority_review",
    canEdit: false,
    canSubmit: false,
    canWithdraw: false,
    submittedOn: "2026-10-02T00:00:00Z",
    stages: [{ id: "st-1", label: "CAB Approval", requiredApprovals: 2, approvals: 0, decision: null }],
    approvers: [
      { id: "a-1", stageId: "st-1", approver: { id: "u-2", name: "Hana Head", email: "hana@example.com" }, state: "REQUESTED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: true },
      { id: "a-2", stageId: "st-1", approver: { id: "u-3", name: "Ivan Head", email: "ivan@example.com" }, state: "REQUESTED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: false },
    ],
  };

  it("shows the proposal read-only, with the approvers", () => {
    mocks.proposal = IN_REVIEW;
    renderPage();
    expect(screen.queryByRole("button", { name: /^save$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: /^name/i })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Implementation plan" })).toHaveTextContent("Steps.");
    const approvers = screen.getByRole("table", { name: "CAB Approval approvers" });
    expect(within(approvers).getByText("Hana Head")).toBeInTheDocument();
    expect(within(approvers).getByText("(you)")).toBeInTheDocument();
  });

  it("says whose standard change it is, linking to the customer", () => {
    mocks.proposal = IN_REVIEW;
    renderPage();
    const link = screen.getByRole("button", { name: "Example Corp" });
    expect(link.closest("p")).toHaveTextContent("Customer: Example Corp");
    fireEvent.click(link);
    expect(mocks.navigate).toHaveBeenCalledWith("/customers/accounts/acct-1", {
      state: { from: "/operations/standard-change-proposals/scp-1" },
    });
  });

  it("marks evidence that is now another customer's, which the CAB cannot approve with", () => {
    mocks.proposal = {
      ...IN_REVIEW,
      evidence: [
        { ...IN_REVIEW.evidence![0], customer: { id: "ACCT-1", name: "Example Corp" } },
        { id: "chg-2", number: "CHG0000002", subject: "Moved", state: "closed", closedOn: null, customer: { id: "acct-2", name: "Other Corp" } },
        { id: "chg-3", number: "CHG0000003", subject: "Orphaned", state: "closed", closedOn: null, customer: null },
      ],
    };
    renderPage();
    const table = screen.getByRole("table", { name: "Evidence" });
    const rows = within(table).getAllByRole("row");
    expect(rows[1]).not.toHaveTextContent(/now a change request/i);
    expect(rows[2]).toHaveTextContent("Now a change request of another customer (Other Corp)");
    expect(rows[3]).toHaveTextContent("Now a change request of no customer");
  });

  it("says when a proposal has not named its customer yet", () => {
    mocks.proposal = { ...IN_REVIEW, customer: null };
    renderPage();
    expect(screen.getByText("not named yet").closest("p")).toHaveTextContent("Customer: not named yet");
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
    expect(mocks.decide).toHaveBeenCalledWith(
      { id: "scp-1", decision: "rejected", comment: "Too broad.", stageId: "st-1" },
      expect.anything(),
    );
  });

  it("authorises with no comment needed, saying a second member must approve too", () => {
    mocks.proposal = IN_REVIEW;
    renderPage();
    expect(screen.queryByRole("button", { name: "Approve" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
    const dialog = screen.getByRole("dialog", { name: /authorise the proposal/i });
    expect(dialog).toHaveTextContent(
      "2 different CAB members must approve; the template change is applied by the last of them",
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Authorise" }));
    expect(mocks.decide).toHaveBeenCalledWith({ id: "scp-1", decision: "approved", stageId: "st-1" }, expect.anything());
  });

  describe("a proposal that moves on while the decision dialog is open", () => {
    /** The same proposal returned to draft and submitted again: a new CAB stage, where the caller can decide again. */
    const RESUBMITTED: BeStandardChangeProposal = {
      ...IN_REVIEW,
      stages: [{ id: "st-9", label: "CAB Approval", requiredApprovals: 2, approvals: 0, decision: null }],
      approvers: IN_REVIEW.approvers!.map((a) => ({ ...a, id: `${a.id}-9`, stageId: "st-9" })),
    };

    it("sends the stage the dialog was opened on, never the one a refetch shows, and says the proposal changed", () => {
      mocks.proposal = IN_REVIEW;
      const refetched = renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
      const dialog = screen.getByRole("dialog", { name: /authorise the proposal/i });
      expect(within(dialog).queryByRole("alert", { name: "Proposal changed" })).not.toBeInTheDocument();
      mocks.proposal = RESUBMITTED;
      refetched();
      expect(within(dialog).getByRole("alert", { name: "Proposal changed" })).toHaveTextContent(/review it again/i);
      fireEvent.click(within(dialog).getByRole("button", { name: "Authorise" }));
      expect(mocks.decide).toHaveBeenCalledTimes(1);
      expect(mocks.decide).toHaveBeenCalledWith({ id: "scp-1", decision: "approved", stageId: "st-1" }, expect.anything());
    });

    it("still names the stage when the refetched proposal leaves the caller nothing to decide", () => {
      mocks.proposal = IN_REVIEW;
      const refetched = renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Reject" }));
      const dialog = screen.getByRole("dialog", { name: /reject the proposal/i });
      mocks.proposal = { ...IN_REVIEW, approvers: IN_REVIEW.approvers!.map((a) => ({ ...a, canDecide: false })) };
      refetched();
      fireEvent.change(within(dialog).getByRole("textbox", { name: /comment/i }), { target: { value: "Too broad." } });
      fireEvent.click(within(dialog).getByRole("button", { name: "Reject" }));
      expect(mocks.decide).toHaveBeenCalledWith(
        { id: "scp-1", decision: "rejected", comment: "Too broad.", stageId: "st-1" },
        expect.anything(),
      );
    });

    it("closes the dialog on a stale-stage refusal (409), says why and reads the proposal again, without retrying", async () => {
      mocks.proposal = IN_REVIEW;
      mocks.decide.mockImplementation((_vars: unknown, opts: { onError?: (err: unknown) => void }) =>
        opts.onError?.(
          new BackendApiError(
            409,
            "the stage you decided on is not the one this proposal now waits on, so nothing was recorded: reload it to see what you are asked to decide",
            { errorCode: "standard_change_decision_stage_stale" },
          ),
        ),
      );
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
      fireEvent.click(within(screen.getByRole("dialog", { name: /authorise the proposal/i })).getByRole("button", { name: "Authorise" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      const notice = screen.getByRole("alert");
      expect(notice).toHaveTextContent("The proposal changed after you opened it, so your decision was not recorded.");
      expect(notice).toHaveFocus();
      expect(mocks.refetch).toHaveBeenCalledTimes(1);
      expect(mocks.decide).toHaveBeenCalledTimes(1);
    });

    it("closes the dialog the same way when the proposal has left review", async () => {
      mocks.proposal = IN_REVIEW;
      mocks.decide.mockImplementation((_vars: unknown, opts: { onError?: (err: unknown) => void }) =>
        opts.onError?.(
          new BackendApiError(409, "this proposal is not waiting for a decision: it is withdrawn", {
            errorCode: "standard_change_proposal_not_in_review",
          }),
        ),
      );
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
      fireEvent.click(within(screen.getByRole("dialog", { name: /authorise the proposal/i })).getByRole("button", { name: "Authorise" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(screen.getByRole("alert")).toHaveTextContent("The proposal changed after you opened it");
      expect(mocks.refetch).toHaveBeenCalledTimes(1);
    });

    it.each([
      ["no code", undefined],
      ["another code", "standard_change_template_unavailable"],
    ])("keeps a 409 with %s in the dialog, with the backend's words, and reads nothing again", (_label, errorCode) => {
      // A clash with the approval itself (here: a template took the name since
      // submit): the proposal has not moved on, and saying it had would be false.
      const words =
        'this customer already has a standard change template named "Rotate" (it took the name after this proposal was submitted): reject it, or ask the proposer to withdraw it and propose again under another name';
      mocks.proposal = IN_REVIEW;
      mocks.decide.mockImplementation((_vars: unknown, opts: { onError?: (err: unknown) => void }) => {
        const err = new BackendApiError(409, words, errorCode ? { errorCode } : undefined);
        mocks.decideError = err;
        opts.onError?.(err);
      });
      const rerender = renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
      fireEvent.click(within(screen.getByRole("dialog", { name: /authorise the proposal/i })).getByRole("button", { name: "Authorise" }));
      rerender(); // react-query re-renders with the mutation's error
      const dialog = screen.getByRole("dialog", { name: /authorise the proposal/i });
      expect(dialog).toHaveTextContent(words);
      expect(screen.queryByText(/the proposal changed after you opened it/i)).not.toBeInTheDocument();
      expect(mocks.refetch).not.toHaveBeenCalled();
    });

    it("keeps the dialog open on any other refusal", () => {
      mocks.proposal = IN_REVIEW;
      mocks.decide.mockImplementation((_vars: unknown, opts: { onError?: (err: unknown) => void }) =>
        opts.onError?.(new BackendApiError(403, "only active internal (WSO2) users can decide a standard change proposal")),
      );
      renderPage();
      fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
      const dialog = screen.getByRole("dialog", { name: /authorise the proposal/i });
      fireEvent.click(within(dialog).getByRole("button", { name: "Authorise" }));
      expect(screen.getByRole("dialog", { name: /authorise the proposal/i })).toBeInTheDocument();
      expect(mocks.refetch).not.toHaveBeenCalled();
    });
  });

  describe("a stage that departures from the CAB leave short of its approvals", () => {
    it("says on the open stage that it cannot be approved as it stands, and what can be done", () => {
      mocks.proposal = {
        ...IN_REVIEW,
        stages: [{ ...IN_REVIEW.stages![0]!, approvals: 0, eligibleApprovers: 1, cannotReachQuorum: true }],
      };
      renderPage();
      const stage = screen.getByRole("region", { name: "CAB Approval approvers" });
      const warning = within(stage).getByRole("alert", { name: "Cannot be approved as it stands" });
      expect(warning).toHaveTextContent("Only 1 CAB member can still approve this stage, which needs 2");
      expect(warning).toHaveTextContent("A CAB member can cancel it, or its proposer can withdraw it and submit a copy");
    });

    it.each([
      ["it can still reach its approvals", { ...IN_REVIEW, stages: [{ ...IN_REVIEW.stages![0]!, eligibleApprovers: 3, cannotReachQuorum: false }] }],
      ["the backend sends no count (an older build)", IN_REVIEW],
      [
        "the proposal is no longer in review",
        {
          ...IN_REVIEW,
          state: "withdrawn",
          stages: [{ ...IN_REVIEW.stages![0]!, eligibleApprovers: 1, cannotReachQuorum: true }],
        },
      ],
    ])("says nothing when %s", (_label, proposal) => {
      mocks.proposal = proposal;
      renderPage();
      expect(screen.queryByRole("alert", { name: "Cannot be approved as it stands" })).not.toBeInTheDocument();
    });
  });

  it("offers no decision to a viewer without a pending row of their own", () => {
    mocks.proposal = { ...IN_REVIEW, approvers: IN_REVIEW.approvers!.map((a) => ({ ...a, canDecide: false })) };
    renderPage();
    expect(screen.queryByRole("button", { name: "Authorise" })).not.toBeInTheDocument();
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
      stages: [{ ...IN_REVIEW.stages![0]!, decision: "REJECTED" }],
      approvers: [],
      decidedOn: "2026-10-03T00:00:00Z",
      decidedBy: { id: "u-2", name: "Hana Head", email: "hana@example.com" },
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
      stages: [{ ...IN_REVIEW.stages![0]!, approvals: 2, decision: "APPROVED" }],
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

describe("StandardChangeProposalPage — what a template fixes, and getting rid of a draft", () => {
  it("asks for the summary and the description before it can be submitted", () => {
    mocks.proposal = { ...DRAFT, summary: null, description: null };
    renderPage();
    expect(submitButton()).toBeDisabled();
    expect(screen.getByText(/still needs a summary, a description/i)).toBeInTheDocument();
    fireEvent.change(screen.getByRole("textbox", { name: /^summary/i }), { target: { value: "  Rotates certificates.  " } });
    fireEvent.click(saveButton());
    expect(lastPatch()).toMatchObject({ summary: "Rotates certificates." });
  });

  it("fixes a priority and the deployment types, or leaves both to whoever raises the change", () => {
    renderPage();
    expect(screen.getByText("Left to whoever raises the change.")).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^priority/i }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "High" }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^deployment types/i }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "Primary Production" }));
    fireEvent.click(within(screen.getByRole("listbox")).getByRole("option", { name: "Staging" }));
    fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });
    expect(
      screen.getByText("A change raised from the template needs at least one deployment, all Primary Production or Staging."),
    ).toBeInTheDocument();
    fireEvent.click(saveButton());
    expect(lastPatch()).toMatchObject({ priority: "high", deploymentTypes: ["primary_production", "staging"] });
  });

  it("deletes a draft never submitted, after a confirmation, instead of withdrawing it", () => {
    mocks.proposal = { ...DRAFT, canDelete: true };
    renderPage();
    expect(screen.queryByRole("button", { name: "Withdraw" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete draft" }));
    const dialog = screen.getByRole("dialog", { name: "Delete SCP0000001?" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete draft" }));
    expect(mocks.remove).toHaveBeenCalledWith("scp-1", expect.anything());
    act(() => mocks.remove.mock.calls[0]![1].onSuccess());
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests?view=standard&show=proposals");
  });
});

describe("StandardChangeProposalPage — the CAB", () => {
  const CAB_STAGE = { id: "st-2", label: "CAB Approval", requiredApprovals: 2, approvals: 1, decision: null };
  const CAB_REVIEW: BeStandardChangeProposal = {
    ...DRAFT,
    state: "authority_review",
    canEdit: false,
    canSubmit: false,
    canWithdraw: false,
    submittedOn: "2026-10-02T00:00:00Z",
    stages: [CAB_STAGE],
    approvers: [
      { id: "a-3", stageId: "st-2", approver: { id: "u-5", name: "Hana Head", email: "hana@example.com" }, state: "REQUESTED", comments: null, updatedOn: "2026-10-03T00:00:00Z", canDecide: true },
      { id: "a-4", stageId: "st-2", approver: { id: "u-6", name: "Ivan Head", email: "ivan@example.com" }, state: "APPROVED", comments: "Fine", updatedOn: "2026-10-03T00:00:00Z", canDecide: false },
    ],
  };
  /** The stage of a proposal submitted before 2026-11-10, when the Standard Change Authority authorised templates. */
  const AUTHORITY_STAGE = { id: "st-3", label: "Standard Change Authority", requiredApprovals: 2, approvals: 2, decision: "APPROVED" as const };
  /** The one-approval CAB stage of a proposal decided under the oldest flow: shown by its own label, nothing more. */
  const OLDER_STAGE = { id: "st-1", label: "CAB Approval", requiredApprovals: 1, approvals: 1, decision: "APPROVED" as const };
  const OLDER_ROW = { id: "a-1", stageId: "st-1", approver: { id: "u-2", name: "Carol Silva", email: "carol@example.com" }, state: "APPROVED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: false };
  const AUTHORITY_ROWS = [
    { id: "a-5", stageId: "st-3", approver: { id: "u-7", name: "Jane Head", email: "jane@example.com" }, state: "APPROVED", comments: null, updatedOn: "2026-10-03T00:00:00Z", canDecide: false },
    { id: "a-6", stageId: "st-3", approver: { id: "u-8", name: "Kim Head", email: "kim@example.com" }, state: "APPROVED", comments: null, updatedOn: "2026-10-03T00:00:00Z", canDecide: false },
  ];

  it("shows its one stage, the CAB's, with its progress", () => {
    mocks.proposal = CAB_REVIEW;
    renderPage();
    expect(screen.getByText("Awaiting CAB approval")).toBeInTheDocument();
    expect(screen.getAllByRole("region", { name: / approvers$/ })).toHaveLength(1);
    const cab = screen.getByRole("region", { name: "CAB Approval approvers" });
    // One stage is not numbered.
    expect(within(cab).getByText("CAB Approval")).toBeInTheDocument();
    expect(cab).not.toHaveTextContent("1. ");
    expect(cab).toHaveTextContent("1 of 2 approvals");
    expect(within(cab).getByText("Hana Head")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Approval" })).toHaveTextContent(
      "its template is authorised by two different CAB members. The proposer cannot authorise their own proposal.",
    );
    expect(screen.getByRole("region", { name: "Approval" })).not.toHaveTextContent("Standard Change Authority");
  });

  it("authorises: the second approval applies it", () => {
    mocks.proposal = CAB_REVIEW;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
    const dialog = screen.getByRole("dialog", { name: /authorise the proposal/i });
    expect(dialog).toHaveTextContent("Yours is the last approval needed: authorising applies the template change.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Authorise" }));
    // It names the stage this page shows, so a decision on a stage settled or closed meanwhile is refused, not recorded.
    expect(mocks.decide).toHaveBeenCalledWith({ id: "scp-1", decision: "approved", stageId: "st-2" }, expect.anything());
  });

  it("says a proposal the CAB approved was approved by the CAB", () => {
    mocks.proposal = {
      ...CAB_REVIEW,
      state: "approved",
      stages: [{ ...CAB_STAGE, approvals: 2, decision: "APPROVED" }],
      approvers: [{ ...CAB_REVIEW.approvers![0]!, state: "APPROVED", canDecide: false }, CAB_REVIEW.approvers![1]!],
    };
    renderPage();
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Approved by the CAB");
    expect(screen.getByRole("region", { name: "Decision" })).not.toHaveTextContent("CAB Approval stage");
  });

  it("says a proposal the CAB rejected was rejected by the CAB", () => {
    mocks.proposal = {
      ...CAB_REVIEW,
      state: "rejected",
      stages: [{ ...CAB_STAGE, decision: "REJECTED" }],
      decisionComment: "Too broad",
    };
    renderPage();
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Rejected by the CAB");
  });

  it("says a proposal the Standard Change Authority decided, before the CAB took over, was decided by it", () => {
    mocks.proposal = { ...CAB_REVIEW, state: "approved", stages: [AUTHORITY_STAGE], approvers: AUTHORITY_ROWS };
    renderPage();
    const authority = screen.getByRole("region", { name: "Standard Change Authority approvers" });
    expect(authority).not.toHaveTextContent("1. ");
    expect(authority).toHaveTextContent("Approved");
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Approved by the Standard Change Authority");
    // The rule is today's; a decided proposal was settled under its own.
    expect(screen.getByRole("region", { name: "Approval" })).not.toHaveTextContent("two different CAB members");
  });

  it("shows an older decided proposal's stages in order, each by its own label and count", () => {
    mocks.proposal = {
      ...CAB_REVIEW,
      state: "approved",
      stages: [OLDER_STAGE, AUTHORITY_STAGE],
      approvers: [...AUTHORITY_ROWS, OLDER_ROW],
    };
    renderPage();
    const older = screen.getByRole("region", { name: "CAB Approval approvers" });
    expect(older).toHaveTextContent("1. CAB Approval");
    expect(older).toHaveTextContent("Approved");
    expect(within(older).getByText("Carol Silva")).toBeInTheDocument();
    const authority = screen.getByRole("region", { name: "Standard Change Authority approvers" });
    expect(authority).toHaveTextContent("2. Standard Change Authority");
    expect(authority).toHaveTextContent("Approved");
    expect(screen.queryByRole("button", { name: /^(Approve|Authorise)$/ })).not.toBeInTheDocument();
    // The authority's approval applied it.
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Approved by the Standard Change Authority");
    expect(screen.getByRole("region", { name: "Approval" })).not.toHaveTextContent("two different CAB members");
  });

  /** The one-approval CAB stage of a proposal returned to draft when the CAB stage was removed (0295): never decided, its rows cancelled. */
  const RETURNED_STAGE = { id: "st-0", label: "CAB Approval", requiredApprovals: 1, approvals: 0, decision: null };
  const RETURNED_ROW = { id: "a-0", stageId: "st-0", approver: { id: "u-2", name: "Carol Silva", email: "carol@example.com" }, state: "CANCELLED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: false };

  it("shows an old stage of a proposal submitted again as not decided, numbered before the stage it waits on", () => {
    mocks.proposal = {
      ...CAB_REVIEW,
      stages: [RETURNED_STAGE, { ...CAB_STAGE, approvals: 0 }],
      approvers: [RETURNED_ROW, { ...CAB_REVIEW.approvers![0]!, canDecide: false }, { ...CAB_REVIEW.approvers![1]!, state: "REQUESTED", comments: null }],
    };
    renderPage();
    const [older, current] = screen.getAllByRole("region", { name: "CAB Approval approvers" });
    expect(older).toHaveTextContent("1. CAB Approval");
    expect(older).toHaveTextContent("Not decided");
    expect(older).not.toHaveTextContent("Waiting for a decision");
    expect(within(older!).getByText("Carol Silva")).toBeInTheDocument();
    expect(current).toHaveTextContent("2. CAB Approval");
    expect(current).toHaveTextContent("0 of 2 approvals");
    expect(screen.getByRole("region", { name: "Approval" })).toHaveTextContent("two different CAB members");
  });

  it("shows the cancelled CAB stage of a proposal returned to draft as not decided", () => {
    mocks.proposal = { ...DRAFT, stages: [RETURNED_STAGE], approvers: [RETURNED_ROW] };
    renderPage();
    const older = screen.getByRole("region", { name: "CAB Approval approvers" });
    expect(older).not.toHaveTextContent("1. ");
    expect(older).toHaveTextContent("Not decided");
    expect(older).not.toHaveTextContent("Waiting for a decision");
    expect(screen.queryByRole("button", { name: /^(Approve|Authorise)$/ })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Approval" })).toHaveTextContent("two different CAB members");
  });

  it("lets a CAB member cancel someone else's proposal, with a reason", () => {
    mocks.proposal = { ...CAB_REVIEW, canCancel: true };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Cancel proposal" }));
    const dialog = screen.getByRole("dialog", { name: "Cancel SCP0000001?" });
    const confirm = within(dialog).getByRole("button", { name: "Cancel the proposal" });
    expect(confirm).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /reason/i }), { target: { value: "The proposer left" } });
    fireEvent.click(confirm);
    expect(mocks.cancel).toHaveBeenCalledWith({ id: "scp-1", reason: "The proposer left" }, expect.anything());
  });

  it("shows a stage left open by a cancellation as cancelled, not as waiting", () => {
    mocks.proposal = { ...CAB_REVIEW, state: "cancelled", decisionComment: "The proposer left" };
    renderPage();
    const cab = screen.getByRole("region", { name: "CAB Approval approvers" });
    expect(cab).toHaveTextContent("Cancelled");
    expect(cab).not.toHaveTextContent("1 of 2 approvals");
  });

  it("shows the summary to whoever reviews it", () => {
    mocks.proposal = CAB_REVIEW;
    renderPage();
    expect(screen.getByRole("region", { name: "What the template specifies" })).toHaveTextContent(
      "SummaryRotates a gateway's TLS certificate.",
    );
  });

  it("names the stage that settled an older proposal decided under the oldest flow", () => {
    mocks.proposal = { ...CAB_REVIEW, state: "approved", stages: [OLDER_STAGE], approvers: [OLDER_ROW] };
    renderPage();
    const decision = screen.getByRole("region", { name: "Decision" });
    // A one-approval "CAB Approval" stage is not today's two-approval one.
    expect(decision).toHaveTextContent("Approved at the CAB Approval stage");
    expect(decision).not.toHaveTextContent("by the CAB");
    // Its one stage is shown, unnumbered, by its own label.
    expect(screen.getByRole("region", { name: "CAB Approval approvers" })).not.toHaveTextContent("1. ");
    expect(screen.getByRole("region", { name: "Approval" })).not.toHaveTextContent("two different CAB members");
  });

  it("names the stage that rejected an older proposal", () => {
    mocks.proposal = { ...CAB_REVIEW, state: "rejected", stages: [{ ...OLDER_STAGE, decision: "REJECTED" }], approvers: [] };
    renderPage();
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Rejected at the CAB Approval stage");
  });

  it("says who ended a cancelled proposal, and why", () => {
    mocks.proposal = {
      ...CAB_REVIEW,
      state: "cancelled",
      decidedOn: "2026-11-12T00:00:00Z",
      decidedBy: { id: "u-5", name: "Hana Head", email: "hana@example.com" },
      decisionComment: "The proposer left",
    };
    renderPage();
    const decision = screen.getByRole("region", { name: "Decision" });
    expect(decision).toHaveTextContent("Cancelled by a CAB member");
    expect(decision).toHaveTextContent("Hana Head");
    expect(decision).toHaveTextContent("The proposer left");
  });

  it("says an older proposal was cancelled by the Standard Change Authority when its stage was that group's", () => {
    mocks.proposal = {
      ...CAB_REVIEW,
      state: "cancelled",
      stages: [{ ...AUTHORITY_STAGE, approvals: 0, decision: null }],
      approvers: [],
      decisionComment: "The proposer left",
    };
    renderPage();
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Cancelled by the Standard Change Authority");
  });

  // Before 2026-11-10 only the Standard Change Authority could cancel someone
  // else's proposal, whatever stage it had reached: a draft has none, and the
  // oldest flow's one-approval CAB stage was not the CAB's to end.
  it.each([
    ["a draft with no stage", []],
    ["one in the oldest flow's one-approval CAB stage", [{ ...OLDER_STAGE, decision: null }]],
  ])("says %s cancelled before 2026-11-10 was cancelled by the Standard Change Authority", (_, stages) => {
    mocks.proposal = { ...CAB_REVIEW, state: "cancelled", stages, approvers: [], decidedOn: "2026-11-09T23:59:00Z" };
    renderPage();
    const decision = screen.getByRole("region", { name: "Decision" });
    expect(decision).toHaveTextContent("Cancelled by the Standard Change Authority");
    expect(decision).not.toHaveTextContent("CAB member");
  });

  it("says a draft cancelled since 2026-11-10 was cancelled by a CAB member", () => {
    mocks.proposal = { ...CAB_REVIEW, state: "cancelled", stages: [], approvers: [], decidedOn: "2026-11-10T08:00:00Z" };
    renderPage();
    expect(screen.getByRole("region", { name: "Decision" })).toHaveTextContent("Cancelled by a CAB member");
  });
});

describe("StandardChangeProposalPage — part of Change requests", () => {
  it("says where it sits: Change requests › Standard changes › Proposals › its number", () => {
    renderPage();
    const trail = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(within(trail).getAllByRole("link").map((l) => l.textContent)).toEqual([
      "Change requests",
      "Standard changes",
      "Proposals",
    ]);
    expect(within(trail).getByRole("link", { name: "Change requests" })).toHaveAttribute(
      "href",
      "/operations/change-requests",
    );
    expect(within(trail).getByRole("link", { name: "Standard changes" })).toHaveAttribute(
      "href",
      "/operations/change-requests?view=standard",
    );
    // All proposals, not only the viewer's: the proposals view's default scope.
    expect(within(trail).getByRole("link", { name: "Proposals" })).toHaveAttribute(
      "href",
      "/operations/change-requests?view=standard&show=proposals",
    );
    expect(within(trail).getByText("SCP0000001")).toHaveAttribute("aria-current", "page");
  });

  it("is recorded in Recent under its number and name, so a pin of it is not just 'Operations'", () => {
    renderPage();
    expect(mocks.recordView).toHaveBeenCalledWith({
      kind: "standard_change",
      id: "scp-1",
      title: "SCP0000001 · Rotate a TLS certificate",
      subtitle: "Standard change proposal",
      href: "/operations/standard-change-proposals/scp-1",
    });
  });

  it("is recorded as an untitled proposal while it has no name", () => {
    mocks.proposal = { ...DRAFT, name: "" };
    renderPage();
    expect(mocks.recordView).toHaveBeenCalledWith(
      expect.objectContaining({ title: "SCP0000001 · Untitled proposal" }),
    );
  });

  it("does not carry the Operations-wide heading: its own heading is the proposal's name", () => {
    renderPage();
    expect(screen.queryByText(/service requests|incidents|problems|runbooks/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Operations" })).not.toBeInTheDocument();
  });

  it("goes Back to all standard change proposals inside Change requests when it was not opened from a list", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests?view=standard&show=proposals");
  });

  it("goes Back to where it was opened from", () => {
    renderPage({ from: "/operations/change-requests/chg-1" });
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-1");
  });
});

describe("StandardChangeProposalPage — the CAB's history, in front of the approvers", () => {
  const REVIEW: BeStandardChangeProposal = {
    ...DRAFT,
    state: "authority_review",
    canEdit: false,
    canSubmit: false,
    canWithdraw: false,
    submittedOn: "2026-10-02T00:00:00Z",
    stages: [{ id: "st-1", label: "CAB Approval", requiredApprovals: 2, approvals: 0, decision: null }],
    approvers: [
      { id: "a-1", stageId: "st-1", approver: { id: "u-2", name: "Hana Head", email: "hana@example.com" }, state: "REQUESTED", comments: null, updatedOn: "2026-10-02T00:00:00Z", canDecide: true },
    ],
    evidence: [
      { id: "chg-1", number: "CHG0000001", subject: "Rotated the certificate on gw-1", state: "closed", closedOn: "2026-09-01T00:00:00Z", cabRejected: true, hasDeviation: false },
      { id: "chg-2", number: "CHG0000002", subject: "Rotated the certificate on gw-2", state: "closed", closedOn: "2026-08-01T00:00:00Z", cabRejected: false, hasDeviation: true },
    ],
    cabHistory: {
      cabRejectionCount: 2,
      cabRejections: [
        {
          changeRequest: { id: "chg-9", number: "CHG0000009", subject: "Rotate the certificate on gw-9", state: "canceled" },
          member: false,
          rejectedOn: "2026-10-01T00:00:00Z",
          rejectedBy: "Alice CAB",
          comment: "Not during the freeze",
        },
        {
          changeRequest: { id: "chg-1", number: "CHG0000001", subject: "Rotated the certificate on gw-1", state: "closed" },
          member: true,
          rejectedOn: "2026-08-20T00:00:00Z",
          rejectedBy: "Bob CAB",
          comment: "",
        },
      ],
      // As entity-service sends it: a rejection's comment is one of the comments too.
      cabCommentCount: 2,
      cabComments: [
        {
          changeRequest: { id: "chg-9", number: "CHG0000009", subject: "Rotate the certificate on gw-9", state: "canceled" },
          member: false,
          approverName: "Alice CAB",
          state: "REJECTED",
          comment: "Not during the freeze",
          decidedOn: "2026-10-01T00:00:00Z",
        },
        {
          changeRequest: { id: "chg-2", number: "CHG0000002", subject: "Rotated the certificate on gw-2", state: "closed" },
          member: true,
          approverName: "Carol CAB",
          state: "APPROVED",
          comment: "Add the verification steps",
          decidedOn: "2026-07-25T00:00:00Z",
        },
      ],
    },
  };

  it("puts a CAB history section with a warning headline directly above the Approval section", () => {
    mocks.proposal = REVIEW;
    renderPage();
    const history = screen.getByRole("region", { name: "CAB history" });
    const approval = screen.getByRole("region", { name: "Approval" });
    expect(history.compareDocumentPosition(approval) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(history.nextElementSibling).toBe(approval);
    expect(within(history).getByRole("alert")).toHaveTextContent("The CAB rejected 2 change requests like this one");
    const rejections = within(history).getByRole("list", { name: "CAB rejections" });
    const items = within(rejections).getAllByRole("listitem");
    expect(items[0]).toHaveTextContent("CHG0000009");
    expect(items[0]).toHaveTextContent("rejected");
    expect(items[0]).toHaveTextContent("by Alice CAB");
    expect(items[0]).toHaveTextContent("a look-alike");
    expect(items[0]).toHaveTextContent('"Not during the freeze"');
    expect(items[1]).not.toHaveTextContent("look-alike");
    const comments = within(history).getByRole("list", { name: "CAB comments" });
    expect(comments).toHaveTextContent("Carol CAB approved");
    expect(comments).toHaveTextContent('"Add the verification steps"');
    fireEvent.click(within(rejections).getByRole("button", { name: "CHG0000009" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-9", {
      state: { from: "/operations/standard-change-proposals/scp-1" },
    });
  });

  it("says when the CAB has said nothing", () => {
    mocks.proposal = { ...REVIEW, cabHistory: { cabRejectionCount: 0, cabRejections: [], cabCommentCount: 0, cabComments: [] } };
    renderPage();
    const history = screen.getByRole("region", { name: "CAB history" });
    expect(history).toHaveTextContent("No CAB rejections or comments");
    expect(within(history).queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows no CAB history section where the backend sends none", () => {
    mocks.proposal = { ...REVIEW, cabHistory: undefined };
    renderPage();
    expect(screen.queryByRole("region", { name: "CAB history" })).not.toBeInTheDocument();
  });

  it("marks the evidence the CAB rejected and the evidence that deviated from its plan", () => {
    mocks.proposal = REVIEW;
    renderPage();
    const rows = within(screen.getByRole("table", { name: "Evidence" })).getAllByRole("row");
    expect(rows[1]).toHaveTextContent("Rejected by the CAB");
    expect(rows[1]).not.toHaveTextContent("Deviated from plan");
    expect(rows[2]).toHaveTextContent("Deviated from plan");
    expect(rows[2]).not.toHaveTextContent("Rejected by the CAB");
  });

  it("sums the history up in the decision dialog, as the approver decides", () => {
    mocks.proposal = REVIEW;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Authorise" }));
    const dialog = screen.getByRole("dialog", { name: /authorise the proposal/i });
    expect(within(dialog).getByRole("note", { name: "CAB history summary" })).toHaveTextContent(
      'The CAB rejected 2 change requests like this one and left 2 comments. Latest: "Not during the freeze"',
    );
  });

  it("sums it up when rejecting too", () => {
    mocks.proposal = REVIEW;
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Reject" }));
    const dialog = screen.getByRole("dialog", { name: /reject the proposal/i });
    expect(within(dialog).getByRole("note", { name: "CAB history summary" })).toHaveTextContent("The CAB rejected 2 change requests");
  });
});
