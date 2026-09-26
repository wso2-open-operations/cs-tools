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
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const postMock = vi.fn();
const showErrorMock = vi.fn();
const postCaseMutateAsyncMock = vi.fn();
const addTagMutateAsyncMock = vi.fn();
const recordDeliveriesMutateAsyncMock = vi.fn();
const listDeliveriesMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));
vi.mock("@features/csm-cases/api/usePostCsmCase", () => ({
  usePostCsmCase: () => ({ mutateAsync: postCaseMutateAsyncMock }),
}));
vi.mock("@features/csm-cases/api/useCaseTags", () => ({
  useAddTagToCase: () => ({ mutateAsync: addTagMutateAsyncMock }),
}));
// Mocked as their own modules (not through the shared postMock above) so
// every existing test's assumptions about postMock's own call count/
// sequence for the real /publish call stay valid unchanged — see the
// dedicated "delivery ledger" describe block below for coverage of these.
vi.mock("@features/csm-announcements/api/useRecordAnnouncementRequestDeliveries", () => ({
  useRecordAnnouncementRequestDeliveries: () => ({ mutateAsync: recordDeliveriesMutateAsyncMock }),
}));
vi.mock("@features/csm-announcements/api/useListAnnouncementRequestDeliveries", () => ({
  useListAnnouncementRequestDeliveries: () => listDeliveriesMock(),
}));

// Imported after the mocks above so the module picks them up.
import { usePublishAnnouncementRequest } from "@features/csm-announcements/api/usePublishAnnouncementRequest";
import type { AnnouncementRequest } from "@features/csm-announcements/types/announcementRequests";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const APPROVED_REQUEST: AnnouncementRequest = {
  id: "req-1",
  kind: "customer",
  state: "approved",
  subject: "Scheduled maintenance",
  description: "<p>Details</p>",
  isSecurityAnnouncement: false,
  audienceDefinition: { scope: "specific", projectIds: ["p-1", "p-2"] },
  resolvedProjectIds: ["p-1", "p-2"],
  resolvedProjectCount: 2,
  createdBy: "jane@example.com",
  createdAt: "2026-07-01T10:00:00Z",
  updatedAt: "2026-07-01T10:00:00Z",
};

beforeEach(() => {
  postMock.mockReset();
  showErrorMock.mockReset();
  postCaseMutateAsyncMock.mockReset();
  addTagMutateAsyncMock.mockReset();
  recordDeliveriesMutateAsyncMock.mockReset();
  recordDeliveriesMutateAsyncMock.mockResolvedValue({ deliveries: [] });
  listDeliveriesMock.mockReset();
  // No persisted deliveries by default — every existing test below starts
  // from a blank ledger, the same as before this hook read one at all.
  // isSuccess: true since a genuinely empty ledger is still a *successful*
  // fetch — hydration must complete (and unblock handlePublish) for it,
  // not stay stuck treating "no rows yet" as still loading.
  listDeliveriesMock.mockReturnValue({ data: null, isLoading: false, isError: false, isSuccess: true });
});

