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

const { postMock, getBlobMock, sftpgoFlag, userRoles } = vi.hoisted(() => ({
  postMock: vi.fn(),
  getBlobMock: vi.fn(),
  sftpgoFlag: { enabled: false },
  // Defaults to a role that can download attachments, so the existing
  // resolution tests below exercise the real fetch/data-URL path; the
  // permission tests further down override this per-case.
  userRoles: { value: ["cs_engineer"] as string[] },
}));

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, getBlob: getBlobMock }),
}));

vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: {
      sftpgoAttachmentStorageEnabled: sftpgoFlag.enabled,
      roles: userRoles.value,
    },
    isLoading: false,
    isError: false,
  }),
}));

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

// These cases cover the older ServiceNow flow, so the entity-service flow
// (on by default) is switched off for the whole file.
beforeEach(() => {
  window.config = { CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED: false } as unknown as typeof window.config;
});

describe("useResolvedInlineImageHtml", () => {
  beforeEach(() => {
    postMock.mockReset();
    getBlobMock.mockReset();
    sftpgoFlag.enabled = false;
    userRoles.value = ["cs_engineer"];
  });

  it("flag off: resolves via GET /attachments/{id}/content into a data: URL", async () => {
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

  it("flag on: resolves via POST /attachments/{id}/share and uses shareUrl directly, without fetching content", async () => {
    sftpgoFlag.enabled = true;
    postMock.mockResolvedValue({
      shareUrl: "https://sftpgo.example.com/web/client/pubshares/abc?compress=false",
    });

    const { result } = renderHook(() => useResolvedInlineImageHtml(HTML), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(getBlobMock).not.toHaveBeenCalled();
    expect(postMock).toHaveBeenCalledTimes(1);
    const [calledPath, calledBody] = postMock.mock.calls[0];
    expect(calledPath).toMatch(/\/attachments\/.+\/share$/);
    expect(calledBody).toEqual({});
    expect(result.current.resolvedHtml).toContain(
      "https://sftpgo.example.com/web/client/pubshares/abc?compress=false",
    );
    expect(result.current.resolvedHtml).not.toContain("data:");
  });

  describe("bare attachment-id src (migrated content)", () => {
    const UUID = "0f15cbcc-c36b-8310-af2f-404599013196";
    const BARE_HTML = `<p><img src="/${UUID}"><br></p>`;

    it("flag off: fetches content by canonical uuid and resolves to a data: URL", async () => {
      getBlobMock.mockResolvedValue(new Blob(["fake"], { type: "image/png" }));
      const { result } = renderHook(
        () => useResolvedInlineImageHtml(BARE_HTML),
        { wrapper },
      );
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(getBlobMock).toHaveBeenCalledTimes(1);
      expect(getBlobMock.mock.calls[0][0]).toBe(`/attachments/${UUID}/content`);
      expect(result.current.resolvedHtml).toContain("data:image/png;base64,");
    });

    it("flag on: creates one share for the canonical uuid and uses its url", async () => {
      sftpgoFlag.enabled = true;
      postMock.mockResolvedValue({ shareUrl: "https://sftpgo.example.com/s/abc" });
      const { result } = renderHook(
        () => useResolvedInlineImageHtml(BARE_HTML),
        { wrapper },
      );
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(postMock).toHaveBeenCalledTimes(1);
      expect(postMock.mock.calls[0][0]).toBe(`/attachments/${UUID}/share`);
      expect(result.current.resolvedHtml).toContain(
        "https://sftpgo.example.com/s/abc",
      );
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

  // A raw base64-embedded image (content authored before/without SFTPGo
  // attachment storage) has no .iix reference at all -- nothing to fetch,
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
