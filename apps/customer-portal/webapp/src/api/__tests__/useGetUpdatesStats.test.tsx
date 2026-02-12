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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useGetUpdatesStats } from "@api/useGetUpdatesStats";
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
  const actual = (await importOriginal()) as {
    ApiQueryKeys: Record<string, string>;
  };
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

describe("useGetUpdatesStats", () => {
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

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return loading state initially", async () => {
    const { result } = renderHook(() => useGetUpdatesStats("project-1"), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(true);
  });

  it("should return mock data when isMockEnabled is true", async () => {
    mockIsMockEnabled = true;
    const { result } = renderHook(() => useGetUpdatesStats("project-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.productsTracked).toBe(4);
    expect(result.current.data?.totalUpdatesInstalled).toBe(70);
    expect(result.current.data?.totalUpdatesPending).toBe(69);
    expect(result.current.data?.securityUpdatesPending).toBe(32);
    expect(mockLogger.debug).toHaveBeenCalledWith(
      expect.stringContaining(
        "Fetching updates stats for project ID: project-1, mock: true",
      ),
    );
  });

  it("should have correct query options", () => {
    renderHook(() => useGetUpdatesStats("project-1"), {
      wrapper,
    });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["updates-stats", "project-1", true],
    })[0];

    expect((query?.options as { staleTime?: number }).staleTime).toBe(
      5 * 60 * 1000,
    );
  });

  it("should fetch from API when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;
    const mockResponse = {
      productsTracked: 5,
      totalUpdatesInstalled: 80,
      totalUpdatesInstalledBreakdown: { regular: 60, security: 20 },
      totalUpdatesPending: 55,
      totalUpdatesPendingBreakdown: { regular: 30, security: 25 },
      securityUpdatesPending: 25,
    };

    const originalWindowConfig = (
      window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config;
    (
      window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = {
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

      const { result } = renderHook(() => useGetUpdatesStats("project-1"), {
        wrapper,
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(result.current.data).toEqual(mockResponse);
      expect(mockGetIdToken).toHaveBeenCalled();
      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.stringContaining(
          "Fetching updates stats for project ID: project-1, mock: false",
        ),
      );
    } finally {
      (window as { config?: unknown }).config = originalWindowConfig;
    }
  });

  it("should handle API error when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;

    const originalWindowConfig = (
      window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config;
    (
      window as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = {
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

      const { result } = renderHook(() => useGetUpdatesStats("project-1"), {
        wrapper,
      });

      await waitFor(() => expect(result.current.isError).toBe(true));
      expect(result.current.error?.message).toContain(
        "Error fetching updates stats: Internal Server Error",
      );
      expect(mockLogger.error).toHaveBeenCalled();
    } finally {
      (window as { config?: unknown }).config = originalWindowConfig;
    }
  });

  it("should not fetch if projectId is empty", () => {
    const { result } = renderHook(() => useGetUpdatesStats(""), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
