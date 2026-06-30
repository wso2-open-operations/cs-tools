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

import { useQuery } from "@tanstack/react-query";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { apiConfig } from "@config/apiConfig";
import { ApiError, parseApiResponseMessage } from "@utils/ApiError";

// Matches csm-portal-backend openapi UserResponse. `id` is the caller's
// platform UUID, consumed by the cases assignee filter to resolve the `@me`
// sentinel. The BFF is expected to populate it shortly (currently a TODO,
// blocked on entity wiring, alongside firstName/lastName/timeZone/roles); it
// stays optional so `@me` degrades gracefully until the field lands.
export interface UsersMeResponse {
  id?: string;
  email?: string;
  phoneNumber?: string;
  lastPasswordUpdateTime?: string;
}

export function useGetUsersMe() {
  const authFetch = useAuthApiClient();

  return useQuery<UsersMeResponse, Error>({
    queryKey: ["users-me"],
    queryFn: async () => {
      const res = await authFetch(`${apiConfig.backendUrl}/users/me`);
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as UsersMeResponse;
    },
    // The signed-in user's profile is effectively static for a session; this is
    // fetched once app-wide via CurrentUserProvider, so keep it fresh to avoid
    // refetches on incidental remounts.
    staleTime: 5 * 60_000,
  });
}
