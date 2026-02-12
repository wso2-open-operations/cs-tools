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
import { useGetProjectDeployments } from "@api/useGetProjectDeployments";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { mockProjectDeployments } from "@models/mockData";

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

describe("useGetProjectDeployments", () => {
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
    const { result } = renderHook(
      () => useGetProjectDeployments("project-1"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(Array.isArray(result.current.data)).toBe(true);
    expect(result.current.data).toHaveLength(mockProjectDeployments.length);
    expect(result.current.data?.[0]).toMatchObject({
      id: expect.any(String),
      name: expect.any(String),
      project: expect.objectContaining({ id: expect.any(String), label: expect.any(String) }),
      type: expect.objectContaining({ id: expect.any(String), label: expect.any(String) }),
    });
    expect(result.current.data?.[0].type.label).toBe("Staging");
  });

  it("should fetch from API when isMockEnabled is false", async () => {
    mockIsMockEnabled = false;
    const mockResponse: typeof mockProjectDeployments = [
      {
        id: "1",
        name: "Prod",
        createdOn: "2026-02-06",
        updatedOn: "2026-02-06",
        description: null,
        url: null,
        project: { id: "p1", label: "My Project" },
        type: { id: "6", label: "Primary Production" },
      },
    ];
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

    const { result } = renderHook(
      () => useGetProjectDeployments("project-123"),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledWith(
      "https://api.test/projects/project-123/deployments",
      expect.objectContaining({ method: "GET" }),
    );
    expect(result.current.data).toEqual(mockResponse);
    expect(result.current.data?.[0].type.label).toBe("Primary Production");

    window.config = originalConfig;
  });

  it("should be disabled when projectId is empty", () => {
    const { result } = renderHook(() => useGetProjectDeployments(""), {
      wrapper,
    });

    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toBeUndefined();
  });
});
