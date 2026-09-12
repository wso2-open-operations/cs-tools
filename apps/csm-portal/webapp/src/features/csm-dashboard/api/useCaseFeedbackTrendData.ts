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
  BeCaseFeedbackAggregateResponse,
  BeCaseFeedbackBucket,
  BeDashboardGroupByConfig,
  BeWidgetPaletteColor,
} from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";
import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";
import {
  shouldRetryWidgetFetch,
  withWidgetFetchSlot,
} from "@features/csm-dashboard/utils/widgetFetchConcurrency";
import type { PieSliceResult, WidgetPieData } from "@features/csm-dashboard/api/useWidgetPieData";

const THREE_MONTHS_DAYS = 92;

/** Switches the trend widget's default "month" bucket to "week" when the
 * selected date range spans 3 months or less — a monthly bucket over a
 * short range collapses to just 1-3 bars, too coarse to be useful, while a
 * weekly bucket over a long range would produce far too many. Any other
 * bucket (an explicit "week"/"day", or a "rating"/"reasons_*" distribution)
 * is left untouched — this override only applies to the trend widget's own
 * default. An unbounded or partially-bounded range (either side missing)
 * also leaves "month" as-is, since the span can't be measured. */
function resolveEffectiveBucket(
  bucket: BeCaseFeedbackBucket | undefined,
  filters: Record<string, unknown>,
): BeCaseFeedbackBucket | undefined {
  if (bucket !== "month") return bucket;
  const dateFrom = filters.dateFrom;
  const dateTo = filters.dateTo;
  if (typeof dateFrom !== "string" || typeof dateTo !== "string") return bucket;
  const from = new Date(dateFrom);
  const to = new Date(dateTo);
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return bucket;
  const spanDays = (to.getTime() - from.getTime()) / 86_400_000;
  if (spanDays >= 0 && spanDays <= THREE_MONTHS_DAYS) return "week";
  return bucket;
}

/** Computes the `[dateFrom, dateTo]` (YYYY-MM-DD) range a date-bucket slice
 * covers, for click-through into the feedback list filtered to just that
 * bucket — the same date-only format the list widget's own date-range
 * filter already sends. `bucketStart` is UTC-midnight per
 * `_computeBucketStart`'s own contract (see `formatBucketLabel`'s doc
 * comment), so every date built here uses `Date.UTC`/`getUTCDate` to stay
 * on the same calendar day the bucket itself represents. */
function bucketDateRange(bucketStart: string, bucket: "day" | "week" | "month"): [string, string] {
  const start = new Date(bucketStart);
  if (Number.isNaN(start.getTime())) return [bucketStart, bucketStart];
  const toISODate = (d: Date): string => d.toISOString().slice(0, 10);
  if (bucket === "day") return [bucketStart, bucketStart];
  if (bucket === "week") {
    const end = new Date(start);
    end.setUTCDate(end.getUTCDate() + 6);
    return [bucketStart, toISODate(end)];
  }
  // "month": bucketStart is always the 1st (see _computeBucketStart) —
  // the last day of that month is the day before the 1st of the next one.
  const end = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + 1, 0));
  return [bucketStart, toISODate(end)];
}

/** Maps a bucket's average rating (1-5 scale) to a fixed red/amber/green
 * palette color instead of the bar chart's default per-index color
 * rotation — a rating trend has an inherent "better/worse" direction that a
 * rotating rainbow of unrelated palette colors doesn't communicate, and
 * would misleadingly suggest each bucket is a distinct unrelated category.
 * Thresholds mirror the underlying CSAT labels (1-2 = Dissatisfied/Very
 * Dissatisfied, 3 = Neutral, 4-5 = Satisfied/Very Satisfied). */
function colorForAvgRating(avgRating: number): BeWidgetPaletteColor {
  if (avgRating < 3) return "error";
  if (avgRating < 4) return "warning";
  return "success";
}

