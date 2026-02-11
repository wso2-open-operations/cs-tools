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
import { describe, expect, it, vi, beforeEach } from "vitest";
import useGetProjectCases from "@api/useGetProjectCases";
import React, { type JSX } from "react";

// Mock @asgardeo/react
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    isLoading: false,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));

// Mock MockConfigProvider
vi.mock("@/providers/MockConfigProvider", () => ({
  useMockConfig: () => ({
    isMockEnabled: true,
  }),
}));

// Mock logger
vi.mock("@/utils/logger", () => ({
  default: {
    debug: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock useLogger hook
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    error: vi.fn(),
  }),
}));

describe("useGetProjectCases", () => {
  let queryClient: QueryClient;
  let wrapper: ({ children }: { children: React.ReactNode }) => JSX.Element;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    vi.clearAllMocks();
  });

  it("should have correct query options", () => {
    const requestBody = { sortBy: { field: "createdOn", order: "desc" } };
    renderHook(() => useGetProjectCases("project-1", requestBody as any), {
      wrapper,
    });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["project-cases", "project-1", requestBody, true],
    })[0];

    expect((query?.options as any).staleTime).toBe(5 * 60 * 1000);
    expect((query?.options as any).refetchOnWindowFocus).toBe(false);
  });

  it("should not fetch if projectId is missing", () => {
    const { result } = renderHook(
      () =>
        useGetProjectCases("", {
          sortBy: { field: "createdOn", order: "desc" },
        }),
      {
        wrapper,
      },
    );

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
  });
});
