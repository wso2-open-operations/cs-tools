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

import type { ReactElement } from "react";
import { act, fireEvent, render as rtlRender, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";
import AnnouncementRequestDialog from "@features/csm-announcements/components/AnnouncementRequestDialog";
import { useGetAnnouncementRequest } from "@features/csm-announcements/api/useGetAnnouncementRequest";
import { useUpdateAnnouncementRequest } from "@features/csm-announcements/api/useUpdateAnnouncementRequest";
import { useRecordAnnouncementRequestDryRun } from "@features/csm-announcements/api/useRecordAnnouncementRequestDryRun";
import { useSubmitAnnouncementRequest } from "@features/csm-announcements/api/useSubmitAnnouncementRequest";
import { useApproveAnnouncementRequest } from "@features/csm-announcements/api/useApproveAnnouncementRequest";
import { useScheduleAnnouncementRequest } from "@features/csm-announcements/api/useScheduleAnnouncementRequest";
import { usePublishAnnouncementRequest } from "@features/csm-announcements/api/usePublishAnnouncementRequest";
import { useCreateAnnouncementRequestUpdate } from "@features/csm-announcements/api/useCreateAnnouncementRequestUpdate";
import { useListAnnouncementRequestUpdates } from "@features/csm-announcements/api/useListAnnouncementRequestUpdates";
import { usePostAnnouncementUpdateComments } from "@features/csm-announcements/api/usePostAnnouncementUpdateComments";
import { useAnnouncementDryRun } from "@features/csm-announcements/api/useAnnouncementDryRun";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import type { AnnouncementRequest } from "@features/csm-announcements/types/announcementRequests";

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ post: vi.fn(), postEmpty: vi.fn() }),
}));

vi.mock("@features/csm-announcements/api/useGetAnnouncementRequest", () => ({
  useGetAnnouncementRequest: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useUpdateAnnouncementRequest", () => ({
  useUpdateAnnouncementRequest: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useRecordAnnouncementRequestDryRun", () => ({
  useRecordAnnouncementRequestDryRun: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useSubmitAnnouncementRequest", () => ({
  useSubmitAnnouncementRequest: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useApproveAnnouncementRequest", () => ({
  useApproveAnnouncementRequest: vi.fn(),
}));

