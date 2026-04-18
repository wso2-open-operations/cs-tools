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
import { useAuthApiClient } from "@utils/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { TimeCardSearchResponse } from "@features/usage-metrics/types/timeTracking";
import type { TimeCardSearchRequest } from "@features/usage-metrics/types/timeTracking";
import type { UseSearchProjectTimeCardsParams } from "@features/usage-metrics/types/usageMetrics";

/**
 * Custom hook to search project time cards with date range filters using infinite query.
 *
 * @param {UseSearchProjectTimeCardsParams} params - Project ID and filters.
 * @returns {UseInfiniteQueryResult<InfiniteData<TimeCardSearchResponse>, Error>} The infinite query result object.
 */
export default function useSearchProjectTimeCards({
  projectId,
  startDate,
  endDate,
  states,
  enabled,
}: UseSearchProjectTimeCardsParams): UseInfiniteQueryResult<
  InfiniteData<TimeCardSearchResponse>,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useInfiniteQuery<TimeCardSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.TIME_CARDS_SEARCH,
      projectId,
      startDate,
      endDate,
      states,
    ],
    queryFn: async ({
      pageParam = 0,
      signal,
    }): Promise<TimeCardSearchResponse> => {
      logger.debug(
        `Searching time cards for project ID: ${projectId}, start: ${startDate}, end: ${endDate}, offset: ${pageParam}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/time-cards/search`;
        const filters: TimeCardSearchRequest["filters"] = {
          ...(startDate && { startDate }),
          ...(endDate && { endDate }),
          ...(states && states.length > 0 && { states }),
        };

        const body: TimeCardSearchRequest = {
          filters,
          pagination: { limit: 10, offset: pageParam as number },
        };

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
          signal,
        });

        logger.debug(
          `[useSearchProjectTimeCards] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(`Error searching time cards: ${response.statusText}`);
        }

        const data: TimeCardSearchResponse = await response.json();
        logger.debug("[useSearchProjectTimeCards] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useSearchProjectTimeCards] Error:", error);
        throw error;
      }
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
