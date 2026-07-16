// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { parseApiResponseMessage } from "@utils/ApiError";

/**
 * Abandons (closes) a Novera conversation via POST /conversations/:id/abandon,
 * moving it to a terminal state so it can no longer be resumed. On success,
 * refreshes the conversation list and stats for the project.
 *
 * @param {string} projectId - Project ID for cache invalidation.
 * @returns {UseMutationResult<void, Error, string>} Mutation keyed by conversationId.
 */
export function useAbandonConversation(
  projectId: string,
): UseMutationResult<void, Error, string> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, string>({
    mutationFn: async (conversationId: string): Promise<void> => {
      if (!conversationId) {
        throw new Error("Conversation ID is required to close a chat");
      }
      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to close a chat");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const response = await authFetch(
        `${baseUrl}/conversations/${conversationId}/abandon`,
        { method: "POST" },
      );
      if (!response.ok) {
        const text = await response.text();
        throw new Error(
          parseApiResponseMessage(text, response.status, response.statusText),
        );
      }
      logger.debug("[useAbandonConversation] Conversation closed:", {
        conversationId,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CONVERSATIONS_SEARCH, projectId],
      });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CONVERSATION_STATS, projectId],
      });
      void queryClient.refetchQueries({
        queryKey: [ApiQueryKeys.CONVERSATIONS_SEARCH, projectId],
        type: "active",
      });
    },
  });
}
