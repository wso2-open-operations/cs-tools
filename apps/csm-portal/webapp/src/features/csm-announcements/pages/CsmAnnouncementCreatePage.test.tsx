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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { AnnouncementRequest } from "@features/csm-announcements/types/announcementRequests";

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const showErrorMock = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  // The "view it" link after a successful dry run uses react-router's Link;
  // a plain anchor is enough for these tests, which only assert its
  // presence/href, not real client-side routing.
  Link: ({
    to,
    children,
    ...rest
  }: {
    to: string;
    children?: ReactNode;
  } & Record<string, unknown>) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
}));
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));
// The real multi-select searches the backend as the user types; stub it with
// a plain multi-value control so this file stays focused on the create form's
// own required-field and draft/submit behavior (the picker itself has its
// own tests via CsmAnnouncementsPage.test.tsx's filter-bar coverage).
vi.mock("@features/csm-cases/components/AsyncProjectMultiSelect", () => ({
  default: ({
    values,
    onChange,
  }: {
    values: string[];
    onChange: (next: string[]) => void;
  }) => (
    <select
      aria-label="Projects"
      multiple
      value={values}
      onChange={(e) =>
        onChange(Array.from(e.target.selectedOptions).map((o) => o.value))
      }
    >
      <option value="proj-1">Project One</option>
      <option value="proj-2">Project Two</option>
    </select>
  ),
}));
// CsmAnnouncementCreatePage imports BackendApiError from the real API client
// module, which reads window.config at module load and throws outside a
// configured runtime — mirrors CreateSecurityReportPage.test.tsx.
// useBackendApi is also mocked here: useResolveAnnouncementAudience calls it
// unconditionally (React Query hooks always run, even when `enabled: false`
// for the default "specific" scope these tests exercise), so a real client
// would hit the same window.config problem.
const projectSearchPostMock = vi.fn();
// useAnnouncementExcludedProjectKeys' GET call; defaults to no keys
// configured so most tests render no chips (an unconfigured deployment is
// the common case), overridden per-test where the chips themselves matter.
const excludedProjectKeysGetMock = vi.fn().mockResolvedValue({ excludedProjectKeys: [] });
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: projectSearchPostMock, get: excludedProjectKeysGetMock }),
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

// ---- announcement_requests hooks (Phase 2 draft/approval workflow) ----
function makeMutationMock<TResult>(defaultResult: TResult) {
  const mutateAsync = vi.fn().mockResolvedValue(defaultResult);
  return { mutate: vi.fn(), mutateAsync, isPending: false, isError: false, error: null as unknown };
}

const DRAFT: AnnouncementRequest = {
  id: "draft-1",
  kind: "customer",
  state: "draft",
  subject: "",
  description: "",
  isSecurityAnnouncement: false,
  audienceDefinition: {},
  createdBy: "you@wso2.com",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-01T10:00:00Z",
};

let createDraftMock = makeMutationMock<AnnouncementRequest>(DRAFT);
let updateDraftMock = makeMutationMock<AnnouncementRequest>(DRAFT);
let recordDryRunMock = makeMutationMock<AnnouncementRequest>(DRAFT);
let submitMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, state: "pending_approval" });

vi.mock("@features/csm-announcements/api/useCreateAnnouncementRequest", () => ({
  useCreateAnnouncementRequest: () => createDraftMock,
}));
vi.mock("@features/csm-announcements/api/useUpdateAnnouncementRequest", () => ({
  useUpdateAnnouncementRequest: () => updateDraftMock,
}));
vi.mock("@features/csm-announcements/api/useRecordAnnouncementRequestDryRun", () => ({
  useRecordAnnouncementRequestDryRun: () => recordDryRunMock,
}));
vi.mock("@features/csm-announcements/api/useSubmitAnnouncementRequest", () => ({
  useSubmitAnnouncementRequest: () => submitMock,
}));

// Imported after the mocks above so the module picks them up.
import CsmAnnouncementCreatePage from "@features/csm-announcements/pages/CsmAnnouncementCreatePage";

function renderPage(): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <CsmAnnouncementCreatePage />
    </QueryClientProvider>,
  );
}