vi.mock("@features/csm-announcements/api/useScheduleAnnouncementRequest", () => ({
  useScheduleAnnouncementRequest: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/usePublishAnnouncementRequest", () => ({
  usePublishAnnouncementRequest: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useCreateAnnouncementRequestUpdate", () => ({
  useCreateAnnouncementRequestUpdate: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useListAnnouncementRequestUpdates", () => ({
  useListAnnouncementRequestUpdates: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/usePostAnnouncementUpdateComments", () => ({
  usePostAnnouncementUpdateComments: vi.fn(),
}));
vi.mock("@features/csm-announcements/api/useAnnouncementDryRun", () => ({
  DRY_RUN_TAG_LABEL: "Dry Run",
  useAnnouncementDryRun: vi.fn(),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: vi.fn(),
}));
// Every existing test below was written to exercise the dialog's actual
// mutation-triggering behavior (approve/edit/submit/publish/post-update),
// which requires canWrite — the real usePortalAccess would derive false
// here since there's no CurrentUserProvider in this test's render tree.
vi.mock("@context/current-user/usePortalAccess", () => ({
  usePortalAccess: () => ({
    hasAnyRole: true,
    canEscalate: true,
    canDownloadAttachment: true,
    canUseOperations: true,
    canUseTimeCardsAndUpdates: true,
    canWrite: true,
  }),
}));
// PublishConfirmationDialog's useResolvedAudiencePreview needs both of
// these — see DirectoryMembersList.test.tsx for the same pattern.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () =>
    vi.fn((url: string) => {
      const id = decodeURIComponent(String(url).split("/").pop() ?? "");
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => ({ id, name: `Project ${id}`, key: id.toUpperCase(), account: { name: "Acme" } }),
      });
    }),
}));
vi.mock("@features/csm-announcements/components/CreateCustomerAnnouncementForm", () => ({
  SECURITY_ANNOUNCEMENT_TAG_LABEL: "Security Announcement",
}));
// The rich-text editor isn't this dialog's concern (see EditorWithSourceToggle's
// own tests) — stubbed to a plain textarea so edits are simple to simulate.
vi.mock("@components/rich-text-editor/EditorWithSourceToggle", () => ({
  default: ({ value, onChange }: { value: string; onChange: (v: string) => void }) => (
    <textarea aria-label="Description" value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

const mockedGet = vi.mocked(useGetAnnouncementRequest);
const mockedUpdate = vi.mocked(useUpdateAnnouncementRequest);
const mockedRecordDryRun = vi.mocked(useRecordAnnouncementRequestDryRun);
const mockedSubmit = vi.mocked(useSubmitAnnouncementRequest);
const mockedApprove = vi.mocked(useApproveAnnouncementRequest);
const mockedSchedule = vi.mocked(useScheduleAnnouncementRequest);
const mockedPublish = vi.mocked(usePublishAnnouncementRequest);
const mockedCreateUpdate = vi.mocked(useCreateAnnouncementRequestUpdate);
const mockedListUpdates = vi.mocked(useListAnnouncementRequestUpdates);
const mockedPostUpdateComments = vi.mocked(usePostAnnouncementUpdateComments);
const mockedDryRun = vi.mocked(useAnnouncementDryRun);
const mockedIdTokenClaims = vi.mocked(useIdTokenClaims);

const BASE_REQUEST: AnnouncementRequest = {
  id: "req-1",
  kind: "customer",
  state: "draft",
  subject: "Scheduled maintenance",
  description: "<p>Details</p>",
  isSecurityAnnouncement: false,
  audienceDefinition: { scope: "specific", projectIds: ["p-1"] },
  createdBy: "jane@example.com",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-01T10:00:00Z",
};

function mockGet(overrides: Partial<AnnouncementRequest> | null, extra: Record<string, unknown> = {}): void {
  mockedGet.mockReturnValue({
    data: overrides === null ? null : { ...BASE_REQUEST, ...overrides },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    ...extra,
  } as unknown as ReturnType<typeof useGetAnnouncementRequest>);
}

const noopMutation = () =>
  ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, isError: false, error: null }) as unknown;

/** The dialog renders a `Link` for the dry-run result, so every render needs router context. */
function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  return rtlRender(<MemoryRouter>{ui}</MemoryRouter>);
}

beforeEach(() => {
  mockedGet.mockReset();
  mockedUpdate.mockReset();
  mockedRecordDryRun.mockReset();
  mockedSubmit.mockReset();
  mockedApprove.mockReset();
  mockedSchedule.mockReset();
  mockedPublish.mockReset();
  mockedCreateUpdate.mockReset();
  mockedListUpdates.mockReset();
  mockedPostUpdateComments.mockReset();
  mockedDryRun.mockReset();
  mockedIdTokenClaims.mockReset();

  // Matches BASE_REQUEST.createdBy by default, so every existing test below
  // (written before Publish was restricted to the creator) keeps exercising
  // the "current user is the creator" path without each needing its own
  // override -- only the dedicated non-creator test overrides this.
  mockedIdTokenClaims.mockReturnValue({ userid: "jane@example.com" });
  mockedUpdate.mockReturnValue(noopMutation() as ReturnType<typeof useUpdateAnnouncementRequest>);
  mockedRecordDryRun.mockReturnValue(noopMutation() as ReturnType<typeof useRecordAnnouncementRequestDryRun>);
  mockedSubmit.mockReturnValue(noopMutation() as ReturnType<typeof useSubmitAnnouncementRequest>);
  mockedApprove.mockReturnValue(noopMutation() as ReturnType<typeof useApproveAnnouncementRequest>);
  mockedSchedule.mockReturnValue(noopMutation() as ReturnType<typeof useScheduleAnnouncementRequest>);
  mockedDryRun.mockReturnValue({
    runningDryRun: false,
    dryRunResult: null,
    canRunDryRun: true,
    handleRunDryRun: vi.fn(),
  });
  mockedPublish.mockReturnValue({
    publishing: false,
    progress: null,
    succeededProjectIds: [],
    failedProjectIds: [],
    failedTagProjectIds: [],
    published: null,
    readyToPublish: true,
    hydratingDeliveries: false,
    hydrationFailed: false,
    retryHydration: vi.fn(),
    publishGivingUpOnFailed: vi.fn(),
    handlePublish: vi.fn(),
  });
  mockedCreateUpdate.mockReturnValue(noopMutation() as ReturnType<typeof useCreateAnnouncementRequestUpdate>);
  mockedListUpdates.mockReturnValue({
    data: { updates: [] },
    isLoading: false,
    isError: false,
  } as unknown as ReturnType<typeof useListAnnouncementRequestUpdates>);
  mockedPostUpdateComments.mockReturnValue({
    posting: false,
    progress: null,
    succeededCaseIds: [],
    failedCaseIds: [],
    done: false,
    handlePost: vi.fn(),
    reset: vi.fn(),
  });
});

describe("AnnouncementRequestDialog — loading/error", () => {
  it("shows skeletons while loading", () => {
    mockGet(null, { isLoading: true });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    // MUI's Dialog portals into document.body, not the local render container.
    expect(document.body.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("shows a retry button on error", () => {
    const refetch = vi.fn();
    mockGet(null, { isError: true, refetch });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(refetch).toHaveBeenCalled();
  });
});

describe("AnnouncementRequestDialog — actor email display", () => {
  // createdByEmail (and its submitted/approved/published siblings) are
  // display-only companions to their own *By id field — never used for the
  // creator-only checks, which always compare claims.userid against the raw
  // *By id. Regression coverage for a real bug this could reintroduce: this
  // dialog used to render the raw, human-unreadable *By id directly.
  it("prefers createdByEmail over the raw createdBy id when both are present", () => {
    mockGet({ createdBy: "e441e951-a2f5-4813-b308-817f128f4660", createdByEmail: "jane@example.com" });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByText(/jane@example.com/)).toBeInTheDocument();
    expect(screen.queryByText(/e441e951-a2f5-4813-b308-817f128f4660/)).not.toBeInTheDocument();
  });

  it("falls back to the raw createdBy id when createdByEmail is absent (a row written before this field existed)", () => {
    mockGet({ createdBy: "e441e951-a2f5-4813-b308-817f128f4660", createdByEmail: undefined });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByText(/e441e951-a2f5-4813-b308-817f128f4660/)).toBeInTheDocument();
  });

  it("still enforces creator-only actions against the id, not the display email", () => {
    // The signed-in user's own claims.userid never matches an email — this
    // confirms the ownership check the "Only <email> can publish" caption
    // above sits next to is still comparing the real ids, unaffected by
    // preferring the email purely for display.
    mockGet({
      state: "approved",
      createdBy: "e441e951-a2f5-4813-b308-817f128f4660",
      createdByEmail: "jane@example.com",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
    });
    mockedIdTokenClaims.mockReturnValue({ userid: "e441e951-a2f5-4813-b308-817f128f4660" });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.queryByText(/only jane@example.com can publish this request/i)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^publish$/i })).not.toBeDisabled();
  });
});

