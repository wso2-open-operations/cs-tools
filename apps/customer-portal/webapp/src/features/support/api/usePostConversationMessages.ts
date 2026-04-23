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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import type { ConversationRequest } from "@features/support/types/conversations";
import type { ConversationResponse } from "@features/support/types/conversations";

/**
 * Posts a follow-up message to an existing conversation.
 * Called after usePostConversations when user sends additional messages in chat.
 *
 * @returns {UseMutationResult<ConversationResponse, Error, { projectId: string; conversationId: string } & ConversationRequest>} Mutation result.
 */
export function usePostConversationMessages(): UseMutationResult<
  ConversationResponse,
  Error,
  { projectId: string; conversationId: string } & ConversationRequest
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<
    ConversationResponse,
    Error,
    { projectId: string; conversationId: string } & ConversationRequest
  >({
    mutationFn: async (
      params: {
        projectId: string;
        conversationId: string;
      } & ConversationRequest,
    ): Promise<ConversationResponse> => {
      const { projectId, conversationId, message, envProducts, region, tier } =
        params;

      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to send messages");
      }

      logger.debug("[usePostConversationMessages] Request:", {
        projectId,
        conversationId,
        messageLength: message.length,
      });

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/projects/${projectId}/conversations/${conversationId}/messages`;
      const response = await authFetch(requestUrl, {
        method: "POST",

        body: JSON.stringify({ message, envProducts, region, tier }),
      });

      if (!response.ok) {
        throw new Error(
          `Conversation messages API error: ${response.status} ${response.statusText}`,
        );
      }

      const data: ConversationResponse = await response.json();
      logger.debug("[usePostConversationMessages] Data received");
      return data;
    },
  });
}
