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

export type AdminUserType = "internal" | "customer";

// Mirrors the webapp's SnUserRole (apps/csm-portal/webapp/src/features/csm-users/types/csmUsers.ts).
export type AdminUserRole =
  | "internal"
  | "agent"
  | "admin"
  | "commenter"
  | "external"
  | "customer"
  | "customer_admin"
  | "partner"
  | "partner_admin";

export type AdminUserSortField = "name" | "createdOn" | "updatedOn";
export type AdminUserSortOrder = "asc" | "desc";

/** User shape returned by the postgres data source. */
export interface UserDto {
  id: string;
  userName: string;
  firstName: string;
  lastName: string;
  email: string;
  phone?: string | null;
  timezone?: string | null;
  userType: AdminUserType;
  createdAt: string;
  updatedAt: string;
}

/**
 * User shape returned by the ServiceNow data source. No `firstName`/`lastName`
 * (use `name`), no `userType` (use `roles`), no `hasMore` on the envelope.
 */
export interface SnUserDto {
  id: string;
  userName: string;
  name: string;
  email: string;
  timeZone?: string | null;
  active: boolean;
  createdOn: string;
  updatedOn: string;
  roles: string[];
}

export interface UserSearchFiltersDto {
  searchQuery?: string;
  /** ServiceNow data source only. */
  roles?: AdminUserRole[];
  /** ServiceNow data source only. */
  active?: boolean;
}

export interface UserSearchPayloadDto {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  filters?: UserSearchFiltersDto;
  sortBy?: {
    field: AdminUserSortField;
    order?: AdminUserSortOrder;
  };
}

/** `/users/search` returns a `oneOf` (postgres `UserDto` vs ServiceNow `SnUserDto`). */
export interface UserSearchResponseDto {
  users: (UserDto | SnUserDto)[];
  total: number;
  limit: number;
  offset: number;
  /** Absent from the ServiceNow envelope; derived from `offset`/`total` when missing. */
  hasMore?: boolean;
}
