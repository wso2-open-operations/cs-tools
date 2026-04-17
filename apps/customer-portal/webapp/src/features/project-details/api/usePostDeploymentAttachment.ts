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
import type { PostDeploymentAttachmentVariables } from "@features/project-details/types/projectDetailsApi";
import type { PostDeploymentAttachmentResponse } from "@features/project-details/types/deployments";

/**
 * Posts an attachment to a deployment (POST /deployments/:deploymentId/attachments).
 * On success, invalidates deployment-attachments and deployments queries.
 *
 * @returns {UseMutationResult<PostDeploymentAttachmentResponse, Error, PostDeploymentAttachmentVariables>} Mutation result.
 */
export function usePostDeploymentAttachment(): UseMutationResult<
  PostDeploymentAttachmentResponse,
  Error,
  PostDeploymentAttachmentVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<
    PostDeploymentAttachmentResponse,
    Error,
    PostDeploymentAttachmentVariables
  >({
    mutationFn: async ({
      deploymentId,
      body,
    }: PostDeploymentAttachmentVariables): Promise<PostDeploymentAttachmentResponse> => {
      try {
        logger.debug("[usePostDeploymentAttachment] Request:", {
          deploymentId,
          name: body.name,
          type: body.type,
          contentLength: body.content?.length ?? 0,
        });

        if (!isSignedIn || isAuthLoading) {
          throw new Error(
            "User must be signed in to upload a deployment document",
          );
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/deployments/${deploymentId}/attachments`;
        const response = await authFetch(requestUrl, {
          method: "POST",

          body: JSON.stringify(body),
        });

        logger.debug(
          "[usePostDeploymentAttachment] Response status:",
          response.status,
        );

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error uploading deployment document: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }

        const data: PostDeploymentAttachmentResponse = await response.json();
        logger.debug("[usePostDeploymentAttachment] Data received:", data);
        return data;
      } catch (error) {
        logger.error("[usePostDeploymentAttachment] Error:", error);
        throw error;
      }
    },
    onSuccess: (_data, { deploymentId }) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId],
      });
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.DEPLOYMENTS],
      });
    },
  });
}
