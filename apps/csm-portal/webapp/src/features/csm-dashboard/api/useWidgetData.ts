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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeWidgetResourceType, BeWidgetShape } from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import {
  hasTeamPlaceholder,
  resolveTeamPlaceholder,
} from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  hasCurrentUserPlaceholder,
  resolveCurrentUserPlaceholder,
} from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";
import {
  shouldRetryWidgetFetch,
  widgetFetchQueueDepth,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";

/** Default number of rows fetched for a `shape: "list"` widget when the
 * template doesn't set its own `listLimit`. */
const DEFAULT_LIST_LIMIT = 4;

export interface WidgetData {
  /** Total matching records — what a `shape: "count"` tile renders. */
  total: number;
  /** The resolved page of records — what a `shape: "list"` tile renders. */
  items: Record<string, unknown>[];
}

/**
 * Computes a widget query's `refetchInterval`. Returns `undefined` (no
 * auto-refetch) unless the caller set an interval — only the CS Overview
 * wallboard does. When an interval IS set, it's suppressed (`false`) while
 * the shared widget-fetch queue still has work in it: the wallboard's ~18
 * tiles each poll on their own timer through a concurrency-1 queue, so
 * without this a slow backend lets each tick stack another full poll wave
 * onto the one still draining until the queue never clears. Skipping a tick
 * while `queueDepth > 0` lets the current wave finish; the tile
 * re-evaluates on its next render (the wallboard re-renders every 60s off
 * `useDashboard`'s own poll) and resumes once the queue is idle. Under a
 * healthy backend the queue empties seconds into each minute, so this never
 * fires.
 */
export function resolveWidgetRefetchInterval(
  refetchIntervalMs: number | undefined,
  queueDepth: number,
): number | false | undefined {
  if (refetchIntervalMs === undefined) return undefined;
  return queueDepth > 0 ? false : refetchIntervalMs;
}

/**
 * Arguments to {@link useWidgetData}. An options object rather than a
 * positional list — the parameter set grew past a dozen (several of them
 * the same optional string/`string | string[]` type), so a positional call
 * both forced `undefined` placeholders to reach a later arg and let two
 * same-typed args be transposed with no type error.
 */
export interface UseWidgetDataOptions {
  widgetId: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  shape: BeWidgetShape;
  listLimit?: number;
  /** Row offset for a `shape: "list"` widget — only meaningful there, and
   * only actually used by `DashboardWidgetPreviewPage`'s pagination; the
   * compact tile always fetches from the start. Defaults to `0`. */
  offset?: number;
  /** Set to `false` to defer the query — used by `DashboardWidgetPreviewPage`
   * while it's still waiting to resolve a `@me`-style filter sentinel (see
   * `widgetPreviewUrl.ts`) into the signed-in user's real id, so a request
   * never goes out with the literal placeholder still in it. Defaults to
   * `true`. */
  enabled?: boolean;
  /** The currently selected team's own `creGroupId`, or an array of every
   * team's `creGroupId` in the current dashboard's family for the "All
   * ABTs" option (see `ALL_TEAMS_SENTINEL`), used to resolve a case
   * widget's `__current_team__` filter placeholder for a `creTeam` filter
   * entry (see `teamFilterPlaceholder.ts`) before it's sent. `undefined`
   * for a non-team-based dashboard, or while the team isn't resolved yet —
   * in which case any `creTeam` entry carrying that placeholder is dropped
   * rather than sent literally. */
  selectedTeamCreGroupId?: string | string[];
  /** The currently selected team's own `sreGroupId`, or an array of every
   * team's `sreGroupId` in the current dashboard's family for the "All
   * ABTs" option — the `sreTeam`-filter counterpart of
   * {@link selectedTeamCreGroupId}, resolved independently. `undefined` in
   * the same cases `selectedTeamCreGroupId` is. */
  selectedTeamSreGroupId?: string | string[];
  /** Only meaningful for shape "list". Opaque sort criteria (see
   * `BeDashboardWidget.sortBy`), forwarded verbatim as this search
   * request's own `sortBy` — same passthrough philosophy as `filters`. The
   * widget config is responsible for a field name valid for that
   * resourceType's own search contract. */
  sortBy?: Record<string, unknown>;
  /** The signed-in user's own platform id (`useCurrentUser().user.id`), used
   * to resolve a widget's `__current_user__` filter placeholder (see
   * `currentUserFilterPlaceholder.ts`) before it's sent — `undefined` while
   * the user profile hasn't loaded yet, in which case any filter entry
   * carrying that placeholder is dropped rather than sent literally. */
  currentUserId?: string;
  /** Background auto-refetch interval in milliseconds — `undefined` (the
   * default) means no auto-refetch at all, the behavior every existing
   * caller had before this parameter existed. Only the CS Overview
   * dashboard's own tiles (`WallboardStatTile`/`WallboardSecondaryStat`,
   * via `useWallboardTileData`) pass a value here, to match the reference
   * wallboard implementation's 60s refresh; `DashboardWidgetTile` and every
   * other caller leaves it unset. */
  refetchIntervalMs?: number;
}

/**
 * Resolves one dashboard widget's own data, independently of any sibling
 * widget on the same dashboard: a single `POST` to that `resourceType`'s own
 * search endpoint (see `WIDGET_RESOURCE_CONFIG`) with the widget's own
 * filters. Always reads both `total` and the item page off the response —
 * a `count`-shape widget only needs `total`, a `list`-shape widget only
 * needs `items`, but fetching both from the one call keeps this a single
 * code path instead of two near-identical ones.
 */
export function useWidgetData({
  widgetId,
  resourceType,
  filters,
  shape,
  listLimit,
  offset = 0,
  enabled = true,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  sortBy,
  currentUserId,
  refetchIntervalMs,
}: UseWidgetDataOptions): UseQueryResult<WidgetData, Error> {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG[resourceType];
  const limit = shape === "list" ? (listLimit ?? DEFAULT_LIST_LIMIT) : 1;
  const effectiveOffset = shape === "list" ? offset : 0;
  const resolvedFilters = resolveCurrentUserPlaceholder(
    resolveRelativeDateFilters(
      resolveTeamPlaceholder(filters, selectedTeamCreGroupId, selectedTeamSreGroupId),
    ),
    currentUserId,
  );
  const effectiveSortBy = shape === "list" ? sortBy : undefined;
  // A `__current_user__` placeholder that survived resolution means the
  // signed-in user's profile hasn't landed yet. Sending these filters would
  // either 400 on the non-UUID value or — before this resolver started
  // failing closed — silently widen a user-scoped widget to every user's
  // records. Hold the request instead; the query re-enables itself on the
  // render after `/users/me` resolves.
  const awaitingCurrentUser = hasCurrentUserPlaceholder(resolvedFilters);
  // Derived, not threaded down as its own prop: a stable serialization of
  // both team-group-id params already reaching this hook, so
  // `withWidgetFetchSlot` can drop this widget's queued fetch when the
  // selected team changes out from under it (see
  // widgetFetchConcurrency.ts's own `teamKey` doc). Constant for a
  // non-team-based dashboard (both undefined) — never drops anything
  // there.
  const teamKey = JSON.stringify([selectedTeamCreGroupId, selectedTeamSreGroupId]);
  // Whether THIS widget's own filters reference `__current_team__` — see
  // `shouldRetryWidgetFetch`'s own doc comment for why a queue-drop retry
  // is only worth anything when they don't (team-independent). Derived
  // from the raw, pre-resolution `filters`, not `resolvedFilters` — by the
  // time resolution runs the placeholder is already gone.
  const isTeamIndependent = !hasTeamPlaceholder(filters);

  return useQuery<WidgetData, Error>({
    queryKey: [
      ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
      widgetId,
      resourceType,
      resolvedFilters,
      limit,
      effectiveOffset,
      effectiveSortBy,
    ],
    enabled: enabled && !awaitingCurrentUser,
    // Self-throttling: a wallboard tile whose interval fires while the
    // shared fetch queue is still draining the previous wave skips that
    // cycle rather than piling on (see `resolveWidgetRefetchInterval`).
    refetchInterval: () =>
      resolveWidgetRefetchInterval(refetchIntervalMs, widgetFetchQueueDepth()),
    queryFn: async (): Promise<WidgetData> => {
      if (!config) {
        // A widget's resourceType came back from the backend (now a
        // runtime-configurable registry, not a compile-time-checked Go
        // literal) with no matching entry here — fail this widget's query
        // rather than crash on the property accesses below.
        throw new Error(`Unsupported widget resourceType: ${resourceType}`);
      }
      // Gated behind a shared concurrency slot (see widgetFetchConcurrency.ts)
      // so an N-widget dashboard doesn't fire N simultaneous searches at
      // customer-entity-service — the search call itself, not this
      // queryFn's synchronous config check above, is what actually hits
      // the network. The provided `signal` is wired to `api.post`'s own
      // `signal` option so widgetFetchConcurrency's own timeout can
      // actually abort this specific in-flight request, not just start a
      // timer nothing observes.
      return withWidgetFetchSlot(async (signal) => {
        // Most resourceTypes' own search contract takes
        // `{filters, pagination:{offset,limit}, sortBy?}` and returns
        // `{total, [itemsKey]: [...]}` — `config.buildSearchRequestBody`/
        // `config.parseSearchResponse` exist only for the resourceType(s)
        // whose real contract diverges from that (today: `case_feedback`'s
        // flat `page`/`pageSize` request and `totalRecords`/`results`
        // response — see `WidgetResourceConfig`'s own doc comments). Both
        // are omitted for every other resourceType, so this stays the exact
        // request/response shape it always was for them.
        const body = config.buildSearchRequestBody
          ? config.buildSearchRequestBody({
              filters: resolvedFilters,
              offset: effectiveOffset,
              limit,
              sortBy: effectiveSortBy,
            })
          : {
              filters: resolvedFilters,
              pagination: { offset: effectiveOffset, limit },
              ...(effectiveSortBy ? { sortBy: effectiveSortBy } : {}),
            };
        const res = await api.post<Record<string, unknown>, Record<string, unknown>>(
          config.searchEndpoint,
          body,
          { signal },
        );
        if (config.parseSearchResponse) {
          return config.parseSearchResponse(res);
        }
        const total = typeof res.total === "number" ? res.total : 0;
        const rawItems = res[config.itemsKey];
        const items = Array.isArray(rawItems)
          ? (rawItems as Record<string, unknown>[])
          : [];
        return { total, items };
      }, teamKey);
    },
    // Explicit per-query retry (not inherited from AppWithConfig's global
    // default) so a widget whose fetch timed out gets one retry — see
    // shouldRetryWidgetFetch's own doc comment for why a timeout must not
    // be a same-tick terminal failure, and why the retry needs no separate
    // "back of the queue" bookkeeping of its own. Wrapped rather than
    // passed directly: react-query's own `retry` option only ever calls
    // the 2-arg `(failureCount, error)` form, so `isTeamIndependent` has to
    // be closed over here instead of threaded through react-query itself.
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, isTeamIndependent),
    staleTime: 300_000,
  });
}
