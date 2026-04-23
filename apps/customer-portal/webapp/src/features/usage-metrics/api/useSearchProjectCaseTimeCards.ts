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

import {
  useInfiniteQuery,
  type UseInfiniteQueryResult,
  type InfiniteData,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  CaseTimeCardSearchResponse,
  TimeCardSearchRequest,
} from "@features/usage-metrics/types/timeTracking";
import type { UseSearchProjectTimeCardsParams } from "@features/usage-metrics/types/usageMetrics";

/**
 * Searches case-level time cards via projects/:projectId/cases/time-cards/search.
 *
 * @param {UseSearchProjectTimeCardsParams} params - Project ID and optional filters.
 * @returns {UseInfiniteQueryResult} The infinite query result.
 */
export default function useSearchProjectCaseTimeCards({
  projectId,
  startDate,
  endDate,
  states,
  enabled,
}: UseSearchProjectTimeCardsParams): UseInfiniteQueryResult<
  InfiniteData<CaseTimeCardSearchResponse>,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<CaseTimeCardSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.CASE_TIME_CARDS_SEARCH,
      projectId,
      startDate,
      endDate,
      states,
    ],
    queryFn: async ({
      pageParam = 0,
      signal,
    }): Promise<CaseTimeCardSearchResponse> => {
      logger.debug(
        `Searching case time cards for project ID: ${projectId}, offset: ${pageParam}`,
      );

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const filters: TimeCardSearchRequest["filters"] = {
        ...(startDate && { startDate }),
        ...(endDate && { endDate }),
        ...(states && states.length > 0 && { states }),
      };

      const body: TimeCardSearchRequest = {
        filters,
        pagination: { limit: 10, offset: pageParam as number },
      };

      const response = await authFetch(
        `${baseUrl}/projects/${projectId}/cases/time-cards/search`,
        { method: "POST", body: JSON.stringify(body), signal },
      );

      if (!response.ok) {
        throw new Error(`Error searching case time cards: ${response.statusText}`);
      }

      const data: CaseTimeCardSearchResponse = await response.json();
      logger.debug("[useSearchProjectCaseTimeCards] Data received:", data);
      return data;
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      return nextOffset < lastPage.totalRecords ? nextOffset : undefined;
    },
    enabled: enabled !== false && !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
