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
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ProjectSupportStats } from "@models/responses";

export interface UseGetProjectSupportStatsOptions {
  incidentId?: string;
  queryId?: string;
  query?: string;
}

/**
 * Custom hook to fetch project support statistics by ID.
 *
 * @param {string} id - The ID of the project.
 * @param {UseGetProjectSupportStatsOptions} options - Optional filters (incidentId, queryId, query).
 * @param {boolean} enabled - Whether to execute the query (default: true).
 * @returns {UseQueryResult<ProjectSupportStats, Error>} The query result object.
 */
export function useGetProjectSupportStats(
  id: string,
  options?: UseGetProjectSupportStatsOptions,
  enabled: boolean = true,
): UseQueryResult<ProjectSupportStats, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  const { incidentId, queryId, query } = options ?? {};

  return useQuery<ProjectSupportStats, Error>({
    queryKey: [ApiQueryKeys.SUPPORT_STATS, id, incidentId, queryId, query],
    queryFn: async (): Promise<ProjectSupportStats> => {
      logger.debug(`Fetching support stats for project ID: ${id}`, options);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const params = new URLSearchParams();
        if (incidentId) params.append("caseTypes", incidentId);
        if (queryId) params.append("caseTypes", queryId);
        if (query) params.set("query", query);

        const queryString = params.toString();
        const requestUrl = `${baseUrl}/projects/${id}/stats/support${
          queryString ? `?${queryString}` : ""
        }`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProjectSupportStats] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching support stats: ${response.statusText}`,
          );
        }

        const data: ProjectSupportStats = await response.json();
        logger.debug("[useGetProjectSupportStats] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectSupportStats] Error:", error);
        throw error;
      }
    },
    enabled: !!id && isSignedIn && !isAuthLoading && enabled,
    staleTime: 5 * 60 * 1000,
  });
}
