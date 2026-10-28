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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
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
}));

const idle = (mutate: ReturnType<typeof vi.fn>) => ({ mutate, isPending: false });

vi.mock("@context/current-user/usePortalAccess", () => ({ usePortalAccess: () => mocks.access }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => mocks.navigate }));
vi.mock("@features/csm-operations/api/useStandardChanges", () => ({
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
  owner: { id: "u-1", name: "Jane Doe", email: "jane@example.com" },
  currentVersion: CURRENT,
  reviewRequired: false,
  reviewRequiredReason: null,
  reviewRequiredOn: null,
  reviewRequiredChangeRequest: null,
  openProposal: null,
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

function renderPage(): void {
  render(
    <MemoryRouter initialEntries={["/operations/standard-changes/tpl-1"]}>
      <Routes>
        <Route path="/operations/standard-changes/:id" element={<StandardChangeTemplateDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

const HERE = { state: { from: "/operations/standard-changes/tpl-1" } };

beforeEach(() => {
  mocks.navigate.mockReset();
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
    expect(mocks.suspend).toHaveBeenCalledWith("tpl-1", expect.anything());
    act(() => mocks.suspend.mock.calls[0]![1].onError(new Error("only the template's owner or a CAB member can suspend it")));
    expect(screen.getByText("only the template's owner or a CAB member can suspend it")).toBeInTheDocument();
  });

  it("resumes a suspended template, which cannot be raised from meanwhile", () => {
    mocks.template = { ...TEMPLATE, state: "suspended" };
    renderPage();
    expect(screen.getByText(/no change request can be raised from this template/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /raise a change request/i })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Resume" }));
    expect(mocks.resume).toHaveBeenCalledWith("tpl-1", expect.anything());
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

  it("says why, links to the change request, and clears it", () => {
    mocks.template = FLAGGED;
    renderPage();
    const flag = screen.getByRole("region", { name: "Review needed" });
    expect(flag).toHaveTextContent("CHG0001234 was rolled back");
    fireEvent.click(within(flag).getByRole("button", { name: "CHG0001234" }));
    expect(mocks.navigate).toHaveBeenCalledWith("/operations/change-requests/chg-1", HERE);
    fireEvent.click(within(flag).getByRole("button", { name: "Clear review" }));
    expect(mocks.clearReview).toHaveBeenCalledWith("tpl-1", expect.anything());
    act(() => mocks.clearReview.mock.calls[0]![1].onError(new Error("only the owner or a CAB member can clear it")));
    expect(screen.getByText("only the owner or a CAB member can clear it")).toBeInTheDocument();
  });

  it("shows no flag when none is raised", () => {
    renderPage();
    expect(screen.queryByRole("region", { name: "Review needed" })).not.toBeInTheDocument();
  });
});
