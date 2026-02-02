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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import useGetProjects from "@/api/useGetProjects";
import { mockProjects } from "@/models/mockData";
import type { ReactNode } from "react";

vi.mock("@/constants/apiConstants", async (importOriginal) => {
  const actual = (await importOriginal()) as any;
  return {
    ...actual,
    API_MOCK_DELAY: 0,
  };
});

// Mock useLogger
vi.mock("@/hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe("useGetProjects", () => {
  it("should return paginated mock projects in pages on query execution", async () => {
    const { result } = renderHook(
      () => useGetProjects({ pagination: { offset: 0, limit: 2 } }),
      {
        wrapper: createWrapper(),
      },
    );

    // Wait for the initial page to finish
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const response = result.current.data;
    expect(response).toBeDefined();
    expect(response?.pages).toHaveLength(1);
    expect(response?.pages[0].projects).toHaveLength(2);
    expect(response?.pages[0].totalRecords).toBe(mockProjects.length);
    expect(response?.pages[0].projects[0].name).toBe(mockProjects[0].name);
    expect(response?.pages[0].offset).toBe(0);
    expect(response?.pages[0].limit).toBe(2);
  });

  it("should handle default pagination if none provided", async () => {
    const { result } = renderHook(() => useGetProjects(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const response = result.current.data;
    expect(response?.pages[0].offset).toBe(0);
    expect(response?.pages[0].limit).toBe(10);
  });

  it("should use 'all' query key and larger limit when fetchAll is true", async () => {
    const { result } = renderHook(() => useGetProjects(undefined, true), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const response = result.current.data;
    // fetchAll sets limit to 100
    expect(response?.pages[0].limit).toBe(100);
  });
});
