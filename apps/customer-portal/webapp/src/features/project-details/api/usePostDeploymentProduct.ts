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
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { PostDeploymentProductVariables } from "@features/project-details/types/projectDetailsApi";

/**
 * Posts a product to a deployment (POST /deployments/:deploymentId/products).
 * On success, invalidates deployment-products queries.
 *
 * @returns {UseMutationResult<void, Error, PostDeploymentProductVariables>} Mutation result.
 */
export function usePostDeploymentProduct(): UseMutationResult<
  void,
  Error,
  PostDeploymentProductVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, PostDeploymentProductVariables>({
    mutationFn: async ({
      deploymentId,
      body,
    }: PostDeploymentProductVariables): Promise<void> => {
      try {
        logger.debug("[usePostDeploymentProduct] Request:", {
          deploymentId,
          productId: body.productId,
          versionId: body.versionId,
        });

        if (!isSignedIn || isAuthLoading) {
          throw new Error(
            "User must be signed in to add a product to deployment",
          );
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/deployments/${deploymentId}/products`;
        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(
          "[usePostDeploymentProduct] Response status:",
          response.status,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error adding product to deployment: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }
      } catch (error) {
        logger.error("[usePostDeploymentProduct] Error:", error);
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