describe("usePublishAnnouncementRequest — guards", () => {
  it("does nothing when the request isn't approved", async () => {
    const { result } = renderHook(
      () => usePublishAnnouncementRequest({ ...APPROVED_REQUEST, state: "draft" }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
  });

  it("shows an error and sends nothing when there's no resolved audience", async () => {
    const { result } = renderHook(
      () => usePublishAnnouncementRequest({ ...APPROVED_REQUEST, resolvedProjectIds: [] }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/no resolved audience/i));
  });
});

describe("usePublishAnnouncementRequest — full success", () => {
  it("creates one case per resolved project, then marks the request published", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      Promise.resolve({ id: `case-${projectId}`, internalId: "X-1", number: "N-1" }),
    );
    postMock.mockResolvedValue({ ...APPROVED_REQUEST, state: "published" });

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(
      expect.objectContaining({ type: "announcement", projectId: "p-1", subject: "Scheduled maintenance" }),
    );
    expect(postMock).toHaveBeenCalledWith(
      "/announcement-requests/req-1/publish",
      { caseIds: expect.arrayContaining(["case-p-1", "case-p-2"]) },
    );
    expect((postMock.mock.calls[0][1] as { caseIds: string[] }).caseIds).toHaveLength(2);
    expect(result.current.published?.state).toBe("published");
    expect(result.current.failedProjectIds).toEqual([]);
    expect(result.current.succeededProjectIds.sort()).toEqual(["p-1", "p-2"]);
  });

  it("attaches the security tag per case when isSecurityAnnouncement is set", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1", internalId: "X-1", number: "N-1" });
    postMock.mockResolvedValue({ ...APPROVED_REQUEST, state: "published" });
    addTagMutateAsyncMock.mockResolvedValue({});

    const { result } = renderHook(
      () =>
        usePublishAnnouncementRequest({
          ...APPROVED_REQUEST,
          isSecurityAnnouncement: true,
          resolvedProjectIds: ["p-1"],
        }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(addTagMutateAsyncMock).toHaveBeenCalledWith({ caseId: "case-1", label: "Security Announcement" });
    expect(result.current.failedTagProjectIds).toEqual([]);
  });
});

describe("usePublishAnnouncementRequest — partial failure and retry", () => {
  it("keeps the succeeded project and only reports the failed one, without marking published", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "p-1"
        ? Promise.resolve({ id: "case-p-1", internalId: "X-1", number: "N-1" })
        : Promise.reject(new Error("boom")),
    );

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(result.current.succeededProjectIds).toEqual(["p-1"]);
    expect(result.current.failedProjectIds).toEqual(["p-2"]);
    expect(result.current.published).toBeNull();
    expect(postMock).not.toHaveBeenCalled();
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/failed for project p-2/i));
  });

  it("retrying only resends to the previously-failed project, not the one that already succeeded", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "p-1"
        ? Promise.resolve({ id: "case-p-1", internalId: "X-1", number: "N-1" })
        : Promise.reject(new Error("boom")),
    );

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });
    await act(async () => {
      await result.current.handlePublish();
    });
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(2);
    postCaseMutateAsyncMock.mockClear();

    // Fix the failing project and retry.
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-p-2", internalId: "X-2", number: "N-2" });
    postMock.mockResolvedValue({ ...APPROVED_REQUEST, state: "published" });

    await act(async () => {
      await result.current.handlePublish();
    });

    // Only the previously-failed project is resent — p-1 isn't duplicated.
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1);
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(expect.objectContaining({ projectId: "p-2" }));
    expect(result.current.failedProjectIds).toEqual([]);
    expect(result.current.succeededProjectIds.sort()).toEqual(["p-1", "p-2"]);
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("retries the publish call (without resending any case) when every project already succeeded but marking published failed last time", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1", internalId: "X-1", number: "N-1" });
    postMock.mockRejectedValueOnce(new Error("db down"));

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });
    await act(async () => {
      await result.current.handlePublish();
    });
    // Every project succeeded, but the bookkeeping publish call itself failed.
    expect(result.current.succeededProjectIds.sort()).toEqual(["p-1", "p-2"]);
    expect(result.current.failedProjectIds).toEqual([]);
    expect(result.current.published).toBeNull();
    postCaseMutateAsyncMock.mockClear();

    postMock.mockResolvedValueOnce({ ...APPROVED_REQUEST, state: "published" });
    await act(async () => {
      await result.current.handlePublish();
    });

    // No case is ever recreated — only the publish call itself is retried.
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    expect(postMock).toHaveBeenCalledTimes(2);
    expect(result.current.published?.state).toBe("published");
  });
});

