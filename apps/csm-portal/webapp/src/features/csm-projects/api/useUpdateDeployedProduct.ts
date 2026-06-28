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
  BeDeployedProductUpdatePayload,
  BeDeployedProductUpdateResponse,
} from "@api/backend/types";

interface UpdateDeployedProductInput {
  /** UUID of the deployed-product record (`DeployedProduct.id`). */
  deployedProductId: string;
  payload: BeDeployedProductUpdatePayload;
}

/**
 * Update a deployed product via
 * `PATCH /deployments/{deploymentId}/products/{productId}`.
 *
 * The BE accepts two mutually exclusive shapes per the `oneOf` spec:
 *  - Detail update: `{ cores?, tps?, description? }` (at least one, no `active`)
 *  - Deactivate: `{ active: false }` (no other fields)
 *
 * The caller is responsible for sending exactly one shape. On success,
 * invalidates the `DEPLOYMENT_PRODUCTS` cache so the panel refetches.
 */
export function useUpdateDeployedProduct(
  deploymentId: string | undefined,
): UseMutationResult<BeDeployedProductUpdateResponse, Error, UpdateDeployedProductInput> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeDeployedProductUpdateResponse, Error, UpdateDeployedProductInput>({
    mutationFn: ({ deployedProductId, payload }): Promise<BeDeployedProductUpdateResponse> =>
      api.patch<BeDeployedProductUpdatePayload, BeDeployedProductUpdateResponse>(
        `/deployments/${encodeURIComponent(deploymentId as string)}/products/${encodeURIComponent(deployedProductId)}`,
        payload,
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_PRODUCTS, deploymentId ?? ""],
      });
    },
  });
}
