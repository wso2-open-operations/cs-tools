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
import { getMockCsmEngagementDetail } from "@features/csm-engagements/api/mocks/engagementsMocks";
import type { CsmEngagementDetail } from "@features/csm-engagements/types/csmEngagements";

const MOCK_LATENCY_MS = 250;

/**
 * Loads a single engagement detail. Returns mock detail when in mock mode.
 */
export function useGetCsmEngagementDetail(
  id: string | undefined,
): UseQueryResult<CsmEngagementDetail, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmEngagementDetail, Error>({
    queryKey: [ApiQueryKeys.CSM_ENGAGEMENT_DETAIL, id],
    enabled: !!id,
    queryFn: async () => {
      if (!id) throw new Error("Engagement id is required");
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(`[useGetCsmEngagementDetail] mock detail for ${id}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        const found = getMockCsmEngagementDetail(id);
        if (!found) throw new Error(`Engagement ${id} not found`);
        return found;
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/engagements/${encodeURIComponent(id)}`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(
          `Error fetching engagement ${id}: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmEngagementDetail;
    },
    staleTime: 30_000,
  });
}
