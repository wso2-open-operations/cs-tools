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
import type { CreateDeploymentRequest } from "@features/project-details/types/deployments";
import type { CreateDeploymentResponse } from "@features/project-details/types/deployments";

/**
 * Hook to create a new deployment for a project.
 *
 * @param {string} projectId - The ID of the project.
 * @returns {UseMutationResult<CreateDeploymentResponse, Error, CreateDeploymentRequest>} Mutation result.
 */
export function usePostCreateDeployment(
  projectId: string,
): UseMutationResult<CreateDeploymentResponse, Error, CreateDeploymentRequest> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<CreateDeploymentResponse, Error, CreateDeploymentRequest>({
    mutationFn: async (
      body: CreateDeploymentRequest,
    ): Promise<CreateDeploymentResponse> => {
      logger.debug("[usePostCreateDeployment] Request payload:", body);

      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to create a deployment");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/projects/${projectId}/deployments`;

        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(
          `[usePostCreateDeployment] Response status: ${response.status}`,
        );

        if (response.status === 409) {
          throw new Error(
            `Deployment name "${body.name}" is already taken. Please choose a different name.`,
          );
        }

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error creating deployment: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }

        const data: CreateDeploymentResponse = await response.json();
        logger.debug("[usePostCreateDeployment] Deployment created:", data);
        return data;
      } catch (error) {
        logger.error("[usePostCreateDeployment] Error:", error);
        throw error;
      }
    },
    onSuccess: () => {
      void queryClient.refetchQueries({
        queryKey: ["project-deployments", projectId],
      });
      void queryClient.refetchQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId],
      });
    },
  });
}
