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

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const { postMock, getBlobMock, userRoles } = vi.hoisted(() => ({
  postMock: vi.fn(),
  getBlobMock: vi.fn(),
  // Defaults to a role that can download attachments, so the existing
  // resolution tests below exercise the real fetch/data-URL path; the
  // permission tests further down override this per-case.
  userRoles: { value: ["cs_engineer"] as string[] },
}));

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, getBlob: getBlobMock }),
  BackendApiError: class BackendApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: {
      roles: userRoles.value,
    },
    isLoading: false,
    isError: false,
  }),
}));

import { BackendApiError } from "@api/backend/client";
import { useResolvedInlineImageHtml } from "@features/csm-cases/api/useResolvedInlineImageHtml";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const ATTACHMENT_SYSID = "0123456789abcdef0123456789abcdef";
const HTML = `<p>see <img src="/inline/${ATTACHMENT_SYSID}.iix"></p>`;

describe("useResolvedInlineImageHtml", () => {
  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    userRoles.value = ["cs_engineer"];
  });

  it("resolves via the streamed comment-image route into a data: URL", async () => {
    getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));

    const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).toHaveBeenCalledTimes(1);
    const [calledPath] = getBlobMock.mock.calls[0];
    expect(calledPath).toContain("/content");
    expect(result.current.resolvedHtml).toContain("data:image/png;base64,");
  });

  describe("bare attachment-id src (migrated content)", () => {
    const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
    const BARE_HTML = `<p><img src="/${UUID}"><br></p>`;

    it("fetches content by canonical uuid and resolves to a data: URL", async () => {
      getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
      const { result } = renderHook(
        () => useResolvedInlineImageHtml(BARE_HTML),
        { wrapper },
      );
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(getBlobMock).toHaveBeenCalledTimes(1);
      expect(getBlobMock.mock.calls[0][0]).toBe(`/attachments/inline/${UUID}/content`);
      expect(result.current.resolvedHtml).toContain("data:image/png;base64,");
    });

    it("does not double-fetch when the same attachment appears bare and as .iix", async () => {
      getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
      const html = `<img src="/${UUID}"><img src="/${UUID.replace(/-/g, "")}.iix">`;
      const { result } = renderHook(() => useResolvedInlineImageHtml(html), {
        wrapper,
      });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(getBlobMock).toHaveBeenCalledTimes(1);
      expect(result.current.resolvedHtml).not.toContain("<img src=\"/");
    });

    it("without the download role, never fetches and shows the permission placeholder", () => {
      userRoles.value = ["viewer"];
      const { result } = renderHook(
        () => useResolvedInlineImageHtml(BARE_HTML),
        { wrapper },
      );
      expect(getBlobMock).not.toHaveBeenCalled();
      expect(postMock).not.toHaveBeenCalled();
      expect(result.current.resolvedHtml).toContain(
        'data-unresolved-reason="permission"',
      );
    });
  });

  it("does not resolve anything when the HTML has no .iix references", () => {
    const { result } = renderHook(
      () => useResolvedInlineImageHtml("<p>no images here</p>"),
      { wrapper },
    );

    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).not.toHaveBeenCalled();
    expect(result.current.isLoading).toBe(false);
  });

  it("without the attachment-download role, never fetches and shows a permission placeholder", async () => {
    userRoles.value = ["viewer"];

    const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(false);
    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).not.toHaveBeenCalled();
    expect(result.current.resolvedHtml).not.toContain("<img");
    expect(result.current.resolvedHtml).toContain('data-unresolved-reason="permission"');
    expect(result.current.resolvedHtml).toContain(
      "You don't have permission to view this image",
    );
  });

  it("the attachment-downloader role alone is enough to resolve images", async () => {
    userRoles.value = ["attachment_downloader"];
    getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));

    const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(getBlobMock).toHaveBeenCalledTimes(1);
    expect(result.current.resolvedHtml).toContain("data:image/png;base64,");
  });

  // A raw base64-embedded image (content authored before images were stored
  // as attachments) has no .iix reference at all -- nothing to fetch,
  // since there's no separate attachment record. See
  // useResolvedInlineImageHtml's own doc comment for why this still needs
  // hiding, even though the underlying content already reached the browser.
  const RAW_BASE64_HTML =
    '<p>see <img src="data:image/png;base64,AAAA"></p>';

  it("without the attachment-download role, a raw base64 image is hidden behind the permission placeholder", () => {
    userRoles.value = ["viewer"];

    const { result } = renderHook(
      () => useResolvedInlineImageHtml(RAW_BASE64_HTML),
      { wrapper },
    );

    expect(postMock).not.toHaveBeenCalled();
    expect(getBlobMock).not.toHaveBeenCalled();
    expect(result.current.resolvedHtml).not.toContain("<img");
    expect(result.current.resolvedHtml).toContain(
      'data-unresolved-reason="permission"',
    );
  });

  it("with the attachment-download role, a raw base64 image renders as-is", () => {
    const { result } = renderHook(
      () => useResolvedInlineImageHtml(RAW_BASE64_HTML),
      { wrapper },
    );

    expect(result.current.resolvedHtml).toBe(RAW_BASE64_HTML);
  });
});

describe("useResolvedInlineImageHtml: entity-service images that are gone", () => {
  const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
  const BARE_HTML = `<p><img src="/${UUID}"></p>`;

  beforeEach(() => {
    getBlobMock.mockReset();
    postMock.mockReset();
    userRoles.value = ["cs_engineer"];
  });

  it("a 410 shows the removed message and tries nothing else", async () => {
    getBlobMock.mockRejectedValue(new BackendApiError(410, "This image was removed."));
    const { result } = renderHook(() => useResolvedInlineImageHtml(BARE_HTML), { wrapper });
    await waitFor(() => expect(result.current.resolvedHtml).toContain("This image was removed."));
    expect(result.current.resolvedHtml).toContain('data-unresolved-reason="removed"');
    expect(result.current.resolvedHtml).not.toContain(UUID);
    expect(getBlobMock).toHaveBeenCalledTimes(1);
  });

  it("a 500 shows the not-available message", async () => {
    getBlobMock.mockRejectedValue(new BackendApiError(500, "boom"));
    const { result } = renderHook(() => useResolvedInlineImageHtml(BARE_HTML), { wrapper });
    // The query retries once (about a second) before it gives up.
    await waitFor(() => expect(result.current.isLoading).toBe(false), { timeout: 5000 });
    expect(result.current.resolvedHtml).toContain("This image is not available.");
    expect(result.current.resolvedHtml).not.toContain("was removed");
  });
});
