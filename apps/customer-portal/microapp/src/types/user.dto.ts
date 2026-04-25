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

export interface MeDto {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  phoneNumber: string | null;
  timeZone: string;
  roles: string[];
}

export type UsersDto = UserDto[];

export interface UserDto {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  isCsAdmin: boolean;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
  membershipStatus: string;
}

export interface CreateContactRequestDto {
  contactEmail: string;
  contactFirstName: string;
  contactLastName: string;
  isCsIntegrationUser: boolean;
  isSecurityContact: boolean;
}

export interface EditMeDto {
  phoneNumber: string;
  timeZone: string;
}
