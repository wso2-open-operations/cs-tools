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
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiQueryKeys } from "@constants/apiConstants";
import { IdleStateContext } from "@providers/IdleStateContext";
import { useCaseActivityStream } from "@features/support/api/useCaseActivityStream";

type Listener = (event?: { data?: string }) => void;

const { MockEventSource } = vi.hoisted(() => {
  class MockEventSource {
    static instances: MockEventSource[] = [];
    listeners = new Map<string, Listener[]>();
    close = vi.fn();

    constructor(
      public readonly url: string,
      public readonly init: { headers: Record<string, string> },
    ) {
      MockEventSource.instances.push(this);
    }

    addEventListener(type: string, fn: Listener): void {
      this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
    }

    emit(type: string, data?: string): void {
      (this.listeners.get(type) ?? []).forEach((fn) => fn({ data }));
    }
  }
  return { MockEventSource };
});

vi.mock("@sanity/eventsource", () => ({ default: MockEventSource }));

const mockGetIdToken = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ getIdToken: mockGetIdToken }),
}));

const mockAuthFetch = vi.fn();
vi.mock("@/hooks/useAuthApiClient", () => ({ useAuthApiClient: () => mockAuthFetch }));

const mockRefreshCaseComments = vi.hoisted(() => vi.fn());
vi.mock("@features/support/api/caseCommentsRefresh", () => ({
  refreshCaseComments: mockRefreshCaseComments,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn(), info: vi.fn() }),
}));

const mockApiConfig = vi.hoisted(() => ({
  streamEnabled: true as boolean,
  streamUrl: "https://stream.example.com" as string | undefined,
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: mockApiConfig }));

const idleRef = { current: false };

function setup(caseId: string | undefined, idle = false) {
  const queryClient = new QueryClient();
  const spy = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <IdleStateContext.Provider value={idleRef.current}>{children}</IdleStateContext.Provider>
    </QueryClientProvider>
  );
  idleRef.current = idle;
  const hook = renderHook(() => useCaseActivityStream(caseId), { wrapper });
  return { spy, ...hook };
}

type Filters = { queryKey?: unknown[]; predicate?: (q: { queryKey: unknown[] }) => boolean };
const callFilters = (spy: ReturnType<typeof setup>["spy"]) =>
  spy.mock.calls.map(([filters]) => (filters ?? {}) as Filters);

/** Whether any recorded invalidation would match a query with this key. */
const invalidates = (spy: ReturnType<typeof setup>["spy"], queryKey: unknown[]): boolean =>
  callFilters(spy).some((f) => {
    if (f.queryKey && !f.queryKey.every((part, i) => queryKey[i] === part)) return false;
    return f.predicate ? f.predicate({ queryKey }) : true;
  });

const caseEvent = (type: string) => JSON.stringify({ caseId: "case-1", type });

