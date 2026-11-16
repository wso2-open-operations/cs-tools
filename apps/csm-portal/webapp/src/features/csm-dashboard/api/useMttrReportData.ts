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
import type { BeDashboardGroupByConfig, BeMttrReport, BeMttrReportBand } from "@api/backend/types";
import {
  shouldRetryWidgetFetch,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";
import type { PieSliceResult, WidgetPieData } from "@features/csm-dashboard/api/useWidgetPieData";

/** The order every `GET /reports/mttr` response's bars render in — matches
 * the band order the backend itself returns, kept explicit here so a bar
 * chart's left-to-right order never depends on object key enumeration. */
const BAND_ORDER: readonly string[] = ["S1-S4", "S1-S3", "S0", "S1", "S2", "S3", "S4"];

function bandSortIndex(band: string): number {
  const i = BAND_ORDER.indexOf(band);
  return i === -1 ? BAND_ORDER.length : i;
}

/**
 * Resolves a `shape: "bar"` `mttr` widget's severity-band data via a
 * single `GET /reports/mttr` call — the `resourceType: "mttr"` counterpart
 * of `useCaseFeedbackTrendData` (see that hook's own doc comment for why a
 * `groupBy.bucket` widget needs a dedicated hook rather than a branch inside
 * `useWidgetGroupByData`: this endpoint's request shape, a `GET` with query
 * params, and response shape (`{bands: [{band, caseCount,
 * avgResolutionHours}], ...}`) are both unrelated to `BeGroupByResponse`).
 *
 * Renders **average resolution hours per band** as the primary metric
 * (`PieSliceResult.value`) — there is no count-based alternative mode the
 * way `case_feedback`'s "rating"/"reasons_*" buckets have, since every mttr
 * widget's one bucket is always "severity". `total` is how many cases
 * the shown bars cover (the shown individual severities' `caseCount`s
 * added up), the closest analogue to `case_feedback`'s `totalRecords` for
 * the tile's header badge.
 *
 * No slice is click-through navigable: unlike a date or rating bucket, a
 * severity band has no existing case-list filter that corresponds to "cases
 * that last entered Solution Proposed in this window" — the same
 * `navigable: false` reasoning `useCaseFeedbackTrendData` applies to its own
 * "reasons_*" buckets.
 *
 * `groupBy` of `undefined` (not an mttr widget, or one with no
 * `groupBy.bucket` — never true in practice, since `DashboardWidgetTile`
 * only calls this hook for `resourceType: "mttr"` widgets with
 * `groupBy.bucket` set) fires no query and returns an empty/zero result,
 * mirroring `useCaseFeedbackTrendData`'s own behavior.
 */
export function useMttrReportData(
  widgetId: string,
  /** This widget's own flat `query` (`months` or `days`, `creTeamIds`) -- already
   * resolved by the caller the same way every other widget's filters are. */
  baseFilters: Record<string, unknown>,
  groupBy: BeDashboardGroupByConfig | undefined,
  /** The dashboard's currently-selected team (the ABT header toggle), or an
   * array for "All ABTs" -- same arguments `useWidgetGroupByData`/
   * `useWidgetPieData` already take. An mttr widget's own data never
   * depends on this (its `creTeamIds` is a fixed, generation-time filter,
   * not this dynamic selection), but the shared fetch queue
   * (`withWidgetFetchSlot`) keys solely on whatever string is passed as its
   * `teamKey` and evicts every OTHER queued widget whenever that string
   * changes from the previous call's (see `acquireWidgetFetchSlot`) --
   * passing the same derived key every team-scoped widget on this page
   * already passes is what keeps an mttr tile from spuriously dropping (or
   * being dropped by) them. */
  selectedTeamCreGroupId: string | string[] | undefined,
  selectedTeamSreGroupId: string | string[] | undefined,
  enabled = true,
): WidgetPieData {
  const api = useBackendApi();

  const months = typeof baseFilters.months === "string" ? baseFilters.months : undefined;
  // The trailing window is `months` or `days` (7), never both.
  const days = !months && typeof baseFilters.days === "string" ? baseFilters.days : undefined;
  const creTeamIds = Array.isArray(baseFilters.creTeamIds)
    ? (baseFilters.creTeamIds.filter((v): v is string => typeof v === "string") as string[])
    : [];
  // Optional: which bars to draw, in the order listed. The one report call
  // always answers with all six bands; this only narrows what is shown, so it
  // is deliberately not part of the query key.
  const wantedBands = Array.isArray(baseFilters.bands)
    ? baseFilters.bands.filter((v): v is string => typeof v === "string")
    : [];
  const active = !!groupBy?.bucket && (!!months || !!days);
  const teamKey = JSON.stringify([selectedTeamCreGroupId, selectedTeamSreGroupId]);

  const query = useQuery({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA, "mttr-report", widgetId, months, days, creTeamIds],
    queryFn: async (): Promise<BeMttrReport | null> => {
      if (!months && !days) return null;
      return withWidgetFetchSlot(async (signal) => {
        const params = new URLSearchParams(months ? { months } : { days: days as string });
        for (const id of creTeamIds) params.append("creTeamId", id);
        const response = await api.getResponse(`/reports/mttr?${params.toString()}`, { signal });
        return (await response.json()) as BeMttrReport;
      }, teamKey);
    },
    enabled: enabled && active,
    // Same reasoning as `useCaseFeedbackTrendData`: this resourceType has no
    // team/current-user placeholder of its own in its filters (creTeamIds is
    // already resolved to real group ids at generation time, not a
    // `__current_team__` sentinel resolved per render), so every fetch here
    // is "team-independent" in `shouldRetryWidgetFetch`'s own sense.
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, true),
    staleTime: 60_000,
  });

  const isLoading = !enabled || (active && query.isLoading);
  const isError = active && query.isError;

  const bands: BeMttrReportBand[] = query.data?.bands ?? [];
  const shown =
    wantedBands.length > 0 ? bands.filter((b) => wantedBands.includes(b.band)) : bands;
  const sorted = [...shown].sort((a, b) => bandSortIndex(a.band) - bandSortIndex(b.band));
  const slices: PieSliceResult[] = sorted.map((b) => ({
    label: b.band,
    query: {},
    navigable: false,
    value: b.avgResolutionHours,
  }));
  // How many cases the shown bars cover: the individual severities shown,
  // added up (S1-S4/S1-S3 are sums of those, so counting them too would count
  // a case twice); only when just a combined bar is shown, that bar's own.
  const shownSingles = shown.filter((b) => b.band !== "S1-S4" && b.band !== "S1-S3");
  const total =
    shownSingles.length > 0
      ? shownSingles.reduce((n, b) => n + b.caseCount, 0)
      : (shown[0]?.caseCount ?? 0);

  return { slices, total, isLoading, isError };
}
