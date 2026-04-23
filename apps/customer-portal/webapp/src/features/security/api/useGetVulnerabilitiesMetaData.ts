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
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { VulnerabilitiesMetaResponse } from "@features/security/types/security";

/**
 * Fetches product vulnerabilities metadata (GET /products/vulnerabilities/meta).
 *
 * @returns {UseQueryResult<VulnerabilitiesMetaResponse, Error>} Query result with severities for filters.
 */
export function useGetVulnerabilitiesMetaData(): UseQueryResult<
  VulnerabilitiesMetaResponse,
  Error
> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<VulnerabilitiesMetaResponse, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VULNERABILITIES_META],
    queryFn: async (): Promise<VulnerabilitiesMetaResponse> => {
      logger.debug("[useGetVulnerabilitiesMetaData] Fetching metadata");

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const requestUrl = `${baseUrl}/products/vulnerabilities/meta`;
      const response = await authFetch(requestUrl, {
        method: "GET",
      });

      if (!response.ok) {
        throw new Error(
          `Error fetching vulnerabilities metadata: ${response.statusText}`,
        );
      }

      const data: VulnerabilitiesMetaResponse = await response.json();
      logger.debug("[useGetVulnerabilitiesMetaData] Data received:", data);
      return data;
    },
    enabled: isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
