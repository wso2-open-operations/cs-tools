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

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  WIDGET_FETCH_CONCURRENCY_LIMIT,
  WIDGET_FETCH_TIMEOUT_MS,
  WidgetFetchQueueDroppedError,
  __resetWidgetFetchConcurrencyForTests,
  shouldRetryWidgetFetch,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";

/** A fixed team key every test in this file that doesn't care about
 * team-switching behavior can pass — keeps those tests unaffected by the
 * team-aware drop logic (a key that never changes never drops anything). */
const TEAM_A = "team-a";
const TEAM_B = "team-b";

describe("withWidgetFetchSlot", () => {
  afterEach(() => {
    __resetWidgetFetchConcurrencyForTests();
  });

  it("never lets more than WIDGET_FETCH_CONCURRENCY_LIMIT calls run at once, even when far more are requested simultaneously", async () => {
    const totalRequests = WIDGET_FETCH_CONCURRENCY_LIMIT * 4 + 3; // deliberately not a clean multiple
    let concurrent = 0;
    let peakConcurrent = 0;
    const started: number[] = [];

    // Every "widget" resolves only once every one of them has requested a
    // slot — the same shape as N dashboard tiles all mounting and firing
    // their query at once. If the cap didn't hold, every one of these
    // would run its body immediately and peakConcurrent would equal
    // totalRequests.
    const releaseGate = () => new Promise<void>((resolve) => setTimeout(resolve, 5));

    const calls = Array.from({ length: totalRequests }, (_, i) =>
      withWidgetFetchSlot(async () => {
        started.push(i);
        concurrent += 1;
        peakConcurrent = Math.max(peakConcurrent, concurrent);
        await releaseGate();
        concurrent -= 1;
        return i;
      }, TEAM_A),
    );

    const results = await Promise.all(calls);

    expect(results).toHaveLength(totalRequests);
    expect(peakConcurrent).toBeLessThanOrEqual(WIDGET_FETCH_CONCURRENCY_LIMIT);
    expect(peakConcurrent).toBeGreaterThan(0);
    // With a real gate every call has to wait through, the cap should
    // actually bind at least once — otherwise this test would pass
    // vacuously even with no cap at all if totalRequests were small.
    expect(peakConcurrent).toBe(WIDGET_FETCH_CONCURRENCY_LIMIT);
    // A LIFO or arbitrary-order queue would also pass every assertion above
    // (peak concurrency is unaffected by ordering) — this is the one that
    // actually proves the queue is FIFO, not just capped.
    expect(started).toEqual(Array.from({ length: totalRequests }, (_, i) => i));
  });

  it("releases a slot as soon as fn rejects, so a failing widget doesn't starve the queue", async () => {
    const results: string[] = [];

    const failing = withWidgetFetchSlot(async () => {
      throw new Error("widget search failed");
    }, TEAM_A).catch(() => {
      results.push("failed");
    });

    // Fill every remaining slot with fast successes.
    const fillers = Array.from({ length: WIDGET_FETCH_CONCURRENCY_LIMIT - 1 }, (_, i) =>
      withWidgetFetchSlot(async () => {
        results.push(`filler-${i}`);
      }, TEAM_A),
    );

    // One more, queued past the cap — only starts once a slot frees up,
    // which requires the failed call above to have released its slot.
    const queued = withWidgetFetchSlot(async () => {
      results.push("queued");
    }, TEAM_A);

    await Promise.all([failing, ...fillers, queued]);

    expect(results).toContain("failed");
    expect(results).toContain("queued");
  });

  it("returns fn's own resolved value unchanged", async () => {
    const value = await withWidgetFetchSlot(async () => ({ total: 42 }), TEAM_A);
    expect(value).toEqual({ total: 42 });
  });

  it("propagates fn's own rejection unchanged", async () => {
    await expect(
      withWidgetFetchSlot(async () => {
        throw new Error("boom");
      }, TEAM_A),
    ).rejects.toThrow("boom");
  });

  describe("timeout", () => {
    // Pure promises/timers, no React/react-query in this describe block —
    // fake timers are reliable here (verified directly: this exact
    // pattern, `vi.advanceTimersByTimeAsync` + a `setTimeout`-driven
    // promise, resolves correctly with no React involved; the
    // React/react-query-specific unreliability that pushed the hook-level
    // retry tests to real timers — see useWidgetData.test.tsx — doesn't
    // apply to this module in isolation).
    afterEach(() => {
      vi.useRealTimers();
    });

    it("aborts fn via the provided signal at exactly WIDGET_FETCH_TIMEOUT_MS, not before", async () => {
      vi.useFakeTimers();
      let aborted = false;

      const call = withWidgetFetchSlot(
        (signal) =>
          new Promise((_resolve, reject) => {
            signal.addEventListener("abort", () => {
              aborted = true;
              reject(Object.assign(new Error("aborted"), { name: "AbortError" }));
            });
          }),
        TEAM_A,
      );
      // Swallow the eventual rejection here — asserted properly below —
      // so it doesn't surface as an unhandled rejection while we're still
      // advancing time toward it.
      call.catch(() => {});

      await vi.advanceTimersByTimeAsync(WIDGET_FETCH_TIMEOUT_MS - 1);
      expect(aborted).toBe(false);

      await vi.advanceTimersByTimeAsync(1);
      expect(aborted).toBe(true);
      await expect(call).rejects.toMatchObject({ name: "AbortError" });

      vi.useRealTimers();
    });

    it("releases the slot on timeout so the next queued call proceeds — at the raw semaphore level, no React involved", async () => {
      vi.useFakeTimers();
      const events: string[] = [];

      withWidgetFetchSlot(
        (signal) =>
          new Promise((_resolve, reject) => {
            events.push("first-started");
            signal.addEventListener("abort", () => {
              reject(Object.assign(new Error("aborted"), { name: "AbortError" }));
            });
          }),
        TEAM_A,
      ).catch(() => {
        events.push("first-rejected");
      });

      const second = withWidgetFetchSlot(async () => {
        events.push("second-started");
        return "second-result";
      }, TEAM_A);

      await vi.advanceTimersByTimeAsync(0);
      // WIDGET_FETCH_CONCURRENCY_LIMIT is 1 — the second call must still be
      // queued, not started, before the first has even timed out.
      expect(events).toEqual(["first-started"]);

      await vi.advanceTimersByTimeAsync(WIDGET_FETCH_TIMEOUT_MS - 1);
      expect(events).toEqual(["first-started"]);

      // Crossing the deadline: the first call's slot releases, and the
      // second — previously queued — call proceeds. This is the assertion
      // that actually matters for this task: a timeout must not leak the
      // slot the way an unresolved test-mock promise (see
      // __resetWidgetFetchConcurrencyForTests) does.
      await vi.advanceTimersByTimeAsync(1);
      await expect(second).resolves.toBe("second-result");
      expect(events).toContain("second-started");

      vi.useRealTimers();
    });
  });

  describe("team-scoped queue drop", () => {
    it("keeps a same-team call queued normally behind an in-flight one (regression check, unaffected by team-awareness)", async () => {
      const events: string[] = [];

      const first = withWidgetFetchSlot(async () => {
        events.push("first-started");
        return "first-result";
      }, TEAM_A);

      const second = withWidgetFetchSlot(async () => {
        events.push("second-started");
        return "second-result";
      }, TEAM_A);

      await expect(Promise.all([first, second])).resolves.toEqual([
        "first-result",
        "second-result",
      ]);
      expect(events).toEqual(["first-started", "second-started"]);
    });

    it("drops queued entries for the old team — rejecting with WidgetFetchQueueDroppedError, never invoking their fn — once a call for a new team arrives", async () => {
      const events: string[] = [];
      // See the next test's own comment on why this is a pre-built deferred
      // gate rather than a promise constructed inside `fn` itself.
      // Definite-assignment: the `Promise` executor runs synchronously on
      // construction, so `releaseActive` is always assigned before the
      // very next line — no `null` state ever actually observable.
      let releaseActive!: () => void;
      const activeGate = new Promise<void>((resolve) => {
        releaseActive = resolve;
      });

      // Occupy the one available slot so everything below actually queues
      // instead of starting immediately (WIDGET_FETCH_CONCURRENCY_LIMIT is 1).
      const active = withWidgetFetchSlot(() => activeGate, TEAM_A);

      const oldTeamQueued = withWidgetFetchSlot(async () => {
        events.push("old-team-fn-invoked");
      }, TEAM_A);

      // A call for a different team arrives while `oldTeamQueued` is still
      // waiting — this is the moment a widget's fetch (re)fires because the
      // selected ABT team switched.
      const newTeamCall = withWidgetFetchSlot(async () => {
        events.push("new-team-fn-invoked");
        return "new-team-result";
      }, TEAM_B);

      await expect(oldTeamQueued).rejects.toBeInstanceOf(WidgetFetchQueueDroppedError);
      expect(events).not.toContain("old-team-fn-invoked");

      // Release the still-active (old-team) fetch — untouched by the drop,
      // left to finish naturally — freeing the slot for the new team's call.
      releaseActive();
      await expect(active).resolves.toBeUndefined();
      await expect(newTeamCall).resolves.toBe("new-team-result");
      expect(events).toContain("new-team-fn-invoked");
    });

    it("does not drop an already-queued entry that matches the new current team key", async () => {
      const events: string[] = [];
      // A deferred gate `fn` awaits directly, set up synchronously here
      // (the `new Promise` executor runs synchronously on construction) so
      // releasing it doesn't depend on timing when `withWidgetFetchSlot`'s
      // own internal microtask chain actually gets around to invoking
      // `fn` — avoids a flaky race against that internal scheduling.
      // Definite-assignment: the `Promise` executor runs synchronously on
      // construction, so `releaseActive` is always assigned before the
      // very next line — no `null` state ever actually observable.
      let releaseActive!: () => void;
      const activeGate = new Promise<void>((resolve) => {
        releaseActive = resolve;
      });

      const active = withWidgetFetchSlot(() => activeGate, TEAM_A);

      // Queued under TEAM_B while TEAM_A is still active/current.
      const sameAsIncoming = withWidgetFetchSlot(async () => {
        events.push("same-as-incoming");
        return "survived";
      }, TEAM_B);

      // Another call for TEAM_B arrives — the *current* team switches to
      // TEAM_B, but `sameAsIncoming` already carries that same key, so it
      // must survive the drop untouched.
      const alsoTeamB = withWidgetFetchSlot(async () => {
        events.push("also-team-b");
        return "also-survived";
      }, TEAM_B);

      releaseActive();
      await active;

      await expect(sameAsIncoming).resolves.toBe("survived");
      await expect(alsoTeamB).resolves.toBe("also-survived");
      expect(events).toEqual(["same-as-incoming", "also-team-b"]);
    });
  });
});

