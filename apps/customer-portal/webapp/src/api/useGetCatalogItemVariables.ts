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
import type { CatalogItemVariablesResponse } from "@models/responses";

/**
 * Fetches catalog item variables (GET /catalogs/:catalogId/items/:itemId).
 *
 * @param {string} catalogId - The catalog ID.
 * @param {string} itemId - The catalog item ID.
 * @returns {UseQueryResult<CatalogItemVariablesResponse, Error>} The query result.
 */
export function useGetCatalogItemVariables(
  catalogId: string,
  itemId: string,
): UseQueryResult<CatalogItemVariablesResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<CatalogItemVariablesResponse, Error>({
    queryKey: [ApiQueryKeys.CATALOG_ITEM_VARIABLES, catalogId, itemId],
    queryFn: async (): Promise<CatalogItemVariablesResponse> => {
      logger.debug(
        `Fetching catalog item variables for catalog ${catalogId}, item ${itemId}`,
      );

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/catalogs/${catalogId}/items/${itemId}`;

      const response = await authFetch(requestUrl, {
        method: "GET",
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(
          `Error fetching catalog item variables: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
        );
      }

      const data: CatalogItemVariablesResponse = await response.json();
      logger.debug(
        `Catalog item variables fetched for catalog ${catalogId}, item ${itemId}`,
        data,
      );
      return data;
    },
    enabled: !!catalogId && !!itemId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
