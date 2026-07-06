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
  BeDeploymentUpdatePayload,
  BeDeploymentUpdateResponse,
} from "@api/backend/types";

interface UpdateDeploymentInput {
  deploymentId: string;
  payload: BeDeploymentUpdatePayload;
}

/**
 * Update a deployment via `PATCH /deployments/{id}` — either detail fields
 * (`name`, `type`, `description`) or deactivation (`active: false`). The
 * backend rejects mixing the two, so callers send one shape per call. On
 * success we invalidate the project's deployment list so the table refetches
 * the authoritative state (a deactivated deployment drops out of the active
 * list). `type` is the string enum per PR #957 (no `typeKey` integer).
 */
export function useUpdateDeployment(
  projectId: string | undefined,
): UseMutationResult<BeDeploymentUpdateResponse, Error, UpdateDeploymentInput> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeDeploymentUpdateResponse, Error, UpdateDeploymentInput>({
    mutationFn: ({ deploymentId, payload }): Promise<BeDeploymentUpdateResponse> =>
      api.patch<BeDeploymentUpdatePayload, BeDeploymentUpdateResponse>(
        `/deployments/${encodeURIComponent(deploymentId)}`,
        payload,
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId ?? ""],
      });
    },
  });
}
