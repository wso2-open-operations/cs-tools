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

import { resolveRelativeDateFilters } from "@features/csm-dashboard/utils/resolveRelativeDateFilters";

/** `YYYY-MM-DD`: a plain date, which the PLG search reads as that whole UTC day. */
const PLAIN_DATE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * The registrations of one point of a `plg_registration` `shape: "line"`
 * widget: the widget's own (flat) filters with `registeredFrom` / `registeredTo`
 * set to the point's day, or to `date` and `endDate` for a week or month
 * point. Both are UTC calendar days as `POST /plg/registrations/volume-trend`
 * buckets them, and the PLG search reads a plain date as that whole UTC day
 * (`plgSearchFilters`), so the list matches the point.
 *
 * The trend counted the point with the widget's own registration bounds
 * applied too, so where the widget has one on the same side, the tighter of
 * the two is kept (the later `registeredFrom`, the earlier `registeredTo`).
 * The widget's relative dates are resolved first, to the plain dates its
 * trend was asked for; a bound that is not a plain date leaves the point's.
 */
export function plgLinePointFilters(
  widgetQuery: Record<string, unknown> | null | undefined,
  date: string,
  endDate?: string,
  now: Date = new Date(),
): Record<string, unknown> {
  const own = resolveRelativeDateFilters(widgetQuery ?? {}, now);
  const tighter = (ownBound: unknown, pointBound: string, later: boolean): string => {
    if (typeof ownBound !== "string" || !PLAIN_DATE.test(ownBound)) return pointBound;
    return later === ownBound > pointBound ? ownBound : pointBound;
  };
  return {
    ...own,
    registeredFrom: tighter(own.registeredFrom, date, true),
    registeredTo: tighter(own.registeredTo, endDate ?? date, false),
  };
}
