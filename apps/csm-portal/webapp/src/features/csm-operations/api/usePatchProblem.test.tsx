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

const patchMock = vi.fn();
const invalidateQueriesMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest; stub it (same approach as usePatchChangeRequest.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ patch: patchMock }),
}));

import { usePatchProblem } from "@features/csm-operations/api/usePatchProblem";

/**
 * Query-client wrapper for `renderHook`, with `invalidateQueries` swapped for
 * a spy — these tests assert the mutation invalidates cached detail/list data
 * on settle (success *and* error), so the call itself is the thing under
 * test. Retries are off so an error case settles on the first rejection.
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

describe("usePatchProblem", () => {
  beforeEach(() => {
    patchMock.mockReset();
    invalidateQueriesMock.mockReset();
  });

  it("PATCHes /problems/{id} with the given patch", async () => {
    patchMock.mockResolvedValue({
      message: "Problem updated successfully",
      problem: { id: "prb-1", state: "ASSESS" },
    });
    const { result } = renderHook(() => usePatchProblem(), { wrapper });

    act(() => {
      result.current.mutate({ id: "prb-1", patch: { transition: "assess", assignedToId: "user-1" } });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(patchMock).toHaveBeenCalledWith("/problems/prb-1", {
      transition: "assess",
      assignedToId: "user-1",
    });
  });

  it("invalidates the problem's detail and the list on success", async () => {
    patchMock.mockResolvedValue({
      message: "ok",
      problem: { id: "prb-1", state: "ASSESS" },
    });
    const { result } = renderHook(() => usePatchProblem(), { wrapper });

    act(() => {
      result.current.mutate({ id: "prb-1", patch: { transition: "assess" } });
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["problem-details", "prb-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["problems"] }),
    );
  });

  // A live ServiceNow business rule can revert/reject a transition after a
  // 200 (or the request can otherwise fail) — see
  // CHANGES-problem-update.md §2. Trusting a stale cached "unchanged" state
  // after such a failure would make an already-attempted transition button
  // look available again; invalidating on error too keeps the next read
  // honest regardless of which side of that ambiguity a given failure falls
  // on. Same lesson `usePatchChangeRequest` already applied.
  it("also invalidates the problem's detail and the list on error", async () => {
    const upstreamError = new Error("State transition rejected");
    patchMock.mockRejectedValue(upstreamError);
    const { result } = renderHook(() => usePatchProblem(), { wrapper });

    act(() => {
      result.current.mutate({ id: "prb-1", patch: { transition: "assess" } });
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(upstreamError);
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["problem-details", "prb-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["problems"] }),
    );
  });
});
