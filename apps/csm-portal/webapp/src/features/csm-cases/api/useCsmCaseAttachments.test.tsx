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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

// `vi.mock` factories are hoisted above top-level `const`s, so anything a
// factory below closes over must itself be created via `vi.hoisted`.
const { postMock, uploadMock, downloadMock, fetchBlobMock, deleteMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
  uploadMock: vi.fn(),
  downloadMock: vi.fn(),
  fetchBlobMock: vi.fn(),
  deleteMock: vi.fn(),
}));

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchTags.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

// The stored-attachment flow itself is covered by entityAttachments.test.ts;
// here the hooks are checked for what they hand it.
vi.mock("@features/csm-cases/api/entityAttachments", () => ({
  WORK_ITEM_ATTACHMENT_TYPE: "work_item",
  uploadEntityAttachment: (...args: unknown[]) => uploadMock(...args),
  downloadEntityAttachment: (...args: unknown[]) => downloadMock(...args),
  fetchEntityAttachmentBlob: (...args: unknown[]) => fetchBlobMock(...args),
  deleteEntityAttachment: (...args: unknown[]) => deleteMock(...args),
  entityAttachmentErrorMessage: (err: unknown, fallback: string) =>
    err instanceof Error ? err.message : fallback,
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

import {
  useGetCsmCaseAttachments,
  useGetCsmDeletedCaseAttachments,
  usePostCsmCaseAttachment,
  useDeleteCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
  useGetCsmCaseAttachmentPreviewSource,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";
import type { BeAttachment } from "@api/backend/types";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const FILE = new File(["hello world"], "hello.txt", { type: "text/plain" });

function beAttachment(id: string): BeAttachment {
  return {
    id,
    referenceId: "case-1",
    referenceType: "case",
    name: `${id}.txt`,
    type: "text/plain",
    sizeBytes: 11,
    createdBy: { id: null, email: "jane.doe@example.com", name: "Jane Doe" },
    createdOn: "2026-07-01T00:00:00Z",
  };
}

describe("useGetCsmDeletedCaseAttachments", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("makes no request until enabled, then asks for the deleted attachments only and maps who deleted them", async () => {
    postMock.mockResolvedValue({
      attachments: [
        {
          ...beAttachment("gone-1"),
          status: "deleted",
          deletedOn: "2026-07-02T00:00:00Z",
          deletedBy: { id: null, email: "sam.admin@example.com", name: "Sam Admin" },
        },
      ],
      total: 1,
      limit: 50,
      offset: 0,
      hasMore: false,
    });

    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) =>
        useGetCsmDeletedCaseAttachments("case-1", "incident", enabled),
      { wrapper, initialProps: { enabled: false } },
    );
    expect(postMock).not.toHaveBeenCalled();

    rerender({ enabled: true });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/attachments/search",
      expect.objectContaining({
        referenceId: "case-1",
        referenceType: "incident",
        deletedOnly: true,
      }),
    );
    expect(result.current.data).toHaveLength(1);
    expect(result.current.data?.[0]).toMatchObject({
      deletedAt: "2026-07-02T00:00:00Z",
      deletedBy: "Sam Admin",
    });
  });

  it("does not send deletedOnly for the ordinary list", async () => {
    postMock.mockResolvedValue({ attachments: [], total: 0, limit: 50, offset: 0, hasMore: false });
    const { result } = renderHook(() => useGetCsmCaseAttachments("case-1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock.mock.calls[0][1]).not.toHaveProperty("deletedOnly");
  });
});

describe("useGetCsmCaseAttachments", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("follows hasMore across pages instead of stopping at the first BE_MAX_PAGE_LIMIT page", async () => {
    // Page 1: a full page (50 rows) with hasMore true. Page 2: a partial
    // page, hasMore false — the loop must fetch both and return all 53
    // attachments, not silently drop the 3 past the first page.
    const page1 = Array.from({ length: 50 }, (_, i) => beAttachment(`a${i}`));
    const page2 = Array.from({ length: 3 }, (_, i) => beAttachment(`a${50 + i}`));
    postMock
      .mockResolvedValueOnce({ attachments: page1, total: 53, limit: 50, offset: 0, hasMore: true })
      .mockResolvedValueOnce({ attachments: page2, total: 53, limit: 50, offset: 50, hasMore: false });

    const { result } = renderHook(() => useGetCsmCaseAttachments("case-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(53);
    expect(postMock).toHaveBeenCalledTimes(2);
    expect(postMock.mock.calls[0][1].pagination).toEqual({ offset: 0, limit: 50 });
    expect(postMock.mock.calls[1][1].pagination).toEqual({ offset: 50, limit: 50 });
  });

  it("stops after a single page when the BE reports no more", async () => {
    const rows = [beAttachment("only-one")];
    postMock.mockResolvedValueOnce({ attachments: rows, total: 1, limit: 50, offset: 0, hasMore: false });

    const { result } = renderHook(() => useGetCsmCaseAttachments("case-2"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toHaveLength(1);
    expect(postMock).toHaveBeenCalledTimes(1);
  });

  it("stops at the safety bound if the BE reports hasMore forever", async () => {
    // Every page is a full, `hasMore: true` page — without the
    // MAX_ATTACHMENT_PAGES bound this would loop forever.
    postMock.mockImplementation(() =>
      Promise.resolve({
        attachments: Array.from({ length: 50 }, (_, i) => beAttachment(`x${i}`)),
        total: 999_999,
        limit: 50,
        offset: 0,
        hasMore: true,
      }),
    );

    const { result } = renderHook(() => useGetCsmCaseAttachments("case-3"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // 200 pages * 50 rows/page, per MAX_ATTACHMENT_PAGES in the hook.
    expect(result.current.data).toHaveLength(200 * 50);
    expect(postMock).toHaveBeenCalledTimes(200);
  });
});

describe("usePostCsmCaseAttachment", () => {
  beforeEach(() => {
    postMock.mockReset();
    uploadMock.mockReset();
  });

  it("uploads the file as a work item attachment through the chunked flow", async () => {
    uploadMock.mockResolvedValue({ id: "att-1" });
    const { result } = renderHook(() => usePostCsmCaseAttachment(), { wrapper });

    await act(() =>
      result.current.mutateAsync({ caseId: "case-1", file: FILE, uploadedBy: "Jane Doe" }),
    );

    expect(uploadMock).toHaveBeenCalledTimes(1);
    expect(uploadMock.mock.calls[0][1]).toMatchObject({
      type: "work_item",
      parentId: "case-1",
      file: FILE,
    });
    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.uploadProgress).toBeNull();
  });

  it.each(["change_request", "incident"] as const)(
    "uploads a %s attachment as a work item attachment and refreshes that list",
    async (referenceType) => {
      uploadMock.mockResolvedValue({ id: "att-1" });
      const { result } = renderHook(() => usePostCsmCaseAttachment(), { wrapper });

      await act(() =>
        result.current.mutateAsync({
          caseId: "item-1",
          file: FILE,
          uploadedBy: "Jane Doe",
          referenceType,
        }),
      );

      expect(uploadMock.mock.calls[0][1]).toMatchObject({ type: "work_item", parentId: "item-1" });
    },
  );

  it("fails with the upload's message", async () => {
    uploadMock.mockRejectedValue(new Error("The upload is larger than allowed."));
    const { result } = renderHook(() => usePostCsmCaseAttachment(), { wrapper });

    await expect(
      act(() => result.current.mutateAsync({ caseId: "case-1", file: FILE, uploadedBy: "Jane Doe" })),
    ).rejects.toThrow("The upload is larger than allowed.");
    expect(result.current.uploadProgress).toBeNull();
  });
});

const ATTACHMENT: CaseAttachment = {
  id: "att-1",
  filename: "hello.txt",
  size: 11,
  contentType: "text/plain",
  uploadedBy: "Jane Doe",
  uploadedAt: "2026-01-01T00:00:00Z",
};

describe("useDownloadCsmCaseAttachment", () => {
  beforeEach(() => {
    downloadMock.mockReset();
  });

  it("streams the attachment through the backend as a work item attachment", async () => {
    downloadMock.mockResolvedValue(undefined);
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), { wrapper });

    await result.current(ATTACHMENT);

    expect(downloadMock).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        type: "work_item",
        id: "att-1",
        name: ATTACHMENT.filename,
        size: ATTACHMENT.size,
        directAboveBytes: 0,
        onSaved: expect.any(Function),
      }),
    );
  });

  it("lets a refusal through", async () => {
    downloadMock.mockRejectedValue(new Error("This attachment was removed."));
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), { wrapper });

    await expect(result.current(ATTACHMENT)).rejects.toThrow("This attachment was removed.");
  });
});

