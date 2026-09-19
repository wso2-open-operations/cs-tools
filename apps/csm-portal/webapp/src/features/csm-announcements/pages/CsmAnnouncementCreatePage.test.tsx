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
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import "@testing-library/jest-dom/vitest";

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
  default: ({
    value,
    onChange,
    disabled,
  }: {
    value: string;
    onChange: (v: string) => void;
    disabled?: boolean;
  }) => (
    <textarea
      aria-label="editor"
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
// The real multi-select searches the backend as the user types; stub it with
// a plain multi-value control so this file stays focused on the create form's
// own required-field and fan-out-submit behavior (the picker itself has its
// own tests via CsmAnnouncementsPage.test.tsx's filter-bar coverage).
vi.mock("@features/csm-cases/components/AsyncProjectMultiSelect", () => ({
  default: ({
    values,
    onChange,
    disabled,
  }: {
    values: string[];
    onChange: (next: string[]) => void;
    disabled?: boolean;
  }) => (
    <select
      aria-label="Projects"
      multiple
      value={values}
      disabled={disabled}
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

describe("CsmAnnouncementCreatePage", () => {
  beforeEach(() => {
    navigateMock.mockReset();
    postCaseMutateAsyncMock.mockReset();
    showErrorMock.mockReset();
    projectSearchPostMock.mockReset();
    excludedProjectKeysGetMock.mockReset().mockResolvedValue({ excludedProjectKeys: [] });
  });

  it("keeps Create disabled until title, description, and at least one project are filled", () => {
    renderPage();
    const submit = screen.getByRole("button", { name: /create announcement/i });
    expect(submit).toBeDisabled();

    fillSubjectAndDescription();
    // Subject + description are filled, but no project is selected yet.
    expect(submit).toBeDisabled();

    selectProjects("proj-1");
    expect(submit).toBeEnabled();
  });

  it("fans out one POST /cases call per selected project, with the same subject/description", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "ann-1" });
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");

    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));
    await screen.findByRole("button", { name: /creating/i });

    await waitFor(() => {
      expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith({
      type: "announcement",
      projectId: "proj-1",
      subject: "Scheduled maintenance",
      description: "<p>Maintenance window details.</p>",
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith({
      type: "announcement",
      projectId: "proj-2",
      subject: "Scheduled maintenance",
      description: "<p>Maintenance window details.</p>",
    });

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
  });

  it("disables Cancel while a batch is in flight — navigating away must not let the in-flight sends silently keep creating cases unseen", async () => {
    let resolveCall: (v: { id: string }) => void = () => {};
    postCaseMutateAsyncMock.mockImplementation(
      () => new Promise((resolve) => { resolveCall = resolve; }),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /cancel/i })).toBeDisabled();
    });

    resolveCall({ id: "ann-1" });
    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
  });

  it("disables the project picker while a batch is in flight — changing the selection mid-send must not disrupt it", async () => {
    let resolveCall: (v: { id: string }) => void = () => {};
    postCaseMutateAsyncMock.mockImplementation(
      () => new Promise((resolve) => { resolveCall = resolve; }),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(screen.getByLabelText("Projects")).toBeDisabled();
    });

    resolveCall({ id: "ann-1" });
    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
  });

  it("shows live send progress as each project's create call settles", async () => {
    const resolvers: Array<(v: { id: string }) => void> = [];
    postCaseMutateAsyncMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvers.push(resolve);
        }),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => expect(resolvers).toHaveLength(2));
    expect(screen.getByText("0/2")).toBeInTheDocument();

    resolvers[0]({ id: "ann-1" });
    await waitFor(() => expect(screen.getByText("1/2")).toBeInTheDocument());
    expect(screen.getByText(/1 succeeded/)).toBeInTheDocument();

    resolvers[1]({ id: "ann-2" });
    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
  });

  it("shows a failed count in the progress card as soon as a project's create call rejects", async () => {
    const resolvers: Array<() => void> = [];
    const rejecters: Array<(e: Error) => void> = [];
    postCaseMutateAsyncMock.mockImplementation(
      () =>
        new Promise((resolve, reject) => {
          resolvers.push(() => resolve({ id: "ann" }));
          rejecters.push(reject);
        }),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => expect(rejecters).toHaveLength(2));
    rejecters[0](new Error("boom"));
    await waitFor(() => expect(screen.getByText(/1 failed/)).toBeInTheDocument());
    // The failed project's own id is visible as a chip immediately, not
    // only after the whole batch finishes and the error banner fires.
    expect(screen.getByText("proj-1")).toBeInTheDocument();

    resolvers[1]();
    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-1"));
    });
  });

  it("reports which project failed on a partial failure, keeps the succeeded one, shows it in the progress card, and stays on the page instead of navigating away", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "proj-1"
        ? Promise.resolve({ id: "ann-1" })
        : Promise.reject(new Error("network down")),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.stringContaining("proj-2"),
      );
    });
    // A create failure means proj-2 never got the announcement — the page
    // stays put (no navigate) so its id stays visible on the progress card
    // for the sender to identify and retry, rather than only living in a
    // toast that's gone the moment the page changes.
    expect(navigateMock).not.toHaveBeenCalled();
    expect(screen.getByText("proj-2")).toBeInTheDocument();
    expect(screen.getByText(/didn.t receive the announcement/i)).toBeInTheDocument();
  });

  it("retries only the failed project on the next submit — never resends to one that already succeeded", async () => {
    let proj2Attempts = 0;
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) => {
      if (projectId === "proj-2") {
        proj2Attempts += 1;
        return proj2Attempts === 1
          ? Promise.reject(new Error("network down"))
          : Promise.resolve({ id: "ann-2-retry" });
      }
      return Promise.resolve({ id: "ann-1" });
    });
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-2"));
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);

    // The button itself now says so — this isn't a generic re-submit.
    const retryButton = await screen.findByRole("button", { name: /retry 1 failed project/i });
    fireEvent.click(retryButton);

    await waitFor(() => {
      expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(3);
    });
    // Only proj-2 (the one that actually failed) was resubmitted — proj-1,
    // which already succeeded, must never be called a second time, or it
    // would end up with two duplicate announcement cases.
    expect(postCaseMutateAsyncMock).toHaveBeenLastCalledWith(
      expect.objectContaining({ projectId: "proj-2" }),
    );
    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
  });

  it("locks subject, description, and the security checkbox while a retry is pending — the resend must match what already went out", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "proj-1"
        ? Promise.resolve({ id: "ann-1" })
        : Promise.reject(new Error("network down")),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-2"));
    });

    expect(screen.getByLabelText(/subject/i)).toBeDisabled();
    expect(screen.getByLabelText("editor")).toBeDisabled();
    expect(
      screen.getByRole("checkbox", { name: /this is a security announcement/i }),
    ).toBeDisabled();
    expect(screen.getByText(/locked while retrying failed projects/i)).toBeInTheDocument();
  });

  it("clears the stale progress card when the audience changes after a partial failure", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "proj-1"
        ? Promise.resolve({ id: "ann-1" })
        : Promise.reject(new Error("network down")),
    );
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-2"));
    });
    expect(screen.getByRole("status")).toBeInTheDocument();

    // Changing the selection is the sender's own signal that they want a
    // different send, not a retry — the old batch's outcome (proj-2 failed)
    // no longer describes this new selection and must not linger on screen.
    selectProjects("proj-1");

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByLabelText(/subject/i)).toBeEnabled();
  });

  it("surfaces a single error and does not navigate when every project fails", async () => {
    postCaseMutateAsyncMock.mockRejectedValue(new Error("network down"));
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(
        "Could not create the announcement. Please try again.",
      );
    });
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("resolves 'All customer projects' via the entity service and fans create-case calls out across the resolved ids, with the default exclusions applied", async () => {
    projectSearchPostMock.mockResolvedValue({
      projects: [
        { id: "resolved-1", name: "Resolved One", key: "R1", account: { id: "a1", name: "Acme" } },
        { id: "resolved-2", name: "Resolved Two", key: "R2", account: { id: "a2", name: "Globex" } },
      ],
      total: 2,
      limit: 50,
      offset: 0,
      hasMore: false,
    });
    postCaseMutateAsyncMock.mockResolvedValue({ id: "ann-1" });
    renderPage();

    fillSubjectAndDescription();
    fireEvent.click(screen.getByRole("radio", { name: /all customer projects/i }));

    // Both default exclusion checkboxes are on and reach the request body,
    // sent to the dedicated audience-resolution endpoint (not the general
    // /projects/search also used by type-ahead pickers) so the backend's
    // mandatory excluded-project-key denylist gets applied too.
    await waitFor(() => {
      expect(projectSearchPostMock).toHaveBeenCalledWith(
        "/announcements/audience/search",
        expect.objectContaining({
          excludeSubscriptionTypes: ["cloud_support", "cloud_evaluation_support"],
          excludeClosureStates: ["Restricted", "Suspended"],
        }),
      );
    });

    const submit = await screen.findByRole("button", { name: /create announcement/i });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);

    await waitFor(() => {
      expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
      expect.objectContaining({ projectId: "resolved-1" }),
    );
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
      expect.objectContaining({ projectId: "resolved-2" }),
    );
  });

  it("shows a failed project's own short key, not its raw id, on the progress card and in the error banner", async () => {
    projectSearchPostMock.mockResolvedValue({
      projects: [
        { id: "resolved-1", name: "Resolved One", key: "CUPPTSUB", account: { id: "a1", name: "Acme" } },
        { id: "resolved-2", name: "Resolved Two", key: "R2", account: { id: "a2", name: "Globex" } },
      ],
      total: 2,
      limit: 50,
      offset: 0,
      hasMore: false,
    });
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "resolved-1"
        ? Promise.reject(new Error("network down"))
        : Promise.resolve({ id: "ann-2" }),
    );
    renderPage();

    fillSubjectAndDescription();
    fireEvent.click(screen.getByRole("radio", { name: /all customer projects/i }));
    const submit = await screen.findByRole("button", { name: /create announcement/i });
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("CUPPTSUB"));
    });
    // The raw uuid-style id never appears in the banner — only its key.
    expect(showErrorMock).not.toHaveBeenCalledWith(expect.stringContaining("resolved-1"));
    // Same on the progress card's own failed-project chip (scoped to that
    // card specifically — the resolved-audience list above it also shows
    // each project's key, so a page-wide query would match both).
    const progressCard = screen.getByRole("status");
    expect(within(progressCard).getByText("CUPPTSUB")).toBeInTheDocument();
    expect(within(progressCard).queryByText("resolved-1")).not.toBeInTheDocument();
  });

  it("attaches a fixed security label to every created case when 'This is a security announcement' is checked", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      Promise.resolve({ id: `case-for-${projectId}` }),
    );
    projectSearchPostMock.mockResolvedValue({ id: "tag-1", label: "Security Announcement", color: null });
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1", "proj-2");
    fireEvent.click(screen.getByRole("checkbox", { name: /this is a security announcement/i }));
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(projectSearchPostMock).toHaveBeenCalledTimes(2);
    });
    expect(projectSearchPostMock).toHaveBeenCalledWith("/cases/case-for-proj-1/tags", {
      label: "Security Announcement",
    });
    expect(projectSearchPostMock).toHaveBeenCalledWith("/cases/case-for-proj-2/tags", {
      label: "Security Announcement",
    });
    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
    // No tag failures, so submitting must not surface an error banner.
    expect(showErrorMock).not.toHaveBeenCalled();
  });

  it("does not attach a tag when the security checkbox is left unchecked", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1" });
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
    });
    expect(projectSearchPostMock).not.toHaveBeenCalled();
  });

  it("reports a tag-attach failure separately from a create failure — the case still stands", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1" });
    projectSearchPostMock.mockRejectedValue(new Error("tag service down"));
    renderPage();

    fillSubjectAndDescription();
    selectProjects("proj-1");
    fireEvent.click(screen.getByRole("checkbox", { name: /this is a security announcement/i }));
    fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.stringContaining("security label couldn't be attached"),
      );
    });
    // The case itself was created successfully, so this is still a navigate-away, not a blocking failure.
    expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
  });

  it("the dry run creates one case in the configured test project (DCPSUB) and shows a link to it", async () => {
    projectSearchPostMock.mockImplementation((url: string, body: unknown) => {
      if (url === "/projects/search") {
        return Promise.resolve({
          projects: [
            { id: "other-1", name: "Other", key: "OTHER" },
            { id: "dcpsub-id", name: "DCPSUB Test Project", key: "DCPSUB" },
          ],
          total: 2,
          limit: 10,
          offset: 0,
          hasMore: false,
        });
      }
      // Tag-attach calls (POST /cases/{id}/tags) also flow through this mock.
      void body;
      return Promise.resolve({ id: "tag-1", label: "Dry Run", color: null });
    });
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-test-1", internalId: "WSO2-9001" });
    renderPage();

    fillSubjectAndDescription();
    fireEvent.click(screen.getByRole("button", { name: /run dry run/i }));

    await waitFor(() => {
      expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
        expect.objectContaining({ type: "announcement", projectId: "dcpsub-id" }),
      );
    });

    expect(await screen.findByText(/WSO2-9001/)).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /view it/i });
    expect(link).toHaveAttribute("href", "/announcements/case-test-1");

    // Doesn't touch the real create flow or navigate away.
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("the dry run surfaces an error and creates nothing when the test project can't be found", async () => {
    projectSearchPostMock.mockResolvedValue({
      projects: [],
      total: 0,
      limit: 10,
      offset: 0,
      hasMore: false,
    });
    renderPage();

    fillSubjectAndDescription();
    fireEvent.click(screen.getByRole("button", { name: /run dry run/i }));

    await waitFor(() => {
      expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("DCPSUB"));
    });
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
  });

  it("the dry run button is disabled until subject and description are filled, independent of any project selection", () => {
    renderPage();
    expect(screen.getByRole("button", { name: /run dry run/i })).toBeDisabled();
    fillSubjectAndDescription();
    expect(screen.getByRole("button", { name: /run dry run/i })).toBeEnabled();
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

  it("unchecking an exclusion drops it from the resolved-audience request", async () => {
    projectSearchPostMock.mockResolvedValue({
      projects: [],
      total: 0,
      limit: 50,
      offset: 0,
      hasMore: false,
    });
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

  it("shows the backend-configured excluded project keys as a checked, disabled checkbox with read-only chips under 'All customer projects'", async () => {
    excludedProjectKeysGetMock.mockResolvedValue({
      excludedProjectKeys: ["Apexia", "Veridian", "Veloxis"],
    });
    projectSearchPostMock.mockResolvedValue({
      projects: [],
      total: 0,
      limit: 50,
      offset: 0,
      hasMore: false,
    });
    renderPage();

    fireEvent.click(screen.getByRole("radio", { name: /all customer projects/i }));

    expect(await screen.findByText("Apexia")).toBeInTheDocument();
    expect(screen.getByText("Veridian")).toBeInTheDocument();
    expect(screen.getByText("Veloxis")).toBeInTheDocument();

    const mandatoryExclusionCheckbox = screen.getByRole("checkbox", {
      name: /exclude these configured projects/i,
    });
    expect(mandatoryExclusionCheckbox).toBeChecked();
    expect(mandatoryExclusionCheckbox).toBeDisabled();
  });

  it("shows no mandatory-exclusion checkbox when nothing is configured", async () => {
    excludedProjectKeysGetMock.mockResolvedValue({ excludedProjectKeys: [] });
    projectSearchPostMock.mockResolvedValue({
      projects: [],
      total: 0,
      limit: 50,
      offset: 0,
      hasMore: false,
    });
    renderPage();

    fireEvent.click(screen.getByRole("radio", { name: /all customer projects/i }));

    await waitFor(() => expect(excludedProjectKeysGetMock).toHaveBeenCalled());
    expect(
      screen.queryByRole("checkbox", { name: /exclude these configured projects/i }),
    ).not.toBeInTheDocument();
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

    it("keeps Create disabled until product, version, subject, and description are filled", async () => {
      mockEolBackend();
      renderPage();
      switchToEolKind();

      const submit = screen.getByRole("button", { name: /create announcement/i });
      expect(submit).toBeDisabled();

      await selectProductAndVersion();
      expect(submit).toBeDisabled();

      fillSubjectAndDescription();
      await waitFor(() => expect(submit).not.toBeDisabled());
    });

    it("resolves the audience for the selected product version and fans out one POST /cases call per resolved project", async () => {
      mockEolBackend();
      postCaseMutateAsyncMock.mockResolvedValue({ id: "ann-eol-1" });
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();

      expect(await screen.findByText("Project A")).toBeInTheDocument();
      expect(screen.getByText("Project B")).toBeInTheDocument();

      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

      await waitFor(() => {
        expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);
      });
      expect(postCaseMutateAsyncMock).toHaveBeenCalledWith({
        type: "announcement",
        projectId: "proj-a",
        subject: "Scheduled maintenance",
        description: "<p>Maintenance window details.</p>",
      });
      expect(postCaseMutateAsyncMock).toHaveBeenCalledWith({
        type: "announcement",
        projectId: "proj-b",
        subject: "Scheduled maintenance",
        description: "<p>Maintenance window details.</p>",
      });
      await waitFor(() => {
        expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
      });
    });

    it("shows live send progress as each resolved project's create call settles", async () => {
      mockEolBackend();
      const resolvers: Array<(v: { id: string }) => void> = [];
      postCaseMutateAsyncMock.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvers.push(resolve);
          }),
      );
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();
      expect(await screen.findByText("Project A")).toBeInTheDocument();

      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

      await waitFor(() => expect(resolvers).toHaveLength(2));
      expect(screen.getByText("0/2")).toBeInTheDocument();

      resolvers[0]({ id: "ann-eol-1" });
      await waitFor(() => expect(screen.getByText("1/2")).toBeInTheDocument());

      resolvers[1]({ id: "ann-eol-2" });
      await waitFor(() => {
        expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
      });
    });

    it("shows the failed project in the progress card and stays on the page when a create call fails", async () => {
      mockEolBackend();
      postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
        projectId === "proj-a"
          ? Promise.resolve({ id: "ann-eol-1" })
          : Promise.reject(new Error("network down")),
      );
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();
      expect(await screen.findByText("Project A")).toBeInTheDocument();

      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

      await waitFor(() => {
        expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-b"));
      });
      // proj-b never got the announcement — stay on this page instead of
      // navigating away, so its id stays visible on the progress card.
      expect(navigateMock).not.toHaveBeenCalled();
      expect(screen.getByText("proj-b")).toBeInTheDocument();
    });

    it("retries only the failed project on the next submit — never resends to one that already succeeded", async () => {
      mockEolBackend();
      let projBAttempts = 0;
      postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) => {
        if (projectId === "proj-b") {
          projBAttempts += 1;
          return projBAttempts === 1
            ? Promise.reject(new Error("network down"))
            : Promise.resolve({ id: "ann-eol-b-retry" });
        }
        return Promise.resolve({ id: "ann-eol-a" });
      });
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();
      expect(await screen.findByText("Project A")).toBeInTheDocument();

      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

      await waitFor(() => {
        expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-b"));
      });
      expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);

      const retryButton = await screen.findByRole("button", { name: /retry 1 failed project/i });
      fireEvent.click(retryButton);

      await waitFor(() => {
        expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(3);
      });
      expect(postCaseMutateAsyncMock).toHaveBeenLastCalledWith(
        expect.objectContaining({ projectId: "proj-b" }),
      );
      await waitFor(() => {
        expect(navigateMock).toHaveBeenCalledWith("/announcements", undefined);
      });
    });

    it("locks subject and description while a retry is pending — the resend must match what already went out", async () => {
      mockEolBackend();
      postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
        projectId === "proj-a"
          ? Promise.resolve({ id: "ann-eol-1" })
          : Promise.reject(new Error("network down")),
      );
      renderPage();
      switchToEolKind();
      await selectProductAndVersion();
      expect(await screen.findByText("Project A")).toBeInTheDocument();

      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /create announcement/i }));

      await waitFor(() => {
        expect(showErrorMock).toHaveBeenCalledWith(expect.stringContaining("proj-b"));
      });

      expect(screen.getByLabelText(/subject/i)).toBeDisabled();
      expect(screen.getByLabelText("editor")).toBeDisabled();
      expect(screen.getByText(/locked while retrying failed projects/i)).toBeInTheDocument();
    });

    it("the dry run creates one case in the configured test project, tagged Dry Run only", async () => {
      mockEolBackend();
      postCaseMutateAsyncMock.mockResolvedValue({ id: "case-eol-test-1", internalId: "WSO2-9002" });
      renderPage();
      switchToEolKind();

      fillSubjectAndDescription();
      fireEvent.click(screen.getByRole("button", { name: /run dry run/i }));

      await waitFor(() => {
        expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
          expect.objectContaining({ type: "announcement", projectId: "dcpsub-id" }),
        );
      });
      expect(await screen.findByText(/WSO2-9002/)).toBeInTheDocument();

      // No security label exists on this flow at all — only the "Dry Run" tag.
      expect(projectSearchPostMock).not.toHaveBeenCalledWith(
        expect.stringContaining("/tags"),
        expect.objectContaining({ label: "Security Announcement" }),
      );
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
