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
});
