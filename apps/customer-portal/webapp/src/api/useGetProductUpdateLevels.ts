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
import type { ProductUpdateLevelsItem } from "@/types/updates";

/**
 * Fetches product update levels from GET /updates/product-update-levels.
 *
 * @returns {UseQueryResult<ProductUpdateLevelsItem[], Error>} The query result.
 */
export function useGetProductUpdateLevels(): UseQueryResult<
  ProductUpdateLevelsItem[],
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProductUpdateLevelsItem[], Error>({
    queryKey: [ApiQueryKeys.PRODUCT_UPDATE_LEVELS],
    queryFn: async (): Promise<ProductUpdateLevelsItem[]> => {
      logger.debug("Fetching product update levels");

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/updates/product-update-levels`;

        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProductUpdateLevels] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching product update levels: ${response.statusText}`,
          );
        }

        const data: ProductUpdateLevelsItem[] = await response.json();
        logger.debug("[useGetProductUpdateLevels] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProductUpdateLevels] Error:", error);
        throw error;
      }
    },
    enabled: isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
