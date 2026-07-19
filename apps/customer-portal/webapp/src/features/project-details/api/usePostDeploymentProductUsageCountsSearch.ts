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
import type {
  InstanceMetricsRequest,
  ProductUsageCountsResponse,
} from "@features/project-details/types/usage";

/**
 * Searches aggregated usage-count metrics (e.g. transactions, users, orgs) for a
 * single product within a deployment.
 *
 * @param {string | undefined} deploymentId - The deployment ID.
 * @param {string | undefined} productId - The deployed product ID.
 * @param {InstanceMetricsRequest} payload - Required date range filters.
 * @returns {UseQueryResult<ProductUsageCountsResponse>} React Query result.
 */
export default function usePostDeploymentProductUsageCountsSearch(
  deploymentId: string | undefined,
  productId: string | undefined,
  payload: InstanceMetricsRequest,
) {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<ProductUsageCountsResponse>({
    queryKey: [
      ApiQueryKeys.DEPLOYMENT_PRODUCT_USAGE_COUNTS,
      deploymentId,
      productId,
      payload,
    ],
    queryFn: async () => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL ?? "";
      const response = await authFetch(
        `${baseUrl}/deployments/${deploymentId}/products/${productId}/metrics/usage-counts/search`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      if (!response.ok) {
        throw new Error(
          `Failed to search deployment product usage counts: ${response.status}`,
        );
      }
      return response.json() as Promise<ProductUsageCountsResponse>;
    },
    enabled: !!deploymentId && !!productId && isSignedIn && !isAuthLoading,
    staleTime: 0,
  });
}
