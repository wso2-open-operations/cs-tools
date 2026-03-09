// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { CatalogSearchResponse } from "@models/responses";

/**
 * Searches catalogs for a deployed product (POST /deployments/products/:deployedProductId/catalogs/search).
 *
 * @param {string} deployedProductId - The deployed product ID.
 * @returns {UseQueryResult<CatalogSearchResponse, Error>} The query result.
 */
export function useSearchCatalogs(
  deployedProductId: string,
): UseQueryResult<CatalogSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<CatalogSearchResponse, Error>({
    queryKey: [ApiQueryKeys.CATALOGS_SEARCH, deployedProductId],
    queryFn: async (): Promise<CatalogSearchResponse> => {
      logger.debug(
        `Searching catalogs for deployed product ID: ${deployedProductId}`,
      );

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/deployments/products/${deployedProductId}/catalogs/search`;

      const response = await authFetch(requestUrl, {
        method: "POST",

        body: JSON.stringify({}),
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(
          `Error searching catalogs: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
        );
      }

      const data: CatalogSearchResponse = await response.json();
      logger.debug(
        `Catalogs fetched for deployed product ID: ${deployedProductId}`,
        data,
      );
      return data;
    },
    enabled: !!deployedProductId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
