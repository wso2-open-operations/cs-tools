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

/**
 * How many widget data-fetch requests (`useWidgetData` / `useWidgetPieData`)
 * may be in flight at once, across the whole app, not just one dashboard.
 * (The cap itself stays app-wide and team-agnostic; only the FIFO *queue*
 * is team-aware — see `withWidgetFetchSlot`'s own `teamKey` parameter and
 * {@link WidgetFetchQueueDroppedError} for why switching the selected ABT
 * team drops the old team's still-queued entries instead of making the
 * new team's widgets wait behind them.)
 *
 * A dashboard with N widgets previously fired N `/…/search` calls to
 * `customer-entity-service` essentially simultaneously on mount (one per
 * widget tile, each an independent `react-query` `queryFn`, no
 * coordination between them) — `abt-engineer` alone has ~20 widgets, and a
 * `shape: "pie"` widget fires one call per slice on top of that. Production
 * logs showed bursts of 5-13 simultaneous `Request timeout: POST
 * /cases/search` within ~170ms of each other on the entity-service, which
 * the BFF surfaced as HTTP 503 ("upstream connect error or disconnect/reset
 * before headers, reset reason: connection termination") at ~15s elapsed.
 *
 * `1` — fully sequential, one widget fetch in flight at a time. This
 * supersedes an earlier `6` (chosen to match Chrome's own default
 * per-origin HTTP/1.1 connection limit, roughly halving the worst observed
 * prod burst while letting most dashboards render with barely perceptible
 * delay): the user explicitly asked for strictly one-by-one loading on the
 * abt-lead dashboard after checking `6` live, prioritizing backend-load
 * reduction over dashboard render speed. `1` costs the most render-latency
 * of any value here — a 20-widget dashboard now takes 20 sequential round
 * trips instead of ~4 batches of 6 — which is the explicit trade the user
 * chose, not an oversight. Tune here only; nothing else in the widget-fetch
 * path should hardcode a concurrency number.
 */
export const WIDGET_FETCH_CONCURRENCY_LIMIT = 1;

/**
 * How long a single widget fetch may stay in flight before it's aborted and
 * treated as failed. Matters a great deal more now that
 * {@link WIDGET_FETCH_CONCURRENCY_LIMIT} is `1`: with no client-side
 * timeout, a single request that never resolves (a dropped connection, a
 * backend that hangs instead of erroring) would hold the app's one and
 * only fetch slot forever, freezing every OTHER widget on every OTHER
 * dashboard for the rest of that browser tab's session — this is the gap
 * flagged as a follow-up risk when the concurrency cap first dropped to 1.
 *
 * `10_000` (10s) — a single, findable constant — matching how
 * `WIDGET_FETCH_CONCURRENCY_LIMIT` is already tuned — rather than a
 * per-call override: nothing about a widget's own config (resource type,
 * shape, dashboard) plausibly needs a different timeout than any other
 * widget, so a per-call parameter would be configurability nobody asked
 * for. Tune here only (or via `__setWidgetFetchTimeoutMsForTests` below,
 * test-only). Was `30_000` initially; dropped to `10_000` per direct
 * instruction, no further reasoning given beyond wanting a hung widget to
 * give up (and free its slot) sooner.
 *
 * `let`, not `const` — mutable ONLY through the test-only setter below.
 * Everything that reads this (this module's own `withWidgetFetchSlot`,
 * plus every doc comment) sees the live value via the module binding, so
 * a test can shrink it to a few milliseconds and use real timers/`waitFor`
 * instead of fake ones — see that function's own comment for why fake
 * timers specifically don't work for this.
 */
export let WIDGET_FETCH_TIMEOUT_MS = 10_000;

let activeCount = 0;

interface Waiter {
  teamKey: string;
  resolve: () => void;
  reject: (error: Error) => void;
}

const waiters: Waiter[] = [];

/**
 * The team (or team-family) whose widgets most recently called into this
 * queue — `null` until the first call. Used purely to detect a CHANGE: the
 * whole app shares one queue (see {@link WIDGET_FETCH_CONCURRENCY_LIMIT}'s
 * own doc comment on why it's module-level, not per-dashboard), so a
 * change here means "the selected ABT team switched," and every entry
 * still queued for the OLD key is abandoned — see
 * {@link WidgetFetchQueueDroppedError}.
 */
