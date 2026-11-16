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


import type { BeLineSeries } from "@api/backend/types";
import { resolveRelativeDatePlaceholder } from "@utils/resolveRelativeDatePlaceholder";
import { mergeWidgetFilters } from "@features/csm-dashboard/utils/widgetFilterMerge";
import { isCaseFieldFilterArray, type WidgetCaseFieldFilterLike } from "./widgetPreviewUrl";

/**
 * The query of one point of a `shape: "line"` widget: the cases created
 * (Opened) or closed (Closed) on that day, or from `date` to `endDate` for a
 * week or month point. Both are `YYYY-MM-DD`, UTC calendar days as `POST
 * /cases/volume-trend` buckets them; the case search reads a date-only `gte`
 * as that day's UTC start and a date-only `lte` as its end, and counts by the
 * same `created_on` / `closed_on` columns as the trend, so the list matches
 * the point. Over the widget's own query, use {@link linePointFilters}, which
 * keeps the widget's own bounds on the same field when they are tighter.
 */
export function linePointQuery(
  date: string,
  series: BeLineSeries,
  endDate?: string,
): Record<string, unknown> {
  const field = series === "opened" ? "createdOn" : "closedOn";
  return {
    filters: [
      { field, op: "gte", values: [date] },
      { field, op: "lte", values: [endDate ?? date] },
    ],
  };
}

/** `YYYY-MM-DD`: a date-only bound, read by the case search as a whole UTC day. */
const DATE_ONLY = /^\d{4}-\d{2}-\d{2}$/;

/**
 * The instant a date bound means, as the trend and the case search read it: a
 * relative date placeholder resolved as the trend's own filters are (the
 * viewer's local day, `resolveRelativeDatePlaceholder`), a date-only value as
 * its UTC day's start (`gte`) or end (`lte`), and anything else as the
 * timestamp it is. `NaN` when it is none of these.
 */
function boundInstant(value: string, op: "gte" | "lte", now: Date): number {
  const resolved = resolveRelativeDatePlaceholder(value, op, now);
  if (resolved !== undefined) return Date.parse(resolved);
  if (DATE_ONLY.test(value)) return Date.parse(`${value}${op === "gte" ? "T00:00:00.000Z" : "T23:59:59.999Z"}`);
  return Date.parse(value);
}

/**
 * The widget's own query with one line point's bounds on it: the point's
 * `linePointQuery`, merged over the widget's filters with
 * `mergeWidgetFilters`, but its `gte` / `lte` intersected with the widget's
 * own on the same field (the later `gte`, the earlier `lte`) instead of
 * replacing them. The trend counted the point with both applied, so the list
 * must too: an Opened point on a widget that also has `createdOn lte` counts
 * only the cases up to that bound. Entity-service keeps one `gte` and one
 * `lte` per field, so the two cannot simply both be sent. The tighter bound
 * keeps its own value (a widget's `__daysAgo:30__` stays a placeholder for
 * the case list); a bound that cannot be read as a date leaves the point's.
 */
export function linePointFilters(
  widgetQuery: Record<string, unknown> | null | undefined,
  date: string,
  series: BeLineSeries,
  endDate?: string,
  now: Date = new Date(),
): Record<string, unknown> {
  const pointFilters = linePointQuery(date, series, endDate).filters as WidgetCaseFieldFilterLike[];
  const field = pointFilters[0].field;
  const own = widgetQuery?.filters;
  const ownFilters = isCaseFieldFilterArray(own) ? own : [];
  const bounds = pointFilters.map((p) => {
    const op = p.op as "gte" | "lte";
    const ownBound = ownFilters.find((f) => f.field === field && f.op === op && typeof f.values?.[0] === "string");
    if (!ownBound?.values || !p.values) return p;
    const ownAt = boundInstant(ownBound.values[0], op, now);
    if (Number.isNaN(ownAt)) return p;
    const pointAt = boundInstant(p.values[0], op, now);
    const ownIsTighter = op === "gte" ? ownAt > pointAt : ownAt < pointAt;
    return ownIsTighter ? ownBound : p;
  });
  return mergeWidgetFilters(widgetQuery, { filters: bounds });
}
