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
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { useGetCallRequests } from "@api/useGetCallRequests";

const mockCallRequestsResponse = { callRequests: [] };

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
  warn: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

let mockIsSignedIn = true;
let mockIsAuthLoading = false;
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: mockIsSignedIn,
    isLoading: mockIsAuthLoading,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));

describe("useGetCallRequests", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;
  const projectId = "proj-123";
  const caseId = "case-456";

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    vi.clearAllMocks();
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("should fetch call requests from API successfully", async () => {
    const mockResponse = mockCallRequestsResponse;
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockResponse),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetCallRequests(projectId, caseId), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.pages?.[0]).toEqual(mockResponse);
    expect(mockFetch).toHaveBeenCalledWith(
      `https://api.test/cases/${caseId}/call-requests/search`,
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ pagination: { limit: 10, offset: 0 } }),
      }),
    );
  });

  it("should have correct query options", () => {
    renderHook(() => useGetCallRequests(projectId, caseId), { wrapper });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["case-call-requests", projectId, caseId],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(
      5 * 60 * 1000,
    );
  });

  it("should handle API errors", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      text: () => Promise.resolve("Error message"),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetCallRequests(projectId, caseId), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain(
      "Error fetching call requests: 500 Internal Server Error - Error message",
    );
  });

  it("should error when CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured", async () => {
    vi.stubGlobal("config", {});
    const mockFetch = vi.fn();
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetCallRequests(projectId, caseId), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("should not fetch when user is not signed in", async () => {
    mockIsSignedIn = false;

    const { result } = renderHook(() => useGetCallRequests(projectId, caseId), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
  });
});