let currentTeamKey: string | null = null;

/**
 * Thrown to a queued (not-yet-started) widget fetch whose `teamKey` no
 * longer matches {@link currentTeamKey} — i.e. the selected ABT team
 * switched while this fetch was still waiting for a slot. Distinguished
 * from an ordinary failure so `shouldRetryWidgetFetch` can decide whether
 * retrying it is worth anything: for a team-SPECIFIC widget (whose filters
 * reference `__current_team__`), the switch already moved its `queryKey`
 * to the new team, so re-entering the queue for the dropped fetch would be
 * pure waste — a query nobody observes anymore. For a team-INDEPENDENT
 * widget, the `queryKey` didn't change, so a retry is the only thing that
 * un-sticks it (see `shouldRetryWidgetFetch`'s own doc comment for the
 * full reasoning). Also lets a caller that wants to tell "queue drop"
 * apart from "the request itself failed" `instanceof` it.
 */
export class WidgetFetchQueueDroppedError extends Error {
  constructor() {
    super("Widget fetch dropped from the queue: the selected team changed.");
    this.name = "WidgetFetchQueueDroppedError";
  }
}

/**
 * Rejects and removes every currently queued (not active) waiter whose own
 * `teamKey` doesn't match `newTeamKey` — called once per actual team
 * change, from whichever widget's fetch call happens to run first after
 * the switch (see `acquireWidgetFetchSlot`). The one fetch that may
 * already be ACTIVE (at most one, given
 * {@link WIDGET_FETCH_CONCURRENCY_LIMIT} = 1) is untouched here: it's left
 * to finish naturally rather than force-aborted, per the explicit
 * "abandon the queue, don't cancel what's in flight" shape this was built
 * to.
 */
function dropWaitersForOldTeam(newTeamKey: string): void {
  for (let i = waiters.length - 1; i >= 0; i -= 1) {
    if (waiters[i].teamKey !== newTeamKey) {
      const [dropped] = waiters.splice(i, 1);
      dropped.reject(new WidgetFetchQueueDroppedError());
    }
  }
}

function acquireWidgetFetchSlot(teamKey: string): Promise<void> {
  // A change in team is detected relative to the value already stored, so
  // it must be checked (and `currentTeamKey` updated) BEFORE this call's
  // own acquire/queue logic below — otherwise this very call could see
  // itself as "the team that changed" and drop its own place in line.
  if (currentTeamKey !== teamKey) {
    currentTeamKey = teamKey;
    dropWaitersForOldTeam(teamKey);
  }
  if (activeCount < WIDGET_FETCH_CONCURRENCY_LIMIT) {
    activeCount += 1;
    return Promise.resolve();
  }
  return new Promise<void>((resolve, reject) => {
    waiters.push({
      teamKey,
      resolve: () => {
        activeCount += 1;
        resolve();
      },
      reject,
    });
  });
}

function releaseWidgetFetchSlot(): void {
  activeCount = Math.max(0, activeCount - 1);
  const next = waiters.shift();
  if (next) next.resolve();
}

/**
 * Runs `fn` once a widget-fetch slot is free, releasing the slot as soon as
 * `fn` settles (success, failure, OR timeout) so the next queued fetch can
 * start. Callers past the cap simply await longer before `fn` starts — no
 * error, no change to `fn`'s own result or to `react-query`'s loading/error
 * state, which is exactly what a queued-but-not-yet-fired widget should
 * show: its normal loading skeleton, same as an in-flight one.
 *
 * `fn` receives an `AbortSignal` — pass it straight through to `api.post`'s
 * own `signal` option — that fires automatically after
 * {@link WIDGET_FETCH_TIMEOUT_MS} of `fn` actually running (the timer
 * starts once a slot is acquired, not while still queued behind another
 * widget — queueing time is governed by the concurrency cap, not this
 * timeout). An aborted fetch rejects like any other failed fetch: the
 * widget's own `queryFn` throws, `react-query` marks it `isError`, and —
 * the part that matters most here — this function's own `finally` still
 * runs and releases the slot, so a hung request degrades to "this one
 * widget shows an error" instead of "every widget behind it in the queue
 * never loads."
 *
 * A hand-rolled FIFO semaphore rather than a dependency (`p-limit` etc.) —
 * neither is already in `package.json`, and the mechanism this needs is a
 * dozen lines.
 *
 * `teamKey` scopes the FIFO *drop* behavior, not the concurrency cap
 * itself (the cap stays app-wide — see {@link WIDGET_FETCH_CONCURRENCY_LIMIT}):
 * when a call arrives with a `teamKey` different from whichever key the
 * queue last saw, every OTHER entry still queued (not yet started) under
 * the old key is rejected with {@link WidgetFetchQueueDroppedError} and
 * removed, so a switch of the selected ABT team doesn't leave the new
 * team's widgets waiting behind the old team's full remaining backlog.
 * Pass a stable, derived key (e.g. a serialization of the selected team's
 * group ids) — a dashboard with no team concept can pass any constant
 * value, since a key that never changes never drops anything.
 */
