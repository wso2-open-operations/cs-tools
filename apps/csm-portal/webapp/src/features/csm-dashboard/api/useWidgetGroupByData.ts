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

import { useQuery } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeDashboardGroupByConfig,
  BeGroupByResponse,
  BeWidgetResourceType,
} from "@api/backend/types";
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
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";
import { usesCaseFieldFilterDsl } from "@features/csm-admin/dashboards/utils/widgetQueryConditions";
import type { PieSliceResult, WidgetPieData } from "@features/csm-dashboard/api/useWidgetPieData";

/**
 * Builds a named bucket's own click-through `query` — the same shape
 * `DashboardWidgetTile`'s slice navigation merges under the widget's base
 * `query` via `mergeWidgetFilters` (see that function's own doc comment for
 * why the two resourceType families below are handled differently). A
 * `case`-DSL resourceType's search contract has no flat top-level key for
 * an arbitrary aggregated field, so it goes through the generic
 * `field`/`op`/`values` predicate array; every other resourceType's own
 * bespoke search contract keys straight off the field name.
 */
function bucketQuery(
  resourceType: BeWidgetResourceType,
  field: string,
  key: string,
): Record<string, unknown> {
  return usesCaseFieldFilterDsl(resourceType)
    ? { filters: [{ field, op: "eq", values: [key] }] }
    : { [field]: key };
}

/**
 * Resolves a `shape: "pie"` widget's per-bucket values via a single
 * server-side `POST {resourceType}/aggregate` call — the `groupBy`
 * counterpart of `useWidgetPieData`'s per-slice `search` calls. The
 * widget's own base `query` is resolved through the exact same
 * placeholder pipeline `useWidgetPieData` applies per-slice
 * (`resolveTeamPlaceholder` -> `resolveRelativeDateFilters` ->
 * `resolveCurrentUserPlaceholder`), just once — there's no per-slice
 * query to merge it under, since there are no slices. A `groupBy` of
 * `undefined` (widget doesn't use this mode) fires no query at all and
 * returns an empty/zero result, mirroring `useWidgetPieData`'s own
 * behavior for an empty `slices` array.
 */
export function useWidgetGroupByData(
  widgetId: string,
  resourceType: BeWidgetResourceType,
  baseFilters: Record<string, unknown>,
  groupBy: BeDashboardGroupByConfig | undefined,
  /** See `useWidgetPieData`'s own doc comment for each of these — same
   * parameters, same resolution order, same reasoning. */
  selectedTeamCreGroupId?: string | string[],
  selectedTeamSreGroupId?: string | string[],
  currentUserId?: string,
  enabled = true,
): WidgetPieData {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG[resourceType];

  const resolvedFilters = resolveCurrentUserPlaceholder(
    resolveRelativeDateFilters(
      resolveTeamPlaceholder(baseFilters, selectedTeamCreGroupId, selectedTeamSreGroupId),
    ),
    currentUserId,
  );
  // See useWidgetPieData: a surviving `__current_user__` means the
  // signed-in user's profile hasn't landed yet, so the query holds rather
  // than searching unscoped.
  const awaitingCurrentUser = hasCurrentUserPlaceholder(resolvedFilters);
  // See useWidgetData's own comment — same derivation, same reasoning.
  const teamKey = JSON.stringify([selectedTeamCreGroupId, selectedTeamSreGroupId]);
  // See useWidgetData's own comment — same derivation (off the raw,
  // pre-resolution `baseFilters`), same reasoning; there's no per-slice
  // query to merge under here, so this is the widget's whole filters
  // object, same as useWidgetData.
  const isTeamIndependent = !hasTeamPlaceholder(baseFilters);

  const query = useQuery({
    queryKey: [
      ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
      "group-by",
      widgetId,
      resourceType,
      resolvedFilters,
      groupBy,
    ],
    queryFn: async (): Promise<BeGroupByResponse> => {
      if (!groupBy) {
        return { groups: [], othersCount: 0, totalRecords: 0 };
      }
      if (!config?.groupByEndpoint) {
        throw new Error(`Unsupported group-by widget resourceType: ${resourceType}`);
      }
      // Same shared concurrency slot (and timeout) useWidgetPieData's own
      // slice fetches use — a groupBy widget fires one call on top of every
      // other widget's own call, so it needs both at least as much.
      return withWidgetFetchSlot(async (signal) => {
        return api.post<
          { filters: Record<string, unknown>; groupBy: string; maxGroups?: number },
          BeGroupByResponse
        >(
          config.groupByEndpoint as string,
          {
            filters: resolvedFilters,
            groupBy: groupBy.field,
            maxGroups: groupBy.maxGroups,
          },
          { signal },
        );
      }, teamKey);
    },
    enabled: enabled && !!groupBy && !awaitingCurrentUser,
    // Same per-query retry override as useWidgetPieData's own slice
    // fetches, same reasoning (see shouldRetryWidgetFetch). Wrapped for the
    // same reason useWidgetData wraps it — react-query's own `retry` option
    // only calls the 2-arg form.
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, isTeamIndependent),
    staleTime: 60_000,
  });

  // Mirrors useWidgetPieData's own isLoading semantics: `!enabled` or a
  // still-unresolved current-user placeholder reports as loading rather
  // than passing through react-query's own `isLoading` for a disabled
  // query (which is `false`).
  const isLoading = !enabled || (!!groupBy && awaitingCurrentUser) || (!!groupBy && query.isLoading);
  const isError = !!groupBy && query.isError;

  const buckets = query.data?.groups ?? [];
  const othersCount = query.data?.othersCount ?? 0;

  const slices: PieSliceResult[] = buckets.map((bucket) => ({
    label: bucket.label,
    // Scopes this slice's own click-through to exactly this bucket (see
    // `bucketQuery`'s own doc comment) — an empty `query` here would merge
    // to the widget's unfiltered base result set instead of this bucket's,
    // since `mergeWidgetFilters({...}, {})` is a no-op.
    query: groupBy ? bucketQuery(resourceType, groupBy.field, bucket.key) : {},
    value: bucket.count,
  }));
  if (othersCount > 0) {
    slices.push({
      label: groupBy?.othersLabel ?? "Others",
      // Unlike a named bucket, "Others" has no `key` of its own — the
      // response only carries a rolled-up count, not a selector for
      // "everything not in a named bucket" this hook could turn into a
      // query. Leaving `query` empty would silently navigate to the
      // widget's unscoped base result set (the exact bug this hook exists
      // to fix for the named buckets above), so this slice is marked
      // non-navigable instead — `DashboardWidgetTile` skips its
      // click-through for a slice carrying this flag.
      query: {},
      navigable: false,
      value: othersCount,
    });
  }
  // Summed from the returned slices themselves (top-N plus the synthetic
  // Others entry) rather than taken from the response's own
  // `totalRecords` directly — same approach useWidgetPieData's own `total`
  // uses (sum of its returned slices), so this hook's `total` always stays
  // internally consistent with what it actually returns even if
  // `totalRecords` and (groups + othersCount) could ever disagree upstream.
  const total = slices.reduce((sum, s) => sum + s.value, 0);

  return { slices, total, isLoading, isError };
}
