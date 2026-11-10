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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeAppConfigResponse } from "@api/backend/types";

/**
 * Small, static, env-var-configured values for chrome with no page/resource
 * of its own (today just `reportIssueUrl`) via `GET /app-config`. Registered
 * on the backend's PermAuthenticated, like `/users/me`, so this resolves
 * even for a signed-in caller not yet granted a portal role — unconditional,
 * no `enabled` gate, with a long `staleTime` since this rarely changes.
 */
export function useGetAppConfig(): UseQueryResult<BeAppConfigResponse, Error> {
  const api = useBackendApi();

  return useQuery<BeAppConfigResponse, Error>({
    queryKey: [ApiQueryKeys.APP_CONFIG],
    queryFn: async (): Promise<BeAppConfigResponse> =>
      (await api.get<BeAppConfigResponse>("/app-config")) ?? { reportIssueUrl: "" },
    staleTime: 5 * 60_000,
  });
}
