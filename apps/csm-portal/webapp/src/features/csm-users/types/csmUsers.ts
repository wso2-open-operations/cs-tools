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

export type UserType = "internal" | "customer";

/**
 * Roles as modelled by the ServiceNow data source. `internal`/`agent`/`admin`
 * are WSO2-internal staff; the rest are external (customer/partner) contacts.
 */
export type SnUserRole =
  | "internal"
  | "agent"
  | "admin"
  | "commenter"
  | "external"
  | "customer"
  | "customer_admin"
  | "partner"
  | "partner_admin";

/** Internal-facing roles: the translation of the old `userType === "internal"`. */
export const INTERNAL_USER_ROLES: SnUserRole[] = ["internal", "agent", "admin"];

export type UserSortField = "name" | "createdOn" | "updatedOn";
export type UserSortOrder = "asc" | "desc";

/** User shape returned by the postgres data source. */
export interface User {
  id: string;
  userName: string;
  firstName: string;
  lastName: string;
  email: string;
  phone?: string | null;
  timezone?: string | null;
  userType: UserType;
  createdAt: string;
  updatedAt: string;
}

/**
 * User shape returned by the ServiceNow data source. No `firstName`/`lastName`
 * (use `name`), no `userType` (use `roles`), no `hasMore` on the envelope.
 */
export interface SnUser {
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

export interface UserSearchFilters {
  searchQuery?: string;
  /** ServiceNow data source only. */
  roles?: SnUserRole[];
  userNames?: string[];
  emails?: string[];
  /** ServiceNow data source only. */
  active?: boolean | null;
}

export interface UserSortBy {
  /** ServiceNow data source only. */
  field: UserSortField;
  order?: UserSortOrder;
}

export interface SearchUsersRequest {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  filters?: UserSearchFilters;
  sortBy?: UserSortBy;
}

/** Postgres-data-source response envelope. */
export interface UserSearchResponse {
  users: User[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

/** ServiceNow-data-source response envelope (no `hasMore`). */
export interface SnUserSearchResponse {
  users: SnUser[];
  total: number;
  limit: number;
  offset: number;
}

/** Either data source's raw envelope; `/users/search` returns a `oneOf`. */
export type SearchUsersResponse = UserSearchResponse | SnUserSearchResponse;

/**
 * Source-agnostic row the UI renders. Both `User` and `SnUser` normalize into
 * this so screens don't branch on the live data source.
 */
export interface NormalizedUser {
  id: string;
  userName: string;
  name: string;
  email: string;
  timezone: string | null;
  /** Present only from the postgres source. */
  userType?: UserType;
  /** Present only from the ServiceNow source. */
  active?: boolean;
  /** Present only from the ServiceNow source. */
  roles?: string[];
}

export interface NormalizedUserSearchResult {
  users: NormalizedUser[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

function isSnUser(u: User | SnUser): u is SnUser {
  return "name" in u || "active" in u || "roles" in u;
}

/** Maps either source's user shape into {@link NormalizedUser}. */
export function normalizeUser(u: User | SnUser): NormalizedUser {
  if (isSnUser(u)) {
    return {
      id: u.id,
      userName: u.userName,
      name: u.name?.trim() || "",
      email: u.email,
      timezone: u.timeZone ?? null,
      active: u.active,
      roles: u.roles,
    };
  }
  return {
    id: u.id,
    userName: u.userName,
    name: `${u.firstName ?? ""} ${u.lastName ?? ""}`.trim(),
    email: u.email,
    timezone: u.timezone ?? null,
    userType: u.userType,
  };
}

/** Normalizes either response envelope; derives `hasMore` when absent. */
export function normalizeUserSearchResponse(
  res: SearchUsersResponse,
): NormalizedUserSearchResult {
  const users = (res.users ?? []).map((u) => normalizeUser(u));
  const hasMore =
    "hasMore" in res
      ? res.hasMore
      : res.offset + users.length < res.total;
  return { users, total: res.total, limit: res.limit, offset: res.offset, hasMore };
}
