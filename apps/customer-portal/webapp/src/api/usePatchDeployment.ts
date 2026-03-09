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
import { useAuthApiClient } from "@api/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { PatchDeploymentRequest } from "@models/requests";

export interface PatchDeploymentVariables {
  projectId: string;
  deploymentId: string;
  body: PatchDeploymentRequest;
}

/**
 * Patches a deployment (PATCH /projects/:projectId/deployments/:deploymentId).
 * On success, invalidates project-deployments and deployment queries.
 *
 * @returns {UseMutationResult<void, Error, PatchDeploymentVariables>} Mutation result.
 */
export function usePatchDeployment(): UseMutationResult<
  void,
  Error,
  PatchDeploymentVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, PatchDeploymentVariables>({
    mutationFn: async ({
      projectId,
      deploymentId,
      body,
    }: PatchDeploymentVariables): Promise<void> => {
      try {
        logger.debug("[usePatchDeployment] Request:", {
          projectId,
          deploymentId,
          body,
        });

        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to update a deployment");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/deployments/${deploymentId}`;
        const response = await authFetch(requestUrl, {
          method: "PATCH",

          body: JSON.stringify(body),
        });

        logger.debug("[usePatchDeployment] Response status:", response.status);

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error updating deployment: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }
      } catch (error) {
        logger.error("[usePatchDeployment] Error:", error);
        throw error;
      }
    },
    onSuccess: (_data, { projectId }) => {
      queryClient.invalidateQueries({
        queryKey: ["project-deployments", projectId],
      });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId],
      });
    },
  });
}
