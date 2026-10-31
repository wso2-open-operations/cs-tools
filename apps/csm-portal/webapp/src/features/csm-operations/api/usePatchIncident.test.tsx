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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const patchMock = vi.fn();
const invalidateQueriesMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as usePatchProblem.test.tsx).
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ patch: patchMock }),
}));

import { refreshIncidentRecords, usePatchIncident } from "@features/csm-operations/api/usePatchIncident";

/** Query-client wrapper with `invalidateQueries` swapped for a spy: what the
 * mutation marks stale is the thing under test. */
function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() => Promise.resolve());
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

/** Every query key invalidated so far. */
function invalidatedKeys(): unknown[][] {
  return invalidateQueriesMock.mock.calls.map((c) => (c[0] as { queryKey: unknown[] }).queryKey);
}

async function patch(state: string): Promise<void> {
  patchMock.mockResolvedValue({ message: "ok", incident: { id: "inc-1", state } });
  const { result } = renderHook(() => usePatchIncident(), { wrapper });
  act(() => {
    result.current.mutate({ id: "inc-1", patch: { state: state as "RESOLVED" } });
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
}

describe("usePatchIncident", () => {
  beforeEach(() => {
    patchMock.mockReset();
    invalidateQueriesMock.mockReset();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("refreshes only this incident's own records for an ordinary update", async () => {
    await patch("ON_HOLD");
    const keys = invalidatedKeys();
    expect(keys).toContainEqual(["incidents"]);
    expect(keys).toContainEqual(["incident-details", "inc-1"]);
    expect(keys).toContainEqual(["incident-activities", "inc-1"]);
    expect(keys).toContainEqual(["incident-comments", "inc-1"]);
    expect(keys).not.toContainEqual(["incident-details"]);
  });

  // Resolving resolves the open child incidents in the same update, whose
  // ids the page does not know: every incident's detail, activity and notes
  // go stale, and the child lists sit under the incidents key.
  it("refreshes every incident's detail, activity and notes when resolving", async () => {
    await patch("RESOLVED");
    const keys = invalidatedKeys();
    expect(keys).toContainEqual(["incidents"]);
    expect(keys).toContainEqual(["incident-details"]);
    expect(keys).toContainEqual(["incident-activities"]);
    expect(keys).toContainEqual(["incident-comments"]);
  });

  it("does it again, child lists included, once the resolve flows have settled", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await patch("RESOLVED");
    invalidateQueriesMock.mockClear();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(7_000);
    });
    const keys = invalidatedKeys();
    expect(keys).toContainEqual(["incident-details"]);
    expect(keys).toContainEqual(["incident-activities"]);
    expect(keys).toContainEqual(["incident-comments"]);
    expect(keys).toContainEqual(["incidents", "child-incidents"]);
  });

  it("does not refresh other incidents when closing, which does not cascade", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await patch("CLOSED");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(7_000);
    });
    const keys = invalidatedKeys();
    expect(keys).toContainEqual(["incident-details", "inc-1"]);
    expect(keys).not.toContainEqual(["incident-details"]);
    expect(keys).not.toContainEqual(["incidents", "child-incidents"]);
  });
});

describe("refreshIncidentRecords", () => {
  beforeEach(() => {
    invalidateQueriesMock.mockReset();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  const count = (key: unknown[]): number =>
    invalidatedKeys().filter((k) => JSON.stringify(k) === JSON.stringify(key)).length;

  // Each prefix-wide refresh refetches every incident page in the cache: a
  // bulk resolve does it once for the run, not once per incident.
  it("refreshes every incident once for a bulk resolve, and once more after the flows", async () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() => Promise.resolve());
    const resolve = { state: "RESOLVED" as const, resolutionCode: "SOLVED_PERMANENTLY" as const, resolutionNotes: "Fixed." };

    refreshIncidentRecords(queryClient, [
      { id: "inc-1", patch: resolve },
      { id: "inc-2", patch: resolve },
      { id: "inc-3", patch: resolve },
    ]);
    for (const prefix of [["incident-details"], ["incident-activities"], ["incident-comments"]]) {
      expect(count(prefix)).toBe(1);
    }
    for (const id of ["inc-1", "inc-2", "inc-3"]) expect(count(["csm-incident-tasks", id])).toBe(1);
    expect(count(["incidents", "child-incidents"])).toBe(0);

    invalidateQueriesMock.mockClear();
    await vi.advanceTimersByTimeAsync(7_000);
    for (const prefix of [["incident-details"], ["incident-activities"], ["incident-comments"]]) {
      expect(count(prefix)).toBe(1);
    }
    expect(count(["incidents", "child-incidents"])).toBe(1);
    for (const id of ["inc-1", "inc-2", "inc-3"]) expect(count(["csm-incident-tasks", id])).toBe(1);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("refreshes each incident's own records for a bulk update that resolves nothing", () => {
    vi.useFakeTimers();
    const queryClient = new QueryClient();
    queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() => Promise.resolve());

    refreshIncidentRecords(queryClient, [
      { id: "inc-1", patch: { state: "IN_PROGRESS" } },
      { id: "inc-2", patch: { state: "IN_PROGRESS" } },
    ]);
    for (const id of ["inc-1", "inc-2"]) {
      expect(count(["incident-details", id])).toBe(1);
      expect(count(["incident-activities", id])).toBe(1);
      expect(count(["incident-comments", id])).toBe(1);
    }
    expect(count(["incident-details"])).toBe(0);
    // One timer for the run's flows, not one per incident.
    expect(vi.getTimerCount()).toBe(1);
  });
});
