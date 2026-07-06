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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeCommentSearchResponse } from "@api/backend/types";
import { uiCommentFromBe } from "@api/backend/mappers";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

/**
 * Load the messages of a case's linked chat conversation (the Novera transcript
 * the case was spawned from), mapped onto the same {@link CsmCaseComment} shape
 * as case comments so they merge into the activity feed as the earliest
 * entries — mirroring the customer portal.
 *
 * Calls `GET /conversations/{id}/messages`, which the backend composes from the
 * generic `/comments/search` with `referenceType: "conversation"`. The query is
 * disabled (returns `[]`) when the case has no linked conversation, so a
 * non-ServiceNow or chat-less case fetches nothing — no data-source gate needed.
 *
 * A single wide page (`limit` capped at BE_MAX_PAGE_LIMIT). Pre-case chats are
 * short; if one ever exceeds that, switch to a paginated wrapper rather than
 * chasing pages here.
 */
export function useGetCsmConversationMessages(
  conversationId: string | null | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const api = useBackendApi();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.CONVERSATION_MESSAGES, conversationId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!conversationId) return [];

      const response = await api.get<BeCommentSearchResponse>(
        `/conversations/${encodeURIComponent(conversationId)}/messages` +
          `?limit=${BE_MAX_PAGE_LIMIT}&offset=0`,
      );
      return (response?.comments ?? []).map((comment) =>
        uiCommentFromBe(comment, { context: "conversation" }),
      );
    },
    enabled: !!conversationId,
    staleTime: 30_000,
  });
}
