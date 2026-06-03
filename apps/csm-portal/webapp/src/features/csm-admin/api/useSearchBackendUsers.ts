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
import { isMockMode, useBackendApi } from "@api/backend/client";
import type {
  BeUserSearchPayload,
  BeUserSearchResponse,
} from "@api/backend/types";
import { getMockUsers } from "@features/csm-admin/api/mocks/adminMocks";

/**
 * Backend-direct user search via `POST /users/search`. Returns a paginated
 * list of users keyed off `searchQuery` so the assignee autocomplete in the
 * cases filter can populate against the real directory.
 *
 * In MOCK mode the response is built from the seeded admin user list with a
 * naive case-insensitive substring match. The shape stays compatible with
 * the real response so consumers don't need to branch.
 */
export function useSearchBackendUsers(
  payload: BeUserSearchPayload = {},
  options: { enabled?: boolean } = {},
): UseQueryResult<BeUserSearchResponse, Error> {
  const api = useBackendApi();

  return useQuery<BeUserSearchResponse, Error>({
    queryKey: [ApiQueryKeys.BACKEND_USERS_SEARCH, payload],
    queryFn: async () => {
      if (isMockMode()) {
        const q = (payload.searchQuery ?? "").trim().toLowerCase();
        const offset = payload.pagination?.offset ?? 0;
        const limit = payload.pagination?.limit ?? 20;
        const all = getMockUsers();
        const matched = q
          ? all.filter(
              (u) =>
                u.name.toLowerCase().includes(q) ||
                u.email.toLowerCase().includes(q),
            )
          : all;
        const page = matched.slice(offset, offset + limit);
        return {
          users: page.map((u) => ({
            id: u.id,
            userName: u.email,
            firstName: u.name.split(" ")[0],
            lastName: u.name.split(" ").slice(1).join(" "),
            email: u.email,
            userType: "internal" as const,
          })),
          total: matched.length,
          limit,
          offset,
          hasMore: offset + page.length < matched.length,
        };
      }
      return api.post<BeUserSearchPayload, BeUserSearchResponse>(
        "/users/search",
        payload,
      );
    },
    staleTime: 30_000,
    enabled: options.enabled ?? true,
  });
}
