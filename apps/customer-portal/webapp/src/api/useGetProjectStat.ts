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
import { getMockProjectStats } from "@/models/mockFunctions";
import { mockProjects } from "@/models/mockData";
import { useLogger } from "@/hooks/useLogger";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { ProjectStatsResponse } from "@/models/responses";

/**
 * Custom hook to fetch project statistics.
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseQueryResult<ProjectStatsResponse, Error>} The query result object.
 */
export function useGetProjectStat(
  projectId: string,
): UseQueryResult<ProjectStatsResponse, Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<ProjectStatsResponse, Error>({
    queryKey: [ApiQueryKeys.PROJECT_STATS, projectId, isMockEnabled],
    queryFn: async (): Promise<ProjectStatsResponse> => {
      logger.debug(
        `Fetching project stats for project ID: ${projectId}, mock: ${isMockEnabled}`,
      );

      if (isMockEnabled) {
        // Mock behavior: simulate network latency.
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

        // Validate project ID (using mock data for validation)
        const projectExists = mockProjects.some((p) => p.id === projectId);
        if (!projectExists) {
          logger.error(`Project stats not found for ID: ${projectId}`);
          throw new Error(`Project stats not found for ID: ${projectId}`);
        }

        const stats: ProjectStatsResponse = getMockProjectStats();

        logger.debug(
          `Project stats fetched successfully for project ID: ${projectId} (mock)`,
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

        const requestUrl = `${baseUrl}/projects/${projectId}/stats`;

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
          `[useGetProjectStat] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching project stats: ${response.statusText}`,
          );
        }

        const data: ProjectStatsResponse = await response.json();
        logger.debug("[useGetProjectStat] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProjectStat] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000,
  });
}
