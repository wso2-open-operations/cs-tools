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
import { classifyConversationQuery } from "@features/csm-projects/utils/conversationQueryScope";
import type {
  BeConversationState,
  BeConversationView,
  BeSearchConversationsPayload,
  BeSearchConversationsResponse,
} from "@api/backend/types";

/** Zero-indexed page + page size, mirroring MUI `TablePagination`. */
export interface ConversationPagination {
  page: number;
  rowsPerPage: number;
}

/**
 * The subset of `BeSearchConversationsFilters` the Conversations tab exposes
 * to the user (`projectIds` is fixed to the surrounding project, not part of
 * this). All optional/empty by default — an unset filter is simply omitted
 * from the request payload rather than sent as an empty array/false, so a
 * default search stays identical to the pre-filter-bar request.
 */
export interface ConversationSearchFilters {
  states?: BeConversationState[];
  searchQuery?: string;
  createdByMe?: boolean;
}

export interface ConversationSearchResult {
  conversations: BeConversationView[];
  total: number;
}

/**
 * A project's chat sessions, via `POST /conversations/search`, sorted most
 * recently active first. `rowsPerPage` is capped at {@link BE_MAX_PAGE_LIMIT}
 * (the entity service's own documented max for this endpoint). Disabled until
 * a project id is provided.
 *
 * The search text is classified (see {@link classifyConversationQuery}): a
 * `CHAT`-number-shaped query is routed as the exact-match `filters.number`
 * field instead of the free-text `filters.searchQuery` scan, mirroring how
 * the global quick-nav palette (`useQuickConversationSearch`) resolves the
 * same typed string.
 */
export function useSearchConversations(
  projectId: string | undefined,
  pagination: ConversationPagination,
  filters: ConversationSearchFilters = {},
): UseQueryResult<ConversationSearchResult, Error> {
  const api = useBackendApi();
  const limit = Math.min(pagination.rowsPerPage, BE_MAX_PAGE_LIMIT);
  const offset = pagination.page * limit;
  const { states, searchQuery, createdByMe } = filters;
  const trimmedSearch = searchQuery?.trim() || undefined;
  const searchScope = trimmedSearch ? classifyConversationQuery(trimmedSearch) : "text";

  return useQuery<ConversationSearchResult, Error>({
    queryKey: [
      ApiQueryKeys.CONVERSATIONS_SEARCH,
      projectId ?? "",
      pagination.page,
      limit,
      states ?? [],
      trimmedSearch ?? "",
      searchScope,
      createdByMe ?? false,
    ],
    queryFn: async (): Promise<ConversationSearchResult> => {
      const res = await api.post<
        BeSearchConversationsPayload,
        BeSearchConversationsResponse
      >("/conversations/search", {
        filters: {
          projectIds: [projectId ?? ""],
          ...(states && states.length > 0 ? { states } : {}),
          ...(trimmedSearch && searchScope === "number"
            ? { number: trimmedSearch }
            : {}),
          ...(trimmedSearch && searchScope === "text"
            ? { searchQuery: trimmedSearch }
            : {}),
          ...(createdByMe ? { createdByMe: true } : {}),
        },
        sortBy: { field: "updatedOn", order: "desc" },
        pagination: { limit, offset },
      });
      return { conversations: res.conversations ?? [], total: res.total };
    },
    enabled: !!projectId,
    staleTime: 30_000,
  });
}
