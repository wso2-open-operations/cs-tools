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

import type { QueryClient } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";

/**
 * Invalidates only the widget-data queries belonging to `widgetIds` — every
 * shape's query key carries a widget id, just at a different position:
 * `[KEY, widgetId, ...]` for count/list (see `useWidgetData`),
 * `[KEY, "pie-slice", widgetId, ...]` for pie/bar via `slices` (see
 * `useWidgetPieData`), `[KEY, "group-by", widgetId, ...]` for pie/bar via
 * `groupBy` (see `useWidgetGroupByData`), `[KEY, "feedback-trend", widgetId,
 * ...]` for the case-feedback bar/date-bucket trend (see
 * `useCaseFeedbackTrendData`).
 *
 * Shared between `DashboardWidgetGrid` (a whole section's worth of widget
 * ids at once) and `DashboardWidgetTile` (its own single widget id) so this
 * query-key-shape knowledge lives in exactly one place.
 */
export function invalidateWidgetQueries(
  queryClient: QueryClient,
  widgetIds: Set<string>,
): Promise<void> {
  return queryClient.invalidateQueries({
    predicate: (query) => {
      const key = query.queryKey;
      if (key[0] !== ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA) return false;
      const widgetId =
        key[1] === "pie-slice" || key[1] === "group-by" || key[1] === "feedback-trend"
          ? key[2]
          : key[1];
      return typeof widgetId === "string" && widgetIds.has(widgetId);
    },
  });
}

/**
 * Mark every dashboard widget that shows call requests stale (and refetch the ones on
 * screen), whatever its id, shape or filters. Widget data is cached for five minutes,
 * so without this a call request that was just completed, scheduled or rejected would
 * stay on "My Call Requests" / "Calls To Attend" (and keep counting in a pie or bar
 * over them) until the cache expired. Keyed on the resource type in the query key, so
 * no other widget is reloaded. The count/list shape is `[WIDGET_DATA, widgetId,
 * resourceType, ...]`; the pie/bar slice and group-by shapes lead with a marker and
 * carry the resource type one slot later (`[WIDGET_DATA, "pie-slice", widgetId,
 * resourceType, ...]`) -- the same special-casing `invalidateWidgetQueries` does for
 * the widget id above. The feedback trend is case feedback, not call requests.
 */
export function invalidateCallRequestWidgetQueries(queryClient: QueryClient): Promise<void> {
  return queryClient.invalidateQueries({
    predicate: (query) => {
      const key = query.queryKey;
      if (key[0] !== ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA) return false;
      const resourceType = key[1] === "pie-slice" || key[1] === "group-by" ? key[3] : key[2];
      return resourceType === "call_request";
    },
  });
}
