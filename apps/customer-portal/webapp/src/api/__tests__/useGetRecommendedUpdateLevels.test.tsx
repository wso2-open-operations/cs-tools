// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGetRecommendedUpdateLevels } from "@api/useGetRecommendedUpdateLevels";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const mockResponse = [
  {
    productName: "wso2am-analytics",
    productBaseVersion: "2.6.0",
    channel: "full",
    startingUpdateLevel: 0,
    endingUpdateLevel: 33,
    installedUpdatesCount: 44,
    installedSecurityUpdatesCount: 23,
    timestamp: 1684415113845,
    recommendedUpdateLevel: 33,
    availableUpdatesCount: 0,
    availableSecurityUpdatesCount: 0,
  },
];

const mockFetchFn = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockResponse),
  status: 200,
} as Response);

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

describe("useGetRecommendedUpdateLevels", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mockLogger.debug.mockClear();
    mockLogger.error.mockClear();
    mockFetchFn.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      status: 200,
    } as Response);
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
    vi.clearAllMocks();
  });

  afterEach(() => {
    delete (window as unknown as { config?: unknown }).config;
    queryClient.clear();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return loading state initially", async () => {
    const { result } = renderHook(() => useGetRecommendedUpdateLevels(), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(true);
  });

  it("should return data from API", async () => {
    const { result } = renderHook(() => useGetRecommendedUpdateLevels(), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data).toEqual(mockResponse);
  });

  it("should have correct query options", () => {
    renderHook(() => useGetRecommendedUpdateLevels(), { wrapper });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["recommended-update-levels"],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(
      5 * 60 * 1000,
    );
  });

  it("should handle API error", async () => {
    mockFetchFn.mockResolvedValueOnce({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);

    const { result } = renderHook(() => useGetRecommendedUpdateLevels(), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain(
      "Error fetching recommended update levels: Internal Server Error",
    );
    expect(mockLogger.error).toHaveBeenCalled();
  });
});
