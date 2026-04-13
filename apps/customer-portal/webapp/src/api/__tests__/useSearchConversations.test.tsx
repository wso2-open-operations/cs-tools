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
import { useSearchConversations } from "@api/useSearchConversations";
import { SortOrder } from "@/types/common";
import React, { type ReactNode } from "react";

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: true,
    isLoading: false,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: vi.fn(),
    error: vi.fn(),
  }),
}));

describe("useSearchConversations", () => {
  let queryClient: QueryClient;
  let wrapper: ({ children }: { children: ReactNode }) => React.ReactElement;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    wrapper = ({ children }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };
    vi.clearAllMocks();
  });

  it("should have correct query key for conversations search", () => {
    const request = {
      pagination: { limit: 10, offset: 0 },
      sortBy: { field: "updatedOn", order: SortOrder.DESC },
    };
    renderHook(() => useSearchConversations("project-1", request), {
      wrapper,
    });

    const query = queryClient.getQueryCache().findAll({
      queryKey: ["conversations-search", "project-1", request],
    })[0];

    expect(query).toBeDefined();
    expect((query?.options as { staleTime?: number }).staleTime).toBe(
      5 * 60 * 1000,
    );
  });

  it("should not fetch if projectId is missing", () => {
    const { result } = renderHook(
      () =>
        useSearchConversations("", {
          pagination: { limit: 5, offset: 0 },
          sortBy: { field: "updatedOn", order: SortOrder.DESC },
        }),
      { wrapper },
    );

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
  });
});
