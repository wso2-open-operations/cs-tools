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

import { useAsgardeo } from "@asgardeo/react";
import { useQuery } from "@tanstack/react-query";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { InstanceMetricsRequest } from "@features/project-details/types/usage";
import type { InstanceMetricsResponse } from "@features/project-details/types/usage";

/**
 * Searches instance metrics for a deployed product within a date range.
 *
 * @param {string | undefined} deployedProductId - The deployed product ID.
 * @param {InstanceMetricsRequest} payload - Required date range filters.
 * @returns {UseQueryResult<InstanceMetricsResponse>} React Query result.
 */
export default function usePostDeployedProductInstancesMetricsSearch(
  deployedProductId: string | undefined,
  payload: InstanceMetricsRequest,
) {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();

  const authFetch = useAuthApiClient();

  return useQuery<InstanceMetricsResponse>({
    queryKey: [
      ApiQueryKeys.DEPLOYED_PRODUCT_INSTANCE_METRICS,
      deployedProductId,
      payload,
    ],
    queryFn: async () => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(
        `${baseUrl}/deployments/products/${deployedProductId}/instances/metrics/search`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) {
        throw new Error(
          `Failed to search deployed product instance metrics: ${response.status}`,
        );
      }
      return response.json() as Promise<InstanceMetricsResponse>;
    },
    enabled: !!deployedProductId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
