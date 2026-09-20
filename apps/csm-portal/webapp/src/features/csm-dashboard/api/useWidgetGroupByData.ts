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
 * Resource types whose entity-service search contract accepts the generic
 * `{field, op, values}` filter-array shape for a groupBy-able field, in
 * addition to whatever `usesCaseFieldFilterDsl`/`CASE_FIELD_DSL_RESOURCE_TYPES`
 * already covers. Deliberately a *separate*, narrower set kept local to
 * `bucketQuery` rather than folded into `usesCaseFieldFilterDsl` itself:
 * that shared set also drives which of the admin dashboard builder's filter
 * operators get offered per resourceType, and incident/problem's own
 * entity-service field allowlists (`incidentFilterFieldSet`/
 * `problemFilterFieldSet` in cs-tools/entity-service) are much narrower than
 * case's — folding them into the shared set would let the builder offer
 * filter combinations neither backend actually accepts. `case`/
 * `service_request`/`security_report_analysis`/`announcement`/`engagement`
 * (the `usesCaseFieldFilterDsl` set) don't need to be repeated here.
 */
const GROUP_BY_FILTER_ARRAY_RESOURCE_TYPES = new Set<BeWidgetResourceType>(["incident", "problem"]);

/**
 * Builds a named bucket's own click-through `query` — the same shape
 * `DashboardWidgetTile`'s slice navigation merges under the widget's base
 * `query` via `mergeWidgetFilters`. The entity-service's search contract for
 * a groupBy-able field genuinely differs by resourceType (confirmed live
 * against real 400s, not just by reading the Go source), so this has three
 * branches:
 *
 * 1. `usesCaseFieldFilterDsl(resourceType)` (`case`/`service_request`/
 *    `security_report_analysis`/`announcement`/`engagement`) plus
 *    `incident`/`problem` (see `GROUP_BY_FILTER_ARRAY_RESOURCE_TYPES` above):
 *    all of these validate a field like "state" through the same generic
 *    `{field, op, values}` predicate array, and every one of them only
 *    accepts `op: "in"` for "state" — never "eq" (`case_filters.go`'s,
 *    `incident_filters.go`'s, and `problem_filters.go`'s own
 *    `badXxxFilterCombo` checks all reject "state"/"eq"). A single-element
 *    `values` array with `op: "in"` is the correct, always-safe way to
 *    express "equals this one value" against any of these contracts.
 * 2. `change_request`: has no generic filter-array entry for "state" at
 *    all — `ChangeRequestFieldFilter`'s own allowed field set
 *    (`createdOn`/`assignmentGroupId`/`approval`) excludes it. Change
 *    request state filtering is instead a bespoke, plural, array-valued
 *    top-level field: `states: [key]` (see
 *    `SearchChangeRequestsFilters.States`, a sibling of `Filters`, not an
 *    entry inside it).
 * 3. Everything else (e.g. `call_request`'s own flat scalar fields, if any
 *    groupBy widget ever uses them): falls back to a flat `{ [field]: key }`,
 *    unchanged from before.
 */
function bucketQuery(
  resourceType: BeWidgetResourceType,
  field: string,
  key: string,
): Record<string, unknown> {
  if (usesCaseFieldFilterDsl(resourceType) || GROUP_BY_FILTER_ARRAY_RESOURCE_TYPES.has(resourceType)) {
    return { filters: [{ field, op: "in", values: [key] }] };
  }
  if (resourceType === "change_request" && field === "state") {
    return { states: [key] };
  }
  return { [field]: key };
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
      // `groupBy.field` is only set for field-based grouping — a
      // `groupBy.bucket` widget (date-bucketed grouping; see that field's own
      // doc comment on `BeDashboardGroupByConfig`) never reaches this hook at
      // all: `DashboardWidgetTile` calls `useCaseFeedbackTrendData` instead,
      // whose own `POST /cases/feedback/aggregate` request/response shape
      // this hook's `groupBy`/`BeGroupByResponse` types don't match. This
      // check is therefore normally dead — it exists only so a
      // misconfigured/future caller that reaches here with a bucket-only
      // `groupBy` fails loudly instead of posting `groupBy: undefined`
      // silently.
      if (!groupBy.field) {
        throw new Error(
          `useWidgetGroupByData: groupBy carries no "field" (bucket-based grouping belongs to useCaseFeedbackTrendData, not this hook) for widget "${widgetId}"`,
        );
      }
      const field = groupBy.field;
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
            groupBy: field,
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
    staleTime: 300_000,
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
    query: groupBy?.field ? bucketQuery(resourceType, groupBy.field, bucket.key) : {},
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
