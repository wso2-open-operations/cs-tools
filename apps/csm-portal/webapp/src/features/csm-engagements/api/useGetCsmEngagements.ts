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
import { getMockCsmEngagements } from "@features/csm-engagements/api/mocks/engagementsMocks";
import type { CsmEngagementsListResponse } from "@features/csm-engagements/types/csmEngagements";

const MOCK_LATENCY_MS = 200;

/**
 * Loads the cross-customer CSM engagements list.
 *
 * Returns mock data when `window.config.CSM_PORTAL_USE_MOCKS` is true so the
 * UI is testable ahead of the backend. Otherwise calls
 * `GET ${BACKEND_BASE_URL}/csm/engagements`.
 */
export function useGetCsmEngagements(): UseQueryResult<
  CsmEngagementsListResponse,
  Error
> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmEngagementsListResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_ENGAGEMENTS],
    queryFn: async () => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug("[useGetCsmEngagements] Returning mock engagements");
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmEngagements();
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const response = await authFetch(`${baseUrl}/csm/engagements`, {
        method: "GET",
      });
      if (!response.ok) {
        throw new Error(
          `Error fetching CSM engagements: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmEngagementsListResponse;
    },
    staleTime: 30_000,
  });
}