describe("useGetCsmCaseAttachmentPreviewSource", () => {
  const PDF: CaseAttachment = { ...ATTACHMENT, filename: "report.pdf", size: 2048, contentType: "application/pdf" };

  beforeEach(() => {
    fetchBlobMock.mockReset();
  });

  it("streams the file and wraps it in a revocable object URL", async () => {
    fetchBlobMock.mockResolvedValue(new Blob(["fake"], { type: "application/pdf" }));
    const createObjectUrlSpy = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:mock-url");
    const { result } = renderHook(() => useGetCsmCaseAttachmentPreviewSource(), { wrapper });

    const source = await result.current(PDF);

    expect(fetchBlobMock).toHaveBeenCalledWith(expect.anything(), { type: "work_item", id: "att-1" });
    expect(createObjectUrlSpy).toHaveBeenCalledTimes(1);
    expect(source).toEqual({ url: "blob:mock-url", revoke: true });
    createObjectUrlSpy.mockRestore();
  });

  it("re-labels the blob only when the listed type is on the safe list", async () => {
    fetchBlobMock.mockResolvedValue(new Blob(["x"], { type: "application/octet-stream" }));
    const createObjectUrlSpy = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:mock-url");
    const { result } = renderHook(() => useGetCsmCaseAttachmentPreviewSource(), { wrapper });

    await result.current({ ...PDF, contentType: "text/html" });
    expect((createObjectUrlSpy.mock.calls[0][0] as Blob).type).toBe("application/octet-stream");
    await result.current(PDF);
    expect((createObjectUrlSpy.mock.calls[1][0] as Blob).type).toBe("application/pdf");
    createObjectUrlSpy.mockRestore();
  });
});

describe("useDeleteCsmCaseAttachment", () => {
  beforeEach(() => {
    deleteMock.mockReset();
  });

  it("deletes through the stored-attachment flow and passes the backend's refusal on", async () => {
    deleteMock.mockResolvedValueOnce(undefined);
    const { result } = renderHook(() => useDeleteCsmCaseAttachment(), { wrapper });

    await act(() => result.current.mutateAsync({ caseId: "case-1", attachmentId: "att-1" }));
    expect(deleteMock).toHaveBeenCalledWith(expect.anything(), { type: "work_item", id: "att-1" });

    deleteMock.mockRejectedValueOnce(new Error("Only the person who uploaded an attachment can delete it."));
    await expect(
      act(() => result.current.mutateAsync({ caseId: "case-1", attachmentId: "att-1" })),
    ).rejects.toThrow("Only the person who uploaded an attachment can delete it.");
  });
});
