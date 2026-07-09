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
import type {
  BeUser,
  BeUserSearchPayload,
  BeUserSearchResponse,
} from "@api/backend/types";

/** A single page of matches is plenty for a type-ahead picker. */
const USER_SEARCH_LIMIT = 20;

/**
 * Type-ahead user search (`POST /users/search`, `filters.searchQuery`) that
 * returns each match's portal `id` — unlike {@link useDirectoryUsers} (which
 * loads the whole directory keyed by email for the cases assignee filter),
 * this is for pickers that need a user's UUID directly (change-request
 * "Requested by" / "Assigned to"). Disabled until the caller has typed
 * something.
 */
export function useSearchUsersByName(
  query: string,
  enabled: boolean,
): UseQueryResult<BeUser[], Error> {
  const api = useBackendApi();
  const q = query.trim();

  return useQuery<BeUser[], Error>({
    queryKey: [ApiQueryKeys.USERS_SEARCH_BY_NAME, q],
    queryFn: async (): Promise<BeUser[]> => {
      const res = await api.post<BeUserSearchPayload, BeUserSearchResponse>(
        "/users/search",
        { filters: { searchQuery: q }, pagination: { offset: 0, limit: USER_SEARCH_LIMIT } },
      );
      return (res.users ?? []).filter((u) => !!u.id);
    },
    enabled: enabled && q.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}
