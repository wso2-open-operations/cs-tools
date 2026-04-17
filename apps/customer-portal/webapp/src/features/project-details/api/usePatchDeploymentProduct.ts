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

import {
  useMutation,
  useQueryClient,
  type UseMutationResult,
} from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/utils/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@/constants/apiConstants";
import type { PatchDeploymentProductVariables } from "@features/project-details/types/projectDetailsApi";

/**
 * Patches a product in a deployment (PATCH /deployments/:deploymentId/products/:productId).
 * On success, invalidates deployment-products queries.
 *
 * @returns {UseMutationResult<void, Error, PatchDeploymentProductVariables>} Mutation result.
 */
export function usePatchDeploymentProduct(): UseMutationResult<
  void,
  Error,
  PatchDeploymentProductVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, PatchDeploymentProductVariables>({
    mutationFn: async ({
      deploymentId,
      productId,
      body,
    }: PatchDeploymentProductVariables): Promise<void> => {
      try {
        logger.debug("[usePatchDeploymentProduct] Request:", {
          deploymentId,
          productId,
          cores: body.cores,
          tps: body.tps,
          updatesCount: body.updates?.length,
        });

        if (!isSignedIn || isAuthLoading) {
          throw new Error(
            "User must be signed in to update a deployment product",
          );
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/deployments/${deploymentId}/products/${productId}`;
        const response = await authFetch(requestUrl, {
          method: "PATCH",

          body: JSON.stringify(body),
        });

        logger.debug(
          "[usePatchDeploymentProduct] Response status:",
          response.status,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error updating deployment product: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }
      } catch (error) {
        logger.error("[usePatchDeploymentProduct] Error:", error);
        throw error;
      }
    },
    onSuccess: (_data, { deploymentId }) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId],
      });
    },
  });
}
