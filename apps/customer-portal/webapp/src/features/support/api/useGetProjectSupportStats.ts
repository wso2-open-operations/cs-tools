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
import { useAuthApiClient } from "@/utils/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@/constants/apiConstants";
import type { ProjectSupportStats } from "@features/project-hub/types/projects";
import { CaseType } from "@features/support/constants/supportConstants";
import type { UseGetProjectSupportStatsOptions } from "@features/support/types/supportApi";

export type { UseGetProjectSupportStatsOptions };

/**
 * Custom hook to fetch project support statistics by ID.
 * Uses default_case for case types when not specified.
 *
 * @param {string} id - The ID of the project.
 * @param {UseGetProjectSupportStatsOptions} options - Optional filters (caseTypes, query).
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

  const { caseTypes = [CaseType.DEFAULT_CASE], query } = options ?? {};

  return useQuery<ProjectSupportStats, Error>({
    queryKey: [ApiQueryKeys.SUPPORT_STATS, id, caseTypes, query],
    queryFn: async (): Promise<ProjectSupportStats> => {
      logger.debug(`Fetching support stats for project ID: ${id}`, options);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const params = new URLSearchParams();
        caseTypes.forEach((t) => {
          if (t) params.append("caseTypes", t);
        });
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

        const raw = (await response.json()) as any;
        const data: ProjectSupportStats = {
          ongoingCases: raw?.ongoingCases ?? 0,
          resolvedPast30DaysCasesCount: raw?.resolvedPast30DaysCasesCount ?? 0,
          resolvedChats: raw?.resolvedChats ?? 0,
          activeChats: raw?.activeChats ?? 0,
        };
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
