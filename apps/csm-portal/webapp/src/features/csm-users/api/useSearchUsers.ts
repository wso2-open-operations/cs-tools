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

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { apiConfig } from "@config/apiConfig";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";
import type {
  SearchUsersRequest,
  SearchUsersResponse,
} from "@features/csm-users/types/csmUsers";

export function useSearchUsers(request: SearchUsersRequest) {
  const authFetch = useAuthApiClient();

  return useQuery<SearchUsersResponse, Error>({
    queryKey: ["csm-users-search", request],
    queryFn: async () => {
      const res = await authFetch(`${apiConfig.backendUrl}/users/search`, {
        method: "POST",
        body: JSON.stringify(request),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as SearchUsersResponse;
    },
    placeholderData: keepPreviousData,
  });
}
