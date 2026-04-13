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
import { PRODUCT_CLASS } from "@constants/commonConstants";
import type { ProductsResponse } from "@/types/products";

/**
 * Fetches products list (GET /products).
 *
 * @param {object} params - offset and limit.
 * @returns {UseQueryResult<ProductsResponse, Error>} The query result.
 */
export function useGetProducts(params?: {
  offset?: number;
  limit?: number;
}): UseQueryResult<ProductsResponse, Error> {
  const { offset = 0, limit = 10 } = params ?? {};
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProductsResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCTS, offset, limit],
    queryFn: async (): Promise<ProductsResponse> => {
      logger.debug(`Fetching products offset=${offset} limit=${limit}`);

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const searchParams = new URLSearchParams();
        searchParams.set("class", PRODUCT_CLASS.PRODUCT_MODEL);
        searchParams.set("offset", String(offset));
        searchParams.set("limit", String(limit));
        const requestUrl = `${baseUrl}/products?${searchParams.toString()}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(`[useGetProducts] Response status: ${response.status}`);

        if (!response.ok) {
          throw new Error(`Error fetching products: ${response.statusText}`);
        }

        const data: ProductsResponse = await response.json();
        logger.debug("[useGetProducts] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProducts] Error:", error);
        throw error;
      }
    },
    enabled: isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  });
}
