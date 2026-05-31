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
import type { SearchDeployedProductsResponse } from "@features/support/types/case";

interface UseSearchDeployedProductsArgs {
  deploymentId: string | undefined;
  limit?: number;
  offset?: number;
}

/**
 * Search deployed products for a specific deployment.
 * Backend route: POST /deployments/{id}/products/search
 */
export function useSearchDeployedProducts({
  deploymentId,
  limit = 100,
  offset = 0,
}: UseSearchDeployedProductsArgs) {
  const authFetch = useAuthApiClient();

  return useQuery<SearchDeployedProductsResponse, Error>({
    queryKey: ["deployed-products-search", deploymentId, limit, offset],
    queryFn: async () => {
      const res = await authFetch(
        `${apiConfig.backendUrl}/deployments/${deploymentId}/products/search`,
        {
          method: "POST",
          body: JSON.stringify({ pagination: { limit, offset } }),
        },
      );
      if (!res.ok) {
        const body = await res.text();
        throw new ApiError(
          res.status,
          res.statusText,
          parseApiResponseMessage(body, res.status, res.statusText),
        );
      }
      return (await res.json()) as SearchDeployedProductsResponse;
    },
    placeholderData: keepPreviousData,
    enabled: Boolean(deploymentId),
  });
}
