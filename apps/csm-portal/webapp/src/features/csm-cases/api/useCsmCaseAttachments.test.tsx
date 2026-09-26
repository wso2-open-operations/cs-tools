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
const { postMock, getBlobMock, uploadFileViaTusMock, sftpgoFlag } = vi.hoisted(
  () => ({
    postMock: vi.fn(),
    getBlobMock: vi.fn(),
    uploadFileViaTusMock: vi.fn(),
    sftpgoFlag: { enabled: false },
  }),
);

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchTags.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, getBlob: getBlobMock }),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { sftpgoAttachmentStorageEnabled: sftpgoFlag.enabled },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/csm-cases/api/attachmentStorageTus", () => ({
  uploadFileViaTus: uploadFileViaTusMock,
}));

vi.mock("@utils/saveBlob", () => ({ saveBlob: vi.fn() }));

import {
  useGetCsmCaseAttachments,
  usePostCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
  useGetCsmCaseAttachmentPreviewSource,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import { saveBlob } from "@utils/saveBlob";
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
    getBlobMock.mockReset();
    uploadFileViaTusMock.mockReset();
    sftpgoFlag.enabled = false;
  });

  it("flag off: sends a single POST /attachments with a base64 file payload, unchanged", async () => {
    postMock.mockResolvedValue({});
    const { result } = renderHook(() => usePostCsmCaseAttachment(), {
      wrapper,
    });

    await act(() =>
      result.current.mutateAsync({
        caseId: "case-1",
        file: FILE,
        uploadedBy: "Jane Doe",
      }),
    );

    expect(uploadFileViaTusMock).not.toHaveBeenCalled();
    expect(postMock).toHaveBeenCalledTimes(1);
    const [path, payload] = postMock.mock.calls[0];
    expect(path).toBe("/attachments");
    expect(payload.storageKey).toBeUndefined();
    expect(payload.sizeBytes).toBeUndefined();
    expect(typeof payload.file).toBe("string");
    expect(payload.file.startsWith("data:")).toBe(true);
    expect(result.current.uploadProgress).toBeNull();
  });

  it("flag on: mints an upload token, uploads via TUS, then confirms with no body", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockImplementation((path: string) => {
      if (path.endsWith("/attachments/upload-token")) {
        return Promise.resolve({
          id: "att-1",
          shareId: "share-1",
          sftpgoBaseUrl: "https://sftpgo.example.com",
          storageKey: "/attachments/cases/case-1/att-1",
        });
      }
      return Promise.resolve({});
    });
    uploadFileViaTusMock.mockResolvedValue(undefined);

    const { result } = renderHook(() => usePostCsmCaseAttachment(), {
      wrapper,
    });

    await act(() =>
      result.current.mutateAsync({
        caseId: "case-1",
        file: FILE,
        uploadedBy: "Jane Doe",
      }),
    );

    // Order: mint token, then TUS upload, then confirm.
    expect(postMock).toHaveBeenCalledTimes(2);
    const [tokenPath, tokenPayload] = postMock.mock.calls[0];
    expect(tokenPath).toBe("/cases/case-1/attachments/upload-token");
    expect(tokenPayload).toEqual({
      filename: FILE.name,
      mimeType: FILE.type,
      sizeBytes: FILE.size,
      description: null,
      referenceType: "case",
    });

    expect(uploadFileViaTusMock).toHaveBeenCalledTimes(1);
    expect(uploadFileViaTusMock.mock.calls[0][0]).toMatchObject({
      sftpgoBaseUrl: "https://sftpgo.example.com",
      shareId: "share-1",
      storageKey: "/attachments/cases/case-1/att-1",
      file: FILE,
    });

    const [confirmPath, confirmPayload] = postMock.mock.calls[1];
    expect(confirmPath).toBe("/cases/case-1/attachments/att-1/confirm");
    expect(confirmPayload).toEqual({});

    // The TUS call must have happened before the confirm call.
    const tusCallOrder = uploadFileViaTusMock.mock.invocationCallOrder[0];
    const confirmCallOrder = postMock.mock.invocationCallOrder[1];
    expect(tusCallOrder).toBeLessThan(confirmCallOrder);

    expect(result.current.uploadProgress).toBeNull();
  });

  it("flag on: exposes upload progress from the TUS callback while in flight", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockImplementation((path: string) => {
      if (path.endsWith("/attachments/upload-token")) {
        return Promise.resolve({
          id: "att-1",
          shareId: "share-1",
          sftpgoBaseUrl: "https://sftpgo.example.com",
          storageKey: "/attachments/cases/case-1/att-1",
        });
      }
      return Promise.resolve({});
    });

    // A deferred promise the test resolves explicitly, so the upload is
    // guaranteed to still be "in flight" while the progress assertion below
    // runs — a `setTimeout(0)` here would race the real TUS call's resolution
    // against `waitFor`'s own polling and could resolve before the test gets
    // a chance to observe the in-flight percentage.
    let resolveUpload: (() => void) | undefined;
    let capturedOnProgress: ((percent: number) => void) | undefined;
    uploadFileViaTusMock.mockImplementation(({ onProgress }) => {
      capturedOnProgress = onProgress;
      return new Promise<void>((resolve) => {
        resolveUpload = resolve;
      });
    });

    const { result } = renderHook(() => usePostCsmCaseAttachment(), {
      wrapper,
    });

    let promise!: Promise<unknown>;
    act(() => {
      promise = result.current.mutateAsync({
        caseId: "case-1",
        file: FILE,
        uploadedBy: "Jane Doe",
      });
    });

    await waitFor(() => expect(capturedOnProgress).toBeDefined());
    act(() => capturedOnProgress!(42));
    await waitFor(() => expect(result.current.uploadProgress).toBe(42));

    await act(async () => {
      resolveUpload!();
      await promise;
    });
    await waitFor(() => expect(result.current.uploadProgress).toBeNull());
  });

  it.each(["change_request", "incident"] as const)(
    "flag on, referenceType %s: skips the mint endpoint and uses the legacy base64 path",
    async (referenceType) => {
      sftpgoFlag.enabled = true;
      postMock.mockResolvedValue({});

      const { result } = renderHook(() => usePostCsmCaseAttachment(), {
        wrapper,
      });

      await act(() =>
        result.current.mutateAsync({
          caseId: "case-1",
          file: FILE,
          uploadedBy: "Jane Doe",
          referenceType,
        }),
      );

      expect(uploadFileViaTusMock).not.toHaveBeenCalled();
      expect(postMock).toHaveBeenCalledTimes(1);
      const [path, payload] = postMock.mock.calls[0];
      expect(path).toBe("/attachments");
      expect(payload.referenceType).toBe(referenceType);
      expect(typeof payload.file).toBe("string");
      expect(payload.file.startsWith("data:")).toBe(true);
      expect(result.current.uploadProgress).toBeNull();
    },
  );
});

