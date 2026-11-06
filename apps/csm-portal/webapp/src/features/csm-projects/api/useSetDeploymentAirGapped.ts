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
  BeDeploymentAirGappedPayload,
  BeDeploymentAirGappedResponse,
} from "@api/backend/types";

interface SetDeploymentAirGappedInput {
  deploymentId: string;
  airGapped: boolean;
}

/**
 * Mark a deployment air-gapped, or clear the mark, via
 * `PUT /deployments/{id}/air-gapped`. Licences issued for an air-gapped
 * deployment tell its products not to publish usage online; licences already
 * downloaded are unchanged. On success the project's deployment list is
 * invalidated so the table shows the stored value.
 */
export function useSetDeploymentAirGapped(
  projectId: string | undefined,
): UseMutationResult<BeDeploymentAirGappedResponse, Error, SetDeploymentAirGappedInput> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<BeDeploymentAirGappedResponse, Error, SetDeploymentAirGappedInput>({
    mutationFn: ({ deploymentId, airGapped }): Promise<BeDeploymentAirGappedResponse> =>
      api.put<BeDeploymentAirGappedPayload, BeDeploymentAirGappedResponse>(
        `/deployments/${encodeURIComponent(deploymentId)}/air-gapped`,
        { airGapped },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId ?? ""],
      });
    },
  });
}
