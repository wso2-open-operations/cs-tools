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
import { useGetProjectDeploymentDetails } from "@api/useGetProjectDeploymentDetails";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    isLoading: false,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe("useGetProjectDeploymentDetails", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return mock deployment details for a valid project ID", async () => {
    const { result } = renderHook(
      () => useGetProjectDeploymentDetails("project-123"),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const data = result.current.data;
    expect(data).toBeDefined();
    expect(data?.deployments).toBeDefined();
    expect(Array.isArray(data?.deployments)).toBe(true);
    expect(data?.deployments.length).toBeGreaterThanOrEqual(1);
    expect(data?.deployments[0]).toMatchObject({
      id: expect.any(String),
      name: expect.any(String),
      status: expect.stringMatching(/^(Healthy|Warning)$/),
      url: expect.any(String),
      version: expect.any(String),
    });
  });

  it("should not be enabled when projectId is empty", () => {
    const { result } = renderHook(() => useGetProjectDeploymentDetails(""), {
      wrapper: createWrapper(),
    });

    expect(result.current.isPending).toBe(true);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
