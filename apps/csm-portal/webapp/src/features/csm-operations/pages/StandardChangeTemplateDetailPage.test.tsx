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

import { act, fireEvent, render, screen, waitForElementToBeRemoved, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.hoisted(() => {
  window.config = { ...window.config, CSM_PORTAL_BACKEND_BASE_URL: "http://localhost:9090" };
});

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  access: { canWrite: true },
  template: undefined as unknown,
  versionCalls: [] as Array<[string | undefined, string | undefined]>,
  suspend: vi.fn(),
  resume: vi.fn(),
  clearReview: vi.fn(),
  create: vi.fn(),
  recordView: vi.fn(),
}));

const idle = (mutate: ReturnType<typeof vi.fn>) => ({ mutate, isPending: false });

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@features/csm-recent/hooks/useRecentViews", () => ({ useRecordRecentView: () => mocks.recordView }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
  MAX_SUSPEND_REASON_LENGTH: 1000,
  useGetStandardChangeTemplate: () => ({ data: mocks.template, isLoading: false, isError: false, refetch: vi.fn() }),
  useGetStandardChangeTemplateVersion: (templateId?: string, versionId?: string) => {
    mocks.versionCalls.push([templateId, versionId]);
    return {
      data: versionId ? OLD_VERSION : undefined,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    };
  },
  useSuspendStandardChangeTemplate: () => idle(mocks.suspend),
  useResumeStandardChangeTemplate: () => idle(mocks.resume),
  useClearStandardChangeTemplateReview: () => idle(mocks.clearReview),
  useCreateStandardChangeProposal: () => idle(mocks.create),
}));

import StandardChangeTemplateDetailPage from "@features/csm-operations/pages/StandardChangeTemplateDetailPage";

const CONTENT = {
  justification: null,
  riskImpactAnalysis: null,
  backoutPlan: "<p>Restore the previous certificate.</p>",
  testPlan: "<p>Check the endpoint.</p>",
  category: "software",
  impact: "low",
  risk: "low",
  rollbackDurationText: "15 mins",
  isPlanningVisibleToCustomers: false,
  customerApprovalRequired: false,
  customerReviewRequired: false,
};
const CURRENT = {
  ...CONTENT,
  id: "ver-2",
  templateId: "tpl-1",
  version: 2,
  proposal: { id: "scp-2", name: "SCP0000002" },
  approvedOn: "2026-10-01T00:00:00Z",
  approvedBy: "cab@example.com",
  subject: "Rotate the TLS certificate on {{host}}",
  description: "<p>Rotate the certificate on {{host}}.</p>",
  implementationPlan: "<p>Steps for {{host}}.</p>",
  parameters: [{ key: "host", label: "Host", type: "text", required: true, helpText: "The gateway host." }],
};
const OLD_VERSION = {
  ...CONTENT,
  id: "ver-1",
  templateId: "tpl-1",
  version: 1,
  proposal: { id: "scp-1", name: "SCP0000001" },
  approvedOn: "2026-01-01T00:00:00Z",
  approvedBy: "cab@example.com",
  subject: "Rotate the certificate (first version)",
  description: "<p>Old text.</p>",
  implementationPlan: "<p>Old steps.</p>",
  parameters: [],
};
const TEMPLATE = {
  id: "tpl-1",
  number: "STD0000001",
  name: "Rotate a TLS certificate",
  state: "active",
  customer: { id: "acct-1", name: "Example Corp" },
  owner: { id: "u-1", name: "Jane Doe", email: "jane@example.com" },
  currentVersion: CURRENT,
  reviewRequired: false,
  reviewRequiredReason: null,
  reviewRequiredOn: null,
  reviewRequiredChangeRequest: null,
  suspendedReason: null,
  suspendedOn: null,
  suspendedBy: null,
  openProposal: null,
  // What the viewer may do: the server's answer (its owner or the CAB
  // suspends; only the CAB resumes or clears the flag).
  canSuspend: true,
  canResume: false,
  canClearReview: false,
  usage: { total: 12, open: 2, closed: 9, rolledBack: 1, canceled: 0, lastUsedOn: "2026-10-20T00:00:00Z" },
  createdOn: "",
  createdBy: "",
  updatedOn: "",
  updatedBy: "",
  versions: [
    { id: "ver-2", version: 2, proposal: { id: "scp-2", name: "SCP0000002" }, approvedOn: "2026-10-01T00:00:00Z", approvedBy: "cab@example.com" },
    { id: "ver-1", version: 1, proposal: { id: "scp-1", name: "SCP0000001" }, approvedOn: "2026-01-01T00:00:00Z", approvedBy: "cab@example.com" },
  ],
};

