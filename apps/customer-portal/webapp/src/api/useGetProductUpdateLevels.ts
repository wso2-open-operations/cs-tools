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
import { getMockProductUpdateLevels } from "@models/mockFunctions";
import { useMockConfig } from "@providers/MockConfigProvider";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys, API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import type { ProductUpdateLevelsResponse } from "@models/responses";

/**
 * Fetches product update levels from GET /updates/product-update-levels.
 * Returns mock data when mock is enabled.
 *
 * @returns {UseQueryResult<ProductUpdateLevelsResponse, Error>} The query result.
 */
export function useGetProductUpdateLevels(): UseQueryResult<
  ProductUpdateLevelsResponse,
  Error
> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<ProductUpdateLevelsResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_UPDATE_LEVELS, isMockEnabled],
    queryFn: async (): Promise<ProductUpdateLevelsResponse> => {
      logger.debug(`Fetching product update levels, mock: ${isMockEnabled}`);

      if (isMockEnabled) {
        await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));

        const data = getMockProductUpdateLevels();

        logger.debug("Product update levels fetched successfully (mock)", data);

        return data;
      }

      try {
        const idToken = await getIdToken();
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/updates/product-update-levels`;

        const response = await fetch(requestUrl, {
          method: "GET",
          headers: addApiHeaders(idToken),
        });

        logger.debug(
          `[useGetProductUpdateLevels] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching product update levels: ${response.statusText}`,
          );
        }

        const data: ProductUpdateLevelsResponse = await response.json();
        logger.debug("[useGetProductUpdateLevels] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProductUpdateLevels] Error:", error);
        throw error;
      }
    },
    enabled: isMockEnabled || (isSignedIn && !isAuthLoading),
    staleTime: 5 * 60 * 1000,
  });
}
