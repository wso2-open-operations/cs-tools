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

import type { AdminUserRole, UserSearchFiltersDto } from "@src/types";

export interface UsersFilters {
  roles: AdminUserRole[];
  active: "all" | "active" | "inactive";
}

export const EMPTY_USERS_FILTERS: UsersFilters = { roles: [], active: "all" };

export function countActiveUsersFilters(filters: UsersFilters): number {
  return filters.roles.length + (filters.active !== "all" ? 1 : 0);
}

export function toUserSearchFilters(search: string, filters: UsersFilters): UserSearchFiltersDto {
  return {
    ...(search.length > 0 && { searchQuery: search }),
    ...(filters.roles.length > 0 && { roles: filters.roles }),
    ...(filters.active !== "all" && { active: filters.active === "active" }),
  };
}
