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
import { useGetCallRequests } from "@features/support/api/useGetCallRequests";

const mockCallRequestsResponse = { callRequests: [] };

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
  warn: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

// Mock useAuthApiClient so tests don't need to stub global fetch
const mockAuthFetch = vi.fn();
vi.mock("@api/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
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
    mockAuthFetch.mockReset();
    vi.clearAllMocks();
    vi.stubGlobal("config", {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    });
  });

  afterEach(() => {
    window.config = originalConfig;
    vi.unstubAllGlobals();
  });

  it("should fetch call requests from API successfully without stateKeys", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockCallRequestsResponse),
    } as Response);

    const { result } = renderHook(() => useGetCallRequests(projectId, caseId), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.pages?.[0]).toEqual(mockCallRequestsResponse);
    expect(mockAuthFetch).toHaveBeenCalledWith(
      `https://api.test/cases/${caseId}/call-requests/search`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ pagination: { limit: 10, offset: 0 } }),
      }),
    );
  });

  it("should include filters.stateKeys in body when stateKeys are provided", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockCallRequestsResponse),
    } as Response);

    const stateKeys = [1, 2, 3, 4, 5];
    const { result } = renderHook(
      () => useGetCallRequests(projectId, caseId, stateKeys),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      `https://api.test/cases/${caseId}/call-requests/search`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          pagination: { limit: 10, offset: 0 },
          filters: { stateKeys },
        }),
      }),
    );
  });

  it("should have correct query options", () => {
    renderHook(() => useGetCallRequests(projectId, caseId), { wrapper });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["case-call-requests", projectId, caseId],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(0);
  });

  it("should handle API errors", async () => {
    mockAuthFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      text: () => Promise.resolve("Error message"),
    } as Response);

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

    const { result } = renderHook(() => useGetCallRequests(projectId, caseId), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
    expect(mockAuthFetch).not.toHaveBeenCalled();
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
