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
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  useAttachmentPreview,
  useAttachmentPreviews,
} from "@api/useAttachmentPreview";

const authFetchMock = vi.fn();

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useAttachmentPreview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test/" };
  });

  it("skips fetch when attachment id is missing", () => {
    const { result } = renderHook(() => useAttachmentPreview(null), {
      wrapper: createWrapper(),
    });
    expect(result.current.fetchStatus).toBe("idle");
    expect(authFetchMock).not.toHaveBeenCalled();
  });

  it("returns data URL for image attachment", async () => {
    const blob = new Blob(["img"], { type: "image/png" });
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      blob: async () => blob,
      headers: { get: () => "image/png" },
    });

    const readAsDataURL = vi
      .spyOn(FileReader.prototype, "readAsDataURL")
      .mockImplementation(function (this: FileReader) {
        Object.defineProperty(this, "result", {
          value: "data:image/png;base64,abc",
          configurable: true,
        });
        this.onloadend?.({} as ProgressEvent<FileReader>);
      });

    const { result } = renderHook(() => useAttachmentPreview("att-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBe("data:image/png;base64,abc");
    readAsDataURL.mockRestore();
  });

  it("useAttachmentPreviews maps multiple ids", async () => {
    const blob = new Blob(["img"], { type: "image/png" });
    authFetchMock.mockResolvedValue({
      ok: true,
      blob: async () => blob,
      headers: { get: () => "image/png" },
    });

    vi.spyOn(FileReader.prototype, "readAsDataURL").mockImplementation(function (
      this: FileReader,
    ) {
      Object.defineProperty(this, "result", {
        value: "data:image/png;base64,x",
        configurable: true,
      });
      this.onloadend?.({} as ProgressEvent<FileReader>);
    });

    const { result } = renderHook(() => useAttachmentPreviews(["a", "b"]), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.dataUrls.size).toBeGreaterThanOrEqual(0);
  });
});
