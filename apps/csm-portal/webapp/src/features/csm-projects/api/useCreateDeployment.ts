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
  BeDeploymentCreatePayload,
  BeDeploymentCreateResponse,
} from "@api/backend/types";

/**
 * Create a deployment via `POST /deployments`. On success we invalidate the
 * project's deployment list so the table refetches the authoritative state.
 *
 * Contract: per PR #957, the BE accepts the string `type` enum directly
 * (e.g. `"staging"`). The old `typeKey` integer field is gone.
 */
export function useCreateDeployment(
  projectId: string | undefined,
): UseMutationResult<BeDeploymentCreateResponse, Error, BeDeploymentCreatePayload> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeDeploymentCreateResponse, Error, BeDeploymentCreatePayload>({
    mutationFn: (payload): Promise<BeDeploymentCreateResponse> =>
      api.post<BeDeploymentCreatePayload, BeDeploymentCreateResponse>(
        "/deployments",
        payload,
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId ?? ""],
      });
    },
  });
}
