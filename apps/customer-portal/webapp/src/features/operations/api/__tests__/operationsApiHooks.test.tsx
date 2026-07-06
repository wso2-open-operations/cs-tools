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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import useGetChangeRequests, {
  useGetChangeRequestsInfinite,
} from "@features/operations/api/useGetChangeRequests";
import useGetChangeRequestDetails from "@features/operations/api/useGetChangeRequestDetails";
import { useGetProjectChangeRequestStats } from "@features/operations/api/useGetProjectChangeRequestStats";
import { usePatchChangeRequest } from "@features/operations/api/usePatchChangeRequest";
import { usePostCase } from "@features/operations/api/usePostCase";
import { useSearchCatalogs } from "@features/operations/api/useSearchCatalogs";
import { useGetCatalogItemVariables } from "@features/operations/api/useGetCatalogItemVariables";
import { SortOrder } from "@/types/common";
import { ChangeRequestSortField } from "@features/operations/types/changeRequests";

const mockAuthFetch = vi.fn();
let mockIsSignedIn = true;
let mockIsAuthLoading = false;

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    isSignedIn: mockIsSignedIn,
    isLoading: mockIsAuthLoading,
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
  }),
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

const baseSearchRequest = {
  sortBy: { field: ChangeRequestSortField.CreatedOn, order: SortOrder.DESC },
};

describe("operations API hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsSignedIn = true;
    mockIsAuthLoading = false;
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };
  });

  it("useGetChangeRequests posts paginated search", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ changeRequests: [], totalRecords: 0, offset: 0, limit: 10 }),
    });

    const { result } = renderHook(
      () => useGetChangeRequests("proj-1", baseSearchRequest, 0, 10),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/projects/proj-1/change-requests/search",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("useGetChangeRequestsInfinite fetches first page", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        changeRequests: [{ id: "cr-1" }],
        totalRecords: 1,
        offset: 0,
        limit: 10,
      }),
    });

    const { result } = renderHook(
      () => useGetChangeRequestsInfinite("proj-1", baseSearchRequest),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/projects/proj-1/change-requests/search"),
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("useGetChangeRequestDetails fetches by id", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "cr-1", number: "CHG001" }),
    });

    const { result } = renderHook(() => useGetChangeRequestDetails("cr-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/change-requests/cr-1",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useGetProjectChangeRequestStats fetches mapped stats", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ stateCount: [], totalRecords: 0 }),
    });

    const { result } = renderHook(() => useGetProjectChangeRequestStats("proj-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/projects/proj-1/stats/change-requests"),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("useSearchCatalogs posts catalog search", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ catalogs: [] }),
    });

    const { result } = renderHook(() => useSearchCatalogs("dp-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/deployments/products/dp-1/catalogs/search",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("useGetCatalogItemVariables fetches variables", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ variables: [] }),
    });

    const { result } = renderHook(
      () => useGetCatalogItemVariables("cat-1", "item-1"),
      { wrapper: createWrapper() },
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/catalogs/cat-1/items/item-1",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("usePatchChangeRequest patches change request", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "cr-1" }),
    });

    const { result } = renderHook(() => usePatchChangeRequest("cr-1"), {
      wrapper: createWrapper(),
    });

    await result.current.mutateAsync({ plannedStartOn: "2026-06-01 10:00:00" });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "https://api.test/change-requests/cr-1",
      expect.objectContaining({ method: "PATCH" }),
    );
  });

  it("usePostCase creates a case", async () => {
    mockAuthFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: "case-1", number: "CS001" }),
    });

    const { result } = renderHook(() => usePostCase(), {
      wrapper: createWrapper(),
    });

    await result.current.mutateAsync({
      projectId: "proj-1",
      deploymentId: "dep-1",
      deployedProductId: "dp-1",
      description: "Need help",
    } as never);

    expect(mockAuthFetch).toHaveBeenCalledWith(
      expect.stringContaining("/cases"),
      expect.objectContaining({ method: "POST" }),
    );
  });
});