describe("usePublishAnnouncementRequest — security-tag failure blocks publish", () => {
  it("does not mark the request published while a security tag is still missing", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1", internalId: "X-1", number: "N-1" });
    addTagMutateAsyncMock.mockRejectedValue(new Error("tag service down"));

    const { result } = renderHook(
      () =>
        usePublishAnnouncementRequest({
          ...APPROVED_REQUEST,
          isSecurityAnnouncement: true,
          resolvedProjectIds: ["p-1"],
        }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(result.current.failedTagProjectIds).toEqual(["p-1"]);
    expect(result.current.published).toBeNull();
    // The case itself was created (and must never be recreated), but the
    // request must not reach the terminal published state with its
    // security tag missing and no way to fix it afterward.
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1);
    expect(postMock).not.toHaveBeenCalled();
  });

  it("retrying a tag-only failure re-attaches the tag to the existing case, without creating another one", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1", internalId: "X-1", number: "N-1" });
    addTagMutateAsyncMock.mockRejectedValueOnce(new Error("tag service down"));

    const { result } = renderHook(
      () =>
        usePublishAnnouncementRequest({
          ...APPROVED_REQUEST,
          isSecurityAnnouncement: true,
          resolvedProjectIds: ["p-1"],
        }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });
    expect(result.current.failedTagProjectIds).toEqual(["p-1"]);
    postCaseMutateAsyncMock.mockClear();
    addTagMutateAsyncMock.mockClear();

    addTagMutateAsyncMock.mockResolvedValue({});
    postMock.mockResolvedValueOnce({ ...APPROVED_REQUEST, state: "published" });

    await act(async () => {
      await result.current.handlePublish();
    });

    // The retry re-attaches the tag on the case already created — it must
    // never call postCase again, which would send a duplicate case.
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    expect(addTagMutateAsyncMock).toHaveBeenCalledWith({ caseId: "case-1", label: "Security Announcement" });
    expect(result.current.failedTagProjectIds).toEqual([]);
    expect(result.current.published?.state).toBe("published");
  });
});

