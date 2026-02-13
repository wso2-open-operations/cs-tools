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

import { renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { usePostAttachments } from "@api/usePostAttachments";
import { ApiQueryKeys } from "@constants/apiConstants";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

const mockGetIdToken = vi.fn().mockResolvedValue("mock-token");
let mockIsSignedIn = true;
let mockIsAuthLoading = false;
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: mockGetIdToken,
    isSignedIn: mockIsSignedIn,
    isLoading: mockIsAuthLoading,
  }),
}));

let mockIsMockEnabled = true;
vi.mock("@providers/MockConfigProvider", () => ({
  useMockConfig: () => ({
    isMockEnabled: mockIsMockEnabled,
  }),
}));

describe("usePostAttachments", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  const variables = {
    caseId: "case-123",
    body: {
      referenceType: "case" as const,
      name: "test.png",
      type: "image/png",
      content: "base64content",
    },
  };

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        mutations: {
          retry: false,
        },
      },
    });
    mockIsMockEnabled = true;
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("throws when isMockEnabled is true", async () => {
    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await expect(result.current.mutateAsync(variables)).rejects.toThrow(
      "Upload attachment is not available when mock is enabled. Disable mock to upload.",
    );
  });

  it("posts to API with correct URL, headers and body when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
    } as Response);

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await result.current.mutateAsync(variables);

    expect(mockFetch).toHaveBeenCalledWith(
      "https://api.test/cases/case-123/attachments",
      expect.objectContaining({
        method: "POST",
        headers: expect.any(Object),
        body: JSON.stringify(variables.body),
      }),
    );
  });

  it("throws when CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured", async () => {
    mockIsMockEnabled = false;
    window.config = {} as typeof window.config;

    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await expect(result.current.mutateAsync(variables)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("throws when API response is not ok", async () => {
    mockIsMockEnabled = false;
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      statusText: "Bad Request",
      status: 400,
      text: () => Promise.resolve("Invalid payload"),
    } as Response);

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await expect(result.current.mutateAsync(variables)).rejects.toThrow(
      /Error uploading attachment: 400/,
    );
  });

  it("throws when user is not signed in", async () => {
    mockIsMockEnabled = false;
    mockIsSignedIn = false;

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;

    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await expect(result.current.mutateAsync(variables)).rejects.toThrow(
      "User must be signed in to upload an attachment",
    );
  });

  it("invalidates case-attachments query on success", async () => {
    mockIsMockEnabled = false;
    const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
    } as Response);

    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostAttachments(), { wrapper });

    await result.current.mutateAsync(variables);

    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: [ApiQueryKeys.CASE_ATTACHMENTS, "case-123"],
    });
  });
});
