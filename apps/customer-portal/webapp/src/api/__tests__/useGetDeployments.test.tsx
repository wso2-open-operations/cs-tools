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
import { useGetDeployments } from "@api/useGetDeployments";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { mockDeployments } from "@models/mockData";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@/constants/apiConstants", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
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
vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => ({
    isMockEnabled: mockIsMockEnabled,
  }),
}));

describe("useGetDeployments", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    mockIsMockEnabled = true;
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return mock data when isMockEnabled is true", async () => {
    const { result } = renderHook(() => useGetDeployments("project-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.deployments).toBeDefined();
    expect(Array.isArray(result.current.data?.deployments)).toBe(true);
    expect(result.current.data?.deployments).toHaveLength(
      mockDeployments.length,
    );
    expect(result.current.data?.deployments[0]).toMatchObject({
      id: expect.any(String),
      name: expect.any(String),
      status: expect.stringMatching(/^(Healthy|Warning)$/),
      url: expect.any(String),
      version: expect.any(String),
      products: expect.any(Array),
      documents: expect.any(Array),
    });
  });

  it("should fetch from API when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;
    const mockResponse = { deployments: mockDeployments };
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
      status: 200,
    } as Response);

    const originalConfig = window.config;
    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetDeployments("project-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/projects/project-123/deployments"),
      expect.objectContaining({
        method: "GET",
        headers: expect.any(Object),
      }),
    );
    expect(result.current.data).toEqual(mockResponse);

    window.config = originalConfig;
  });

  it("should throw error when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    mockIsMockEnabled = false;
    const originalConfig = window.config;
    window.config = {} as typeof window.config;

    const { result } = renderHook(() => useGetDeployments("project-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );

    window.config = originalConfig;
  });

  it("should throw error when API response is not ok", async () => {
    mockIsMockEnabled = false;
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);

    const originalConfig = window.config;
    window.config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    } as typeof window.config;
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => useGetDeployments("project-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "Error fetching deployments: Internal Server Error",
    );

    window.config = originalConfig;
  });

  it("should not be enabled when projectId is empty", () => {
    const { result } = renderHook(() => useGetDeployments(""), {
      wrapper,
    });

    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
