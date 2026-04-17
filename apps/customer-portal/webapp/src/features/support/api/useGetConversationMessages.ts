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

import {
  useInfiniteQuery,
  type InfiniteData,
  type UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/utils/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@/constants/apiConstants";
import type { ConversationMessagesResponse } from "@features/support/types/conversations";
import type { UseGetConversationMessagesOptions } from "@features/support/types/supportApi";

export type { UseGetConversationMessagesOptions };

/**
 * Fetches conversation messages using GET /conversations/{conversationId}/messages
 * with limit/offset as an infinite query.
 *
 * @param {string} conversationId - The conversation ID.
 * @param {UseGetConversationMessagesOptions} [options] - Optional configuration (page size).
 * @returns {UseInfiniteQueryResult<InfiniteData<ConversationMessagesResponse>, Error>} Infinite query result.
 */
export function useGetConversationMessages(
  conversationId: string,
  options?: UseGetConversationMessagesOptions,
): UseInfiniteQueryResult<InfiniteData<ConversationMessagesResponse>, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const pageSize = options?.pageSize ?? 10;

  return useInfiniteQuery<
    ConversationMessagesResponse,
    Error,
    InfiniteData<ConversationMessagesResponse>
  >({
    queryKey: [ApiQueryKeys.CONVERSATION_MESSAGES, conversationId, pageSize],
    queryFn: async ({ pageParam }): Promise<ConversationMessagesResponse> => {
      const offset = typeof pageParam === "number" ? pageParam : 0;

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/conversations/${encodeURIComponent(
        conversationId,
      )}/messages?limit=${pageSize}&offset=${offset}`;

      logger.debug(
        `[useGetConversationMessages] Fetching messages for conversationId=${conversationId}, limit=${pageSize}, offset=${offset}`,
      );

      const response = await authFetch(requestUrl, {
        method: "GET",
      });

      logger.debug(
        `[useGetConversationMessages] Response status: ${response.status}`,
      );

      if (!response.ok) {
        throw new Error(
          `Error fetching conversation messages: ${response.status} ${response.statusText}`,
        );
      }

      const data: ConversationMessagesResponse = await response.json();
      logger.debug("[useGetConversationMessages] Data received:", data);
      return data;
    },
    enabled: !!conversationId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
    getNextPageParam: (lastPage) => {
      const nextOffset = lastPage.offset + lastPage.limit;
      if (nextOffset >= lastPage.totalRecords) {
        return undefined;
      }
      return nextOffset;
    },
    initialPageParam: 0,
  });
}
