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
import type {
  RecommendationSearchRequest,
  RecommendationSearchResponse,
} from "@features/support/types/recommendations";

/**
 * POST /conversations/recommendations/search — KB article recommendations for chat/case context.
 *
 * @param {RecommendationSearchRequest | null} payload - Request body, or null to skip.
 * @param {boolean} enabled - When false, the query does not run.
 * @returns {UseQueryResult<RecommendationSearchResponse, Error>} Query result.
 */
export function useConversationRecommendationsSearch(
  payload: RecommendationSearchRequest | null,
  enabled: boolean,
): UseQueryResult<RecommendationSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  const msgCount = payload?.chatHistory.length ?? 0;
  const firstTs = payload?.chatHistory[0]?.timestamp ?? "";
  const lastTs = payload?.chatHistory[msgCount - 1]?.timestamp ?? "";

  return useQuery<RecommendationSearchResponse, Error>({
    queryKey: [
      ApiQueryKeys.CONVERSATION_RECOMMENDATIONS_SEARCH,
      msgCount,
      firstTs,
      lastTs,
    ],
    queryFn: async (): Promise<RecommendationSearchResponse> => {
      if (!payload) {
        throw new Error("Recommendation payload is required");
      }
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/conversations/recommendations/search`;
      logger.debug("[useConversationRecommendationsSearch] POST", requestUrl);

      const response = await authFetch(requestUrl, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        throw new Error(
          `Recommendations request failed: ${response.status} ${response.statusText}`,
        );
      }

      const data: RecommendationSearchResponse = await response.json();
      logger.debug("[useConversationRecommendationsSearch] Response received");
      return data;
    },
    enabled:
      enabled &&
      !!payload &&
      payload.chatHistory.length > 0 &&
      isSignedIn &&
      !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
