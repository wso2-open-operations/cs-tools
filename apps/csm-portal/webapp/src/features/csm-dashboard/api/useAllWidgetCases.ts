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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeWidgetResourceType } from "@api/backend/types";
import { WIDGET_RESOURCE_CONFIG } from "@features/csm-dashboard/config/widgetResourceConfig";

/** Most records the grouped-by-account preview loads. A widget that groups by
 * account is a narrow, operational slice (open cases with an active
 * escalation), so this is a safety valve against an unbounded loop rather
 * than a limit anyone should hit; when it does bind, the page says so. */
export const MAX_GROUPED_WIDGET_RECORDS = 1000;

export interface AllWidgetCases {
  /** Every record fetched, in the search endpoint's own order. */
  items: Record<string, unknown>[];
  /** The search endpoint's own total, which exceeds `items.length` only
   * when {@link MAX_GROUPED_WIDGET_RECORDS} cut the fetch short. */
  total: number;
  truncated: boolean;
}

/**
 * Pages through a case-family widget's whole result set (`POST
 * /cases/search` with the widget's own, already-resolved filters, at the
 * backend's page-size cap) so the caller can group it client-side. Used only
 * by the grouped-by-account preview; every other preview pages server-side.
 */
export function useAllWidgetCases({
  widgetId,
  resourceType,
  filters,
}: {
  widgetId: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
}): UseQueryResult<AllWidgetCases, Error> {
  const api = useBackendApi();
  const config = WIDGET_RESOURCE_CONFIG[resourceType];

  return useQuery<AllWidgetCases, Error>({
    // Same leading slots as `useWidgetData`'s key, so `invalidateWidgetQueries`
    // refreshes this too.
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_WIDGET_DATA, widgetId, resourceType, "all", filters],
    queryFn: async ({ signal }): Promise<AllWidgetCases> => {
      if (!config) throw new Error(`Unsupported widget resourceType: ${resourceType}`);
      const items: Record<string, unknown>[] = [];
      let total = 0;
      while (items.length < MAX_GROUPED_WIDGET_RECORDS) {
        const res = await api.post<Record<string, unknown>, Record<string, unknown>>(
          config.searchEndpoint,
          { filters, pagination: { offset: items.length, limit: BE_MAX_PAGE_LIMIT } },
          { signal },
        );
        total = typeof res.total === "number" ? res.total : 0;
        const raw = res[config.itemsKey];
        const page = Array.isArray(raw) ? (raw as Record<string, unknown>[]) : [];
        items.push(...page);
        // A short (or empty) page is the last one; `total` guards a backend
        // that keeps returning full pages past the end.
        if (page.length < BE_MAX_PAGE_LIMIT || items.length >= total) break;
      }
      return { items, total, truncated: items.length < total };
    },
    staleTime: 300_000,
  });
}
