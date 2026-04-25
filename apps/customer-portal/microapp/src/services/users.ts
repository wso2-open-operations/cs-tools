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

import type {
  CreateContactRequestDto,
  EditContactRequestDto,
  EditMeDto,
  Me,
  MeDto,
  Role,
  User,
  UserDto,
  ValidateContactRequestDto,
  ValidateContactResponseDto,
  UsersDto,
} from "@src/types";
import { mutationOptions, queryOptions } from "@tanstack/react-query";
import apiClient from "@src/services/apiClient";
import { toApiError } from "@utils/ApiError";

import {
  PROJECT_USERS_ENDPOINT,
  PROJECT_USERS_VALIDATION_ENDPOINT,
  USER_ACTIONS_ENDPOINT,
  USERS_ME_ENDPOINT,
} from "@config/endpoints";

const getMe = async (): Promise<Me> => {
  const response = (await apiClient.get<MeDto>(USERS_ME_ENDPOINT)).data;
  return toMe(response);
};

const getUsers = async (id: string): Promise<User[]> => {
  const response = (await apiClient.get<UsersDto>(PROJECT_USERS_ENDPOINT(id))).data;
  return response.map(toUser);
};

const createContact = async (id: string, body: CreateContactRequestDto): Promise<void> => {
  try {
    await apiClient.post(PROJECT_USERS_ENDPOINT(id), body);
  } catch (error) {
    throw toApiError(error, "Failed to invite user. Please try again.");
  }
};

const validateContact = async (id: string, body: ValidateContactRequestDto): Promise<ValidateContactResponseDto> => {
  try {
    return (await apiClient.post<ValidateContactResponseDto>(PROJECT_USERS_VALIDATION_ENDPOINT(id), body)).data;
  } catch (error) {
    throw toApiError(error, "Email validation failed. Please try again.");
  }
};

const deleteContact = async (id: string, email: string): Promise<void> => {
  try {
    await apiClient.delete(USER_ACTIONS_ENDPOINT(id, email));
  } catch (error) {
    throw toApiError(error, "Failed to delete user. Please try again.");
  }
};

const editContact = async (
  id: string,
  email: string,
  body: EditContactRequestDto,
): Promise<void> => {
  try {
    await apiClient.patch(USER_ACTIONS_ENDPOINT(id, email), body);
  } catch (error) {
    throw toApiError(error, "Failed to edit user. Please try again.");
  }
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
  if (dto.isCsAdmin) roles.push("Admin User");
  if (dto.isCsIntegrationUser) roles.push("System User");
  if (dto.isSecurityContact) roles.push("Security User");
  if (dto.isPortalUser ?? !dto.isCsIntegrationUser) roles.push("Portal User");

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

  validate: (projectId: string) =>
    mutationOptions({
      mutationFn: (body: ValidateContactRequestDto) => validateContact(projectId, body),
    }),

  edit: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: (body: EditContactRequestDto) => editContact(projectId, email, body),
    }),

  delete: (projectId: string, email: string) =>
    mutationOptions({
      mutationFn: () => deleteContact(projectId, email),
    }),
};