/** Formats one bucket's own `bucketStart` for the x-axis label, at the
 * granularity implied by `bucket` — "month" reads as "Aug 2026" (the day is
 * meaningless at that granularity), "week"/"day" as "Aug 1" (the year is
 * omitted; a trend chart spans at most a handful of months in practice, and
 * dropping it keeps the label short enough not to overlap its neighbors).
 * Falls back to the raw string for anything `Date` can't parse, rather than
 * rendering "Invalid Date" — the entity service is the source of this value,
 * not something validated client-side.
 *
 * Formatted in `timeZone: "UTC"`, deliberately NOT the viewer's own local
 * time (unlike `resolveRelativeDateFilters`, which is intentionally
 * local-time for a different reason — see that module's own doc comment): a
 * date-only `bucketStart` like `"2026-08-01"` parses as UTC midnight, and
 * reading it back in a negative-UTC-offset timezone (most of the Americas)
 * without pinning the formatter to UTC would roll it back to the previous
 * day/month on the label — e.g. "Jul 2026" for a bucket the response itself
 * calls August. Pinning to UTC makes the label match the bucket's own wire
 * value everywhere, regardless of the viewer's timezone. */
function formatBucketLabel(bucketStart: string, bucket: BeCaseFeedbackBucket): string {
  // "rating"/"reasons_*" modes group by a fixed label (rating name or reason
  // chip) instead of time: bucketStart is already that label, not a date to
  // parse.
  if (bucket === "rating" || bucket.startsWith("reasons_")) return bucketStart;
  const date = new Date(bucketStart);
  if (Number.isNaN(date.getTime())) return bucketStart;
  return new Intl.DateTimeFormat("en-US", {
    timeZone: "UTC",
    month: "short",
    year: bucket === "month" ? "numeric" : undefined,
    day: bucket === "month" ? undefined : "numeric",
  }).format(date);
}

/**
 * Resolves a `shape: "bar"`/`"pie"` `case_feedback` widget's bucketed data
 * via a single `POST /cases/feedback/aggregate` call — the `groupBy.bucket`
 * counterpart of `useWidgetGroupByData`'s `groupBy.field` (see that field's
 * own doc comment on `BeDashboardGroupByConfig` for why this is a dedicated
 * hook rather than a branch inside that one: the request body (`{filters,
 * bucket}`, no `groupBy`/`maxGroups`) and response shape (`{buckets:
 * [{bucketStart, avgRating, count}], totalRecords}`, no `groups`/
 * `othersCount`) are both unrelated to `BeGroupByResponse`).
 *
 * Renders **average rating per bucket** as the primary metric for date
 * buckets (this task's own stated default) — every returned slice's `value`
 * is `avgRating` (1-5), not `count`, in that mode. `count` is still read off
 * the response (as `WidgetPieData.total`, the tile's own header badge — see
 * `DashboardWidgetTile`), just not per-bucket. For "rating"/"reasons_*"
 * modes, `value` is `count` instead — see the slice-building code below.
 *
 * A date-bucket (day/week/month) or "rating" slice is click-through
 * navigable — into the feedback list's own dashboard-widget preview page
 * (`WIDGET_RESOURCE_CONFIG.case_feedback.buildHref`), filtered to that
 * bucket's own date range or rating value respectively. A "reasons_*" slice
 * stays `navigable: false`: which chip a customer picked isn't a field the
 * feedback list's search can filter by, so there's no safe selector to
 * scope to — see the slice-building code below for the full reasoning.
 *
 * `groupBy` of `undefined`, or one carrying no `bucket` (a field-based
 * `groupBy` reaching this hook by mistake — never true in practice, since
 * `DashboardWidgetTile` only calls this hook when `groupBy.bucket` is set),
 * fires no query and returns an empty/zero result, mirroring
 * `useWidgetGroupByData`'s own behavior for an undefined `groupBy`.
 */
