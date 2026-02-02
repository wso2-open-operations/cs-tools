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
import { useGetDashboardMockStats } from "@/api/useGetDashboardMockStats";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

// Mock logger
const mockLogger = {
  debug: vi.fn(),
  error: vi.fn(),
};
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => mockLogger,
}));

describe("useGetDashboardMockStats", () => {
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
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return loading state initially", async () => {
    const { result } = renderHook(() => useGetDashboardMockStats("project-1"), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(true);
  });

  it("should return mock dashboard stats after fetching", async () => {
    const { result } = renderHook(() => useGetDashboardMockStats("project-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.totalCases).toBeDefined();
    expect(result.current.data?.totalCases.trend).toBeDefined();

    expect(mockLogger.debug).toHaveBeenCalledWith(
      expect.stringContaining(
        "Fetching dashboard mock stats for project ID: project-1",
      ),
    );
  });

  it("should not fetch if projectId is missing", () => {
    const { result } = renderHook(() => useGetDashboardMockStats(""), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
