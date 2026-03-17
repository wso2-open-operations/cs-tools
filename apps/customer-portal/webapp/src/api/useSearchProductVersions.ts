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
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ProductVersionsSearchResponse } from "@models/responses";
import type { ProductVersionsSearchRequest } from "@models/requests";

/**
 * Fetches product versions (POST /products/:productId/versions/search).
 *
 * @param {string} productId - The product ID.
 * @param {object} params - pagination (limit, offset).
 * @returns {UseQueryResult<ProductVersionsSearchResponse, Error>} The query result.
 */
export function useSearchProductVersions(
  productId: string,
  params?: { limit?: number; offset?: number },
): UseQueryResult<ProductVersionsSearchResponse, Error> {
  const { limit = 10, offset = 0 } = params ?? {};
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProductVersionsSearchResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VERSIONS_SEARCH, productId, limit, offset],
    queryFn: async (): Promise<ProductVersionsSearchResponse> => {
      logger.debug(
        `Searching versions for product ${productId}, offset=${offset} limit=${limit}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/products/${productId}/versions/search`;
        const body: ProductVersionsSearchRequest = {
          pagination: { limit, offset },
        };

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(
          `[useSearchProductVersions] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error searching product versions: ${response.statusText}`,
          );
        }

        const data: ProductVersionsSearchResponse = await response.json();
        logger.debug("[useSearchProductVersions] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useSearchProductVersions] Error:", error);
        throw error;
      }
    },
    enabled: !!productId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