describe("AnnouncementRequestDialog — draft", () => {
  it("shows editable fields and a Submit button disabled until subject and description are filled", () => {
    mockGet({ state: "draft", subject: "", description: "" });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    const submitBtn = screen.getByRole("button", { name: /submit for approval/i });
    expect(submitBtn).toBeDisabled();
  });

  it("clicking Submit for approval runs the dry run, records it, then submits — one action", async () => {
    mockGet({ state: "draft" });
    const handleRunDryRun = vi.fn().mockResolvedValue({ caseId: "case-123", displayId: "WSO2-1" });
    mockedDryRun.mockReturnValue({
      runningDryRun: false,
      dryRunResult: null,
      canRunDryRun: true,
      handleRunDryRun,
    });
    const recordDryRunMutateAsync = vi.fn().mockResolvedValue({});
    mockedRecordDryRun.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: recordDryRunMutateAsync,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useRecordAnnouncementRequestDryRun>);
    const submitMutateAsync = vi.fn().mockResolvedValue({});
    mockedSubmit.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: submitMutateAsync,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useSubmitAnnouncementRequest>);

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    const submitBtn = screen.getByRole("button", { name: /submit for approval/i });
    expect(submitBtn).not.toBeDisabled();
    fireEvent.click(submitBtn);

    expect(handleRunDryRun).toHaveBeenCalled();
    await vi.waitFor(() => expect(recordDryRunMutateAsync).toHaveBeenCalledWith({ id: "req-1", caseId: "case-123" }));
    await vi.waitFor(() => expect(submitMutateAsync).toHaveBeenCalledWith({ id: "req-1" }));
  });

  it("does not record or submit anything when the dry run itself fails", async () => {
    mockGet({ state: "draft" });
    const handleRunDryRun = vi.fn().mockResolvedValue(null);
    mockedDryRun.mockReturnValue({
      runningDryRun: false,
      dryRunResult: null,
      canRunDryRun: true,
      handleRunDryRun,
    });
    const recordDryRunMutateAsync = vi.fn();
    mockedRecordDryRun.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: recordDryRunMutateAsync,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useRecordAnnouncementRequestDryRun>);

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /submit for approval/i }));

    await vi.waitFor(() => expect(handleRunDryRun).toHaveBeenCalled());
    expect(recordDryRunMutateAsync).not.toHaveBeenCalled();
  });

  it("saves edited content via the update mutation, addressed by id", () => {
    mockGet({ state: "draft" });
    const updateMutate = vi.fn();
    mockedUpdate.mockReturnValue({
      mutate: updateMutate,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useUpdateAnnouncementRequest>);

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    fireEvent.change(screen.getByDisplayValue("Scheduled maintenance"), {
      target: { value: "Updated subject" },
    });
    fireEvent.click(screen.getByRole("button", { name: /save changes/i }));

    expect(updateMutate).toHaveBeenCalledWith(
      expect.objectContaining({ id: "req-1", subject: "Updated subject" }),
    );
  });

  it("disables Submit for approval after an edit until it's saved — submit takes no body, so an unsaved edit would silently never reach the approver", () => {
    mockGet({ state: "draft" });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    const submitBtn = screen.getByRole("button", { name: /submit for approval/i });
    expect(submitBtn).not.toBeDisabled();

    fireEvent.change(screen.getByDisplayValue("Scheduled maintenance"), {
      target: { value: "Scheduled maintenance (updated)" },
    });

    expect(screen.getByRole("button", { name: /submit for approval/i })).toBeDisabled();
    expect(screen.getByText(/save your changes first/i)).toBeInTheDocument();
  });
});

