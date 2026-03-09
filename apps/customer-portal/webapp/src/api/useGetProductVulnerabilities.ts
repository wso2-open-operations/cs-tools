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
import type { ProductVulnerability } from "@models/responses";

/**
 * Fetches a single product vulnerability by vulnerabilityId (GET /products/vulnerabilities/:vulnerabilityId).
 *
 * @param {string} vulnerabilityId - The vulnerability id (e.g. XRAY-999003).
 * @returns {UseQueryResult<ProductVulnerability, Error>} Query result with product vulnerability detail.
 */
export function useGetProductVulnerabilities(
  vulnerabilityId: string,
): UseQueryResult<ProductVulnerability, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProductVulnerability, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VULNERABILITY, vulnerabilityId],
    queryFn: async (): Promise<ProductVulnerability> => {
      logger.debug(
        `Fetching product vulnerability: vulnerabilityId=${vulnerabilityId}`,
      );

      try {
        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/products/vulnerabilities/${encodeURIComponent(vulnerabilityId)}`;
        const response = await authFetch(requestUrl, {
          method: "GET",
        });

        logger.debug(
          `[useGetProductVulnerabilities] Response status: ${response.status}`,
        );

        if (!response.ok) {
          throw new Error(
            `Error fetching product vulnerability: ${response.status} ${response.statusText}`,
          );
        }

        const data: ProductVulnerability = await response.json();
        logger.debug("[useGetProductVulnerabilities] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[useGetProductVulnerabilities] Error:", error);
        throw error;
      }
    },
    enabled: !!vulnerabilityId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
