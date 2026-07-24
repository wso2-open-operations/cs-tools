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
import apiClient from "./apiClient";

interface CurrentUserIdDto {
  id?: string;
}

// Only the platform user id is needed here (for the "Assigned to me" case filter), so this stays
// deliberately smaller than a full user-profile service.
const getCurrentUserId = async (): Promise<string | null> => {
  const { data } = await apiClient.get<CurrentUserIdDto>(USERS_ME_ENDPOINT);
  return data.id ?? null;
};

export const currentUser = {
  id: () =>
    queryOptions({
      queryKey: ["currentUser", "id"],
      queryFn: getCurrentUserId,
      staleTime: 5 * 60_000,
    }),
};
