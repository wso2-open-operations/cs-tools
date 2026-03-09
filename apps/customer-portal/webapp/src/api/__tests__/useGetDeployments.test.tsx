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

const mockDeploymentsResponse = {
  deployments: [
    {
      id: "dep-1",
      name: "Production",
      createdOn: "2026-02-19 15:13:16",
      updatedOn: "2026-02-19 15:13:16",
      description: null,
      url: "https://example.com",
      project: { id: "proj-1", label: "Test Project" },
      type: { id: "3", label: "Staging" },
    },
  ],
};

const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockDeploymentsResponse),
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

describe("useGetDeployments", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockDeploymentsResponse),
      status: 200,
    } as Response);
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return deployments from API", async () => {
    const { result } = renderHook(() => useGetDeployments("project-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.deployments).toBeDefined();
    expect(Array.isArray(result.current.data?.deployments)).toBe(true);
    expect(result.current.data?.deployments).toHaveLength(1);
    expect(result.current.data?.deployments[0]).toMatchObject({
      id: "dep-1",
      name: "Production",
      project: { id: "proj-1", label: "Test Project" },
      type: { id: "3", label: "Staging" },
    });
    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/projects/project-123/deployments"),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("should throw error when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    (window as unknown as { config?: unknown }).config = {};

    const { result } = renderHook(() => useGetDeployments("project-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("should throw error when API response is not ok", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: false,
      statusText: "Internal Server Error",
      status: 500,
    } as Response);

    const { result } = renderHook(() => useGetDeployments("project-123"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe(
      "Error fetching deployments: Internal Server Error",
    );
  });

  it("should not be enabled when projectId is empty", () => {
    const { result } = renderHook(() => useGetDeployments(""), {
      wrapper,
    });

    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
