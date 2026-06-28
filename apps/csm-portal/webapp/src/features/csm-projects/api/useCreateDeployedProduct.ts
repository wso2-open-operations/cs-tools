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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeDeployedProductCreatePayload,
  BeDeployedProductCreateResponse,
} from "@api/backend/types";

/**
 * Create a deployed product under a deployment via
 * `POST /deployments/{id}/products`. On success, invalidates the
 * `DEPLOYMENT_PRODUCTS` cache for this deployment so the panel refetches.
 */
export function useCreateDeployedProduct(
  deploymentId: string | undefined,
): UseMutationResult<BeDeployedProductCreateResponse, Error, BeDeployedProductCreatePayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeDeployedProductCreateResponse, Error, BeDeployedProductCreatePayload>({
    mutationFn: (payload): Promise<BeDeployedProductCreateResponse> =>
      api.post<BeDeployedProductCreatePayload, BeDeployedProductCreateResponse>(
        `/deployments/${encodeURIComponent(deploymentId as string)}/products`,
        payload,
      ),
    onSuccess: () => {
      // Invalidate both the "records" sub-cache (DeployedProductsPanel) and
      // the options sub-cache (useDeployedProductOptions in case-create).
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId ?? ""],
      });
    },
  });
}
