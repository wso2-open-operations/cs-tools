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

import { keepPreviousData, useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeGroup, BeGroupSearchPayload, BeGroupSearchResponse } from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const GROUP_SEARCH_LIMIT = 20;

export interface SearchGroupsOptions {
  /** Only groups that are the support group of at least one service. */
  supportGroupsOnly?: boolean;
}

/**
 * Type-ahead group search (`POST /groups/search`) for the "Assignment group"
 * pickers. Fires as soon as the dropdown opens, even with an empty query, so
 * the picker shows a default page of groups instead of looking broken until
 * the caller types something.
 *
 * `options.supportGroupsOnly` narrows the results to the groups an incident
 * may be created with (see {@link useSearchSupportGroups}); without it the
 * request body and query key are exactly what they always were.
 */
export function useSearchGroups(
  query: string,
  enabled: boolean,
  // A string is what `AsyncEntitySelect` hands every `useSearch` hook as its
  // optional `searchExtra`; groups have no use for one, so it is ignored.
  options?: SearchGroupsOptions | string,
): UseQueryResult<BeGroup[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  const supportGroupsOnly = typeof options === "object" && options.supportGroupsOnly === true;

  return useQuery<BeGroup[], Error>({
    queryKey: supportGroupsOnly
      ? [ApiQueryKeys.GROUPS_SEARCH, q, { supportGroupsOnly }]
      : [ApiQueryKeys.GROUPS_SEARCH, q],
    queryFn: async (): Promise<BeGroup[]> => {
      const res = await api.post<BeGroupSearchPayload, BeGroupSearchResponse>(
        "/groups/search",
        {
          filters: supportGroupsOnly ? { searchQuery: q, supportGroupsOnly } : { searchQuery: q },
          pagination: { offset: 0, limit: GROUP_SEARCH_LIMIT },
        },
      );
      return res.groups ?? [];
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}

/**
 * {@link useSearchGroups} narrowed to service support groups — a stable
 * module-level hook so it can be handed to `AsyncEntitySelect`'s `useSearch`
 * (which must never be an inline closure).
 */
export function useSearchSupportGroups(
  query: string,
  enabled: boolean,
): UseQueryResult<BeGroup[], Error> {
  return useSearchGroups(query, enabled, { supportGroupsOnly: true });
}
