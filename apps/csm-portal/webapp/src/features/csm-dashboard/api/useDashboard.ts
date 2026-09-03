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
import type { BeDashboard } from "@api/backend/types";

/**
 * A single dashboard's display metadata plus every widget template
 * registered for it: display metadata and each widget's own filter criteria.
 * This does not resolve any widget's data — callers render one tile per
 * entry and each tile resolves its own data independently via its own
 * `POST /cases/search` call (see `useWidgetCaseCount`).
 *
 * Disabled while `dashboardId` is `undefined`, e.g. before the dashboard
 * list has loaded and an initial selection has been made.
 *
 * @param refetchIntervalMs Background auto-refetch interval in
 * milliseconds — `undefined` (the default) means no auto-refetch at all,
 * the behavior every caller had before this parameter existed. Only the
 * CS Overview dashboard (`WallboardDashboard`) passes a value here, to
 * match `digiops-cs`'s own Wallboard.tsx 60s refresh; every other
 * dashboard's own call site leaves it unset.
 */
export function useDashboard(
  dashboardId: string | undefined,
  refetchIntervalMs?: number,
): UseQueryResult<BeDashboard | null, Error> {
  const api = useBackendApi();

  return useQuery<BeDashboard | null, Error>({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_DETAIL, dashboardId ?? ""],
    queryFn: async (): Promise<BeDashboard | null> => {
      if (!dashboardId) return null;
      return api.get<BeDashboard>(`/dashboards/${dashboardId}`);
    },
    enabled: dashboardId !== undefined,
    staleTime: 30_000,
    refetchInterval: refetchIntervalMs,
  });
}
