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
import { describe, expect, it, vi } from "vitest";
import { useGetProjectSupportStats } from "@/api/useGetProjectSupportStats";
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

describe("useGetProjectSupportStats", () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );

  it("should return loading state initially", async () => {
    const { result } = renderHook(
      () => useGetProjectSupportStats("project-1"),
      {
        wrapper,
      },
    );

    expect(result.current.isLoading).toBe(true);
  });

  it("should return data after fetching", async () => {
    const { result } = renderHook(
      () => useGetProjectSupportStats("project-1"),
      {
        wrapper,
      },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.totalCases).toBeGreaterThanOrEqual(0);
    expect(mockLogger.debug).toHaveBeenCalledWith(
      expect.stringContaining(
        "Fetching support stats for project ID: project-1",
      ),
    );
  });

  it("should not fetch if id is missing", () => {
    const { result } = renderHook(() => useGetProjectSupportStats(""), {
      wrapper,
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.fetchStatus).toBe("idle");
  });
});