function selectProjects(...values: string[]): void {
  const select = screen.getByLabelText("Projects") as HTMLSelectElement;
  Array.from(select.options).forEach((o) => {
    o.selected = values.includes(o.value);
  });
  fireEvent.change(select);
}

function fillSubjectAndDescription(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), {
    target: { value: "Scheduled maintenance" },
  });
  fireEvent.change(screen.getByLabelText("editor"), {
    target: { value: "<p>Maintenance window details.</p>" },
  });
}

/** Mocks the dry-run test-project lookup + case creation, which "Submit for approval" runs internally. */
function mockSuccessfulDryRun(): void {
  projectSearchPostMock.mockImplementation((url: string) => {
    if (url === "/projects/search") {
      return Promise.resolve({
        projects: [{ id: "dcpsub-id", name: "DCPSUB Test Project", key: "DCPSUB" }],
        total: 1,
        limit: 10,
        offset: 0,
        hasMore: false,
      });
    }
    return Promise.resolve({ id: "tag-1", label: "Dry Run", color: null });
  });
  postCaseMutateAsyncMock.mockResolvedValue({ id: "case-test-1", internalId: "WSO2-9001" });
}

describe("CsmAnnouncementCreatePage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
    projectSearchPostMock.mockReset();
    excludedProjectKeysGetMock.mockReset().mockResolvedValue({ excludedProjectKeys: [] });
    createDraftMock = makeMutationMock<AnnouncementRequest>(DRAFT);
    updateDraftMock = makeMutationMock<AnnouncementRequest>(DRAFT);
    recordDryRunMock = makeMutationMock<AnnouncementRequest>(DRAFT);
    submitMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, state: "pending_approval" });
  });

  it("keeps Save as draft disabled until subject and description are filled — no project selection required", () => {
    renderPage();
    const saveDraft = screen.getByRole("button", { name: /save as draft/i });
    expect(saveDraft).toBeDisabled();

    fillSubjectAndDescription();
    // No project picked at all — a draft can still be saved.
    expect(saveDraft).toBeEnabled();
  });

  it("keeps Submit for approval disabled until subject, description, and an audience are present", () => {
    renderPage();
    const submit = screen.getByRole("button", { name: /submit for approval/i });
    expect(submit).toBeDisabled();

    fillSubjectAndDescription();
    // Content filled but no project picked yet.
    expect(submit).toBeDisabled();

    selectProjects("proj-1");
    expect(submit).toBeEnabled();
  });

  it("clicking Submit for approval runs the dry run, creates the draft, records it, submits, and navigates — one action", async () => {
    mockSuccessfulDryRun();
    renderPage();
    fillSubjectAndDescription();
    selectProjects("proj-1");

    fireEvent.click(screen.getByRole("button", { name: /submit for approval/i }));

    await waitFor(() => expect(postCaseMutateAsyncMock).toHaveBeenCalled());
    await waitFor(() =>
      expect(createDraftMock.mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          kind: "customer",
          subject: "Scheduled maintenance",
          description: "<p>Maintenance window details.</p>",
          isSecurityAnnouncement: false,
          audienceDefinition: { scope: "specific", projectIds: ["proj-1"] },
        }),
      ),
    );
    await waitFor(() =>
      expect(recordDryRunMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1", caseId: "case-test-1" }),
    );
    await waitFor(() => expect(submitMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1" }));
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
  });

  it("does not create a draft, record a dry run, or submit when the test project can't be found", async () => {
    projectSearchPostMock.mockResolvedValue({ projects: [], total: 0, limit: 10, offset: 0, hasMore: false });
    renderPage();
    fillSubjectAndDescription();
    selectProjects("proj-1");

    fireEvent.click(screen.getByRole("button", { name: /submit for approval/i }));

    await waitFor(() => expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("DCPSUB")));
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    expect(createDraftMock.mutateAsync).not.toHaveBeenCalled();
    expect(recordDryRunMock.mutateAsync).not.toHaveBeenCalled();
    expect(submitMock.mutateAsync).not.toHaveBeenCalled();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("surfaces the server's own error message when the submit step is rejected", async () => {
    mockSuccessfulDryRun();
    submitMock.mutateAsync.mockRejectedValue(
      new Error("a dry run must be recorded before submitting for approval"),
    );
    renderPage();
    fillSubjectAndDescription();
    selectProjects("proj-1");

    fireEvent.click(screen.getByRole("button", { name: /submit for approval/i }));

    await waitFor(() =>
      expect(showErrorMock).toHaveBeenCalledWith("a dry run must be recorded before submitting for approval"),
    );
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("clicking Save as draft creates a new draft (no dry run) and navigates to the Pending tab", async () => {
    renderPage();
    fillSubjectAndDescription();

    fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

    await waitFor(() =>
      expect(createDraftMock.mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ kind: "customer", subject: "Scheduled maintenance" }),
      ),
    );
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
  });

  it("surfaces an error and does not navigate when Save as draft fails", async () => {
    createDraftMock.mutateAsync.mockRejectedValue(new Error("network down"));
    renderPage();
    fillSubjectAndDescription();

    fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

    await waitFor(() =>
      expect(showErrorMock).toHaveBeenCalledWith("Could not save this draft. Please try again."),
    );
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("attaches a fixed security label to the dry-run case and the draft payload when 'This is a security announcement' is checked", async () => {
    mockSuccessfulDryRun();
    renderPage();
    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("checkbox", { name: /this is a security announcement/i }));

    fireEvent.click(screen.getByRole("button", { name: /submit for approval/i }));

    await waitFor(() =>
      expect(createDraftMock.mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ isSecurityAnnouncement: true }),
      ),
    );
  });

  it("defaults to the customer-announcement form and switches to the EOL form", () => {
    renderPage();

    // Default kind: the customer form's own fields are present — including
    // the security-announcement checkbox, which only that form has.
    expect(screen.getByLabelText(/subject/i)).toBeInTheDocument();
    expect(screen.getByText(/this is a security announcement/i)).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /^product$/i })).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("radio", { name: /product version \/ eol announcement/i }),
    );

    // Switching kind unmounts the customer form entirely and shows the EOL
    // form instead — both forms have a Subject field, but only the EOL form
    // has the product/version pickers, and only the customer form has the
    // security-announcement checkbox.
    expect(screen.queryByText(/this is a security announcement/i)).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /^product$/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/subject/i)).toBeInTheDocument();
  });

  it("unchecking an exclusion drops it from the resolved-audience preview request", async () => {
    projectSearchPostMock.mockResolvedValue({ projects: [], total: 0, limit: 50, offset: 0, hasMore: false });
    renderPage();

    fireEvent.click(screen.getByRole("radio", { name: /all customer projects/i }));
    fireEvent.click(
      screen.getByRole("checkbox", { name: /exclude cloud support/i }),
    );

    await waitFor(() => {
      expect(projectSearchPostMock).toHaveBeenCalledWith(
        "/announcements/audience/search",
        expect.objectContaining({
          excludeSubscriptionTypes: undefined,
          excludeClosureStates: ["Restricted", "Suspended"],
        }),
      );
    });
  });

  it("shows the backend-configured excluded project keys as read-only chips under 'All customer projects'", async () => {
    excludedProjectKeysGetMock.mockResolvedValue({
      excludedProjectKeys: ["Apexia", "Veridian", "Veloxis"],
    });
    projectSearchPostMock.mockResolvedValue({ projects: [], total: 0, limit: 50, offset: 0, hasMore: false });
    renderPage();

    fireEvent.click(screen.getByRole("radio", { name: /all customer projects/i }));

    expect(await screen.findByText("Apexia")).toBeInTheDocument();
    expect(screen.getByText("Veridian")).toBeInTheDocument();
    expect(screen.getByText("Veloxis")).toBeInTheDocument();
  });

  describe("EOL / product-version flow", () => {
    function mockEolBackend(): void {
      projectSearchPostMock.mockImplementation((url: string) => {
        if (url === "/products/search") {
          return Promise.resolve({
            products: [
              { id: "prod-1", name: "WSO2 API Manager" },
              { id: "prod-2", name: "WSO2 Identity Server" },
            ],
            total: 2,
            limit: 20,
            offset: 0,
            hasMore: false,
          });
        }
        if (url === "/products/prod-1/versions/search") {
          return Promise.resolve({
            productVersions: [
              { id: "ver-1", version: "4.2.0", supportEolDate: "2025-09-08" },
            ],
            total: 1,
            limit: 20,
            offset: 0,
            hasMore: false,
          });
        }
        if (url === "/products/prod-2/versions/search") {
          return Promise.resolve({
            productVersions: [{ id: "ver-2", version: "7.0.0" }],
            total: 1,
            limit: 20,
            offset: 0,
            hasMore: false,
          });
        }
        if (url === "/deployed-products/projects/search") {
          return Promise.resolve({
            projects: [
              { id: "proj-a", name: "Project A" },
              { id: "proj-b", name: "Project B" },
            ],
            total: 2,
            limit: 20,
            offset: 0,
            hasMore: false,
          });
        }
        // /projects/search (dry run's test-project lookup) and tag-attach calls.
        return Promise.resolve({
          projects: [{ id: "dcpsub-id", name: "DCPSUB Test Project", key: "DCPSUB" }],
          total: 1,
          limit: 10,
          offset: 0,
          hasMore: false,
        });
      });
    }

    function switchToEolKind(): void {
      fireEvent.click(
        screen.getByRole("radio", { name: /product version \/ eol announcement/i }),
      );
    }

    async function selectProductAndVersion(): Promise<void> {
      const productSelect = screen.getByRole("combobox", { name: /^product$/i });
      fireEvent.mouseDown(productSelect);
      fireEvent.click(await screen.findByRole("option", { name: /wso2 api manager/i }));

      const versionSelect = screen.getByRole("combobox", { name: /^version$/i });
      fireEvent.mouseDown(versionSelect);
      fireEvent.click(await screen.findByRole("option", { name: /4\.2\.0/i }));
    }

    it("keeps Save as draft disabled until product, version, subject, and description are filled", async () => {
      mockEolBackend();
      renderPage();
      switchToEolKind();

      const saveDraft = screen.getByRole("button", { name: /save as draft/i });
      expect(saveDraft).toBeDisabled();

      await selectProductAndVersion();
      expect(saveDraft).toBeDisabled();

      fillSubjectAndDescription();
      await waitFor(() => expect(saveDraft).toBeEnabled());
    });

    it("Submit for approval is disabled until the resolved audience is non-empty", async () => {
      mockEolBackend();
      renderPage();
      switchToEolKind();
      fillSubjectAndDescription();

      // No product/version picked — resolvedAudience.total stays 0.
      const submit = screen.getByRole("button", { name: /submit for approval/i });
      expect(submit).toBeDisabled();
    });

    it("clicking Submit for approval runs the dry run, creates the eol draft, records it, and submits", async () => {
      mockEolBackend();
      postCaseMutateAsyncMock.mockResolvedValue({ id: "case-eol-test-1", internalId: "WSO2-9002" });
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();
      expect(await screen.findByText("Project A")).toBeInTheDocument();
      fillSubjectAndDescription();

      const submit = await screen.findByRole("button", { name: /submit for approval/i });
      await waitFor(() => expect(submit).toBeEnabled());
      fireEvent.click(submit);

      await waitFor(() => expect(postCaseMutateAsyncMock).toHaveBeenCalled());
      await waitFor(() =>
        expect(createDraftMock.mutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            kind: "eol",
            audienceDefinition: { productId: "prod-1", productVersionId: "ver-1" },
          }),
        ),
      );
      await waitFor(() =>
        expect(recordDryRunMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1", caseId: "case-eol-test-1" }),
      );
      await waitFor(() => expect(submitMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1" }));
      await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
    });

    it("selecting a different product resets the previously chosen version", async () => {
      mockEolBackend();
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();

      const productSelect = screen.getByRole("combobox", { name: /^product$/i });
      fireEvent.mouseDown(productSelect);
      fireEvent.click(await screen.findByRole("option", { name: /wso2 identity server/i }));

      const versionSelect = screen.getByRole("combobox", { name: /^version$/i });
      expect(versionSelect).not.toHaveTextContent("4.2.0");
    });
  });
});
