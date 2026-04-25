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

import type { CreateContactRequestDto, EditMeDto, Me, MeDto, Role, User, UserDto, UsersDto } from "@src/types";
import { mutationOptions, queryOptions } from "@tanstack/react-query";
import apiClient from "@src/services/apiClient";

import { PROJECT_USERS_ENDPOINT, USER_ACTIONS_ENDPOINT, USERS_ME_ENDPOINT } from "@config/endpoints";

const getMe = async (): Promise<Me> => {
  const response = (await apiClient.get<MeDto>(USERS_ME_ENDPOINT)).data;
  return toMe(response);
};

const getUsers = async (id: string): Promise<User[]> => {
  const response = (await apiClient.get<UsersDto>(PROJECT_USERS_ENDPOINT(id))).data;
  return response.map(toUser);
};

const createContact = async (id: string, body: CreateContactRequestDto): Promise<void> => {
  await apiClient.post(PROJECT_USERS_ENDPOINT(id), body);
};

const deleteContact = async (id: string, email: string): Promise<void> => {
  await apiClient.delete(USER_ACTIONS_ENDPOINT(id, email));
};

const editContact = async (
  id: string,
  email: string,
  body: Partial<Omit<CreateContactRequestDto, "contactEmail" | "contactFirstName" | "contactLastName">>,
): Promise<void> => {
  await apiClient.patch(USER_ACTIONS_ENDPOINT(id, email), body);
};

const editMe = async (body: Partial<EditMeDto>): Promise<Partial<EditMeDto>> => {
  const response = await apiClient.patch(USERS_ME_ENDPOINT, body);
  return response.data;
};

/* Mappers */
function toMe(dto: MeDto): Me {
  return {
    id: dto.id,
    email: dto.email,
    firstName: dto.firstName,
    lastName: dto.lastName,
    phoneNumber: dto.phoneNumber ?? undefined,
    timezone: dto.timeZone,
    roles: dto.roles,
  };
}

function toUser(dto: UserDto): User {
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
    roles: roles,
    lastActive: new Date(), // TODO: Replace this placeholder with actual last active date from backend when available
  };
}

/* Query Options */
export const users = {
  me: () => queryOptions({ queryKey: ["me"], queryFn: getMe }),

  editMe: () =>
    mutationOptions({
      mutationFn: (body: Partial<EditMeDto>) => editMe(body),
    }),

  all: (projectId: string) => queryOptions({ queryKey: ["users", projectId], queryFn: () => getUsers(projectId) }),

  create: (projectId: string) =>
    mutationOptions({
      mutationFn: (body: CreateContactRequestDto) => createContact(projectId, body),
    }),

  edit: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: (
        body: Partial<Omit<CreateContactRequestDto, "contactEmail" | "contactFirstName" | "contactLastName">>,
      ) => editContact(projectId, email, body),
    }),

  delete: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: () => deleteContact(projectId, email),
    }),
};
