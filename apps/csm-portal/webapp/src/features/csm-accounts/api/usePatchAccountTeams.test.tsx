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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import type { Account } from "@features/csm-accounts/types/csmAccounts";

const patchMock = vi.fn();
const invalidateQueriesMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest; stub it (same approach as usePatchProblem.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ patch: patchMock }),
}));

import { usePatchAccountTeams } from "@features/csm-accounts/api/usePatchAccountTeams";

const UPDATED_ACCOUNT: Account = {
  id: "acct-1",
  sfId: "sf-1",
  name: "Acme Corp",
  activationDate: "2026-01-01T00:00:00Z",
  ownerId: "owner-1",
  hasAgent: true,
  hasKbReferences: false,
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-01T00:00:00Z",
  creTeam: { id: "team-cre-2", name: "CRE Beta" },
};

/**
 * Query-client wrapper for `renderHook`, with `invalidateQueries` swapped for
 * a spy — same convention `usePatchProblem.test.tsx` uses so the invalidation
 * call itself (not a real refetch) is what's asserted.
 */
function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(
    () => Promise.resolve(),
  );
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("usePatchAccountTeams", () => {
  beforeEach(() => {
    patchMock.mockReset();
    invalidateQueriesMock.mockReset();
  });

  it("PATCHes /accounts/{id} with the given patch and returns the updated account", async () => {
    patchMock.mockResolvedValue(UPDATED_ACCOUNT);

    const { result } = renderHook(() => usePatchAccountTeams("acct-1"), { wrapper });

    act(() => {
      result.current.mutate({ creTeamId: "team-cre-2" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(UPDATED_ACCOUNT);
    expect(patchMock).toHaveBeenCalledWith("/accounts/acct-1", { creTeamId: "team-cre-2" });
  });

  it("forwards an explicit null verbatim (the hook does not interpret the payload)", async () => {
    patchMock.mockResolvedValue({ ...UPDATED_ACCOUNT, creTeam: null });

    const { result } = renderHook(() => usePatchAccountTeams("acct-1"), { wrapper });

    act(() => {
      result.current.mutate({ creTeamId: null });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledWith("/accounts/acct-1", { creTeamId: null });
  });

  it("invalidates this account's detail query and the accounts list search cache on success", async () => {
    patchMock.mockResolvedValue(UPDATED_ACCOUNT);

    const { result } = renderHook(() => usePatchAccountTeams("acct-1"), { wrapper });

    act(() => {
      result.current.mutate({ sreTeamId: "team-sre-1" });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-account-detail", "acct-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ predicate: expect.any(Function) }),
    );
  });

  it("rejects without calling the backend when accountId is missing", async () => {
    const { result } = renderHook(() => usePatchAccountTeams(undefined), { wrapper });

    act(() => {
      result.current.mutate({ creTeamId: "team-cre-2" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(patchMock).not.toHaveBeenCalled();
  });

  it("surfaces a failed PATCH as a mutation error", async () => {
    const upstreamError = new Error("Team not found");
    patchMock.mockRejectedValue(upstreamError);

    const { result } = renderHook(() => usePatchAccountTeams("acct-1"), { wrapper });

    act(() => {
      result.current.mutate({ creTeamId: "does-not-exist" });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(upstreamError);
  });
});
