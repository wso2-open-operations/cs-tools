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
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { ProductVulnerabilitiesSearchRequest } from "@features/security/types/security";
import type { ProductVulnerabilitiesSearchResponse } from "@features/security/types/security";
import { parseApiResponseMessage } from "@utils/ApiError";

/**
 * Searches product vulnerabilities (POST /products/vulnerabilities/search).
 *
 * @param {ProductVulnerabilitiesSearchRequest} request - Search filters, pagination, and sort.
 * @returns {UseQueryResult<ProductVulnerabilitiesSearchResponse, Error>} Query result.
 */
export function usePostProductVulnerabilitiesSearch(
  request: ProductVulnerabilitiesSearchRequest,
): UseQueryResult<ProductVulnerabilitiesSearchResponse, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProductVulnerabilitiesSearchResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VULNERABILITIES_SEARCH, request],
    queryFn: async (): Promise<ProductVulnerabilitiesSearchResponse> => {
      logger.debug(
        "[usePostProductVulnerabilitiesSearch] Request payload:",
        request,
      );

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/products/vulnerabilities/search`;
      const response = await authFetch(requestUrl, {
        method: "POST",

        body: JSON.stringify(request),
      });

      logger.debug(
        `[usePostProductVulnerabilitiesSearch] Response status: ${response.status}`,
      );

      if (!response.ok) {
        const text = await response.text();
        throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
      }

      const data: ProductVulnerabilitiesSearchResponse = await response.json();
      logger.debug(
        "[usePostProductVulnerabilitiesSearch] Data received:",
        data,
      );
      return data;
    },
    enabled: isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