describe("usePublishAnnouncementRequest — delivery ledger", () => {
  it("records succeeded/failed outcomes for the pass after a partial failure", async () => {
    postCaseMutateAsyncMock.mockImplementation(({ projectId }: { projectId: string }) =>
      projectId === "p-2"
        ? Promise.reject(new Error("boom"))
        : Promise.resolve({ id: `case-${projectId}`, internalId: "X-1", number: "N-1" }),
    );

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(recordDeliveriesMutateAsyncMock).toHaveBeenCalledWith({
      id: "req-1",
      payload: {
        deliveries: expect.arrayContaining([
          { projectId: "p-1", caseId: "case-p-1", status: "succeeded" },
          { projectId: "p-2", status: "failed" },
        ]),
      },
    });
  });

  it("records tag_failed (with the real caseId) when the security tag attach fails", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1", internalId: "X-1", number: "N-1" });
    addTagMutateAsyncMock.mockRejectedValue(new Error("tag service down"));

    const { result } = renderHook(
      () =>
        usePublishAnnouncementRequest({
          ...APPROVED_REQUEST,
          isSecurityAnnouncement: true,
          resolvedProjectIds: ["p-1"],
        }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(recordDeliveriesMutateAsyncMock).toHaveBeenCalledWith({
      id: "req-1",
      payload: { deliveries: [{ projectId: "p-1", caseId: "case-1", status: "tag_failed" }] },
    });
  });

  it("does not fail handlePublish when saving the ledger itself fails — the real cases already exist either way", async () => {
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-1", internalId: "X-1", number: "N-1" });
    postMock.mockResolvedValue({ ...APPROVED_REQUEST, state: "published" });
    recordDeliveriesMutateAsyncMock.mockRejectedValue(new Error("ledger write failed"));

    const { result } = renderHook(
      () => usePublishAnnouncementRequest({ ...APPROVED_REQUEST, resolvedProjectIds: ["p-1"] }),
      { wrapper },
    );
    await act(async () => {
      await result.current.handlePublish();
    });

    expect(result.current.published?.state).toBe("published");
    expect(showErrorMock).toHaveBeenCalledWith(expect.stringMatching(/couldn't be saved/i));
  });

  it("hydrates from a persisted ledger so a reopened dialog resumes instead of resending to already-succeeded projects", async () => {
    listDeliveriesMock.mockReturnValue({
      data: {
        deliveries: [
          { id: "d-1", announcementRequestId: "req-1", projectId: "p-1", caseId: "case-1", status: "succeeded", createdOn: "x", updatedOn: "x" },
        ],
      },
      isLoading: false,
      isError: false,
      isSuccess: true,
    });
    postCaseMutateAsyncMock.mockResolvedValue({ id: "case-p-2", internalId: "X-1", number: "N-1" });
    postMock.mockResolvedValue({ ...APPROVED_REQUEST, state: "published" });

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });

    // Hydration seeds succeededProjectIds from the persisted ledger before
    // handlePublish is ever called.
    expect(result.current.succeededProjectIds).toEqual(["p-1"]);

    await act(async () => {
      await result.current.handlePublish();
    });

    // Only p-2 (not already in the ledger) gets a real case created.
    expect(postCaseMutateAsyncMock).toHaveBeenCalledTimes(1);
    expect(postCaseMutateAsyncMock).toHaveBeenCalledWith(expect.objectContaining({ projectId: "p-2" }));
    expect(postMock).toHaveBeenCalledWith(
      "/announcement-requests/req-1/publish",
      { caseIds: expect.arrayContaining(["case-1", "case-p-2"]) },
    );
  });

  it("hydrates tag_failed deliveries into both succeededProjectIds (the case is real) and failedTagProjectIds (the tag still needs a retry)", async () => {
    listDeliveriesMock.mockReturnValue({
      data: {
        deliveries: [
          { id: "d-1", announcementRequestId: "req-1", projectId: "p-1", caseId: "case-1", status: "tag_failed", createdOn: "x", updatedOn: "x" },
        ],
      },
      isLoading: false,
      isError: false,
      isSuccess: true,
    });

    const { result } = renderHook(
      () =>
        usePublishAnnouncementRequest({
          ...APPROVED_REQUEST,
          isSecurityAnnouncement: true,
          resolvedProjectIds: ["p-1"],
        }),
      { wrapper },
    );

    expect(result.current.succeededProjectIds).toEqual(["p-1"]);
    expect(result.current.failedTagProjectIds).toEqual(["p-1"]);

    addTagMutateAsyncMock.mockResolvedValue({});
    postMock.mockResolvedValue({ ...APPROVED_REQUEST, state: "published" });

    await act(async () => {
      await result.current.handlePublish();
    });

    // The retry re-attaches the tag on the hydrated case id — never
    // re-creates the case, since it was never lost to begin with.
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
    expect(addTagMutateAsyncMock).toHaveBeenCalledWith({ caseId: "case-1", label: "Security Announcement" });
    expect(result.current.published?.state).toBe("published");
  });
});

describe("usePublishAnnouncementRequest — readyToPublish gates handlePublish until the ledger hydrates", () => {
  // Without this gate, clicking Publish in the narrow window before the
  // ledger GET resolves would compute pendingProjectIds from an empty
  // succeededProjectIds — resending a real, duplicate case to every project
  // that already succeeded in an earlier session.
  it("refuses to run while the ledger is still loading, and reports readyToPublish false", async () => {
    listDeliveriesMock.mockReturnValue({ data: undefined, isLoading: true, isError: false, isSuccess: false });

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });

    expect(result.current.readyToPublish).toBe(false);
    expect(result.current.hydratingDeliveries).toBe(true);
    expect(result.current.hydrationFailed).toBe(false);

    await act(async () => {
      await result.current.handlePublish();
    });
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();
  });

  it("refuses to run when the ledger failed to load, and exposes a retry", async () => {
    const refetch = vi.fn();
    listDeliveriesMock.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      isSuccess: false,
      refetch,
    });

    const { result } = renderHook(() => usePublishAnnouncementRequest(APPROVED_REQUEST), { wrapper });

    expect(result.current.readyToPublish).toBe(false);
    expect(result.current.hydrationFailed).toBe(true);
    expect(result.current.hydratingDeliveries).toBe(false);

    await act(async () => {
      await result.current.handlePublish();
    });
    expect(postCaseMutateAsyncMock).not.toHaveBeenCalled();

    result.current.retryHydration();
    expect(refetch).toHaveBeenCalled();
  });

  it("is always ready when the request isn't approved — nothing to gate", () => {
    const { result } = renderHook(
      () => usePublishAnnouncementRequest({ ...APPROVED_REQUEST, state: "draft" }),
      { wrapper },
    );
    expect(result.current.readyToPublish).toBe(true);
  });
});
