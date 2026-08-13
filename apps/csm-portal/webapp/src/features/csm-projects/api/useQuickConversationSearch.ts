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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { classifyConversationQuery } from "@features/csm-projects/utils/conversationQueryScope";
import type {
  BeSearchConversationsPayload,
  BeSearchConversationsResponse,
} from "@api/backend/types";

/** Don't fire a search until the user has typed something searchable — mirrors {@link
 * "@features/csm-cases/api/useQuickCaseSearch".QUICK_CASE_MIN_QUERY_LEN}. */
export const QUICK_CONVERSATION_MIN_QUERY_LEN = 2;

/** A small result page — the palette only shows the top few hits. */
const QUICK_CONVERSATION_LIMIT = 5;

/**
 * Classifies a typed (already-trimmed) quick-search string into the scope
 * {@link useQuickConversationSearch} should route it through. Thin re-export
 * of the shared {@link classifyConversationQuery}, kept under this name so
 * the palette's own imports read the same way the other three entity kinds'
 * `classifyQuick*Query` do.
 */
export const classifyQuickConversationQuery = classifyConversationQuery;

/** One hit from the global-search conversation lookup, enough for the palette's result row. */
export interface QuickConversationHit {
  id: string;
  number?: string | null;
  initiatorName?: string;
  initialMessage?: string | null;
}

/**
 * Conversation lookup for the global quick-nav palette. Calls
 * `POST /conversations/search` — same endpoint the project Work-items
 * "Chats" tab uses, just capped to a handful of hits and shaped for the
 * palette instead of a paginated table.
 *
 * Unlike the tab's own search (scoped to one project via `projectIds`), the
 * palette searches across every project the caller can see: `projectIds` is
 * optional on this endpoint (the entity service only validates the ids it's
 * given are well-formed UUIDs; it does not require the array to be
 * non-empty), so it's simply omitted here.
 *
 * Routes the typed text one of two ways (see {@link classifyQuickConversationQuery}):
 * a `CHAT`-number-shaped string goes through as an exact-match, first-class
 * field filter (an indexed lookup); anything else falls back to the same
 * free-text `searchQuery` search the Chats tab uses. Pass `forceFreeText` to
 * opt back into the free-text path even when the query matches the exact
 * pattern — the quick-nav palette's "search in subject and description too"
 * affordance uses this to widen a scoped result.
 *
 * Disabled until the trimmed text reaches {@link QUICK_CONVERSATION_MIN_QUERY_LEN},
 * so opening the palette costs no network.
 */
export function useQuickConversationSearch(
  query: string,
  options?: { forceFreeText?: boolean },
): UseQueryResult<QuickConversationHit[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  const scope = options?.forceFreeText ? "text" : classifyQuickConversationQuery(q);

  return useQuery<QuickConversationHit[], Error>({
    queryKey: [ApiQueryKeys.CONVERSATIONS_SEARCH, "quick-search", q, scope],
    queryFn: async (): Promise<QuickConversationHit[]> => {
      const res = await api.post<
        BeSearchConversationsPayload,
        BeSearchConversationsResponse
      >("/conversations/search", {
        pagination: { offset: 0, limit: QUICK_CONVERSATION_LIMIT },
        filters: scope === "number" ? { number: q } : { searchQuery: q },
      });
      return (res.conversations ?? [])
        .filter((c): c is typeof c & { id: string } => !!c.id)
        .map((c) => ({
          id: c.id,
          number: c.number,
          initiatorName: c.createdBy?.name || c.createdBy?.email || undefined,
          initialMessage: c.initialMessage,
        }));
    },
    enabled: q.length >= QUICK_CONVERSATION_MIN_QUERY_LEN,
    staleTime: 15_000,
  });
}
