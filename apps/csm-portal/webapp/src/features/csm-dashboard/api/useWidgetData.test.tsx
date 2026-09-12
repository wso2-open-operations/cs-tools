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

import { render, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import {
  resolveWidgetRefetchInterval,
  useWidgetData,
} from "@features/csm-dashboard/api/useWidgetData";
import {
  WIDGET_FETCH_CONCURRENCY_LIMIT,
  __resetWidgetFetchConcurrencyForTests,
  __setWidgetFetchTimeoutMsForTests,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

/** One dashboard tile: mounts its own independent useWidgetData query, same
 * as DashboardWidgetTile does per widget — the real fan-out this task caps. */
function Widget({ id }: { id: string }) {
  useWidgetData({ widgetId: id, resourceType: "case", filters: { states: ["open"] }, shape: "count" });
  return null;
}

function Dashboard({ widgetIds }: { widgetIds: string[] }) {
  return (
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      {widgetIds.map((id) => (
        <Widget key={id} id={id} />
      ))}
    </QueryClientProvider>
  );
}

describe("useWidgetData", () => {
  beforeEach(() => {
    postMock.mockReset();
    __resetWidgetFetchConcurrencyForTests();
  });

  it("issues one search for the widget's own filters, shape count uses limit 1", async () => {
    postMock.mockResolvedValue({ total: 7, items: [] });

    renderHook(() => useWidgetData({ widgetId: "w1", resourceType: "case", filters: { states: ["open"] }, shape: "count" }), { wrapper });

    await waitFor(() => expect(postMock).toHaveBeenCalled());
    expect(postMock).toHaveBeenCalledWith(
      "/cases/search",
      {
        filters: { states: ["open"] },
        pagination: { offset: 0, limit: 1 },
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("caps concurrent in-flight /cases/search calls at WIDGET_FETCH_CONCURRENCY_LIMIT when a dashboard's worth of widgets all mount at once", async () => {
    // A dashboard with more widgets than abt-engineer's ~20 — deliberately
    // not a clean multiple of the cap.
    const widgetCount = WIDGET_FETCH_CONCURRENCY_LIMIT * 3 + 4;

    let inFlight = 0;
    let peakInFlight = 0;
    const releasers: Array<() => void> = [];

    postMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          inFlight += 1;
          peakInFlight = Math.max(peakInFlight, inFlight);
          releasers.push(() => {
            inFlight -= 1;
            resolve({ total: 1, items: [] });
          });
        }),
    );

    const widgetIds = Array.from({ length: widgetCount }, (_, i) => `w${i}`);
    render(<Dashboard widgetIds={widgetIds} />);

    // Let every widget's query mount and request a slot.
    await waitFor(() => expect(postMock.mock.calls.length).toBeGreaterThan(0));

    // Every widget beyond the cap must still be queued, not fired — the
    // core assertion this task exists to prove.
    expect(peakInFlight).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
    expect(postMock.mock.calls.length).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);

    // Drain the queue in batches, exactly as a real backend finishing
    // requests over time would — the queued widgets fire in turn and the
    // cap continues to hold, until every widget has fetched.
    while (postMock.mock.calls.length < widgetCount || releasers.length > 0) {
      const batch = releasers.splice(0, releasers.length);
      batch.forEach((release) => release());
      await waitFor(() => expect(inFlight).toBe(0));
      expect(peakInFlight).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
      if (postMock.mock.calls.length >= widgetCount) break;
      await waitFor(() => expect(postMock.mock.calls.length).toBeGreaterThan(0));
    }

    await waitFor(() => expect(postMock.mock.calls.length).toBe(widgetCount));
    expect(peakInFlight).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
  });

  describe("client-side request timeout", () => {
    // Real timers throughout this block, deliberately — see
    // __setWidgetFetchTimeoutMsForTests's own doc comment: this app's
    // actual react-query + React versions did not observably propagate a
    // settled query's state to a hook's result under
    // vi.useFakeTimers()/advanceTimersByTimeAsync in direct testing (a bare
    // two-hook useQuery repro, no custom code at all, stayed "pending"
    // forever), so real timers + a shrunk-down real timeout is the
    // reliable path here, not a stylistic choice.
    const TEST_TIMEOUT_MS = 60;
    // For "must NOT have happened yet" assertions: a plain real sleep
    // shorter than TEST_TIMEOUT_MS, not `waitFor`'s own default ~50ms poll
    // interval — that interval is close enough to a very small
    // TEST_TIMEOUT_MS that a `waitFor` resolving on its first poll could
    // already be past the timeout by the time this test's next assertion
    // runs, producing a flaky false negative unrelated to the actual
    // behaviour being proven.
    const SHORT_SETTLE_MS = 15;

    function sleep(ms: number): Promise<void> {
      return new Promise((resolve) => setTimeout(resolve, ms));
    }

    // react-query's own default retryDelay (~1000ms for the first retry)
    // would make every test in this block take a real ~1s+ for no benefit
    // to what's being proven — this wrapper's QueryClient shortens it to a
    // few ms. Local to this describe block; every other test in this file
    // keeps using the shared, unmodified `wrapper`.
    function wrapperWithFastRetry({ children }: { children: ReactNode }) {
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false, retryDelay: 50 } },
      });
      return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
    }

    beforeEach(() => {
      __setWidgetFetchTimeoutMsForTests(TEST_TIMEOUT_MS);
    });

    afterEach(() => {
      __setWidgetFetchTimeoutMsForTests(10_000);
    });

    /** A hung `postMock` implementation: never settles on its own, only
     * rejects (with the same `AbortError` shape a real aborted `fetch`
     * would produce) once its own `signal` fires. Pushes `label` onto
     * `events` the instant it's invoked, so tests can assert call ORDER,
     * not just call count. */
    function hungCall(events: string[], label: string) {
      return (_path: string, _body: unknown, options?: { signal?: AbortSignal }) => {
        events.push(label);
        return new Promise((_resolve, reject) => {
          options?.signal?.addEventListener("abort", () => {
            const abortError = new Error("The operation was aborted.");
            abortError.name = "AbortError";
            reject(abortError);
          });
        });
      };
    }

    it("aborts a widget fetch that never resolves once the configured timeout elapses, and releases the slot so the next queued widget's fetch fires immediately (not after the retry)", async () => {
      const events: string[] = [];
      let callCount = 0;
      postMock.mockImplementation((path, body, options?: { signal?: AbortSignal }) => {
        callCount += 1;
        if (callCount === 1) return hungCall(events, "w-hung:attempt-1")(path, body, options);
        events.push("w-queued:call");
        return Promise.resolve({ total: 5, items: [] });
      });

      const first = renderHook(
        () => useWidgetData({ widgetId: "w-hung", resourceType: "case", filters: { states: ["open"] }, shape: "count" }),
        { wrapper: wrapperWithFastRetry },
      );
      const second = renderHook(
        () => useWidgetData({ widgetId: "w-queued", resourceType: "case", filters: { states: ["open"] }, shape: "count" }),
        { wrapper: wrapperWithFastRetry },
      );

      // Widget 1 fires immediately (it acquires the — sole, since
      // WIDGET_FETCH_CONCURRENCY_LIMIT is 1 — slot); widget 2 is queued
      // behind it and must still be waiting well before the timeout.
      await sleep(SHORT_SETTLE_MS);
      expect(events).toEqual(["w-hung:attempt-1"]);
      expect(first.result.current.isLoading).toBe(true);

      // Crossing the deadline aborts widget 1's first attempt, which
      // releases its slot immediately — widget 2 must fire right away, not
      // wait for widget 1's retry.
      await waitFor(() => expect(events).toContain("w-queued:call"));
      expect(events).toEqual(["w-hung:attempt-1", "w-queued:call"]);
      await waitFor(() =>
        expect(second.result.current.data).toEqual({ total: 5, items: [] }),
      );
      // Widget 1 itself is NOT yet in a terminal error state — a timeout is
      // retried once (see the next test), so it's still "loading" (now
      // waiting out react-query's retry backoff), not failed.
      expect(first.result.current.isError).toBe(false);
      expect(first.result.current.isLoading).toBe(true);
    });

    it("retries a timed-out widget exactly once, after the rest of the queue has already had its turn, and shows the resolved data on success", async () => {
      const events: string[] = [];
      // One shared mock, keyed purely on call order — the module-level
      // semaphore doesn't distinguish which widget owns a given attempt,
      // so neither does this test's setup: whichever widget's queryFn
      // actually calls postMock next determines what happens.
      let callCount = 0;
      postMock.mockImplementation((path, body, options?: { signal?: AbortSignal }) => {
        callCount += 1;
        if (callCount === 1) {
          // w-retry's 1st attempt: hangs, only rejects on abort.
          return hungCall(events, "w-retry:attempt-1")(path, body, options);
        }
        if (callCount === 2) {
          // w-other: resolves immediately — the "rest of the queue" the
          // retry must land behind.
          events.push("w-other:call");
          return Promise.resolve({ total: 2, items: [] });
        }
        // w-retry's 2nd attempt (the retry): succeeds.
        events.push("w-retry:attempt-2");
        return Promise.resolve({ total: 9, items: [] });
      });

      const retrying = renderHook(
        () => useWidgetData({ widgetId: "w-retry", resourceType: "case", filters: { states: ["open"] }, shape: "count" }),
        { wrapper: wrapperWithFastRetry },
      );
      const other = renderHook(
        () => useWidgetData({ widgetId: "w-other", resourceType: "case", filters: { severities: ["critical"] }, shape: "count" }),
        { wrapper: wrapperWithFastRetry },
      );

      // Attempt 1 fires immediately; the other widget queues behind it and
      // must still be waiting well before the timeout.
      await sleep(SHORT_SETTLE_MS);
      expect(events).toEqual(["w-retry:attempt-1"]);

      // Attempt 1 times out — its slot releases immediately, so the queued
      // widget fires right away, well before the retry (which is on its
      // own ~50ms backoff, started only once attempt 1 actually failed).
      await waitFor(() => expect(events).toContain("w-other:call"));
      expect(events).toEqual(["w-retry:attempt-1", "w-other:call"]);
      await waitFor(() =>
        expect(other.result.current.data).toEqual({ total: 2, items: [] }),
      );
      // The retry must not have appeared yet, this soon after the other
      // widget's own data first became available — this is the ordering
      // assertion that actually matters, not just eventual call counts.
      expect(events).toEqual(["w-retry:attempt-1", "w-other:call"]);

      // The retried widget's 2nd attempt fires afterward and succeeds.
      await waitFor(() => expect(events).toContain("w-retry:attempt-2"));
      expect(events).toEqual(["w-retry:attempt-1", "w-other:call", "w-retry:attempt-2"]);
      await waitFor(() =>
        expect(retrying.result.current.data).toEqual({ total: 9, items: [] }),
      );
      expect(retrying.result.current.isError).toBe(false);
    });

    it("does not retry forever: a widget whose retry ALSO times out reaches a real terminal error state, with no 3rd attempt", async () => {
      const events: string[] = [];
      let callCount = 0;
      postMock.mockImplementation((path, body, options?: { signal?: AbortSignal }) => {
        callCount += 1;
        return hungCall(events, `attempt-${callCount}`)(path, body, options);
      });

      const { result } = renderHook(
        () => useWidgetData({ widgetId: "w-double-timeout", resourceType: "case", filters: {}, shape: "count" }),
        { wrapper: wrapperWithFastRetry },
      );

      // Attempt 1 times out.
      await waitFor(() => expect(events).toContain("attempt-1"));
      // Not terminal yet — one retry is still owed.
      expect(result.current.isError).toBe(false);

      // Attempt 2 (the retry) also times out.
      await waitFor(() => expect(events).toContain("attempt-2"));
      expect(events).toEqual(["attempt-1", "attempt-2"]);

      // NOW it's a real terminal failure.
      await waitFor(() => expect(result.current.isError).toBe(true));

      // Give a hypothetical 3rd attempt every chance to have fired — it
      // must not: the one-retry budget is exhausted.
      await new Promise((resolve) => setTimeout(resolve, TEST_TIMEOUT_MS * 3));
      expect(events).toEqual(["attempt-1", "attempt-2"]);
      expect(result.current.isError).toBe(true);
    });
  });
});

describe("resolveWidgetRefetchInterval", () => {
  it("returns undefined (no auto-refetch) when the caller set no interval, regardless of queue depth", () => {
    expect(resolveWidgetRefetchInterval(undefined, 0)).toBeUndefined();
    expect(resolveWidgetRefetchInterval(undefined, 5)).toBeUndefined();
  });

  it("returns the configured interval when the shared fetch queue is idle", () => {
    expect(resolveWidgetRefetchInterval(60_000, 0)).toBe(60_000);
  });

  it("suppresses the refetch (false) while the queue still has a wave draining, so ticks don't stack", () => {
    expect(resolveWidgetRefetchInterval(60_000, 1)).toBe(false);
    expect(resolveWidgetRefetchInterval(60_000, 12)).toBe(false);
  });
});
