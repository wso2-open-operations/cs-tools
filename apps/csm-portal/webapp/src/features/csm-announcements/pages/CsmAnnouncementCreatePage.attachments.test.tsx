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

/*
 * What the create forms do with an announcement's files, end to end through the real
 * attachments controller: files picked before the draft exists are held, then uploaded
 * to the draft once it is saved, and the files of a submitted request are copied onto
 * its dry-run case before it goes for approval. The rest of the form's behaviour is in
 * CsmAnnouncementCreatePage.test.tsx.
 */

const navigateMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const showErrorMock = vi.fn();
const postMock = vi.fn();
const postEmptyMock = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  Link: ({ to, children, ...rest }: { to: string; children?: ReactNode } & Record<string, unknown>) => (
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
vi.mock("@features/csm-cases/components/AsyncProjectMultiSelect", () => ({
  default: ({ values, onChange }: { values: string[]; onChange: (next: string[]) => void }) => (
    <select
      aria-label="Projects"
      multiple
      value={values}
      onChange={(e) => onChange(Array.from(e.target.selectedOptions).map((o) => o.value))}
    >
      <option value="proj-1">Project One</option>
    </select>
  ),
}));
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({
    post: postMock,
    postEmpty: postEmptyMock,
    get: vi.fn().mockResolvedValue({ excludedProjectKeys: [], attachments: [] }),
  }),
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

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

function fillSubjectAndDescription(): void {
  fireEvent.change(screen.getByLabelText(/subject/i), { target: { value: "Scheduled maintenance" } });
  fireEvent.change(screen.getByLabelText("editor"), { target: { value: "<p>Maintenance window details.</p>" } });
}

function selectProject(): void {
  const select = screen.getByLabelText("Projects") as HTMLSelectElement;
  Array.from(select.options).forEach((o) => {
    o.selected = o.value === "proj-1";
  });
  fireEvent.change(select);
}

const fileInput = (): HTMLInputElement => document.querySelector('input[type="file"]') as HTMLInputElement;
const pdf = (name: string): File => new File(["%PDF-1.4"], name, { type: "application/pdf" });
const pick = (...files: File[]): void => {
  fireEvent.change(fileInput(), { target: { files } });
};

/** The backend: a DCPSUB test project for the dry run, and files that upload unless named in `refuse`. */
function mockBackend(refuse: string[] = []): void {
  postMock.mockImplementation((url: string, body: { name?: string }) => {
    if (url === "/projects/search") {
      return Promise.resolve({
        projects: [{ id: "dcpsub-id", name: "DCPSUB Test Project", key: "DCPSUB" }],
        total: 1,
        limit: 10,
        offset: 0,
        hasMore: false,
      });
    }
    if (url === "/announcement-requests/draft-1/attachments") {
      return refuse.includes(body.name ?? "")
        ? Promise.reject(new Error(`"${body.name}" is not allowed`))
        : Promise.resolve({ id: `att-${body.name}`, name: body.name });
    }
    return Promise.resolve({ id: "tag-1", label: "Dry Run", color: null });
  });
  postEmptyMock.mockResolvedValue({ attached: 1, alreadyAttached: 0 });
  postCaseMutateAsyncMock.mockResolvedValue({ id: "case-test-1", internalId: "WSO2-9001" });
}

const uploadCalls = (): string[] =>
  postMock.mock.calls
    .filter((c) => c[0] === "/announcement-requests/draft-1/attachments")
    .map((c) => (c[1] as { name: string }).name);

describe("CsmAnnouncementCreatePage — attachments", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
    postMock.mockReset();
    postEmptyMock.mockReset();
    createDraftMock = makeMutationMock<AnnouncementRequest>(DRAFT);
    updateDraftMock = makeMutationMock<AnnouncementRequest>(DRAFT);
    recordDryRunMock = makeMutationMock<AnnouncementRequest>(DRAFT);
    submitMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, state: "pending_approval" });
    mockBackend();
  });

  it("offers an optional Attachments field on the form", () => {
    renderPage();
    expect(screen.getByText("Attachments")).toBeInTheDocument();
    expect(screen.getByText("Add files")).toBeInTheDocument();
    expect(screen.getByText(/optional · pdf, images, word, excel, powerpoint, text/i)).toBeInTheDocument();
  });

  it("holds picked files in the form, and tells the sender they are saved with the draft", () => {
    renderPage();
    pick(pdf("advisory.pdf"));

    expect(screen.getByText("advisory.pdf")).toBeInTheDocument();
    expect(screen.getByText("Saved with the draft")).toBeInTheDocument();
    expect(uploadCalls()).toEqual([]);
  });

  it("never uploads a file that was removed before saving", async () => {
    renderPage();
    fillSubjectAndDescription();
    pick(pdf("keep.pdf"), pdf("drop.pdf"));
    fireEvent.click(screen.getByRole("button", { name: "Remove drop.pdf" }));

    fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

    await waitFor(() => expect(navigateMock).toHaveBeenCalled());
    expect(uploadCalls()).toEqual(["keep.pdf"]);
  });

  it("refuses an unsupported file straight away, with the reason, and holds nothing", () => {
    renderPage();
    pick(new File(["MZ"], "setup.exe"));

    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/can't be sent with an announcement/));
    expect(screen.queryByText("setup.exe")).not.toBeInTheDocument();
  });

  describe("Save as draft", () => {
    it("creates the draft, uploads the held files to it in order, then leaves", async () => {
      renderPage();
      fillSubjectAndDescription();
      pick(pdf("a.pdf"), pdf("b.pdf"));

      fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

      await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
      expect(createDraftMock.mutateAsync).toHaveBeenCalledTimes(1);
      expect(uploadCalls()).toEqual(["a.pdf", "b.pdf"]);
      expect(createDraftMock.mutateAsync.mock.invocationCallOrder[0]).toBeLessThan(
        postMock.mock.invocationCallOrder[postMock.mock.calls.findIndex((c) => String(c[0]).endsWith("/attachments"))],
      );
    });

    it("stays on the page when a file is refused, keeps the files, and a second save retries on the same draft", async () => {
      mockBackend(["b.pdf"]);
      renderPage();
      fillSubjectAndDescription();
      pick(pdf("a.pdf"), pdf("b.pdf"));

      fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));
      await waitFor(() =>
        expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/"b\.pdf" could not be attached/)),
      );
      expect(navigateMock).not.toHaveBeenCalled();
      expect(createDraftMock.mutateAsync).toHaveBeenCalledTimes(1);
      expect(screen.getByText("b.pdf")).toBeInTheDocument();

      // The draft exists now: saving again updates it and retries only what is left.
      mockBackend();
      postMock.mockClear();
      fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

      await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
      expect(createDraftMock.mutateAsync).toHaveBeenCalledTimes(1);
      expect(updateDraftMock.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ id: "draft-1" }));
      expect(uploadCalls()).toEqual(["b.pdf"]);
    });

    it("saves a draft with no files exactly as before: no file request at all", async () => {
      renderPage();
      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

      await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
      expect(uploadCalls()).toEqual([]);
      expect(postEmptyMock).not.toHaveBeenCalled();
    });
  });

  describe("Submit for approval", () => {
    function startSubmit(): void {
      fillSubjectAndDescription();
      selectProject();
      fireEvent.click(screen.getByRole("button", { name: /submit for approval/i }));
    }

    it("uploads the files to the draft, records the dry run, copies the files onto the dry-run case, then submits", async () => {
      renderPage();
      pick(pdf("advisory.pdf"));
      startSubmit();

      await waitFor(() => expect(submitMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1" }));
      await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));

      expect(uploadCalls()).toEqual(["advisory.pdf"]);
      expect(recordDryRunMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1", caseId: "case-test-1" });
      expect(postEmptyMock).toHaveBeenCalledWith("/announcement-requests/draft-1/cases/case-test-1/attachments");

      const uploaded = postMock.mock.invocationCallOrder[
        postMock.mock.calls.findIndex((c) => c[0] === "/announcement-requests/draft-1/attachments")
      ];
      expect(createDraftMock.mutateAsync.mock.invocationCallOrder[0]).toBeLessThan(uploaded);
      expect(uploaded).toBeLessThan(recordDryRunMock.mutateAsync.mock.invocationCallOrder[0]);
      expect(recordDryRunMock.mutateAsync.mock.invocationCallOrder[0]).toBeLessThan(postEmptyMock.mock.invocationCallOrder[0]);
      expect(postEmptyMock.mock.invocationCallOrder[0]).toBeLessThan(submitMock.mutateAsync.mock.invocationCallOrder[0]);
    });

    it("does not record, copy or submit when a file cannot be uploaded to the draft", async () => {
      mockBackend(["advisory.pdf"]);
      renderPage();
      pick(pdf("advisory.pdf"));
      startSubmit();

      await waitFor(() =>
        expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/"advisory\.pdf" could not be attached/)),
      );
      expect(recordDryRunMock.mutateAsync).not.toHaveBeenCalled();
      expect(postEmptyMock).not.toHaveBeenCalled();
      expect(submitMock.mutateAsync).not.toHaveBeenCalled();
      expect(navigateMock).not.toHaveBeenCalled();
    });

    it("does not submit when the files cannot be copied onto the dry-run case, and says submitting again retries", async () => {
      postEmptyMock.mockRejectedValue(new Error("downstream refused"));
      renderPage();
      pick(pdf("advisory.pdf"));
      startSubmit();

      await waitFor(() =>
        expect(showErrorMock).toHaveBeenCalledWith(
          expect.stringMatching(/dry-run case was created, but its attachments couldn't be added: downstream refused.*Submit again to retry/),
        ),
      );
      expect(submitMock.mutateAsync).not.toHaveBeenCalled();
      expect(navigateMock).not.toHaveBeenCalled();
    });

    it("submits a request with no files exactly as before: the dry-run case is left alone", async () => {
      renderPage();
      startSubmit();

      await waitFor(() => expect(submitMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1" }));
      await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
      expect(uploadCalls()).toEqual([]);
      expect(postEmptyMock).not.toHaveBeenCalled();
    });
  });
});

