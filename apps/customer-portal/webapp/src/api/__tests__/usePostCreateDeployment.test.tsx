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
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import { usePostCreateDeployment } from "@api/usePostCreateDeployment";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
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

describe("usePostCreateDeployment", () => {
  let queryClient: QueryClient;
  const originalConfig = window.config;
  const projectId = "proj-123";

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  const requestBody = {
    deploymentTypeKey: 4,
    description: "test description",
    name: "test deployment",
  };

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        mutations: {
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

  it("should create a deployment successfully", async () => {
    const mockResponse = {
      createdBy: "user1",
      createdOn: "2026-02-17",
      id: "dep-123",
    };

    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve(mockResponse),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostCreateDeployment(projectId), {
      wrapper,
    });

    const data = await result.current.mutateAsync(requestBody);

    expect(data).toEqual(mockResponse);
    expect(mockFetch).toHaveBeenCalledWith(
      "https://api.test/projects/proj-123/deployments",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(requestBody),
      }),
    );
  });

  it("should handle error during deployment creation", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Internal Server Error",
      text: () => Promise.resolve("Error message"),
    } as Response);
    vi.stubGlobal("fetch", mockFetch);

    const { result } = renderHook(() => usePostCreateDeployment(projectId), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "Error creating deployment: 500 Internal Server Error - Error message",
    );
  });

  it("should throw when CUSTOMER_PORTAL_BACKEND_BASE_URL is missing", async () => {
    vi.stubGlobal("config", {});

    const { result } = renderHook(() => usePostCreateDeployment(projectId), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured",
    );
  });

  it("should throw when user is not signed in", async () => {
    mockIsSignedIn = false;

    const { result } = renderHook(() => usePostCreateDeployment(projectId), {
      wrapper,
    });

    await expect(result.current.mutateAsync(requestBody)).rejects.toThrow(
      "User must be signed in to create a deployment",
    );
  });
});
