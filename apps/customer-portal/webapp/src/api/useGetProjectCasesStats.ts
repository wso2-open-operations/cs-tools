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

  return useQuery<ProjectCasesStats, Error>({
    queryKey: [ApiQueryKeys.CASES_STATS, id],
    queryFn: async (): Promise<ProjectCasesStats> => {
      logger.debug(`Fetching case stats for project ID: ${id}`);

      /**
       * Mock behavior: simulate network latency for the in-memory mock data.
       * This is intended only for development/demo use and should be removed or
       * replaced when wiring this hook to the real backend API.
       */
      await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

      const stats: ProjectCasesStats = getMockProjectCasesStats();

      logger.debug(
        `Case stats fetched successfully for project ID: ${id}`,
        stats,
      );

      return stats;
    },
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  });
}
