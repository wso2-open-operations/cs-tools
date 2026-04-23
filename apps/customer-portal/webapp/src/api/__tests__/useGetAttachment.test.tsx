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

import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { useGetAttachment } from "@api/useGetAttachment";

const mockAuthFetch = vi.fn();

vi.mock("@api/useAuthApiClient", () => ({
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

describe("useGetAttachment", () => {
  let queryClient: QueryClient;
  let prevConfig: Window["config"];
  let openSpy: ReturnType<typeof vi.spyOn>;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    prevConfig = window.config;
    window.config = testWindowConfig;
    openSpy = vi.spyOn(window, "open").mockImplementation(() => null);
    queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.config = prevConfig;
    openSpy.mockRestore();
  });

  it("should not call authFetch when inline content is present", async () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click");
    const { result } = renderHook(() => useGetAttachment(), { wrapper });

    await act(async () => {
      await result.current.downloadAttachment({
        id: "x",
        name: "a.txt",
        type: "text/plain",
        content: "data:text/plain;base64,QUJD",
      });
    });

    expect(mockAuthFetch).not.toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalled();
    clickSpy.mockRestore();
  });

  it("should GET /attachments/:id and trigger download from JSON content", async () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click");
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          content: "data:image/png;base64,QUJD",
          id: "att-1",
          name: "n.png",
          type: "image/png",
        }),
    });

    const { result } = renderHook(() => useGetAttachment(), { wrapper });

    await act(async () => {
      await result.current.downloadAttachment({
        id: "att-1",
        name: "fallback.png",
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.example/v1.0/attachments/att-1",
      { method: "GET" },
    );
    expect(clickSpy).toHaveBeenCalled();
    clickSpy.mockRestore();
  });

  it("should open downloadUrl when GET fails and URL is set", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Error",
      text: () => Promise.resolve(""),
    });

    const { result } = renderHook(() => useGetAttachment(), { wrapper });

    await act(async () => {
      await result.current.downloadAttachment({
        id: "att-1",
        name: "x",
        downloadUrl: "https://sn.example/file",
      });
    });

    expect(openSpy).toHaveBeenCalledWith(
      "https://sn.example/file",
      "_blank",
      "noopener,noreferrer",
    );
  });

  it("should reject when GET fails and no downloadUrl", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: false,
      status: 404,
      statusText: "Not Found",
      text: () => Promise.resolve(""),
    });

    const { result } = renderHook(() => useGetAttachment(), { wrapper });

    await act(async () => {
      await expect(
        result.current.downloadAttachment({ id: "att-1", name: "x" }),
      ).rejects.toThrow(/Download failed/);
    });
  });

  it("should expose downloadingId while request is pending", async () => {
    let resolveFetch: (value: Response) => void = () => {};
    const pending = new Promise<Response>((r) => {
      resolveFetch = r;
    });
    mockAuthFetch.mockReturnValueOnce(pending);

    const { result } = renderHook(() => useGetAttachment(), { wrapper });

    let downloadPromise: Promise<void> = Promise.resolve();
    await act(async () => {
      downloadPromise = result.current.downloadAttachment({
        id: "pending-id",
        name: "f.bin",
      });
    });

    await waitFor(() => {
      expect(result.current.isDownloading).toBe(true);
      expect(result.current.downloadingId).toBe("pending-id");
    });

    await act(async () => {
      resolveFetch({
        ok: true,
        json: () =>
          Promise.resolve({
            content: "data:application/octet-stream;base64,QQ==",
            id: "pending-id",
            name: "f.bin",
            type: "application/octet-stream",
          }),
      } as Response);
      await downloadPromise;
    });

    await waitFor(() => {
      expect(result.current.isDownloading).toBe(false);
      expect(result.current.downloadingId).toBeNull();
    });
  });
});
