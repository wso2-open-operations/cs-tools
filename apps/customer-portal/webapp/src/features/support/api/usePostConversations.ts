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

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import type { ConversationRequest } from "@features/support/types/conversations";
import type { ConversationResponse } from "@features/support/types/conversations";

/**
 * Posts a message to the project conversations API (Novera chat).
 * Called when user hits "Submit & Get Help" on the describe-issue page.
 *
 * @returns {UseMutationResult<ConversationResponse, Error, { projectId: string } & ConversationRequest>} Mutation result.
 */
export function usePostConversations(): UseMutationResult<
  ConversationResponse,
  Error,
  { projectId: string } & ConversationRequest
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<
    ConversationResponse,
    Error,
    { projectId: string } & ConversationRequest
  >({
    mutationFn: async (
      params: { projectId: string } & ConversationRequest,
    ): Promise<ConversationResponse> => {
      try {
        const { projectId, message, envProducts, region, tier } = params;

        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to send messages");
        }

        logger.debug("[usePostConversations] Request:", {
          projectId,
          messageLength: message.length,
          envProducts: Object.keys(envProducts || {}),
          region,
          tier,
        });

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/conversations`;
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 40_000);

        const response = await authFetch(requestUrl, {
          method: "POST",
          signal: controller.signal,
          body: JSON.stringify({ message, envProducts, region, tier }),
        });

        clearTimeout(timeoutId);

        logger.debug(
          `[usePostConversations] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Conversations API error: ${response.status} ${response.statusText}`,
          );
        }

        const data: ConversationResponse = await response.json();
        logger.debug("[usePostConversations] Data received");
        return data;
      } catch (error) {
        if (error instanceof Error && error.name === "AbortError") {
          throw new Error(
            "Conversations API request timed out after 40 seconds",
          );
        }
        logger.error(
          "[usePostConversations] Error:",
          error instanceof Error ? error.message : String(error),
        );
        throw error;
      }
    },
  });
}