describe("CsmAnnouncementCreatePage — attachments on the EOL form", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
    postMock.mockReset();
    postEmptyMock.mockReset();
    createDraftMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, kind: "eol" });
    updateDraftMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, kind: "eol" });
    recordDryRunMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, kind: "eol" });
    submitMock = makeMutationMock<AnnouncementRequest>({ ...DRAFT, kind: "eol", state: "pending_approval" });

    postMock.mockImplementation((url: string, body: { name?: string }) => {
      if (url === "/products/search") {
        return Promise.resolve({
          products: [{ id: "prod-1", name: "WSO2 API Manager" }],
          total: 1,
          limit: 20,
          offset: 0,
          hasMore: false,
        });
      }
      if (url === "/products/prod-1/versions/search") {
        return Promise.resolve({
          productVersions: [{ id: "ver-1", version: "4.2.0", supportEolDate: "2025-09-08" }],
          total: 1,
          limit: 20,
          offset: 0,
          hasMore: false,
        });
      }
      if (url === "/deployed-products/projects/search") {
        return Promise.resolve({
          projects: [{ id: "proj-a", name: "Project A" }],
          total: 1,
          limit: 20,
          offset: 0,
          hasMore: false,
        });
      }
      if (url === "/announcement-requests/draft-1/attachments") {
        return Promise.resolve({ id: `att-${body.name}`, name: body.name });
      }
      // The dry run's test-project lookup and tag attach.
      return Promise.resolve({
        projects: [{ id: "dcpsub-id", name: "DCPSUB Test Project", key: "DCPSUB" }],
        total: 1,
        limit: 10,
        offset: 0,
        hasMore: false,
      });
    });
    postEmptyMock.mockResolvedValue({ attached: 1, alreadyAttached: 0 });
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-eol-test-1", internalId: "WSO2-9002" });
  });

  async function fillEolForm(): Promise<void> {
    fireEvent.click(screen.getByRole("radio", { name: /product version \/ eol announcement/i }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^product$/i }));
    fireEvent.click(await screen.findByRole("option", { name: /wso2 api manager/i }));
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^version$/i }));
    fireEvent.click(await screen.findByRole("option", { name: /4\.2\.0/i }));
    expect(await screen.findByText("Project A")).toBeInTheDocument();
    fillSubjectAndDescription();
  }

  it("offers the same Attachments field", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("radio", { name: /product version \/ eol announcement/i }));
    expect(screen.getByText("Attachments")).toBeInTheDocument();
    expect(screen.getByText("Add files")).toBeInTheDocument();
  });

  it("Save as draft creates the draft, uploads the held files to it, then leaves", async () => {
    renderPage();
    await fillEolForm();
    pick(pdf("end-of-life.pdf"));

    fireEvent.click(screen.getByRole("button", { name: /save as draft/i }));

    await waitFor(() => expect(navigateMock).toHaveBeenCalledWith("/announcements?tab=pending", undefined));
    expect(createDraftMock.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ kind: "eol" }));
    expect(uploadCalls()).toEqual(["end-of-life.pdf"]);
  });

  it("Submit for approval uploads the files, records the dry run, copies the files onto the dry-run case, then submits", async () => {
    renderPage();
    await fillEolForm();
    pick(pdf("end-of-life.pdf"));

    const submit = await screen.findByRole("button", { name: /submit for approval/i });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);

    await waitFor(() => expect(submitMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1" }));
    expect(uploadCalls()).toEqual(["end-of-life.pdf"]);
    expect(recordDryRunMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1", caseId: "case-eol-test-1" });
    expect(postEmptyMock).toHaveBeenCalledWith("/announcement-requests/draft-1/cases/case-eol-test-1/attachments");
    expect(postEmptyMock.mock.invocationCallOrder[0]).toBeLessThan(submitMock.mutateAsync.mock.invocationCallOrder[0]);
  });

  it("does not submit when the files cannot be copied onto the dry-run case", async () => {
    postEmptyMock.mockRejectedValue(new Error("downstream refused"));
    renderPage();
    await fillEolForm();
    pick(pdf("end-of-life.pdf"));

    const submit = await screen.findByRole("button", { name: /submit for approval/i });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);

    await waitFor(() =>
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/dry-run case was created, but its attachments/)),
    );
    expect(submitMock.mutateAsync).not.toHaveBeenCalled();
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("submits with no files exactly as before: the dry-run case is left alone", async () => {
    renderPage();
    await fillEolForm();

    const submit = await screen.findByRole("button", { name: /submit for approval/i });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);

    await waitFor(() => expect(submitMock.mutateAsync).toHaveBeenCalledWith({ id: "draft-1" }));
    expect(uploadCalls()).toEqual([]);
    expect(postEmptyMock).not.toHaveBeenCalled();
  });
});
