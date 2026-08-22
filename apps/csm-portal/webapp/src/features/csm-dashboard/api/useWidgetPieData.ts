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

import { useQueries } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeDashboardPieSlice, BeWidgetResourceType } from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { mergeWidgetFilters } from "@features/csm-dashboard/utils/widgetFilterMerge";
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
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";

export interface PieSliceResult extends BeDashboardPieSlice {
  value: number;
  /** Defaults to `true` (every hand-authored slice is click-through by
   * design). `false` only for a groupBy widget's synthetic "Others" bucket
   * when the API gives it no safe selector of its own to navigate to (see
   * `useWidgetGroupByData`) — `DashboardWidgetTile` skips click-through for
   * a slice carrying this. */
  navigable?: boolean;
}

export interface WidgetPieData {
  slices: PieSliceResult[];
  total: number;
  isLoading: boolean;
  isError: boolean;
}

/**
 * Resolves a `shape: "pie"` widget's per-slice values: one independent
 * `POST {resourceType}/search` per slice — its own `query` merged under
 * the widget's own base `query` (slice keys win on conflict) — with
 * `pagination: { limit: 1 }`, reading `total` off each. The exact same
 * mechanism `shape: "count"` (see `useWidgetData`) uses, just fired once
 * per slice instead of once for the whole widget. An empty `slices` array
 * fires no queries at all.
 */
export function useWidgetPieData(
  widgetId: string,
  resourceType: BeWidgetResourceType,
  baseFilters: Record<string, unknown>,
  slices: BeDashboardPieSlice[],
  /** The currently selected team's own `creGroupId`, or an array of every
   * team's `creGroupId` in the current dashboard's family for the "All
   * ABTs" option (see `ALL_TEAMS_SENTINEL`), for resolving a
   * `__current_team__` filter placeholder for a `creTeam` filter entry (see
   * `teamFilterPlaceholder.ts`) — applied AFTER `mergeWidgetFilters`, since
   * a slice's own `query` may carry the placeholder too, not just the
   * widget's base `query`. */
  selectedTeamCreGroupId?: string | string[],
  /** The currently selected team's own `sreGroupId`, or an array of every
   * team's `sreGroupId` in the current dashboard's family for the "All
   * ABTs" option — the `sreTeam`-filter counterpart of
   * {@link selectedTeamCreGroupId}, resolved independently, applied the
   * same AFTER `mergeWidgetFilters`. */
  selectedTeamSreGroupId?: string | string[],
  /** The signed-in user's own platform id (`useCurrentUser().user.id`), for
   * resolving a `__current_user__` filter placeholder (see
   * `currentUserFilterPlaceholder.ts`) — applied AFTER `mergeWidgetFilters`,
   * same as `selectedTeamCreGroupId`/`selectedTeamSreGroupId`, since a
   * slice's own `query` may carry the placeholder too, not just the
   * widget's base `query`. */
  currentUserId?: string,
  /** Set to `false` to hold every slice query without firing it — used by
   * `DashboardWidgetTile` to defer a pie/bar widget's fetch until its tile
   * has actually scrolled into (or near) the viewport (see
   * `useElementVisibleOnce`). Defaults to `true` (fires immediately, same
   * as before this parameter existed) so `DashboardWidgetPreviewPage`-style
   * callers that always render one widget full-page — never lazily — don't
   * need to pass anything. */
  enabled = true,
): WidgetPieData {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG[resourceType];

  const resolvedSliceFilters = slices.map((slice) =>
    resolveCurrentUserPlaceholder(
      resolveRelativeDateFilters(
        resolveTeamPlaceholder(
          mergeWidgetFilters(baseFilters, slice.query),
          selectedTeamCreGroupId,
          selectedTeamSreGroupId,
        ),
      ),
      currentUserId,
    ),
  );
  // See useWidgetData: a surviving `__current_user__` means the signed-in
  // user's profile hasn't landed yet, so every slice holds rather than
  // searching unscoped. Computed here rather than inside the query factory
  // so the returned isLoading can report the wait — a disabled query is not
  // "loading" to react-query, and a pie/bar tile must not paint an empty
  // chart while it is really still waiting on identity.
  const awaitingCurrentUser = resolvedSliceFilters.some(hasCurrentUserPlaceholder);
  // See useWidgetData's own comment — same derivation, same reasoning.
  const teamKey = JSON.stringify([selectedTeamCreGroupId, selectedTeamSreGroupId]);
  // Whether EACH slice's own merged filters (base `query` + that slice's
  // own `query`, pre-resolution — a slice's own `query` may carry the
  // placeholder even when the widget's base `query` doesn't) reference
  // `__current_team__` — see useWidgetData's own comment for why this must
  // be computed off the raw filters, not the resolved ones, and
  // `shouldRetryWidgetFetch`'s own doc comment for how it's used.
  const sliceIsTeamIndependent = slices.map(
    (slice) => !hasTeamPlaceholder(mergeWidgetFilters(baseFilters, slice.query)),
  );

  const queries = useQueries({
    queries: slices.map((_slice, index) => {
      const filters = resolvedSliceFilters[index];
      const isTeamIndependent = sliceIsTeamIndependent[index];
      return {
        queryKey: [
          ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
          "pie-slice",
          widgetId,
          resourceType,
          filters,
        ],
        queryFn: async (): Promise<number> => {
          if (!config) {
            throw new Error(`Unsupported widget resourceType: ${resourceType}`);
          }
          // Same shared concurrency slot (and timeout) useWidgetData's
          // search uses — a pie widget fires one call per slice on top of
          // every other widget's own call, so it needs both at least as
          // much.
          return withWidgetFetchSlot(async (signal) => {
            const res = await api.post<
              { filters: Record<string, unknown>; pagination: { offset: number; limit: number } },
              Record<string, unknown>
            >(
              config.searchEndpoint,
              {
                filters,
                pagination: { offset: 0, limit: 1 },
              },
              { signal },
            );
            return typeof res.total === "number" ? res.total : 0;
          }, teamKey);
        },
        enabled: enabled && !awaitingCurrentUser,
        // Same per-query retry override as useWidgetData, same reasoning
        // (see shouldRetryWidgetFetch) — a pie/bar slice fetch that timed
        // out gets one retry too. Wrapped for the same reason useWidgetData
        // wraps it — react-query's own `retry` option only calls the 2-arg
        // form.
        retry: (failureCount: number, error: Error) =>
          shouldRetryWidgetFetch(failureCount, error, isTeamIndependent),
        staleTime: 60_000,
      };
    }),
  });

  // `!enabled` (still waiting to scroll into view) reports as loading
  // rather than as react-query's own `isLoading` for a disabled query
  // (which is `false` — a query that never started isn't "loading" to
  // react-query) — this hook's own `isLoading` is a widget-level "don't
  // paint real data yet" signal, not a passthrough of query-fetch state.
  const isLoading = !enabled || awaitingCurrentUser || queries.some((q) => q.isLoading);
  const isError = queries.some((q) => q.isError);
  const results: PieSliceResult[] = slices.map((slice, i) => ({
    ...slice,
    value: queries[i]?.data ?? 0,
  }));
  const total = results.reduce((sum, s) => sum + s.value, 0);

  return { slices: results, total, isLoading, isError };
}
