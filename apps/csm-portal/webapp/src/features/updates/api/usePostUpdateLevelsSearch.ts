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
import type {
  SearchUpdatesPayload,
  UpdateLevelsSearchResponse,
} from "@features/updates/types/updates";

export function usePostUpdateLevelsSearch(payload: SearchUpdatesPayload | null) {
  const authFetch = useAuthApiClient();

  return useQuery<UpdateLevelsSearchResponse, Error>({
    queryKey: ["updates", "levels-search", payload],
    enabled: Boolean(payload),
    queryFn: async () => {
      if (!payload) throw new Error("payload required");
      const res = await authFetch(`${apiConfig.backendUrl}/updates/levels/search`, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as UpdateLevelsSearchResponse;
    },
  });
}
