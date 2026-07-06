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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { InfiniteData } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import useGetMetadata from "@api/useGetMetadata";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useInfiniteProjects, {
  flattenProjectPages,
  getTotalRecords,
} from "@api/useGetProjects";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";
import type { SearchProjectsResponse } from "@features/project-hub/types/projects";
import { SortOrder } from "@/types/common";

const authFetchMock = vi.fn();

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn() }),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("shared API hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };
  });

  it("useGetMetadata fetches metadata", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ timeZones: [{ id: "UTC", label: "UTC" }] }),
    });

    const { result } = renderHook(() => useGetMetadata(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.timeZones).toHaveLength(1);
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/metadata",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useGetProjectFeatures fetches project features", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ permissions: [] }),
    });

    const { result } = renderHook(() => useGetProjectFeatures("proj-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/proj-1/features",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useGetProjectCasesPage posts case search with pagination", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ cases: [], totalRecords: 0 }),
    });

    const { result } = renderHook(
      () =>
        useGetProjectCasesPage(
          "proj-1",
          { sortBy: { field: "createdOn", order: SortOrder.DESC } },
          0,
          10,
        ),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/projects/proj-1/cases/search"),
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("useInfiniteProjects searches projects and paginates", async () => {
    authFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        projects: [{ id: "1", name: "A" }],
        totalRecords: 2,
      }),
    });

    const { result } = renderHook(() => useInfiniteProjects({ pageSize: 1 }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      "https://api.test/projects/search",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("flattenProjectPages and getTotalRecords aggregate infinite data", () => {
    const data: InfiniteData<SearchProjectsResponse> = {
      pageParams: [0],
      pages: [
        { projects: [{ id: "1", name: "A" } as never], totalRecords: 2, offset: 0, limit: 10 },
        { projects: [{ id: "2", name: "B" } as never], totalRecords: 2, offset: 10, limit: 10 },
      ],
    };

    expect(flattenProjectPages(data)).toHaveLength(2);
    expect(getTotalRecords(data)).toBe(2);
    expect(flattenProjectPages(undefined)).toEqual([]);
    expect(getTotalRecords(undefined)).toBe(0);
  });

  it("usePostProjectDeploymentsSearchAll fetches all deployment pages", async () => {
    authFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ deployments: [], totalRecords: 0, offset: 0, limit: 10 }),
    });

    const { result } = renderHook(
      () => usePostProjectDeploymentsSearchAll("proj-1"),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/projects/proj-1/deployments/search"),
      expect.objectContaining({ method: "POST" }),
    );
  });
});