export async function withWidgetFetchSlot<T>(
  fn: (signal: AbortSignal) => Promise<T>,
  teamKey: string,
): Promise<T> {
  await acquireWidgetFetchSlot(teamKey);
  const controller = new AbortController();
  const timeoutId = setTimeout(() => {
    controller.abort();
  }, WIDGET_FETCH_TIMEOUT_MS);
  try {
    return await fn(controller.signal);
  } finally {
    clearTimeout(timeoutId);
    releaseWidgetFetchSlot();
  }
}

/**
 * `react-query` `retry` predicate for `useWidgetData`/`useWidgetPieData`'s
 * own queries — set explicitly per-query (not left to inherit
 * `AppWithConfig.tsx`'s app-wide `shouldRetryQuery`) so a widget query's
 * retry policy is scoped to widget fetches only, not a silent app-wide
 * behavior change smuggled in through this task.
 *
 * Retries exactly once (same "one retry, then stop" shape as the app's own
 * global default for a 502/503) when the failure was THIS module's own
 * timeout abort — `error.name === "AbortError"` is a reliable enough
 * signal here specifically because nothing else in the widget-fetch path
 * ever calls `AbortController.abort()`; `withWidgetFetchSlot` is the only
 * source. A timed-out widget is deliberately NOT treated as a terminal
 * failure on its first attempt — see {@link WIDGET_FETCH_TIMEOUT_MS}'s own
 * intent: a slow widget should get out of everyone else's way (already
 * true from the timeout + slot release alone) AND get a second chance
 * once the rest of the queue has had a turn, rather than sitting in a
 * permanent error state for the rest of the dashboard's life.
 *
 * That "after the rest of the queue" ordering needs no separate mechanism:
 * every OTHER widget on the same dashboard already called
 * `withWidgetFetchSlot` (and so entered the FIFO `waiters` queue below) at
 * mount time, well before this one's timeout could possibly have fired —
 * so by the time `react-query` re-invokes this queryFn for the retry, its
 * fresh call to `acquireWidgetFetchSlot()` necessarily lands behind
 * whichever of those widgets are still waiting. A retry re-enters the
 * exact same queue as everyone else; it gets no special priority.
 *
 * Also retries once on a 502/503 from the backend itself — deliberately
 * NOT just inheriting the app's own global default (see `shouldRetryQuery`
 * in `AppWithConfig.tsx`), since setting ANY explicit `retry` option on a
 * query replaces the whole option for that query rather than adding to
 * it — a widget query needs its own predicate that does everything the
 * global one does, plus the timeout case.
 *
 * `failureCount` here is 0 on the FIRST failure, not 1 — react-query's own
 * retryer calls `retry(failureCount, error)` BEFORE incrementing its
 * internal counter (verified against `@tanstack/query-core`'s own
 * `retryer.ts` directly rather than assumed), so `failureCount >= 1` is
 * what caps this at exactly one retry (two attempts total): first failure
 * sees `0` (retry allowed), second sees `1` (stop). An off-by-one here
 * (`>= 2`, matching the global default's own threshold — copied from it
 * without re-deriving this) silently allowed a 2nd retry — caught in this
 * task's own verification, not by inspection alone.
 *
 * `isTeamIndependent` — whether THIS query's own filters (see
 * `hasTeamPlaceholder` in `teamFilterPlaceholder.ts`) reference
 * `__current_team__` — decides whether a queue-drop is worth retrying at
 * all (see the `WidgetFetchQueueDroppedError` branch below); it plays no
 * part in the timeout/502/503 branches, which retry unconditionally
 * regardless of a widget's own team-scoping.
 */
