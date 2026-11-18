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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";

/** Far more than the regions accounts hold, as the backend's own region check uses. */
const REGION_LIST_GROUPS = 1000;

interface RegionAggregateResponse {
  groups?: { key?: string; label?: string }[];
}

/**
 * The account regions (e.g. "APAC") as the account records write them, for the
 * projects list's Region filter. Read from `POST /cases/aggregate` grouped by
 * region, the same list the backend checks a dashboard's region filter
 * against: there is no account-region endpoint, and a region no case's account
 * has is one almost no project has either. Sorted by name, blanks dropped.
 * A failed read yields an empty list (the filter then offers nothing to pick).
 *
 * @param enabled - Fetch only while the picker is open or a region is chosen.
 */
export function useAccountRegions(enabled: boolean): UseQueryResult<string[], Error> {
  const api = useBackendApi();

  return useQuery<string[], Error>({
    queryKey: [ApiQueryKeys.ACCOUNT_REGIONS],
    queryFn: async (): Promise<string[]> => {
      const res = await api.post<{ groupBy: string; maxGroups: number }, RegionAggregateResponse>(
        "/cases/aggregate",
        { groupBy: "region", maxGroups: REGION_LIST_GROUPS },
      );
      const labels = (res.groups ?? [])
        .map((g) => (g.label || g.key || "").trim())
        .filter(Boolean);
      return Array.from(new Set(labels)).sort((a, b) => a.localeCompare(b));
    },
    enabled,
    staleTime: 10 * 60_000,
  });
}
