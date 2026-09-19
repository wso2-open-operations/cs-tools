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
 * How many case-create (and, for the customer flow, the follow-up tag-attach)
 * requests either announcement form may have in flight at once. The resolved
 * audience for "All customer projects" can be a few thousand projects at the
 * bound `useResolveAnnouncementAudience`/`useResolveProductVersionAudience`
 * enforce (see those hooks' own `MAX_AUDIENCE_PAGES`) — firing every one of
 * those as a simultaneous `POST /cases` (plus a second simultaneous
 * `POST /cases/{id}/tags` per success, for the customer flow) risked a burst
 * large enough to overload the backend, the same class of problem already
 * hit and fixed for dashboard widget fetches (see
 * `csm-dashboard/utils/widgetFetchConcurrency.ts`). That module's own limiter
 * isn't reused here: its concurrency budget and FIFO queue are deliberately
 * app-wide and widget-specific (team-switch drop semantics baked in), so
 * sharing it would make announcement sends compete with unrelated dashboard
 * widget fetches for the same single global slot. `5` mirrors that other
 * module's own prior default before it was tightened to `1` for a different,
 * dashboard-specific reason (see that constant's own doc comment) — Chrome's
 * per-origin HTTP/1.1 connection limit, a reasonable default absent a more
 * specific number for this endpoint.
 */
export const ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT = 5;

/**
 * Runs `fn` over every item in `items`, at most `limit` invocations in
 * flight at once, resolving once all have settled. Same result shape as
 * `Promise.allSettled` — one entry per item, in the original order — so a
 * caller already written against `Promise.allSettled` only needs to swap
 * the call itself, not its own result-handling logic.
 *
 * `onSettle`, when given, fires once per item as soon as that item's own
 * `fn` call settles (not in original-index order — whichever worker
 * finishes next) — the hook a caller needs to drive a live "N/total"
 * progress indicator while the batch is still running, rather than only
 * finding out the outcome after every item has finished.
 */
export async function settleWithConcurrencyLimit<T, R>(
  items: T[],
  limit: number,
  fn: (item: T) => Promise<R>,
  onSettle?: (result: PromiseSettledResult<R>, item: T, index: number) => void,
): Promise<PromiseSettledResult<R>[]> {
  const results: PromiseSettledResult<R>[] = new Array(items.length);
  let nextIndex = 0;

  async function worker(): Promise<void> {
    for (;;) {
      const i = nextIndex++;
      if (i >= items.length) return;
      let result: PromiseSettledResult<R>;
      try {
        result = { status: "fulfilled", value: await fn(items[i]) };
      } catch (error) {
        result = { status: "rejected", reason: error };
      }
      results[i] = result;
      onSettle?.(result, items[i], i);
    }
  }

  const workerCount = Math.max(1, Math.min(limit, items.length));
  await Promise.all(Array.from({ length: workerCount }, () => worker()));
  return results;
}
