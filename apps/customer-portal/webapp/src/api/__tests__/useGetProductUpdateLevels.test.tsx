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
import { useGetProductUpdateLevels } from "@api/useGetProductUpdateLevels";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@constants/apiConstants", async (importOriginal) => {
  const actual = (await importOriginal()) as { ApiQueryKeys: Record<string, string> };
  return {
    ...actual,
    API_MOCK_DELAY: 0,
  };
});

const mockGetIdToken = vi.fn().mockResolvedValue("mock-token");
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: mockGetIdToken,
    isSignedIn: true,
    isLoading: false,
  }),
}));

let mockIsMockEnabled = true;
vi.mock("@providers/MockConfigProvider", () => ({
  useMockConfig: () => ({
    isMockEnabled: mockIsMockEnabled,
  }),
}));

describe("useGetProductUpdateLevels", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    mockLogger.debug.mockClear();
    mockLogger.error.mockClear();
    mockIsMockEnabled = true;
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

  it("should return mock data when isMockEnabled is true", async () => {
    mockIsMockEnabled = true;
    const { result } = renderHook(() => useGetProductUpdateLevels(), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data?.length).toBeGreaterThan(0);
    expect(result.current.data?.[0]["product-name"]).toBeDefined();
    expect(result.current.data?.[0]["product-update-levels"]).toBeDefined();
    expect(mockLogger.debug).toHaveBeenCalledWith(
      expect.stringContaining("Fetching product update levels, mock: true"),
    );
  });

  it("should have correct query options", () => {
    renderHook(() => useGetProductUpdateLevels(), {
      wrapper,
    });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["product-update-levels", true],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(
      5 * 60 * 1000,
    );
  });

  it("should fetch from API when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;
    const mockResponse = [
      {
        "product-name": "wso2das",
        "product-update-levels": [
          {
            "product-base-version": "3.2.0",
            channel: "full",
            "update-levels": [1, 2, 3],
          },
        ],
      },
    ];

    const originalWindowConfig = (window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }).config;
    (window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.example.com",
    };

    try {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue({
          ok: true,
          json: () => Promise.resolve(mockResponse),
          status: 200,
        } as Response),
      );

      const { result } = renderHook(() => useGetProductUpdateLevels(), {
        wrapper,
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockResponse);
      expect(mockGetIdToken).toHaveBeenCalled();
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.stringContaining("Fetching product update levels, mock: false"),
      );
    } finally {
      (window as { config?: unknown }).config = originalWindowConfig;
    }
  });

  it("should handle API error when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;

    const originalWindowConfig = (window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }).config;
    (window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.example.com",
    };

    try {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue({
          ok: false,
          statusText: "Internal Server Error",
          status: 500,
        } as Response),
      );

      const { result } = renderHook(() => useGetProductUpdateLevels(), {
        wrapper,
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
      expect(result.current.error?.message).toContain(
        "Error fetching product update levels: Internal Server Error",
      );
      expect(mockLogger.error).toHaveBeenCalled();
    } finally {
      (window as { config?: unknown }).config = originalWindowConfig;
    }
  });
});
