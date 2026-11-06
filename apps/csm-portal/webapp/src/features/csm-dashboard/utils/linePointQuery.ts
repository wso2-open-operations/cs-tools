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

/**
 * The query of one point of a `shape: "line"` widget: the cases created
 * (Opened) or closed (Closed) on that day, or from `date` to `endDate` for a
 * week or month point. Both are `YYYY-MM-DD`, UTC calendar days as `POST
 * /cases/volume-trend` buckets them; the case search reads a date-only `gte`
 * as that day's UTC start and a date-only `lte` as its end, and counts by the
 * same `created_on` / `closed_on` columns as the trend, so the list matches
 * the point. Merged over the widget's own query with `mergeWidgetFilters`, so
 * it replaces the widget's own entries on that field.
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
