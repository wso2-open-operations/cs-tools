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
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";

interface UserByIdRecord {
  id: string;
  userName: string;
  firstName: string;
  lastName: string;
  email: string;
}

interface UsersByIdsResponse {
  users: UserByIdRecord[];
}

/**
 * Resolves a batch of user ids to display names via POST /users/by-ids.
 * `POST /users/search` with a `userIds` filter looked like the natural
 * reuse here, but that filter is only supported for the ServiceNow data
 * source -- calling it against Postgres (this platform's actual data
 * source) fails with a 400. `/users/by-ids` is the Postgres-safe
 * equivalent, added specifically for this id-to-name use case.
 */
export function useUsersByIds(ids: string[]): UseQueryResult<Map<string, string>, Error> {
  const api = useBackendApi();
  return useQuery<Map<string, string>, Error>({
    queryKey: [ApiQueryKeys.CSM_KB_ARTICLES, "users-by-ids", ...ids].sort() as unknown as unknown[],
    queryFn: async () => {
      if (ids.length === 0) return new Map<string, string>();
      const res = await api.post<{ ids: string[] }, UsersByIdsResponse>("/users/by-ids", { ids });
      const map = new Map<string, string>();
      for (const u of res.users ?? []) {
        map.set(u.id, u.userName || u.email);
      }
      return map;
    },
    enabled: ids.length > 0,
    staleTime: 60_000,
  });
}
