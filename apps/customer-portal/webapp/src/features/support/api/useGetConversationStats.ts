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
import type { ConversationStats } from "@features/support/types/conversations";
import type { UseGetConversationStatsOptions } from "@features/support/types/supportApi";

export type { UseGetConversationStatsOptions };

/**
 * Custom hook to fetch conversation statistics for a project.
 *
 * @param {string} projectId - The ID of the project.
 * @param {UseGetConversationStatsOptions} [options] - Optional query options.
 * @returns {UseQueryResult<ConversationStats, Error>} The query result object.
 */
export function useGetConversationStats(
  projectId: string,
  options?: UseGetConversationStatsOptions,
): UseQueryResult<ConversationStats, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const { createdByMe, enabled = true } = options ?? {};

  return useQuery<ConversationStats, Error>({
    queryKey: [ApiQueryKeys.CONVERSATION_STATS, projectId, createdByMe],
    queryFn: async (): Promise<ConversationStats> => {
      logger.debug(`Fetching conversation stats for project ID: ${projectId}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        let requestUrl = `${baseUrl}/projects/${projectId}/stats/conversations`;

        if (createdByMe) {
          requestUrl += "?createdBy=me";
        }

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetConversationStats] Response status for ${projectId}: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching conversation stats: ${response.statusText}`,
          );
        }

        const data: ConversationStats = await response.json();

        logger.debug(
          `[useGetConversationStats] Successfully fetched conversation stats for ${projectId}`,
        );

        return data;
      } catch (error) {
        logger.error(
          `[useGetConversationStats] Error fetching conversation stats for ${projectId}: ${error}`,
        );

        throw error;
      }
    },
    enabled: (enabled ?? true) && !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000, // 5 minutes
    gcTime: 10 * 60 * 1000, // 10 minutes
    retry: 2,
  });
}