function renderPage(state?: { from: string }): void {
  render(
    <MemoryRouter initialEntries={[{ pathname: "/operations/standard-changes/tpl-1", state }]}>
      <Routes>
        <Route path="/operations/standard-changes/:id" element={<StandardChangeTemplateDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

const HERE = { state: { from: "/operations/standard-changes/tpl-1" } };

beforeEach(() => {
  mocks.navigate.mockReset();
  mocks.recordView.mockReset();
  mocks.access = { canWrite: true };
  mocks.template = TEMPLATE;
  mocks.versionCalls = [];
  mocks.suspend.mockReset();
  mocks.resume.mockReset();
  mocks.clearReview.mockReset();
  mocks.create.mockReset();
});

describe("StandardChangeTemplateDetailPage — content", () => {
  it("shows the current version with its placeholders highlighted, its parameters and its usage", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: "Rotate a TLS certificate" })).toBeInTheDocument();
    expect(screen.getByText("Version 2")).toBeInTheDocument();
    const plan = screen.getByRole("region", { name: "Implementation plan" });
    expect(plan.querySelector('mark[data-param="host"]')).toHaveTextContent("{{host}}");
    const params = screen.getByRole("table", { name: "Parameters" });
    expect(within(params).getByText("{{host}}")).toBeInTheDocument();
    expect(within(params).getByText("The gateway host.")).toBeInTheDocument();
    expect(screen.getByText(/12 \(2 open, 9 closed, 1 rolled back, 0 canceled\)/)).toBeInTheDocument();
  });

  it("opens an older version from the history, and goes back to the current one", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "View version 1" }));
    expect(mocks.versionCalls.at(-1)).toEqual(["tpl-1", "ver-1"]);
    expect(screen.getByText(/version 1 \(not the current version\)/i)).toBeInTheDocument();
    expect(screen.getByText("Rotate the certificate (first version)")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /back to the current version/i }));
    expect(screen.queryByText(/not the current version/i)).not.toBeInTheDocument();
    expect(mocks.versionCalls.at(-1)).toEqual(["tpl-1", undefined]);
  });
});