describe("AnnouncementRequestDialog — pending_approval", () => {
  it("shows read-only content with Mark as approved and Edit, and approving calls the mutation", () => {
    mockGet({ state: "pending_approval", submittedBy: "jane@example.com", submittedAt: "2026-07-02T10:00:00Z" });
    const approveMutate = vi.fn();
    mockedApprove.mockReturnValue({
      mutate: approveMutate,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useApproveAnnouncementRequest>);

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    // Read-only: no editable subject field.
    expect(screen.queryByDisplayValue("Scheduled maintenance")).not.toBeInTheDocument();
    expect(screen.getByText("Scheduled maintenance")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /mark as approved/i }));
    expect(approveMutate).toHaveBeenCalled();
  });

  it("requires confirming before switching to edit mode", () => {
    mockGet({ state: "pending_approval" });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    expect(screen.getByText(/revert this request to draft/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /continue editing/i }));
    expect(screen.getByDisplayValue("Scheduled maintenance")).toBeInTheDocument();
  });
});

describe("AnnouncementRequestDialog — approved", () => {
  it("keeps content editable in place and shows a Publish button", () => {
    mockGet({
      state: "approved",
      resolvedProjectIds: ["p-1", "p-2"],
      resolvedProjectCount: 2,
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.getByDisplayValue("Scheduled maintenance")).toBeInTheDocument();
    expect(screen.getByText("2 projects")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^publish$/i })).toBeInTheDocument();
  });

  it("opens a confirmation popup before calling handlePublish, showing what's about to be sent", async () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    const handlePublish = vi.fn();
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: [],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish,
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /^publish$/i }));

    // The popup shows the same subject before anything is actually sent —
    // handlePublish must not fire just from opening it.
    expect(await screen.findByText("Confirm before sending")).toBeInTheDocument();
    expect(screen.getAllByText("Scheduled maintenance").length).toBeGreaterThan(0);
    expect(handlePublish).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /^send to 1 project$/i }));
    expect(handlePublish).toHaveBeenCalled();
  });

  it("shows live progress while publishing", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1", "p-2"], resolvedProjectCount: 2 });
    mockedPublish.mockReturnValue({
      publishing: true,
      progress: { completed: 1, total: 2 },
      succeededProjectIds: [],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByText("1/2")).toBeInTheDocument();
    expect(screen.getByText(/sending announcement/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /publishing/i })).toBeDisabled();
  });

  it("does not say 'Announcement sent' while a security-tag retry or the bookkeeping publish call is still in flight — the button already says Publishing…", () => {
    // Every case already succeeded (progress is null: neither the tag-retry
    // pass nor the final /publish bookkeeping call touches it), but the
    // whole operation isn't done yet — publishing is still true.
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    mockedPublish.mockReturnValue({
      publishing: true,
      progress: null,
      succeededProjectIds: ["p-1"],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByText(/sending announcement/i)).toBeInTheDocument();
    expect(screen.queryByText(/^announcement sent$/i)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /publishing/i })).toBeDisabled();
  });

  it("offers a retry with only the failed projects listed after a partial failure", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1", "p-2"], resolvedProjectCount: 2 });
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: ["p-1"],
      failedProjectIds: ["p-2"],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByRole("button", { name: /retry failed projects/i })).toBeInTheDocument();
    expect(screen.getByText(/announcement sent with failures/i)).toBeInTheDocument();
    expect(screen.getByText("p-2")).toBeInTheDocument();
    // The succeeded tally is deliberately hidden once there's an outstanding
    // failure to retry -- it's either stale history (reopening a request
    // with prior progress) or redundant with the retry flow itself; only
    // the still-failing project needs attention.
    expect(screen.queryByText(/\d+ succeeded/i)).not.toBeInTheDocument();
    expect(screen.queryByText("1/2")).not.toBeInTheDocument();
  });

  it("offers Publish anyway for a partial failure, and confirming calls publishGivingUpOnFailed", async () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1", "p-2"], resolvedProjectCount: 2 });
    const publishGivingUpOnFailed = vi.fn();
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: ["p-1"],
      failedProjectIds: ["p-2"],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed,
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    // Resolved to its real short key ("P-2", per this file's own
    // useAuthApiClient mock returning key: id.toUpperCase()), not the raw
    // frozen project id — both in the send-progress card's own chip and the
    // confirmation dialog's list of what's about to be permanently skipped.
    await vi.waitFor(() => expect(screen.getByText("P-2")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /publish anyway/i }));
    expect(screen.getByText(/publish without the failed projects/i)).toBeInTheDocument();
    await vi.waitFor(() => expect(screen.getAllByText("P-2").length).toBeGreaterThan(1));
    expect(screen.queryByText("p-2")).not.toBeInTheDocument();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /^publish anyway$/i }));
    });
    expect(publishGivingUpOnFailed).toHaveBeenCalled();
  });

  it("does not offer Publish anyway while a security-tag attach is still failing", () => {
    mockGet({
      state: "approved",
      resolvedProjectIds: ["p-1", "p-2"],
      resolvedProjectCount: 2,
      isSecurityAnnouncement: true,
    });
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: ["p-1", "p-2"],
      failedProjectIds: [],
      failedTagProjectIds: ["p-2"],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /publish anyway/i })).not.toBeInTheDocument();
  });

  it("still shows the full succeeded tally once every project has been delivered", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1", "p-2"], resolvedProjectCount: 2 });
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: ["p-1", "p-2"],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByText(/^announcement sent$/i)).toBeInTheDocument();
    expect(screen.getByText("2/2")).toBeInTheDocument();
    expect(screen.getByText(/2 succeeded/i)).toBeInTheDocument();
  });

  it("locks content while a failed-project retry is pending, so the retry can't diverge from what already succeeded", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1", "p-2"], resolvedProjectCount: 2 });
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: ["p-1"],
      failedProjectIds: ["p-2"],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.getByDisplayValue("Scheduled maintenance")).toBeDisabled();
    expect(screen.getByRole("button", { name: /save changes/i })).toBeDisabled();
    expect(screen.getByText(/locked while retrying failed projects/i)).toBeInTheDocument();
  });

  it("disables Publish and shows a hint after editing the subject without saving", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    const publishBtn = screen.getByRole("button", { name: /^publish$/i });
    expect(publishBtn).not.toBeDisabled();

    fireEvent.change(screen.getByDisplayValue("Scheduled maintenance"), {
      target: { value: "Scheduled maintenance (updated)" },
    });

    expect(screen.getByRole("button", { name: /^publish$/i })).toBeDisabled();
    expect(screen.getByText(/save your changes first/i)).toBeInTheDocument();
  });

  it("clicking Publish while there are unsaved changes does not call handlePublish", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    const handlePublish = vi.fn();
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: [],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish,
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    fireEvent.change(screen.getByDisplayValue("Scheduled maintenance"), {
      target: { value: "Scheduled maintenance (updated)" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^publish$/i }));

    expect(handlePublish).not.toHaveBeenCalled();
  });

  it("disables Publish and explains why when the current user isn't the request's creator", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    // BASE_REQUEST.createdBy is jane@example.com — a different signed-in
    // user must not be able to trigger the real send just because they can
    // see the approved request (e.g. after approving it themselves).
    mockedIdTokenClaims.mockReturnValue({ userid: "someone-else@example.com" });
    const handlePublish = vi.fn();
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: [],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish,
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    const publishBtn = screen.getByRole("button", { name: /^publish$/i });
    expect(publishBtn).toBeDisabled();
    expect(screen.getByText(/only jane@example.com can publish this request/i)).toBeInTheDocument();

    fireEvent.click(publishBtn);
    expect(handlePublish).not.toHaveBeenCalled();
  });

  // useIdTokenClaims genuinely returns undefined for a moment after mount
  // while it decodes the ID token asynchronously — even for the real
  // creator, who is signed in the whole time. Without distinguishing that
  // from "loaded, and it's someone else," this would flash the wrong
  // "only X can publish" denial at the very user it's meant to allow.
  it("disables Publish without the non-creator message while claims are still loading", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    mockedIdTokenClaims.mockReturnValue(undefined);
    const handlePublish = vi.fn();
    mockedPublish.mockReturnValue({
      publishing: false,
      progress: null,
      succeededProjectIds: [],
      failedProjectIds: [],
      failedTagProjectIds: [],
      published: null,
      readyToPublish: true,
      hydratingDeliveries: false,
      hydrationFailed: false,
      retryHydration: vi.fn(),
      publishGivingUpOnFailed: vi.fn(),
      handlePublish,
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    const publishBtn = screen.getByRole("button", { name: /^publish$/i });
    expect(publishBtn).toBeDisabled();
    expect(screen.queryByText(/can publish this request/i)).not.toBeInTheDocument();

    fireEvent.click(publishBtn);
    expect(handlePublish).not.toHaveBeenCalled();
  });

  it("shows the due date when set", () => {
    mockGet({
      state: "approved",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      dueOn: "2026-08-01T00:00:00Z",
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);
    expect(screen.getByText(/^Due /)).toBeInTheDocument();
  });

  it("shows the scheduled time with a Cancel schedule action when scheduledFor is set", () => {
    mockGet({
      state: "approved",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      scheduledFor: "2026-08-01T00:00:00Z",
    });
    const mutate = vi.fn();
    mockedSchedule.mockReturnValue({
      mutate,
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useScheduleAnnouncementRequest>);
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.getByText(/^Scheduled to publish on /)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Schedule for later…" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Cancel schedule" }));
    expect(mutate).toHaveBeenCalledWith({ id: "req-1", scheduledFor: null });
  });

  it("offers a Schedule for later control when no schedule is set, opening a picker with Confirm disabled until a time is entered", () => {
    mockGet({ state: "approved", resolvedProjectIds: ["p-1"], resolvedProjectCount: 1 });
    const mutate = vi.fn();
    mockedSchedule.mockReturnValue({
      mutate,
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useScheduleAnnouncementRequest>);
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.queryByText(/^Scheduled to publish on /)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Schedule for later…" }));

    expect(screen.getAllByLabelText(/Publish at/)[0]).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /confirm schedule/i })).toBeDisabled();
    expect(mutate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("button", { name: /confirm schedule/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Schedule for later…" })).toBeInTheDocument();
  });
});

