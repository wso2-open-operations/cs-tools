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
import type {
  ChangeRequestStatsResponse,
  ChangeRequestStats,
} from "@models/responses";

/**
 * Maps the API response to ChangeRequestStats.
 *
 * @param {ChangeRequestStatsResponse} response - The API response.
 * @returns {ChangeRequestStats} The mapped stats object.
 */
function mapChangeRequestStats(
  response: ChangeRequestStatsResponse,
): ChangeRequestStats {
  const { totalCount, stateCount } = response;

  // Find scheduled count (label: "Scheduled")
  const scheduled =
    stateCount.find((state) => state.label === "Scheduled")?.count ?? 0;

  // Calculate in-progress count (sum of: Implement, Review, Customer Approval, Customer Review)
  const inProgressLabels = [
    "Implement",
    "Review",
    "Customer Approval",
    "Customer Review",
  ];
  const inProgress = stateCount
    .filter((state) => inProgressLabels.includes(state.label))
    .reduce((sum, state) => sum + state.count, 0);

  // Calculate completed count (sum of: Closed, Canceled, Rollback)
  const completedLabels = ["Closed", "Canceled", "Rollback"];
  const completed = stateCount
    .filter((state) => completedLabels.includes(state.label))
    .reduce((sum, state) => sum + state.count, 0);

  return {
    totalRequests: totalCount,
    scheduled,
    inProgress,
    completed,
  };
}

/**
 * Custom hook to fetch project change request statistics.
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseQueryResult<ChangeRequestStats, Error>} The query result object.
 */
export function useGetProjectChangeRequestStats(
  projectId: string,
): UseQueryResult<ChangeRequestStats, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ChangeRequestStats, Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUEST_STATS, projectId],
    queryFn: async (): Promise<ChangeRequestStats> => {
      logger.debug(
        `[useGetProjectChangeRequestStats] Fetching change request stats for project ID: ${projectId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/stats/change-requests`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProjectChangeRequestStats] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching change request stats: ${response.statusText}`,
          );
        }

        const data: ChangeRequestStatsResponse = await response.json();
        logger.debug("[useGetProjectChangeRequestStats] Data received:", data);

        // Map the API response to the expected stats format
        const mappedStats = mapChangeRequestStats(data);
        logger.debug(
          "[useGetProjectChangeRequestStats] Mapped stats:",
          mappedStats,
        );

        return mappedStats;
      } catch (error) {
        logger.error("[useGetProjectChangeRequestStats] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  });
}
