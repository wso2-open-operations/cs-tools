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

import { renderHook, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { useGetAttachmentContent } from "@api/useGetAttachmentContent";

const mockAuthFetch = vi.fn();

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

const testWindowConfig: Window["config"] = {
  CUSTOMER_PORTAL_AUTH_BASE_URL: "https://auth",
  CUSTOMER_PORTAL_AUTH_CLIENT_ID: "cid",
  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: "https://in",
  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: "https://out",
  CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.example/v1.0",
  CUSTOMER_PORTAL_THEME: "default",
  CUSTOMER_PORTAL_LOG_LEVEL: "error",
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE: false,
};

describe("useGetAttachmentContent", () => {
  let queryClient: QueryClient;
  let prevConfig: Window["config"];
  let openSpy: ReturnType<typeof vi.spyOn>;
  let createObjectURLSpy: ReturnType<typeof vi.spyOn>;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    prevConfig = window.config;
    window.config = testWindowConfig;
    openSpy = vi.spyOn(window, "open").mockImplementation(() => null);

    createObjectURLSpy = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob://test");

    queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.config = prevConfig;
    openSpy.mockRestore();
    createObjectURLSpy.mockRestore();
  });

  it("should GET /attachments/:id/content and trigger blob download", async () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click");
    const testBlob = new Blob(["abc"], {
      type: "application/octet-stream",
    });

    mockAuthFetch.mockResolvedValue({
      ok: true,
      headers: {
        get: (name: string) => {
          if (name === "content-type") return "application/octet-stream";
          if (name === "content-disposition")
            return 'attachment; filename="downloaded.txt"';
          return null;
        },
      },
      blob: () => Promise.resolve(testBlob),
    } as unknown as Response);

    const { result } = renderHook(() => useGetAttachmentContent(), {
      wrapper,
    });

    await act(async () => {
      await result.current.downloadAttachment({
        id: "att-1",
        name: "fallback.png",
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.example/v1.0/attachments/att-1/content",
      { method: "GET" },
    );
    expect(createObjectURLSpy).toHaveBeenCalledWith(testBlob);
    expect(clickSpy).toHaveBeenCalled();
    clickSpy.mockRestore();
  });
});