describe("useDownloadCsmCaseAttachment", () => {
  const ATTACHMENT: CaseAttachment = {
    id: "att-1",
    filename: "hello.txt",
    size: 11,
    contentType: "text/plain",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-01T00:00:00Z",
  };

  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    sftpgoFlag.enabled = false;
    vi.mocked(saveBlob).mockReset();
  });

  it("flag off: fetches the content blob and saves it, unchanged", async () => {
    getBlobMock.mockResolvedValue(new Blob(["hello"], { type: "text/plain" }));
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), {
      wrapper,
    });

    await result.current(ATTACHMENT);

    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).toHaveBeenCalledWith("/attachments/att-1/content");
    expect(saveBlob).toHaveBeenCalledTimes(1);
  });

  it("flag on: creates exactly one share for the clicked attachment and never fetches the content blob", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockResolvedValue({
      shareUrl: "https://sftpgo.example.com/web/client/pubshares/abc",
    });
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), {
      wrapper,
    });

    await result.current(ATTACHMENT);

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith("/attachments/att-1/share", {});
    expect(getBlobMock).not.toHaveBeenCalled();
    expect(saveBlob).not.toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalledTimes(1);

    clickSpy.mockRestore();
  });
});

describe("useGetCsmCaseAttachmentPreviewSource", () => {
  const ATTACHMENT: CaseAttachment = {
    id: "att-1",
    filename: "report.pdf",
    size: 2048,
    contentType: "application/pdf",
    uploadedBy: "Jane Doe",
    uploadedAt: "2026-01-01T00:00:00Z",
  };

  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    sftpgoFlag.enabled = false;
  });

  it("flag off: fetches the content blob and wraps it in a revocable object URL", async () => {
    getBlobMock.mockResolvedValue(
      new Blob(["fake"], { type: "application/pdf" }),
    );
    const createObjectUrlSpy = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:mock-url");

    const { result } = renderHook(() => useGetCsmCaseAttachmentPreviewSource(), {
      wrapper,
    });

    const source = await result.current(ATTACHMENT);

    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).toHaveBeenCalledWith("/attachments/att-1/content");
    expect(createObjectUrlSpy).toHaveBeenCalledTimes(1);
    expect(source).toEqual({ url: "blob:mock-url", revoke: true });

    createObjectUrlSpy.mockRestore();
  });

  it("flag on: resolves a read-scoped share URL as-is, without fetching the content blob", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockResolvedValue({
      shareUrl: "https://sftpgo.example.com/web/client/pubshares/abc",
    });

    const { result } = renderHook(() => useGetCsmCaseAttachmentPreviewSource(), {
      wrapper,
    });

    const source = await result.current(ATTACHMENT);

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith("/attachments/att-1/share", {});
    expect(getBlobMock).not.toHaveBeenCalled();
    expect(source).toEqual({
      url: "https://sftpgo.example.com/web/client/pubshares/abc",
      revoke: false,
    });
  });
});
