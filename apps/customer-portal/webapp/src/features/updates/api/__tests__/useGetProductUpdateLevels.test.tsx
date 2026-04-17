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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGetProductUpdateLevels } from "@features/updates/api/useGetProductUpdateLevels";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

const mockProductUpdateLevelsResponse = [
  {
    productName: "wso2das",
    productUpdateLevels: [
      {
        productBaseVersion: "3.2.0",
        channel: "full",
        updateLevels: [1, 2, 3],
      },
    ],
  },
];

const { mockAuthFetch } = vi.hoisted(() => ({
  mockAuthFetch: vi.fn(),
}));

vi.mock("@/utils/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

describe("useGetProductUpdateLevels", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockProductUpdateLevelsResponse),
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
    vi.unstubAllGlobals();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return loading state initially", async () => {
    const { result } = renderHook(() => useGetProductUpdateLevels(), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(true);
  });

  it("should return data from API", async () => {
    const { result } = renderHook(() => useGetProductUpdateLevels(), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data?.length).toBeGreaterThan(0);
    expect(result.current.data?.[0].productName).toBe("wso2das");
    expect(result.current.data?.[0].productUpdateLevels).toBeDefined();
  });

  it("should have correct query options", () => {
    renderHook(() => useGetProductUpdateLevels(), {
      wrapper,
    });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["product-update-levels"],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(
      5 * 60 * 1000,
    );
  });

  it("should handle API error", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);

    const { result } = renderHook(() => useGetProductUpdateLevels(), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toContain(
      "Error fetching product update levels: Internal Server Error",
    );
    expect(mockLogger.error).toHaveBeenCalled();
  });
});
