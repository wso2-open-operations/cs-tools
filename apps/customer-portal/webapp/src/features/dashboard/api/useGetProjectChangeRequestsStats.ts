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
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ChangeRequestStatsResponse } from "@features/operations/types/changeRequests";

export interface UseGetProjectChangeRequestsStatsOptions {
  enabled?: boolean;
}

/**
 * Custom hook to fetch change request statistics for a project.
 *
 * @param {string} id - The ID of the project.
 * @param {UseGetProjectChangeRequestsStatsOptions} [options] - Optional configuration (enabled).
 * @returns {UseQueryResult<ChangeRequestStatsResponse, Error>} The query result object.
 */
export function useGetProjectChangeRequestsStats(
  id: string,
  options?: UseGetProjectChangeRequestsStatsOptions,
): UseQueryResult<ChangeRequestStatsResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const enabled =
    (options?.enabled ?? true) && !!id && isSignedIn && !isAuthLoading;

  return useQuery<ChangeRequestStatsResponse, Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUEST_STATS, id],
    queryFn: async (): Promise<ChangeRequestStatsResponse> => {
      logger.debug(`Fetching change request stats for project ID: ${id}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${id}/stats/change-requests`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProjectChangeRequestsStats] Response status for ${id}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching change request stats: ${response.statusText}`,
          );
        }

        const raw = (await response.json()) as any;

        const data: ChangeRequestStatsResponse = {
          totalCount: raw?.totalCount ?? 0,
          activeCount: raw?.activeCount,
          outstandingCount: raw?.outstandingCount,
          stateCount: raw?.stateCount ?? [],
        };

        logger.debug("[useGetProjectChangeRequestsStats] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectChangeRequestsStats] Error:", error);
        throw error;
      }
    },
    enabled,
    staleTime: 0,
  });
}
