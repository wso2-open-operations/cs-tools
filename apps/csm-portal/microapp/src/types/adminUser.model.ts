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

import type { AdminUserType, SnUserDto, UserDto } from "./adminUser.dto";

/**
 * Source-agnostic row the UI renders. Both {@link UserDto} and {@link SnUserDto}
 * normalize into this so the list doesn't branch on the live data source —
 * mirrors the webapp's NormalizedUser (csmUsers.ts).
 */
export interface AdminUser {
  id: string;
  userName: string;
  name: string;
  email: string;
  timezone: string | null;
  /** Present only from the postgres source. */
  userType?: AdminUserType;
  /** Present only from the ServiceNow source. */
  active?: boolean;
  /** Present only from the ServiceNow source. */
  roles?: string[];
}

function isSnUser(u: UserDto | SnUserDto): u is SnUserDto {
  return "name" in u || "active" in u || "roles" in u;
}

export function toAdminUser(u: UserDto | SnUserDto): AdminUser {
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
