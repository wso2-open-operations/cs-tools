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
import { useMockConfig } from "@providers/MockConfigProvider";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys, API_MOCK_DELAY } from "@constants/apiConstants";
import { addApiHeaders } from "@utils/apiUtils";
import { getMockDeploymentProducts } from "@models/mockFunctions";
import type { DeploymentProductItem } from "@models/responses";

export interface FetchDeploymentProductsOptions {
  getIdToken: () => Promise<string>;
  isMockEnabled: boolean;
}

/**
 * Fetcher for deployment products. Used by useGetDeploymentsProducts and by
 * useQueries when fetching products for multiple deployments.
 *
 * @param {string} deploymentId - The deployment ID.
 * @param {FetchDeploymentProductsOptions} options - getIdToken and isMockEnabled.
 * @returns {Promise<DeploymentProductItem[]>} Deployment products array.
 */
export async function fetchDeploymentProducts(
  deploymentId: string,
  options: FetchDeploymentProductsOptions,
): Promise<DeploymentProductItem[]> {
  const { getIdToken, isMockEnabled } = options;

  if (isMockEnabled) {
    await new Promise((resolve) => setTimeout(resolve, API_MOCK_DELAY));
    return getMockDeploymentProducts(deploymentId);
  }

  const idToken = await getIdToken();
  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const requestUrl = `${baseUrl}/deployments/${deploymentId}/products`;

  const response = await fetch(requestUrl, {
    method: "GET",
    headers: addApiHeaders(idToken),
  });

  if (!response.ok) {
    throw new Error(
      `Error fetching deployment products: ${response.statusText}`,
    );
  }

  return response.json();
}

/**
 * Fetches products for a deployment (GET /deployments/:deploymentId/products).
 * Use product.label for case classification API productDetails. Pass deploymentId
 * from useGetProjectDeployments response (each deployment's id).
 *
 * @param {string} deploymentId - The deployment ID (from project deployments).
 * @returns {UseQueryResult<DeploymentProductItem[], Error>} The query result.
 */
export function useGetDeploymentsProducts(
  deploymentId: string,
): UseQueryResult<DeploymentProductItem[], Error> {
  const logger = useLogger();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useQuery<DeploymentProductItem[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId, isMockEnabled],
    queryFn: async (): Promise<DeploymentProductItem[]> => {
      logger.debug(
        `Fetching deployment products for deployment ID: ${deploymentId}, mock: ${isMockEnabled}`,
      );
      const data = await fetchDeploymentProducts(deploymentId, {
        getIdToken,
        isMockEnabled,
      });
      logger.debug(
        `Deployment products fetched for deployment ID: ${deploymentId}`,
        data,
      );
      return data;
    },
    enabled:
      !!deploymentId && (isMockEnabled || (isSignedIn && !isAuthLoading)),
    staleTime: 5 * 60 * 1000,
  });
}
