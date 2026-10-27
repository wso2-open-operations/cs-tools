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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const { postMock, getBlobMock, delMock, saveBlobMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
  getBlobMock: vi.fn(),
  delMock: vi.fn(),
  saveBlobMock: vi.fn(),
}));

vi.mock("@api/backend/client", () => {
  class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  }
  return {
    BackendApiError,
    useBackendApi: () => ({ post: postMock, getBlob: getBlobMock, del: delMock }),
  };
});

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { sftpgoAttachmentStorageEnabled: false, roles: ["cs_engineer"] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@utils/saveBlob", () => ({ saveBlob: saveBlobMock }));

import { BackendApiError } from "@api/backend/client";
import {
  useDeleteCsmCaseAttachment,
  useDownloadCsmCaseAttachment,
  useGetCsmCaseAttachmentPreviewSource,
  usePostCsmCaseAttachment,
} from "@features/csm-cases/api/useCsmCaseAttachments";
import { useResolvedInlineImageHtml } from "@features/csm-cases/api/useResolvedInlineImageHtml";
import type { CaseAttachment } from "@features/csm-cases/types/csmCases";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

const ATT: CaseAttachment = {
  id: "11111111-1111-4111-8111-111111111111",
  filename: "report.pdf",
  size: 11,
  contentType: "application/pdf",
  uploadedBy: "Jane",
  uploadedAt: "2026-07-01T00:00:00Z",
};

function apiError(status: number): Error {
  return new (BackendApiError as unknown as new (s: number, m: string) => Error)(status, "x");
}

function enable(on: boolean): void {
  window.config = { CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED: on } as unknown as typeof window.config;
}

const link = (over: Record<string, unknown> = {}) => ({
  id: ATT.id,
  number: "WIA1",
  name: "Entity Name.pdf",
  available: true,
  url: "https://files.test/s/abc",
  ...over,
});

describe("entity attachments behind the flag", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    delMock.mockReset();
    saveBlobMock.mockReset();
    fetchMock = vi.fn(async () => new Response("bytes", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    URL.createObjectURL = vi.fn(() => "blob:preview");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    window.config = {} as typeof window.config;
  });

  it("flag off: download never touches the entity routes", async () => {
    enable(false);
    getBlobMock.mockResolvedValue(new Blob(["x"], { type: "application/pdf" }));
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await result.current(ATT);
    });
    expect(postMock).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(getBlobMock).toHaveBeenCalledWith("/attachments/" + ATT.id + "/content");
  });

  it("flag on: download fetches the one-use link and saves under the attachment's name", async () => {
    enable(true);
    postMock.mockResolvedValue({ links: [link()] });
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await result.current(ATT);
    });
    expect(postMock).toHaveBeenCalledWith("/attachments/download", {
      type: "work_item",
      ids: [ATT.id],
      purpose: "download",
    });
    expect(fetchMock).toHaveBeenCalledWith("https://files.test/s/abc", { method: "GET", credentials: "omit" });
    expect(saveBlobMock.mock.calls[0][1]).toBe("Entity Name.pdf");
    expect(getBlobMock).not.toHaveBeenCalled();
  });

  it("flag on: an attachment entity-service does not hold falls back to the old download", async () => {
    enable(true);
    postMock.mockResolvedValue({ links: [] });
    getBlobMock.mockResolvedValue(new Blob(["x"], { type: "application/pdf" }));
    const { result } = renderHook(() => useDownloadCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await result.current(ATT);
    });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(getBlobMock).toHaveBeenCalledWith("/attachments/" + ATT.id + "/content");
    expect(saveBlobMock.mock.calls[0][1]).toBe("report.pdf");
  });

  it("flag on: preview asks for a preview link and returns a revocable blob URL", async () => {
    enable(true);
    postMock.mockResolvedValue({ links: [link()] });
    const { result } = renderHook(() => useGetCsmCaseAttachmentPreviewSource(), { wrapper });
    let source: { url: string; revoke: boolean } | undefined;
    await act(async () => {
      source = await result.current(ATT);
    });
    expect(postMock.mock.calls[0][1]).toMatchObject({ purpose: "preview" });
    expect(source).toEqual({ url: "blob:preview", revoke: true });
  });

  it("flag on: upload runs the three steps and never sends the legacy base64 payload", async () => {
    enable(true);
    postMock
      .mockResolvedValueOnce({
        attachmentId: "att-1",
        number: "WIA1",
        uploadUrl: "https://files.test/up",
        fieldName: "filenames",
        expiresAt: "2026-10-27T10:15:00Z",
        maxBytes: 100 * 1024 * 1024,
      })
      .mockResolvedValueOnce({ id: "att-1", status: "COMPLETE" });
    fetchMock.mockResolvedValue(new Response("", { status: 201 }));
    const { result } = renderHook(() => usePostCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({
        caseId: "case-1",
        file: new File(["hello world"], "hello.txt", { type: "text/plain" }),
        uploadedBy: "Jane",
      });
    });
    expect(postMock.mock.calls.map((c) => c[0])).toEqual([
      "/attachments/upload",
      "/attachments/upload/att-1/confirm",
    ]);
    expect(postMock.mock.calls[0][1]).toMatchObject({ type: "work_item", parentId: "case-1", sizeBytes: 11 });
    await waitFor(() => expect(result.current.uploadProgress).toBeNull());
  });

  it("flag on: an upload refused for a closed case shows the backend's message", async () => {
    enable(true);
    postMock.mockRejectedValueOnce(
      Object.assign(apiError(409), { message: "This case is closed, so new attachments can't be added." }),
    );
    const { result } = renderHook(() => usePostCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await expect(
        result.current.mutateAsync({
          caseId: "case-1",
          file: new File(["x"], "a.txt"),
          uploadedBy: "Jane",
        }),
      ).rejects.toThrow("This case is closed, so new attachments can't be added.");
    });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("flag on: delete posts to /attachments/delete; a 404 falls back to the old DELETE", async () => {
    enable(true);
    postMock.mockResolvedValueOnce(null);
    const { result } = renderHook(() => useDeleteCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ caseId: "case-1", attachmentId: ATT.id });
    });
    expect(postMock).toHaveBeenCalledWith("/attachments/delete", { type: "work_item", id: ATT.id });
    expect(delMock).not.toHaveBeenCalled();

    postMock.mockRejectedValueOnce(apiError(404));
    delMock.mockResolvedValueOnce(null);
    await act(async () => {
      await result.current.mutateAsync({ caseId: "case-1", attachmentId: ATT.id });
    });
    expect(delMock).toHaveBeenCalledWith("/attachments/" + ATT.id);
  });

  it("flag on: a 403 on delete is reported, not retried the old way", async () => {
    enable(true);
    postMock.mockRejectedValueOnce(
      Object.assign(apiError(403), { message: "Only the person who uploaded an attachment can delete it." }),
    );
    const { result } = renderHook(() => useDeleteCsmCaseAttachment(), { wrapper });
    await act(async () => {
      await expect(
        result.current.mutateAsync({ caseId: "case-1", attachmentId: ATT.id }),
      ).rejects.toThrow("Only the person who uploaded an attachment can delete it.");
    });
    expect(delMock).not.toHaveBeenCalled();
  });

  describe("comment images", () => {
    const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
    const HTML = `<p><img src="/${UUID}"></p>`;

    it("flag on: streams the image from the entity route into a data: URL", async () => {
      enable(true);
      getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
      const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), { wrapper });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(getBlobMock).toHaveBeenCalledTimes(1);
      expect(getBlobMock.mock.calls[0][0]).toBe(`/attachments/inline/${UUID}/content`);
      expect(result.current.resolvedHtml).toContain("data:image/png;base64,");
    });

    it("flag on: an image entity-service does not hold (404) falls back to the old content route", async () => {
      enable(true);
      getBlobMock
        .mockRejectedValueOnce(apiError(404))
        .mockResolvedValueOnce(new Blob(["fake"], { type: "image/png" }));
      const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), { wrapper });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(getBlobMock.mock.calls.map((c) => c[0])).toEqual([
        `/attachments/inline/${UUID}/content`,
        `/attachments/${UUID}/content`,
      ]);
      expect(result.current.resolvedHtml).toContain("data:image/png;base64,");
    });

    it("flag off: never calls the entity route", async () => {
      enable(false);
      getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
      const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), { wrapper });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(getBlobMock.mock.calls.map((c) => c[0])).toEqual([`/attachments/${UUID}/content`]);
    });
  });
});