export function useCaseFeedbackTrendData(
  widgetId: string,
  /** This widget's own base filters (`CaseFeedbackAggregateFilters` —
   * `accountIds`/`dateFrom`/`dateTo`; NOT the case-search DSL). Already
   * resolved for the `__dateRangeFrom__`/`__dateRangeTo__` placeholder by
   * the caller (`DashboardWidgetGrid`'s `renderTile` — see
   * `dateRangeFilterPlaceholder.ts`), the single merge point shared with the
   * `shape: "list"` grid widget that reads its own filters off the exact
   * same `widget.query`, so this hook only ever sees concrete values (or
   * none). Only `resolveRelativeDateFilters` runs here — a fail-open no-op
   * for this resourceType's own flat `dateFrom`/`dateTo` shape (it only
   * resolves the case-search DSL's `{filters: [...]}`  shape), kept for
   * parity with every other widget hook and as a forward-compatible no-op
   * should a relative-date placeholder ever be supported here too. */
  baseFilters: Record<string, unknown>,
  groupBy: BeDashboardGroupByConfig | undefined,
  enabled = true,
): WidgetPieData {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG.case_feedback;

  const resolvedFilters = resolveRelativeDateFilters(baseFilters);
  const bucket = resolveEffectiveBucket(groupBy?.bucket, resolvedFilters);

  const query = useQuery({
    queryKey: [
      ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA,
      "feedback-trend",
      widgetId,
      resolvedFilters,
      bucket,
    ],
    queryFn: async (): Promise<BeCaseFeedbackAggregateResponse> => {
      if (!bucket) {
        return { buckets: [], totalRecords: 0 };
      }
      if (!config?.groupByEndpoint) {
        throw new Error("case_feedback resourceType has no groupByEndpoint configured");
      }
      // Same shared concurrency slot (and timeout) every other widget fetch
      // uses — see useWidgetGroupByData's own comment. This resourceType has
      // no team concept at all (see the retry option's own comment below), so
      // a constant `teamKey` ("case_feedback") is passed — it never drops
      // this fetch from the shared FIFO queue.
      return withWidgetFetchSlot(async (signal) => {
        return api.post<
          { filters: Record<string, unknown>; bucket: BeCaseFeedbackBucket },
          BeCaseFeedbackAggregateResponse
        >(
          config.groupByEndpoint as string,
          { filters: resolvedFilters, bucket },
          { signal },
        );
      }, "case_feedback");
    },
    enabled: enabled && !!bucket,
    // This dashboard carries no team/current-user placeholder at all (see
    // `dateRangeFilterPlaceholder.ts` — case_feedback's own filters shape has
    // no team-scoped field), so every fetch here is "team-independent" in
    // `shouldRetryWidgetFetch`'s own sense — always eligible for the
    // queue-drop retry it applies to a widget whose own filters can't change
    // out from under an in-flight fetch.
    retry: (failureCount, error) => shouldRetryWidgetFetch(failureCount, error, true),
    staleTime: 60_000,
  });

  const isLoading = !enabled || (!!bucket && query.isLoading);
  const isError = !!bucket && query.isError;

  // "rating"/"reasons_*" modes group by a fixed label instead of time, so
  // the meaningful per-slice metric is how many responses fell into that
  // group (count), not avgRating — which for "rating" is always just the
  // rating value itself (a constant, not a useful slice size), and for
  // "reasons_*" carries no meaningful average at all (each row is a
  // categorical chip selection, not a rating).
  const isCountBased = bucket === "rating" || !!bucket?.startsWith("reasons_");
  // Red/amber/green by value only makes sense for the rating dimension
  // itself — a "reasons_*" bucket's slices are categorical chip choices
  // within a single, already-fixed rating level, with no better/worse
  // ordering of their own, so they keep the bar/pie chart's own default
  // per-index color rotation instead (color left undefined).
  const isRatingColored = bucket === "rating" || (!!bucket && !bucket.startsWith("reasons_"));
  const buckets = query.data?.buckets ?? [];
  const slices: PieSliceResult[] = buckets.map((b) => {
    // Click-through target, by bucket kind:
    // - date buckets (day/week/month): the exact date range that bucket
    //   covers, filtering the feedback list to just those submissions.
    // - "rating": the exact rating value, via the new rating filter —
    //   avgRating in this mode is always just the rating value itself
    //   (every row in the bucket shares it), so it's a safe round-trip.
    // - "reasons_*": no click-through — a reason chip isn't a field the
    //   feedback list's own search can filter by (it's a separate
    //   asmt_metric_result row, not a column on the feedback record
    //   itself), so there's no safe selector to scope to, same reasoning
    //   `useWidgetGroupByData`'s synthetic "Others" bucket uses for its
    //   own `navigable: false`.
    let sliceQuery: Record<string, unknown> = {};
    let navigable = false;
    if (bucket === "day" || bucket === "week" || bucket === "month") {
      const [dateFrom, dateTo] = bucketDateRange(b.bucketStart, bucket);
      sliceQuery = { dateFrom, dateTo };
      navigable = true;
    } else if (bucket === "rating") {
      sliceQuery = { rating: Math.round(b.avgRating) };
      navigable = true;
    }
    return {
      label: bucket ? formatBucketLabel(b.bucketStart, bucket) : b.bucketStart,
      query: sliceQuery,
      navigable,
      value: isCountBased ? b.count : b.avgRating,
      color: isRatingColored ? colorForAvgRating(b.avgRating) : undefined,
    };
  });
  const total = query.data?.totalRecords ?? 0;

  return { slices, total, isLoading, isError };
}
