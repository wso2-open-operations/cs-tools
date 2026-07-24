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

import type { UserMeDto } from "./user.dto";

export interface UserProfile {
  id: string | null;
  email: string;
  fullName: string;
  phoneNumber: string | null;
  timeZone: string | null;
  /** Platform roles from `GET /users/me`, used to gate role-specific features
   * (e.g. the time-card Approvals tab). Empty when the user has none. */
  roles: string[];
}

export function toUserProfile(dto: UserMeDto): UserProfile {
  return {
    id: dto.id ?? null,
    email: dto.email,
    fullName: [dto.firstName, dto.lastName].filter(Boolean).join(" ").trim() || dto.email,
    phoneNumber: dto.phoneNumber ?? null,
    timeZone: dto.timeZone ?? null,
    roles: dto.roles ?? [],
  };
}
