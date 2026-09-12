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
