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
import type { ConversationSummaryResponse } from "@features/support/types/supportApi";

export type { ConversationSummaryResponse };

/**
 * Fetch conversation summary for a specific conversation.
 *
 * @param {string} projectId - Project ID.
 * @param {string} conversationId - Conversation ID.
 * @returns {UseQueryResult} React Query result object.
 */
export default function useGetConversationSummary(
  projectId: string,
  conversationId: string,
): UseQueryResult<ConversationSummaryResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ConversationSummaryResponse, Error>({
    queryKey: [ApiQueryKeys.CONVERSATION_SUMMARY, projectId, conversationId],
    queryFn: async (): Promise<ConversationSummaryResponse> => {
      logger.debug(
        `Fetching conversation summary for project: ${projectId}, conversation: ${conversationId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/conversations/${conversationId}/summary`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        if (!response.ok) {
          throw new Error(
            `Error fetching conversation summary: ${response.statusText}`,
          );
        }

        const data: ConversationSummaryResponse = await response.json();
        logger.debug("[useGetConversationSummary] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetConversationSummary] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && !!conversationId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
