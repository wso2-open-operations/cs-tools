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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { getMockAbtDashboard } from "@features/csm-dashboard/api/mocks/dashboardMocks";
import type {
  CsmAbtDashboardData,
  DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

const MOCK_LATENCY_MS = 250;

/**
 * Loads the CSM ABT dashboard payload.
 *
 * When `window.config.CSM_PORTAL_USE_MOCKS` is true, the hook resolves with
 * in-memory mock data so the UI can be built ahead of csm-portal/backend.
 * Otherwise it calls `GET ${BACKEND_BASE_URL}/csm/dashboard?scope=...`.
 */
export function useGetCsmDashboard(
  scope: DashboardScope,
): UseQueryResult<CsmAbtDashboardData, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmAbtDashboardData, Error>({
    queryKey: [ApiQueryKeys.CSM_ABT_DASHBOARD, scope],
    queryFn: async (): Promise<CsmAbtDashboardData> => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(
          `[useGetCsmDashboard] Returning mock data for scope=${scope}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockAbtDashboard(scope);
      }

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
    staleTime: 30_000,
  });
}
