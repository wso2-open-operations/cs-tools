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

import { infiniteQueryOptions } from "@tanstack/react-query";
import { USERS_SEARCH_ENDPOINT } from "@config/endpoints";
import type { UserSearchFiltersDto, UserSearchPayloadDto, UserSearchResponseDto } from "@src/types";
import { toAdminUser, type AdminUser } from "@src/types";
import apiClient from "./apiClient";

export interface AdminUserSearchResult {
  users: AdminUser[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

const searchUsers = async (payload: UserSearchPayloadDto = {}): Promise<AdminUserSearchResult> => {
  const { data } = await apiClient.post<UserSearchResponseDto>(USERS_SEARCH_ENDPOINT, payload);
  const users = data.users.map(toAdminUser);
  // The ServiceNow envelope omits `hasMore`; derive it the same way the webapp's
  // normalizeUserSearchResponse does (csmUsers.ts).
  const hasMore = data.hasMore ?? data.offset + users.length < data.total;
  return { users, total: data.total, limit: data.limit, offset: data.offset, hasMore };
};

const ADMIN_USERS_PAGE_LIMIT = 20;

export const adminUsers = {
  // Mirrors the webapp's useSearchUsers.ts (POST /users/search), paged via infinite scroll —
  // the mobile equivalent of CsmUsersPage's TablePagination.
  infinite: (filters: UserSearchFiltersDto) =>
    infiniteQueryOptions({
      queryKey: ["admin-users", "infinite", filters],
      queryFn: ({ pageParam }) =>
        searchUsers({
          filters,
          sortBy: { field: "name", order: "asc" },
          pagination: { offset: pageParam, limit: ADMIN_USERS_PAGE_LIMIT },
        }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => (lastPage.hasMore ? lastPage.offset + lastPage.limit : undefined),
    }),
};
