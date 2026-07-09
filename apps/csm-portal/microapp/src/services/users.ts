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

import { queryOptions } from "@tanstack/react-query";
import { USERS_ME_ENDPOINT } from "@config/endpoints";
import type { UserMeDto, UserMeUpdateDto } from "@src/types";
import { toUserProfile, type UserProfile } from "@src/types";
import apiClient from "./apiClient";

const getMe = async (): Promise<UserProfile> => {
  const { data } = await apiClient.get<UserMeDto>(USERS_ME_ENDPOINT);
  return toUserProfile(data);
};

// PATCH /users/me accepts phoneNumber and timeZone; the response echoes back only the field(s) that were updated.
const updateMe = async (payload: UserMeUpdateDto): Promise<UserMeUpdateDto> => {
  const { data } = await apiClient.patch<UserMeUpdateDto>(USERS_ME_ENDPOINT, payload);
  return data;
};

export const users = {
  me: () =>
    queryOptions({
      queryKey: ["users", "me"],
      queryFn: getMe,
    }),

  updateMe,
};
