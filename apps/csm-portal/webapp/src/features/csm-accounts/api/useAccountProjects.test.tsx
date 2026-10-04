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
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import type { Project, SearchProjectsResponse } from "@features/csm-projects/types/csmProjects";

// useAccountProjects.ts reads window.config at module load (via
// @config/apiConfig) and calls the real identity-provider-backed useAuthApiClient;
// neither is present under vitest, so mock both. authFetchMock stands in for
// the fetch wrapper and is what the hook actually calls.
const authFetchMock = vi.fn();
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

import { useAccountProjects } from "@features/csm-accounts/api/useAccountProjects";

function project(id: string, accountId: string): Project {
  return {
    id,
    name: `Project ${id}`,
    key: id.toUpperCase(),
    account: { id: accountId, name: `Account ${accountId}` },
    subscriptionType: undefined,
    endDate: undefined,
  } as unknown as Project;
}

function jsonResponse(body: SearchProjectsResponse): Response {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useAccountProjects", () => {
  beforeEach(() => {
    authFetchMock.mockReset();
  });

  it("sends the accountId filter and returns the server-filtered projects", async () => {
    authFetchMock.mockResolvedValueOnce(
      jsonResponse({
        projects: [project("p1", "acc-1")],
        total: 1,
        limit: 100,
        offset: 0,
        hasMore: false,
      }),
    );

    const { result } = renderHook(() => useAccountProjects("acc-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.projects.map((p) => p.id)).toEqual(["p1"]);
    const [, requestInit] = authFetchMock.mock.calls[0];
    expect(JSON.parse(requestInit.body as string)).toMatchObject({
      accountId: "acc-1",
    });
  });

  it("returns an empty list when the account has no projects", async () => {
    authFetchMock.mockResolvedValueOnce(
      jsonResponse({ projects: [], total: 0, limit: 100, offset: 0, hasMore: false }),
    );

    const { result } = renderHook(() => useAccountProjects("acc-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.projects).toEqual([]);
  });

  it("does not fetch when accountId is undefined", () => {
    renderHook(() => useAccountProjects(undefined), { wrapper });
    expect(authFetchMock).not.toHaveBeenCalled();
  });
});
