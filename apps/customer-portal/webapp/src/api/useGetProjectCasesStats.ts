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
import { useMockConfig } from "@/providers/MockConfigProvider";
import { getMockProjectCasesStats } from "@/models/mockFunctions";
import { useLogger } from "@/hooks/useLogger";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { ProjectCasesStats } from "@/models/responses";

/**
 * Custom hook to fetch project case statistics by ID.
 *
 * @param {string} id - The ID of the project.
 * @returns {UseQueryResult<ProjectCasesStats, Error>} The query result object.
 */
export function useGetProjectCasesStats(
  id: string,
): UseQueryResult<ProjectCasesStats, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<ProjectCasesStats, Error>({
    queryKey: [ApiQueryKeys.CASES_STATS, id, isMockEnabled],
    queryFn: async (): Promise<ProjectCasesStats> => {
      logger.debug(
        `Fetching case stats for project ID: ${id}, mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        // Mock behavior: simulate network latency for the in-memory mock data.
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

        const stats: ProjectCasesStats = getMockProjectCasesStats();

        logger.debug(
          `Case stats fetched successfully for project ID: ${id} (mock)`,
          stats,
        );

        return stats;
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = import.meta.env.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${id}/stats/cases`;

        const response = await fetch(requestUrl, {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
            Authorization: `Bearer ${idToken}`,
            "x-user-id-token": idToken,
          },
        });

        logger.debug(
          `[useGetProjectCasesStats] Response status for ${id}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(`Error fetching case stats: ${response.statusText}`);
        }

        const data: ProjectCasesStats = await response.json();
        logger.debug("[useGetProjectCasesStats] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectCasesStats] Error:", error);
        throw error;
      }
    },
    enabled: !!id && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000,
  });
}
