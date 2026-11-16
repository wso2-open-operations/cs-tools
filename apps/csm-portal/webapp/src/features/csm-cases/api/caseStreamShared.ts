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

import type { QueryClient } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";

/**
 * staleTime for the per-case comment and activity queries. Both are refreshed
 * by the `case_updated` stream (see invalidateCaseById) and by their own
 * mutations, so a longer window only suppresses redundant refetches when
 * remounting a case view. Not for queries the stream does not cover. Matches
 * the case detail query.
 */
export const CASE_QUERY_STALE_TIME_MS = 30_000;

/** Base delay before the first reconnect attempt after the stream errors out or drops. */
const RECONNECT_BASE_DELAY_MS = 3_000;
/** Reconnect delay never grows past this while still within MAX_BACKOFF_ATTEMPTS. */
const RECONNECT_MAX_DELAY_MS = 30_000;
/**
 * After this many consecutive failures (the backoff curve has long since
 * maxed out at RECONNECT_MAX_DELAY_MS by then), stop retrying every <=30s —
 * a sustained outage/misconfiguration (e.g. the backend rejecting every
 * caller with a 403) would otherwise have every client call the backend
 * roughly twice a minute forever.
 */
export const MAX_BACKOFF_ATTEMPTS = 10;
/** Retry interval once MAX_BACKOFF_ATTEMPTS is exceeded — still eventually recovers once the backend does, at a fraction of the call volume. */
export const RECONNECT_IDLE_DELAY_MS = 5 * 60_000;

/**
 * Exponential backoff with full jitter (attempt 0 is a random delay in
 * [0, base), attempt 1 in [0, base*2), ... capped at max) — a sustained
 * backend outage or misconfiguration would otherwise have every client
 * retry in lockstep every RECONNECT_BASE_DELAY_MS forever, hammering the
 * endpoint indefinitely instead of backing off. Past MAX_BACKOFF_ATTEMPTS
 * consecutive failures, falls back to a slow idle retry instead of
 * continuing to hammer the endpoint every <=30s.
 */
export function reconnectDelay(attempt: number): number {
  if (attempt >= MAX_BACKOFF_ATTEMPTS) return RECONNECT_IDLE_DELAY_MS;
  const capped = Math.min(RECONNECT_MAX_DELAY_MS, RECONNECT_BASE_DELAY_MS * 2 ** attempt);
  return Math.random() * capped;
}

/**
 * Invalidates the per-case queries that can reflect a `case_updated` event
 * (comment, state, assignee, severity or acknowledgement change; the payload
 * carries only the case id, never the new values).
 *
 * - comments / activities / detail / SLAs: keyed by case id, so the case is
 *   invalidated precisely. Detail is declared as `[key, caseId, userEmail]`,
 *   which the `[key, caseId]` prefix covers. SLAs show the acknowledgement and
 *   response/resolution clocks that state, severity and ack changes move.
 */
export function invalidateCaseById(queryClient: QueryClient, caseId: string): void {
  const byCase = [
    ApiQueryKeys.CSM_CASE_COMMENTS,
    ApiQueryKeys.CSM_CASE_ACTIVITIES,
    ApiQueryKeys.CSM_CASE_DETAIL,
    ApiQueryKeys.WORK_ITEM_SLAS,
  ];
  for (const key of byCase) {
    void queryClient.invalidateQueries({ queryKey: [key, caseId] });
  }
}

/**
 * A comments/activities query fetched this recently is not revalidated when
 * its case id (re)joins the stream: a first-time open has just fetched it, and
 * refetching would double the two heaviest case queries for nothing. Sits above
 * the stream's id-set debounce (300ms) plus a typical fetch time.
 */
export const RECENT_FETCH_MS = 2_000;

/**
 * Revalidates only the comments and activities of a case whose id has just
 * (re)joined the subscribed stream set. While a case is not subscribed it
 * receives no frames, so cached comments/activities can be missing updates
 * that landed in the gap, yet still be inside CASE_QUERY_STALE_TIME_MS when the
 * case view remounts.
 *
 * Only a query that holds cached data older than RECENT_FETCH_MS is
 * invalidated. One with no data yet (never fetched, or still on its first
 * fetch) or fetched moments ago is already current.
 */
export function invalidateCaseThreadById(queryClient: QueryClient, caseId: string): void {
  for (const key of [ApiQueryKeys.CSM_CASE_COMMENTS, ApiQueryKeys.CSM_CASE_ACTIVITIES]) {
    const queryKey = [key, caseId];
    const { dataUpdatedAt = 0 } = queryClient.getQueryState(queryKey) ?? {};
    if (dataUpdatedAt === 0 || Date.now() - dataUpdatedAt < RECENT_FETCH_MS) continue;
    void queryClient.invalidateQueries({ queryKey });
  }
}

/**
 * Invalidates the case-scoped list-shaped queries by prefix, since they carry
 * filter params the stream layer does not have. Only mounted queries refetch,
 * the rest are just marked stale.
 *
 * - CSM_CASES: every list/search/quick-search/dashboard-card variant.
 * - CSM_CASE_CHILDREN: keyed by the *parent* id, which the stream layer does
 *   not know (the updated case may be a child row on its parent's page).
 */
export function invalidateCaseLists(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASES] });
  void queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CASE_CHILDREN] });
}

/** Everything a single `case_updated` event for `caseId` must refetch. */
export function invalidateCaseQueries(queryClient: QueryClient, caseId: string): void {
  invalidateCaseById(queryClient, caseId);
  invalidateCaseLists(queryClient);
}