describe("shouldRetryWidgetFetch", () => {
  /** Matches what `withWidgetFetchSlot`'s own timeout produces when it
   * aborts — see that function's own `controller.abort()` call. */
  const abortError = Object.assign(new Error("The operation was aborted."), {
    name: "AbortError",
  });

  it("retries once on this module's own timeout abort, regardless of isTeamIndependent", () => {
    // react-query calls its `retry` predicate with `failureCount` starting
    // at 0 on the FIRST failure (verified directly against
    // @tanstack/query-core's retryer.ts — NOT 1, which is the mistake this
    // test guards against reintroducing). The timeout branch doesn't care
    // about team-scoping at all — only the queue-drop branch does.
    expect(shouldRetryWidgetFetch(0, abortError, true)).toBe(true);
    expect(shouldRetryWidgetFetch(0, abortError, false)).toBe(true);
  });

  it("does not retry a second time once the retry itself has also failed", () => {
    expect(shouldRetryWidgetFetch(1, abortError, true)).toBe(false);
  });

  it("retries once on a 502/503 from the backend, same as the app's own global default, regardless of isTeamIndependent", () => {
    const badGateway = Object.assign(new Error("Bad Gateway"), { status: 502 });
    const serviceUnavailable = Object.assign(new Error("Service Unavailable"), {
      response: { status: 503 },
    });
    expect(shouldRetryWidgetFetch(0, badGateway, true)).toBe(true);
    expect(shouldRetryWidgetFetch(0, serviceUnavailable, false)).toBe(true);
    expect(shouldRetryWidgetFetch(1, badGateway, true)).toBe(false);
  });

  it("retries once (but not twice) a dropped-from-queue error for a team-independent widget", () => {
    // A widget whose `filters` don't reference the selected team keeps the
    // same `queryKey` across a team switch, so nothing else would ever
    // naturally re-trigger a query left stuck by a queue drop — one retry
    // is required to unstick it (see the function's own doc comment), but
    // still capped at exactly one so a flip-flopping team can't loop.
    expect(shouldRetryWidgetFetch(0, new WidgetFetchQueueDroppedError(), true)).toBe(true);
    expect(shouldRetryWidgetFetch(1, new WidgetFetchQueueDroppedError(), true)).toBe(false);
  });

  it("does not retry a dropped-from-queue error for a team-specific widget", () => {
    // A widget whose `filters` DO reference the selected team already got a
    // brand-new `queryKey` from the switch itself (see
    // `resolveTeamPlaceholder`) — retrying the dropped fetch would just
    // re-populate a cache entry for a `queryKey` nobody reads anymore.
    expect(shouldRetryWidgetFetch(0, new WidgetFetchQueueDroppedError(), false)).toBe(false);
  });

  it("does not retry an ordinary failure (e.g. a 400/404/500), same as before this task's retry policy existed", () => {
    const notFound = Object.assign(new Error("Not Found"), { status: 404 });
    const serverError = Object.assign(new Error("Internal Server Error"), { status: 500 });
    const genericFailure = new Error("Unsupported widget resourceType: bogus");
    expect(shouldRetryWidgetFetch(0, notFound, true)).toBe(false);
    expect(shouldRetryWidgetFetch(0, serverError, true)).toBe(false);
    expect(shouldRetryWidgetFetch(0, genericFailure, true)).toBe(false);
  });
});
