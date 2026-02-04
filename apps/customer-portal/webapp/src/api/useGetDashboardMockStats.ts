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
import { getMockDashboardStats } from "@/models/mockFunctions";
import { useLogger } from "@/hooks/useLogger";
import { ApiQueryKeys, API_MOCK_DELAY } from "@/constants/apiConstants";
import type { DashboardMockStats } from "@/models/responses";

/**
 * Custom hook to fetch dashboard mock statistics.
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseQueryResult<DashboardMockStats, Error>} The query result object.
 */
export function useGetDashboardMockStats(
  projectId: string,
): UseQueryResult<DashboardMockStats, Error> {
  const logger = useLogger();

  return useQuery<DashboardMockStats, Error>({
    queryKey: [ApiQueryKeys.DASHBOARD_STATS, projectId],
    queryFn: async (): Promise<DashboardMockStats> => {
      logger.debug(
        `Fetching dashboard mock stats for project ID: ${projectId}`,
      );

      /**
       * Mock behavior: simulate network latency.
       */
      await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

      const stats: DashboardMockStats = getMockDashboardStats();

      logger.debug(
        `Dashboard mock stats fetched successfully for project ID: ${projectId}`,
        stats,
      );

      return stats;
    },
    enabled: !!projectId,
    staleTime: 5 * 60 * 1000,
  });
}
