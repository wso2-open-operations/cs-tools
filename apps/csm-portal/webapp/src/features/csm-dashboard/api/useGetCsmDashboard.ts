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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CsmAbtDashboardData,
  DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Loads the CSM ABT dashboard payload from `GET ${BACKEND_BASE_URL}/csm/dashboard`.
 *
 * NOTE: that endpoint does not exist on the backend yet, so the query is
 * **disabled** — it never fires (avoiding a guaranteed 404 on every dashboard
 * load). The fetch logic is kept so it can be re-enabled in one line once the
 * BE ships `/csm/dashboard`: flip `DASHBOARD_ENDPOINT_READY` to true.
 */
const DASHBOARD_ENDPOINT_READY = false;

export function useGetCsmDashboard(
  scope: DashboardScope,
): UseQueryResult<CsmAbtDashboardData, Error> {
  const authFetch = useAuthApiClient();

  return useQuery<CsmAbtDashboardData, Error>({
    queryKey: [ApiQueryKeys.CSM_ABT_DASHBOARD, scope],
    queryFn: async (): Promise<CsmAbtDashboardData> => {
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const url = `${baseUrl}/csm/dashboard?scope=${encodeURIComponent(scope)}`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(
          `Error fetching CSM dashboard: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmAbtDashboardData;
    },
    enabled: DASHBOARD_ENDPOINT_READY,
    staleTime: 30_000,
  });
}
