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

import apiClient from "@infrastructure/api/client";
import type { CreateContactRequestDto, EditMeDto, MeDto, UsersDto } from "@features/users/types/user.dto";
import type { Me, User } from "@features/users/types/user.model";
import { toMe, toUser } from "@features/users/mappers/user.mapper";
import { PROJECT_USERS_ENDPOINT, USER_ACTIONS_ENDPOINT, USERS_ME_ENDPOINT } from "@config/endpoints";

export const getMe = async (): Promise<Me> => {
  const response = (await apiClient.get<MeDto>(USERS_ME_ENDPOINT)).data;
  return toMe(response);
};

export const getUsers = async (id: string): Promise<User[]> => {
  const response = (await apiClient.get<UsersDto>(PROJECT_USERS_ENDPOINT(id))).data;
  return response.map(toUser);
};

export const createContact = async (id: string, body: CreateContactRequestDto): Promise<void> => {
  await apiClient.post(PROJECT_USERS_ENDPOINT(id), body);
};

export const deleteContact = async (id: string, email: string): Promise<void> => {
  await apiClient.delete(USER_ACTIONS_ENDPOINT(id, email));
};

export const editContact = async (
  id: string,
  email: string,
  body: Partial<Omit<CreateContactRequestDto, "contactEmail" | "contactFirstName" | "contactLastName">>,
): Promise<void> => {
  await apiClient.patch(USER_ACTIONS_ENDPOINT(id, email), body);
};

export const editMe = async (body: Partial<EditMeDto>): Promise<Partial<EditMeDto>> => {
  return (await apiClient.patch(USERS_ME_ENDPOINT, body)).data;
};