export function shouldRetryWidgetFetch(
  failureCount: number,
  error: Error,
  isTeamIndependent: boolean,
): boolean {
  if (failureCount >= 1) return false;
  // A queue-drop means the selected team changed while this fetch was still
  // waiting for a slot. The queue-drop's `teamKey` mismatch is keyed off the
  // PAGE-level selected team, not off whether this particular widget's own
  // `filters` actually reference `__current_team__` — so a TEAM-INDEPENDENT
  // widget (whose filters don't reference the team) keeps the exact same
  // `queryKey` across the switch, and nothing else would ever naturally
  // re-trigger it (react-query only auto-refetches on a `queryKey` change).
  // Without a retry here, that widget is left stuck in a permanent error
  // state until something unrelated forces a full remount. Allowing exactly
  // one retry (still capped by `failureCount >= 1` above, so a rapidly
  // flip-flopping team can't loop) is safe: by the time the retry's
  // `queryFn` actually executes, react-query has re-synced the query's
  // active `queryFn` to whatever the most recent render passed in, so the
  // retry re-enters `withWidgetFetchSlot` with whatever `teamKey` is CURRENT
  // by then, not a stale one — and since this query's `queryKey` never
  // changed, a slot is free immediately and it succeeds.
  //
  // A TEAM-SPECIFIC widget (filters DO reference `__current_team__`) is the
  // opposite case: the team switch already gave it a brand-new resolved
  // `queryKey` (see `resolveTeamPlaceholder`), so react-query mounts a fresh
  // query for the new team regardless of what happens to the old, dropped
  // one — retrying the dropped fetch would just re-populate a cache entry
  // for a `queryKey` nobody reads anymore. Refusing to retry here isn't a
  // regression from the team-independent case above; it's the correct
  // no-op, since the widget's own re-render already did the work a retry
  // would have tried to do.
  if (error instanceof WidgetFetchQueueDroppedError) return isTeamIndependent;
  if (error?.name === "AbortError") return true;
  const errorWithStatus = error as Error & {
    response?: { status?: number };
    status?: number;
  };
  const statusCode = errorWithStatus.response?.status || errorWithStatus.status;
  return statusCode === 502 || statusCode === 503;
}

/**
 * Test-only escape hatch: clears every held/queued slot. `activeCount` and
 * `waiters` are module-level (deliberately — the cap is app-wide, not
 * per-dashboard), which means a test that mounts a widget whose fetch is
 * left permanently pending (`postMock.mockReturnValue(new Promise(() => {}))`,
 * a real pattern this codebase uses to assert a loading state) never
 * releases the slot it acquired — harmless at a cap of 6 (five slots still
 * free for the rest of that test file), but at a cap of 1 it permanently
 * starves every OTHER test in the same file, since there is nothing left
 * to acquire. Call this in a `beforeEach`/`afterEach` in any test file that
 * exercises a widget whose fetch may be left unresolved.
 */
export function __resetWidgetFetchConcurrencyForTests(): void {
  activeCount = 0;
  waiters.length = 0;
  currentTeamKey = null;
}

/**
 * Test-only escape hatch: overrides {@link WIDGET_FETCH_TIMEOUT_MS} for the
 * rest of the current test file/run. A real integration test that needs to
 * prove "aborts at ~the configured timeout, releases the slot, retries
 * once" through actual `react-query` + React rendering cannot reliably use
 * `vi.useFakeTimers()` for that proof — `@tanstack/query-core`'s internal
 * notification scheduling (`notifyManager`, `systemSetTimeoutZero`) and
 * React's own scheduler did not observably settle under
 * `vi.advanceTimersByTimeAsync` in this app's actual dependency versions
 * when verified directly (a bare two-hook `useQuery` test, no custom code
 * at all, stayed `pending` forever after generous fake-timer advancement +
 * flushing) — so real timers + a shrunk-down real timeout is the reliable
 * path, not a workaround for a mistake in this module's own code. Always
 * pair a call to this with one restoring the real default afterward (see
 * any test file that uses it).
 */
export function __setWidgetFetchTimeoutMsForTests(ms: number): void {
  WIDGET_FETCH_TIMEOUT_MS = ms;
}
