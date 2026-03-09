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
import type { ConversationSearchRequest } from "@models/requests";
import type { ConversationSearchResponse } from "@models/responses";

/**
 * Fetches conversations for a project via POST /projects/:projectId/conversations/search.
 *
 * @param {string} projectId - The project ID.
 * @param {ConversationSearchRequest} request - The search request body.
 * @returns {UseQueryResult<ConversationSearchResponse, Error>} The query result.
 */
export function useSearchConversations(
  projectId: string,
  request: ConversationSearchRequest,
): UseQueryResult<ConversationSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ConversationSearchResponse, Error>({
    queryKey: [ApiQueryKeys.CONVERSATIONS_SEARCH, projectId, request],
    queryFn: async (): Promise<ConversationSearchResponse> => {
      logger.debug(
        `Fetching conversations for project ID: ${projectId}`,
        request,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/conversations/search`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(request),
        });

        logger.debug(
          `[useSearchConversations] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching conversations: ${response.statusText}`,
          );
        }

        const data: ConversationSearchResponse = await response.json();
        logger.debug("[useSearchConversations] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useSearchConversations] Error:", error);
        throw error;
      }
    },
    enabled: !!projectId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