describe("useCaseActivityStream", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRefreshCaseComments.mockResolvedValue(undefined);
    MockEventSource.instances = [];
    mockGetIdToken.mockResolvedValue("id-token");
    mockApiConfig.streamEnabled = true;
    mockApiConfig.streamUrl = "https://stream.example.com";
  });

  it("connects with auth headers to the case stream URL", async () => {
    setup("case 1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
    const [source] = MockEventSource.instances;
    expect(source.url).toBe("https://stream.example.com/cases/case%201/activities/stream");
    expect(source.init.headers).toEqual({
      Authorization: "Bearer id-token",
      "x-user-id-token": "id-token",
    });
  });

  it("refreshes only the newest comments on a comment_added event", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated", caseEvent("case.comment_added"));

    expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
    expect(mockRefreshCaseComments.mock.calls[0][2]).toBe("case-1");
    expect(spy).not.toHaveBeenCalled();
  });

  it.each([
    ["status_changed", caseEvent("case.status_changed")],
    ["an unknown type", caseEvent("case.something_new")],
    ["unparseable data", "not json{"],
    ["no data", undefined],
    ["a non-object payload", "42"],
  ])("refreshes details, case lists, stats and comments on %s", async (_label, data) => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated", data);

    expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
    expect(invalidates(spy, [ApiQueryKeys.CASE_DETAILS, "project-1", "case-1"])).toBe(true);
    expect(invalidates(spy, [ApiQueryKeys.PROJECT_CASES, "project-1", {}, 10])).toBe(true);
    expect(invalidates(spy, [ApiQueryKeys.PROJECT_CASES, "page", "project-1", {}, 0, 10])).toBe(true);
    expect(invalidates(spy, [ApiQueryKeys.CASES_STATS, "project-1"])).toBe(true);
    expect(invalidates(spy, [ApiQueryKeys.SUPPORT_STATS, "project-1"])).toBe(true);
    expect(invalidates(spy, [ApiQueryKeys.PROJECT_STATS, "project-1"])).toBe(true);
  });

  it("never invalidates the project filters query", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated", caseEvent("case.status_changed"));
    MockEventSource.instances[0].emit("case_updated", "garbage");

    expect(invalidates(spy, [ApiQueryKeys.PROJECT_CASES, "filters", "project-1"])).toBe(false);
  });

  it("only refetches details of the streamed case and leaves comments to the incremental refresh", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated", caseEvent("case.status_changed"));

    expect(invalidates(spy, [ApiQueryKeys.CASE_DETAILS, "project-1", "other-case"])).toBe(false);
    expect(invalidates(spy, [ApiQueryKeys.CASE_COMMENTS, "project-1", "case-1", "infinite"])).toBe(false);
  });

  it("does not invalidate unrelated keys", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated", caseEvent("case.status_changed"));

    expect(invalidates(spy, [ApiQueryKeys.PROJECTS])).toBe(false);
    expect(invalidates(spy, [ApiQueryKeys.DEPLOYMENTS])).toBe(false);
  });

  describe("connection catch-up and backoff", () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });
    afterEach(() => {
      vi.useRealTimers();
      vi.restoreAllMocks();
    });

    it("first open refreshes comments only, not case details", async () => {
      const { spy } = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

      MockEventSource.instances[0].emit("open");

      expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
      expect(spy).not.toHaveBeenCalled();
    });

    it("a reconnect open also refetches case details, but not lists, stats or filters", async () => {
      vi.spyOn(Math, "random").mockReturnValue(0.5);
      const { spy } = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("open");
      MockEventSource.instances[0].emit("error");
      mockRefreshCaseComments.mockClear();

      await vi.advanceTimersByTimeAsync(61_000);
      expect(MockEventSource.instances).toHaveLength(2);
      MockEventSource.instances[1].emit("open");

      expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
      expect(invalidates(spy, [ApiQueryKeys.CASE_DETAILS, "project-1", "case-1"])).toBe(true);
      expect(invalidates(spy, [ApiQueryKeys.PROJECT_CASES, "project-1"])).toBe(false);
      expect(invalidates(spy, [ApiQueryKeys.CASES_STATS])).toBe(false);
    });

    it("catches up at most once per 60 seconds per case", async () => {
      vi.spyOn(Math, "random").mockReturnValue(0.01);
      const { spy } = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("open");
      MockEventSource.instances[0].emit("error");

      await vi.advanceTimersByTimeAsync(5_000);
      expect(MockEventSource.instances).toHaveLength(2);
      MockEventSource.instances[1].emit("open");

      expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
      expect(spy).not.toHaveBeenCalled();

      MockEventSource.instances[1].emit("error");
      await vi.advanceTimersByTimeAsync(61_000);
      MockEventSource.instances[2].emit("open");
      expect(mockRefreshCaseComments).toHaveBeenCalledTimes(2);
      expect(invalidates(spy, [ApiQueryKeys.CASE_DETAILS, "project-1", "case-1"])).toBe(true);
    });

    it("keeps backing off when the connection opens and is cut straight away", async () => {
      vi.spyOn(Math, "random").mockReturnValue(0.99);
      setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

      MockEventSource.instances[0].emit("open");
      MockEventSource.instances[0].emit("error");
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(2);

      MockEventSource.instances[1].emit("open");
      MockEventSource.instances[1].emit("error");
      // The second delay is drawn from [0, 6s), not [0, 3s) again.
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(2);
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(3);
    });

    it("resets the backoff once a connection has stayed open for 30 seconds", async () => {
      vi.spyOn(Math, "random").mockReturnValue(0.99);
      setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("open");
      MockEventSource.instances[0].emit("error");
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(2);

      MockEventSource.instances[1].emit("open");
      await vi.advanceTimersByTimeAsync(30_000);
      MockEventSource.instances[1].emit("error");
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(3);
    });

    it("resets the backoff after a received event", async () => {
      vi.spyOn(Math, "random").mockReturnValue(0.99);
      setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("error");
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(2);

      MockEventSource.instances[1].emit("open");
      MockEventSource.instances[1].emit("case_updated", caseEvent("case.comment_added"));
      MockEventSource.instances[1].emit("error");
      await vi.advanceTimersByTimeAsync(3_000);
      expect(MockEventSource.instances).toHaveLength(3);
    });

    it("treats the first open after idle as a reconnect", async () => {
      const { rerender, spy } = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("open");

      idleRef.current = true;
      rerender();
      await vi.advanceTimersByTimeAsync(61_000);
      idleRef.current = false;
      rerender();
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(2));
      MockEventSource.instances[1].emit("open");

      expect(invalidates(spy, [ApiQueryKeys.CASE_DETAILS, "project-1", "case-1"])).toBe(true);
    });
  });

  describe("swapping the connection before the gateway cap", () => {
    const MAX_AGE = 4 * 60_000;

    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });
    afterEach(() => {
      vi.useRealTimers();
      vi.restoreAllMocks();
    });

    const openFirst = async () => {
      const hook = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("open");
      mockRefreshCaseComments.mockClear();
      return hook;
    };

    it("opens a new connection before the cap and keeps the old one until it is open", async () => {
      await openFirst();

      await vi.advanceTimersByTimeAsync(MAX_AGE);
      expect(MockEventSource.instances).toHaveLength(2);
      expect(MockEventSource.instances[0].close).not.toHaveBeenCalled();

      MockEventSource.instances[1].emit("open");
      expect(MockEventSource.instances[0].close).toHaveBeenCalledTimes(1);
      expect(MockEventSource.instances[1].close).not.toHaveBeenCalled();
    });

    it("asks for a fresh token for the new connection", async () => {
      await openFirst();
      mockGetIdToken.mockClear();
      mockGetIdToken.mockResolvedValue("refreshed-token");

      await vi.advanceTimersByTimeAsync(MAX_AGE);

      expect(mockGetIdToken).toHaveBeenCalledTimes(1);
      expect(MockEventSource.instances[1].init.headers.Authorization).toBe("Bearer refreshed-token");
    });

    it("needs no catch-up after a swap", async () => {
      const { spy } = await openFirst();

      await vi.advanceTimersByTimeAsync(MAX_AGE);
      MockEventSource.instances[1].emit("open");

      expect(mockRefreshCaseComments).not.toHaveBeenCalled();
      expect(spy).not.toHaveBeenCalled();
    });

    it("keeps swapping every MAX_AGE", async () => {
      await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);
      MockEventSource.instances[1].emit("open");

      await vi.advanceTimersByTimeAsync(MAX_AGE);
      expect(MockEventSource.instances).toHaveLength(3);
      MockEventSource.instances[2].emit("open");
      expect(MockEventSource.instances[1].close).toHaveBeenCalledTimes(1);
    });

    it("still handles updates from the new connection", async () => {
      await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);
      MockEventSource.instances[1].emit("open");

      MockEventSource.instances[1].emit("case_updated", caseEvent("case.comment_added"));

      expect(mockRefreshCaseComments).toHaveBeenCalledTimes(1);
    });

    it("keeps the old connection when the replacement fails, and retries", async () => {
      await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);
      MockEventSource.instances[1].emit("error");

      expect(MockEventSource.instances[0].close).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(15_000);
      expect(MockEventSource.instances).toHaveLength(3);
    });

    it("stops retrying a replacement that keeps failing", async () => {
      await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);
      for (let i = 1; i <= 4; i += 1) {
        MockEventSource.instances[i].emit("error");
        await vi.advanceTimersByTimeAsync(15_000);
      }

      // The first replacement plus three retries; the old connection is untouched.
      expect(MockEventSource.instances).toHaveLength(5);
      expect(MockEventSource.instances[0].close).not.toHaveBeenCalled();
    });

    it("falls back to a normal reconnect when the old connection is cut mid-swap", async () => {
      vi.spyOn(Math, "random").mockReturnValue(0.5);
      await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);

      MockEventSource.instances[0].emit("error");
      expect(MockEventSource.instances[1].close).toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(10_000);
      expect(MockEventSource.instances).toHaveLength(3);
    });

    it("ignores an error from a connection that has already been swapped out", async () => {
      await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);
      MockEventSource.instances[1].emit("open");

      MockEventSource.instances[0].emit("error");
      await vi.advanceTimersByTimeAsync(10_000);

      expect(MockEventSource.instances).toHaveLength(2);
    });

    it("closes both connections on unmount mid-swap", async () => {
      const { unmount } = await openFirst();
      await vi.advanceTimersByTimeAsync(MAX_AGE);

      unmount();

      expect(MockEventSource.instances[0].close).toHaveBeenCalled();
      expect(MockEventSource.instances[1].close).toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(10 * 60_000);
      expect(MockEventSource.instances).toHaveLength(2);
    });
  });

  it("does not connect when streaming is disabled or the URL is unset", async () => {
    mockApiConfig.streamEnabled = false;
    setup("case-1");
    mockApiConfig.streamEnabled = true;
    mockApiConfig.streamUrl = undefined;
    setup("case-1");
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(0);
  });

  it("does not connect without a case id", async () => {
    setup(undefined);
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(0);
  });

  it("closes the stream on unmount", async () => {
    const { unmount } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
    unmount();
    expect(MockEventSource.instances[0].close).toHaveBeenCalled();
  });

  it("does not connect while idle", async () => {
    setup("case-1", true);
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(0);
    expect(mockGetIdToken).not.toHaveBeenCalled();
  });

  it("closes the open stream when idle starts and reconnects fresh when it clears", async () => {
    const { rerender } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
    const first = MockEventSource.instances[0];

    idleRef.current = true;
    rerender();
    expect(first.close).toHaveBeenCalled();
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(1);

    idleRef.current = false;
    rerender();
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(2));
  });

  it("cancels a pending reconnect when idle starts", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { rerender } = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("error");

      idleRef.current = true;
      rerender();
      await vi.advanceTimersByTimeAsync(10 * 60_000);
      expect(MockEventSource.instances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