describe("StandardChangeTemplateDetailPage — its customer", () => {
  it("says whose template it is, linking to the customer, and lists the customer beside the owner", () => {
    renderPage();
    const link = screen.getByRole("button", { name: "Example Corp" });
    expect(link.closest("p")).toHaveTextContent("For Example Corp: used only for this customer's projects.");
    fireEvent.click(link);
    expect(mocks.navigate).toHaveBeenCalledWith("/customers/accounts/acct-1", HERE);
    expect(screen.getByText("Customer").nextElementSibling).toHaveTextContent("Example Corp");
    expect(screen.queryByRole("region", { name: "No customer recorded" })).not.toBeInTheDocument();
  });

  it("says an older template with no customer recorded cannot be used, and offers no change request from it", () => {
    mocks.template = { ...TEMPLATE, customer: null };
    renderPage();
    const note = screen.getByRole("region", { name: "No customer recorded" });
    expect(note).toHaveTextContent(/belongs to one customer and is used only for that customer's projects/i);
    expect(note).toHaveTextContent(/no change request can be raised from it/i);
    expect(screen.getByRole("button", { name: /raise a change request/i })).toBeDisabled();
    expect(screen.getByText("Customer").nextElementSibling).toHaveTextContent("—");
  });
});

describe("StandardChangeTemplateDetailPage — actions", () => {
  it("raises a change request from it on the create form, preselected", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /raise a change request/i }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/new?standardChangeTemplateId=tpl-1", HERE);
  });

  it.each([
    ["Propose a revision", "modify"],
    ["Propose retirement", "retire"],
  ])("%s: starts a %s proposal from the template and opens it", (label, kind) => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: label }));
    expect(mocks.create).toHaveBeenCalledWith({ kind, templateId: "tpl-1" }, expect.anything());
    act(() => mocks.create.mock.calls[0]![1].onSuccess({ id: "scp-9" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-9", HERE);
  });

  it("shows why a proposal could not be started", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Propose a revision" }));
    act(() => mocks.create.mock.calls[0]![1].onError(new Error("the template has a revision open already")));
    expect(screen.getByText("the template has a revision open already")).toBeInTheDocument();
  });

  it("points at an open revision or retirement instead of offering another", () => {
    mocks.template = { ...TEMPLATE, openProposal: { id: "scp-5", name: "SCP0000005" } };
    renderPage();
    expect(screen.getByRole("button", { name: "Propose a revision" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Propose retirement" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "SCP0000005" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-5", HERE);
  });

  it("suspends it after a confirmation, and shows the server's reason when refused", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    const dialog = screen.getByRole("dialog", { name: /suspend STD0000001/i });
    fireEvent.click(within(dialog).getByRole("button", { name: "Suspend" }));
    expect(mocks.suspend).toHaveBeenCalledWith({ id: "tpl-1", reason: "" }, expect.anything());
    act(() => mocks.suspend.mock.calls[0]![1].onError(new Error("only the template's owner or a CAB member can suspend it")));
    expect(screen.getByText("only the template's owner or a CAB member can suspend it")).toBeInTheDocument();
  });

  it("sends the reason typed in the confirmation, and starts the next one empty", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    const dialog = screen.getByRole("dialog", { name: /suspend STD0000001/i });
    const reason = within(dialog).getByRole("textbox", { name: /reason \(optional\)/i });
    expect(reason).toHaveAttribute("maxlength", "1000");
    fireEvent.change(reason, { target: { value: "Rolled back twice on prod" } });
    expect(within(dialog).getByText(/25\/1000/)).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Suspend" }));
    expect(mocks.suspend).toHaveBeenCalledWith({ id: "tpl-1", reason: "Rolled back twice on prod" }, expect.anything());
    // The confirmation closes once the server agrees.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    act(() => mocks.suspend.mock.calls[0]![1].onSuccess());

    await waitForElementToBeRemoved(() => screen.queryByRole("dialog"));
    fireEvent.click(screen.getByRole("button", { name: "Suspend" }));
    expect(screen.getByRole("textbox", { name: /reason \(optional\)/i })).toHaveValue("");
  });

  it("shows why, by whom and when a suspended template was suspended", () => {
    mocks.template = {
      ...TEMPLATE,
      state: "suspended",
      suspendedReason: "Rolled back twice on prod",
      suspendedOn: "2026-10-20T09:30:00Z",
      suspendedBy: "carol.silva@example.com",
    };
    renderPage();
    const note = screen.getByRole("region", { name: "Suspended" });
    expect(note).toHaveTextContent(/^Suspended: Rolled back twice on prod \(by carol\.silva@example\.com on .*2026.*\)/);
    expect(note).toHaveTextContent(/no change request can be raised from this template/i);
  });

  it("says only that it is suspended when no reason was given", () => {
    mocks.template = { ...TEMPLATE, state: "suspended", suspendedOn: "2026-10-20T09:30:00Z", suspendedBy: "carol.silva@example.com" };
    renderPage();
    const note = screen.getByRole("region", { name: "Suspended" });
    expect(note).toHaveTextContent(/^Suspended \(by carol\.silva@example\.com on .*\)No change request/);
  });

  it("shows no suspension note on an active template", () => {
    renderPage();
    expect(screen.queryByRole("region", { name: "Suspended" })).not.toBeInTheDocument();
  });

  it("resumes a suspended template, with a note, which cannot be raised from meanwhile", () => {
    mocks.template = { ...TEMPLATE, state: "suspended", canSuspend: false, canResume: true };
    renderPage();
    expect(screen.getByText(/no change request can be raised from this template/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /raise a change request/i })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    const dialog = screen.getByRole("dialog");
    const resume = within(dialog).getByRole("button", { name: "Resume" });
    expect(resume).toBeDisabled();
    fireEvent.change(within(dialog).getByRole("textbox", { name: /note/i }), { target: { value: "Root cause fixed" } });
    fireEvent.click(resume);
    expect(mocks.resume).toHaveBeenCalledWith({ id: "tpl-1", note: "Root cause fixed" }, expect.anything());
  });

  it("keeps a refused resume open with its note and the server's reason", () => {
    mocks.template = { ...TEMPLATE, state: "suspended", canSuspend: false, canResume: true };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    const dialog = screen.getByRole("dialog");
    fireEvent.change(within(dialog).getByRole("textbox", { name: /note/i }), { target: { value: "Root cause fixed" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Resume" }));
    act(() => mocks.resume.mock.calls[0]![1].onError(new Error("only a CAB member can do this")));
    expect(within(screen.getByRole("dialog")).getByText("only a CAB member can do this")).toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByRole("textbox", { name: /note/i })).toHaveValue("Root cause fixed");
  });

  it("says who can resume a suspended template, and offers no Resume to anyone else", () => {
    mocks.template = { ...TEMPLATE, state: "suspended", canSuspend: false, canResume: false };
    renderPage();
    expect(screen.queryByRole("button", { name: "Resume" })).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Suspended" })).toHaveTextContent(
      "Only a CAB member can resume it.",
    );
  });

  it("offers no Suspend to whoever may not suspend it", () => {
    mocks.template = { ...TEMPLATE, canSuspend: false };
    renderPage();
    expect(screen.queryByRole("button", { name: "Suspend" })).not.toBeInTheDocument();
  });

  it("shows the template's summary and its history", () => {
    mocks.template = {
      ...TEMPLATE,
      currentVersion: { ...CURRENT, summary: "Rotates a gateway's TLS certificate. Not for client certificates." },
      history: [
        { action: "resumed", note: "Root cause fixed", changeRequest: null, actor: "head@example.com", createdOn: "2026-10-21T00:00:00Z" },
        {
          action: "review_flagged",
          note: "CHG0001234 was rolled back",
          changeRequest: { id: "chg-1", name: "CHG0001234" },
          actor: "csm",
          createdOn: "2026-10-20T00:00:00Z",
        },
      ],
    };
    renderPage();
    expect(screen.getByText("Rotates a gateway's TLS certificate. Not for client certificates.")).toBeInTheDocument();
    const rows = within(screen.getByRole("table", { name: "Template history" })).getAllByRole("row");
    expect(rows[1]).toHaveTextContent("ResumedRoot cause fixedhead@example.com");
    expect(rows[2]).toHaveTextContent("Flagged for review");
    fireEvent.click(within(rows[2]!).getByRole("button", { name: "CHG0001234" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-1", HERE);
  });

  it("records a retirement in the history, with the proposal that retired it", () => {
    mocks.template = {
      ...TEMPLATE,
      state: "retired",
      history: [
        {
          action: "retired",
          note: "<p>No longer <strong>used</strong></p>",
          changeRequest: null,
          proposal: { id: "scp-9", name: "SCP0000009" },
          actor: "auth2@example.com",
          createdOn: "2026-11-06T00:00:00Z",
        },
      ],
    };
    renderPage();
    const rows = within(screen.getByRole("table", { name: "Template history" })).getAllByRole("row");
    // The rationale is rich text: rendered, never shown as markup.
    expect(rows[1]).toHaveTextContent("RetiredNo longer used (SCP0000009)auth2@example.com");
    expect(rows[1]).not.toHaveTextContent("<p>");
    expect(within(rows[1]!).getByText("used").tagName).toBe("STRONG");
    fireEvent.click(within(rows[1]!).getByRole("button", { name: "SCP0000009" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/standard-change-proposals/scp-9", HERE);
  });

  it("shows no review flag on a retired template: it is never used again", () => {
    mocks.template = { ...TEMPLATE, state: "retired", reviewRequired: true, reviewRequiredReason: "Rolled back" };
    renderPage();
    expect(screen.queryByRole("region", { name: "Review needed" })).not.toBeInTheDocument();
    expect(screen.queryByText(/can still be used/)).not.toBeInTheDocument();
  });

  it("offers nothing but reading for a retired template", () => {
    mocks.template = { ...TEMPLATE, state: "retired" };
    renderPage();
    expect(screen.getByRole("button", { name: /raise a change request/i })).toBeDisabled();
    for (const name of ["Propose a revision", "Propose retirement", "Suspend", "Resume"]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
  });

  it("offers no write action without write access", () => {
    mocks.access = { canWrite: false };
    renderPage();
    for (const name of ["Raise a change request", "Propose a revision", "Propose retirement", "Suspend"]) {
      expect(screen.queryByRole("button", { name })).not.toBeInTheDocument();
    }
  });
});

describe("StandardChangeTemplateDetailPage — review flag", () => {
  const FLAGGED = {
    ...TEMPLATE,
    reviewRequired: true,
    reviewRequiredReason: "CHG0001234 was rolled back",
    reviewRequiredOn: "2026-10-20T00:00:00Z",
    reviewRequiredChangeRequest: { id: "chg-1", name: "CHG0001234" },
  };

  it("says why, links to the change request, and clears it with a note", () => {
    mocks.template = { ...FLAGGED, canClearReview: true };
    renderPage();
    const flag = screen.getByRole("region", { name: "Review needed" });
    expect(flag).toHaveTextContent("CHG0001234 was rolled back");
    fireEvent.click(within(flag).getByRole("button", { name: "CHG0001234" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-1", HERE);
    fireEvent.click(within(flag).getByRole("button", { name: "Clear review" }));
    const dialog = screen.getByRole("dialog", { name: /clear the review flag/i });
    fireEvent.change(within(dialog).getByRole("textbox", { name: /note/i }), { target: { value: "One-off network fault" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Clear review" }));
    expect(mocks.clearReview).toHaveBeenCalledWith({ id: "tpl-1", note: "One-off network fault" }, expect.anything());
    act(() => mocks.clearReview.mock.calls[0]![1].onError(new Error("only a CAB member can do this")));
    expect(screen.getByText("only a CAB member can do this")).toBeInTheDocument();
  });

  it("offers Clear review only to whoever may clear it, and says who can", () => {
    mocks.template = FLAGGED;
    renderPage();
    const flag = screen.getByRole("region", { name: "Review needed" });
    expect(within(flag).queryByRole("button", { name: "Clear review" })).not.toBeInTheDocument();
    expect(flag).toHaveTextContent("a CAB member clears it once reviewed");
  });

  it("shows no flag when none is raised", () => {
    renderPage();
    expect(screen.queryByRole("region", { name: "Review needed" })).not.toBeInTheDocument();
  });
});

describe("StandardChangeTemplateDetailPage — part of Change requests", () => {
  it("says where it sits: Change requests › Standard changes › its number", () => {
    renderPage();
    const trail = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(within(trail).getAllByRole("link").map((l) => l.textContent)).toEqual(["Change requests", "Standard changes"]);
    expect(within(trail).getByRole("link", { name: "Change requests" })).toHaveAttribute(
      "href",
      "/operations/change-requests",
    );
    expect(within(trail).getByRole("link", { name: "Standard changes" })).toHaveAttribute(
      "href",
      "/operations/change-requests?view=standard",
    );
    expect(within(trail).getByText("STD0000001")).toHaveAttribute("aria-current", "page");
  });

  it("goes Back to Standard changes inside Change requests when it was not opened from a list", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests?view=standard");
  });

  it("is recorded in Recent under its number and name, so a pin of it is not just 'Operations'", () => {
    renderPage();
    expect(mocks.recordView).toHaveBeenCalledWith({
      kind: "standard_change",
      id: "tpl-1",
      title: "STD0000001 · Rotate a TLS certificate",
      subtitle: "Standard change template",
      href: "/operations/standard-changes/tpl-1",
    });
  });

  it("records nothing before the template has loaded", () => {
    mocks.template = undefined;
    renderPage();
    expect(mocks.recordView).not.toHaveBeenCalled();
  });

  it("goes Back to where it was opened from", () => {
    renderPage({ from: "/operations/change-requests/chg-1" });
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-1");
  });
});
