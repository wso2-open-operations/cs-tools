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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const authFetchMock = vi.fn();

// The hooks read runtime config via @config/apiConfig and the real
// auth-backed client at module load; neither exists under vitest.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

import { useAdminSearchUsers, useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { ApiError } from "@utils/ApiError";

function jsonResponse(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

const EMPTY_PAGE = { users: [], total: 0, limit: 20, offset: 0, hasMore: false };
const REQUEST = { pagination: { limit: 20, offset: 0 }, filters: { searchQuery: "jane" } };

function makeWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function newClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

describe("useSearchUsers / useAdminSearchUsers", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
    authFetchMock.mockImplementation(async () => jsonResponse(EMPTY_PAGE));
  });

  it("useSearchUsers (pickers) posts to /users/search", async () => {
    const { result } = renderHook(() => useSearchUsers(REQUEST), {
      wrapper: makeWrapper(newClient()),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(authFetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = authFetchMock.mock.calls[0];
    expect(url).toBe("https://example.test/users/search");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(REQUEST);
  });

  it("useAdminSearchUsers posts the same body to /admin/users/search", async () => {
    const { result } = renderHook(() => useAdminSearchUsers(REQUEST), {
      wrapper: makeWrapper(newClient()),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(authFetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = authFetchMock.mock.calls[0];
    expect(url).toBe("https://example.test/admin/users/search");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual(REQUEST);
  });

  it("never shares cached results between the two routes for an identical request", async () => {
    const queryClient = newClient();
    const wrapper = makeWrapper(queryClient);

    const picker = renderHook(() => useSearchUsers(REQUEST), { wrapper });
    await waitFor(() => expect(picker.result.current.isSuccess).toBe(true));
    const admin = renderHook(() => useAdminSearchUsers(REQUEST), { wrapper });
    await waitFor(() => expect(admin.result.current.isSuccess).toBe(true));

    // Same request, two routes: both must hit the network, not the cache.
    const urls = authFetchMock.mock.calls.map(([url]) => url);
    expect(urls).toEqual([
      "https://example.test/users/search",
      "https://example.test/admin/users/search",
    ]);

    const keys = queryClient
      .getQueryCache()
      .getAll()
      .map((q) => q.queryKey);
    expect(keys).toHaveLength(2);
    expect(keys).toContainEqual(["csm-users-search", REQUEST]);
    expect(keys).toContainEqual(["csm-users-search", "admin", REQUEST]);
    // Both sit under the prefix that user mutations invalidate.
    expect(keys.every((k) => k[0] === "csm-users-search")).toBe(true);
  });

  it("surfaces a 403 from the admin route as an ApiError carrying the status and message", async () => {
    authFetchMock.mockResolvedValue({
      ok: false,
      status: 403,
      statusText: "Forbidden",
      text: async () => JSON.stringify({ message: "Insufficient permissions" }),
    } as unknown as Response);
    const { result } = renderHook(() => useAdminSearchUsers(REQUEST), {
      wrapper: makeWrapper(newClient()),
    });
    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.error).toBeInstanceOf(ApiError);
    expect((result.current.error as ApiError).status).toBe(403);
  });
});
