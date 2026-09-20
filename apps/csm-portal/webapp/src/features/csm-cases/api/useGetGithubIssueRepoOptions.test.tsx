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

const getMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useDashboardList.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));

import { useGetGithubIssueRepoOptions } from "@features/csm-cases/api/useGetGithubIssueRepoOptions";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useGetGithubIssueRepoOptions", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("fetches the repo option catalogue from a single call", async () => {
    getMock.mockResolvedValue({
      githubIssueRepoOptions: [
        { value: "asgardeo", displayLabel: "Asgardeo", owner: "wso2-enterprise", repo: "wso2-iam-internal" },
        { value: "choreo", displayLabel: "WSO2 Developer Platform (Choreo)", owner: "wso2-enterprise", repo: "choreo" },
      ],
    });

    const { result } = renderHook(() => useGetGithubIssueRepoOptions(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getMock).toHaveBeenCalledTimes(1);
    expect(getMock).toHaveBeenCalledWith("/metadata");
    expect(result.current.data).toEqual([
      { value: "asgardeo", displayLabel: "Asgardeo", owner: "wso2-enterprise", repo: "wso2-iam-internal" },
      { value: "choreo", displayLabel: "WSO2 Developer Platform (Choreo)", owner: "wso2-enterprise", repo: "choreo" },
    ]);
  });

  it("treats an empty array as zero configured options, not an error", async () => {
    getMock.mockResolvedValue({ githubIssueRepoOptions: [] });

    const { result } = renderHook(() => useGetGithubIssueRepoOptions(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });

  it("surfaces a query error when the call fails", async () => {
    getMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useGetGithubIssueRepoOptions(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe("boom");
  });

  it("surfaces a query error rather than an empty list when the endpoint 404s", async () => {
    // api.get resolves 404 to null; GET /metadata always returns 200 in
    // practice (an empty array when unconfigured), so a null here means the
    // endpoint itself is missing, not "zero options configured" — must not
    // be silently treated as the latter.
    getMock.mockResolvedValue(null);

    const { result } = renderHook(() => useGetGithubIssueRepoOptions(), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
