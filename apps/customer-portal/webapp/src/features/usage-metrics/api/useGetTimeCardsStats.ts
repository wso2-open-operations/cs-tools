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
import type { ProjectTimeTrackingStats } from "@features/usage-metrics/types/timeTracking";
import type { UseGetTimeCardsStatsParams } from "@features/usage-metrics/types/usageMetrics";

/**
 * Fetches time card statistics for a project within a date range.
 * GET /projects/:projectId/stats/time-cards?startDate=YYYY-MM-DD&endDate=YYYY-MM-DD
 *
 * @param {UseGetTimeCardsStatsParams} params - Project ID and date range.
 * @returns {UseQueryResult<ProjectTimeTrackingStats, Error>} The query result object.
 */
export default function useGetTimeCardsStats({
  projectId,
  startDate,
  endDate,
}: UseGetTimeCardsStatsParams): UseQueryResult<
  ProjectTimeTrackingStats,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProjectTimeTrackingStats, Error>({
    queryKey: [ApiQueryKeys.TIME_TRACKING_STATS, projectId, startDate, endDate],
    queryFn: async (): Promise<ProjectTimeTrackingStats> => {
      logger.debug(
        `[useGetTimeCardsStats] Fetching stats for project ${projectId}, ${startDate} to ${endDate}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const params = new URLSearchParams({
          startDate,
          endDate,
        });
        const requestUrl = `${baseUrl}/projects/${projectId}/stats/time-cards?${params.toString()}`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetTimeCardsStats] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching time cards stats: ${response.statusText}`,
          );
        }

        const rawData: ProjectTimeTrackingStats = await response.json();
        logger.debug("[useGetTimeCardsStats] Raw data received:", rawData);

        // Convert minutes to hours
        const data: ProjectTimeTrackingStats = {
          totalHours: Math.round((rawData.totalHours / 60) * 100) / 100,
          billableHours: Math.round((rawData.billableHours / 60) * 100) / 100,
          nonBillableHours:
            Math.round((rawData.nonBillableHours / 60) * 100) / 100,
        };

        logger.debug(
          "[useGetTimeCardsStats] Converted data (minutes to hours):",
          data,
        );
        return data;
      } catch (error) {
        logger.error("[useGetTimeCardsStats] Error:", error);
        throw error;
      }
    },
    enabled:
      !!projectId && !!startDate && !!endDate && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