describe("AnnouncementRequestDialog — published", () => {
  it("shows a read-only summary with no action buttons", () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.queryByDisplayValue("Scheduled maintenance")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /submit for approval/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /mark as approved/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^publish$/i })).not.toBeInTheDocument();
  });

  it("shows a persistent Security chip next to the subject once published, not just during the editable checkbox state", () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
      isSecurityAnnouncement: true,
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.getByText("Security")).toBeInTheDocument();
  });

  it("shows no Security chip for a non-security published request", () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
      isSecurityAnnouncement: false,
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.queryByText("Security")).not.toBeInTheDocument();
  });

  it("shows a Delivered to list with each project's own case number when caseMembers is passed (e.g. opened from a batch row)", () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1", "p-2"],
      resolvedProjectCount: 2,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
      publishedCaseIds: ["case-1", "case-2"],
    });
    render(
      <AnnouncementRequestDialog
        requestId="req-1"
        onClose={vi.fn()}
        caseMembers={[
          { caseId: "case-1", caseNumber: "CS0001", wso2CaseId: "ACME-1", projectName: "Acme" },
          { caseId: "case-2", caseNumber: "CS0002", wso2CaseId: "BOLT-1", projectName: "Bolt" },
        ]}
      />,
    );

    expect(screen.getByText("Delivered to 2 projects")).toBeInTheDocument();
    expect(screen.getByText("Acme")).toBeInTheDocument();
    expect(screen.getByText("CS0001")).toBeInTheDocument();
    expect(screen.getByText("Bolt")).toBeInTheDocument();
    expect(screen.getByText("CS0002")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Acme.*CS0001/s })).toHaveAttribute(
      "href",
      "/announcements/case-1",
    );
  });

  it("shows no Delivered to section when caseMembers is empty (e.g. opened from the Pending tab)", () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
      publishedCaseIds: ["case-1"],
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.queryByText(/^delivered to/i)).not.toBeInTheDocument();
  });

  it("posting an update records it, then fans out a comment to every published case, via a confirmation popup", async () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1", "p-2"],
      resolvedProjectCount: 2,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
      publishedCaseIds: ["case-1", "case-2"],
    });
    const createUpdateMutateAsync = vi.fn().mockResolvedValue({ id: "u-1", content: "A correction." });
    mockedCreateUpdate.mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: createUpdateMutateAsync,
      isPending: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useCreateAnnouncementRequestUpdate>);
    const handlePost = vi.fn().mockResolvedValue(undefined);
    mockedPostUpdateComments.mockReturnValue({
      posting: false,
      progress: null,
      succeededCaseIds: [],
      failedCaseIds: [],
      done: false,
      handlePost,
      reset: vi.fn(),
    });

    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("Description"), { target: { value: "A correction." } });
    fireEvent.click(screen.getByRole("button", { name: /^post update$/i }));

    expect(await screen.findByText("Confirm update")).toBeInTheDocument();
    expect(createUpdateMutateAsync).not.toHaveBeenCalled();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /^post to 2 cases$/i }));
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(createUpdateMutateAsync).toHaveBeenCalledWith({
      id: "req-1",
      payload: { content: "A correction." },
    });
    expect(handlePost).toHaveBeenCalledWith(["case-1", "case-2"], "A correction.", "jane@example.com");
  });

  it("disables Post update and explains why for a request published before case tracking existed", () => {
    mockGet({
      state: "published",
      resolvedProjectIds: ["p-1"],
      resolvedProjectCount: 1,
      publishedBy: "jane@example.com",
      publishedAt: "2026-07-03T10:00:00Z",
    });
    render(<AnnouncementRequestDialog requestId="req-1" onClose={vi.fn()} />);

    expect(screen.queryByRole("button", { name: /^post update$/i })).not.toBeInTheDocument();
    expect(screen.getByText(/published before case tracking existed/i)).toBeInTheDocument();
  });
});
