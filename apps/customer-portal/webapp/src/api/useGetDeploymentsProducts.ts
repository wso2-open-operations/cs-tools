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
import type {
  DeploymentProductItem,
  DeployedProductsResponsePayload,
} from "@models/responses";
import { isDeployedProductsResponse } from "@models/responses";

export type FetchFn = (url: string, init?: RequestInit) => Promise<Response>;

export interface FetchDeploymentProductsOptions {
  fetchFn: FetchFn;
}

/**
 * Fetcher for deployment products. Used by useGetDeploymentsProducts and by
 * useQueries when fetching products for multiple deployments.
 *
 * @param {string} deploymentId - The deployment ID.
 * @param {FetchDeploymentProductsOptions} options - fetchFn.
 * @returns {Promise<DeployedProductsResponse>} Deployment products response.
 */
export async function fetchDeploymentProducts(
  deploymentId: string,
  options: FetchDeploymentProductsOptions & {
    offset?: number;
    limit?: number;
  },
): Promise<DeployedProductsResponsePayload> {
  const { fetchFn, offset = 0, limit = 10 } = options;

  const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;

  if (!baseUrl) {
    throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
  }

  const searchParams = new URLSearchParams();
  searchParams.set("offset", String(offset));
  searchParams.set("limit", String(limit));

  const requestUrl = `${baseUrl}/deployments/${deploymentId}/products?${searchParams.toString()}`;

  const response = await fetchFn(requestUrl, { method: "GET" });

  if (!response.ok) {
    throw new Error(
      `Error fetching deployment products: ${response.statusText}`,
    );
  }

  const data = (await response.json()) as DeployedProductsResponsePayload;
  return data;
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
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useQuery<DeploymentProductItem[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId],
    queryFn: async (): Promise<DeploymentProductItem[]> => {
      logger.debug(
        `Fetching deployment products for deployment ID: ${deploymentId}`,
      );
      const data = await fetchDeploymentProducts(deploymentId, {
        fetchFn: authFetch,
      });
      logger.debug(
        `Deployment products fetched for deployment ID: ${deploymentId}`,
        data,
      );

      if (Array.isArray(data)) {
        return data;
      }

      if (isDeployedProductsResponse(data)) {
        return data.deployedProducts;
      }

      logger.warn(
        "Unexpected deployment products response shape, returning empty array.",
        data,
      );
      return [];
    },
    enabled: !!deploymentId && isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });
}
