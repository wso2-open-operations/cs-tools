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

import type { MeDto, UserDto } from "@features/users/types/user.dto";
import type { Me, Role, User } from "@features/users/types/user.model";

export function toMe(dto: MeDto): Me {
  return {
    id: dto.id,
    email: dto.email,
    firstName: dto.firstName,
    lastName: dto.lastName,
    phoneNumber: dto.phoneNumber ?? undefined,
    timezone: dto.timeZone ?? undefined,
    roles: dto.roles,
  };
}

export function toUser(dto: UserDto): User {
  const roles: Role[] = [];
  if (dto.isCsAdmin) roles.push("Admin");
  if (dto.isSecurityContact) roles.push("System User");
  else roles.push("Portal User");

  return {
    id: dto.id,
    email: dto.email,
    firstName: dto.firstName,
    lastName: dto.lastName,
    status: dto.membershipStatus === "REGISTERED" ? "registered" : "invited",
    roles,
    lastActive: new Date(), // placeholder until backend provides actual last active date
  };
}
