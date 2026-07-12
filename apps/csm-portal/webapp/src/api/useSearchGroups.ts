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

/**
 * Type-ahead group search (`POST /groups/search`) for the "Assignment group"
 * picker on the change-request create form. Disabled until the caller has
 * typed something — the group catalogue isn't loaded up front.
 */
export function useSearchGroups(
  query: string,
  enabled: boolean,
): UseQueryResult<BeGroup[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeGroup[], Error>({
    queryKey: [ApiQueryKeys.GROUPS_SEARCH, q],
    queryFn: async (): Promise<BeGroup[]> => {
      const res = await api.post<BeGroupSearchPayload, BeGroupSearchResponse>(
        "/groups/search",
        { filters: { searchQuery: q }, pagination: { offset: 0, limit: GROUP_SEARCH_LIMIT } },
      );
      return res.groups ?? [];
    },
    enabled: enabled && q.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
