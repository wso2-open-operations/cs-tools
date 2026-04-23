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
import type { DeleteAttachmentVariables } from "@features/support/types/supportApi";

export type { DeleteAttachmentVariables };

/**
 * Deletes an attachment by ID (DELETE /attachments/:id).
 * On success, invalidates case/deployment attachment queries if identifiers are provided.
 *
 * @returns {UseMutationResult<void, Error, DeleteAttachmentVariables>} Mutation result.
 */
export function useDeleteAttachment(): UseMutationResult<
  void,
  Error,
  DeleteAttachmentVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, DeleteAttachmentVariables>({
    mutationFn: async ({
      attachmentId,
    }: DeleteAttachmentVariables): Promise<void> => {
      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to delete an attachment");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/attachments/${attachmentId}`;
        const response = await authFetch(requestUrl, {
          method: "DELETE",
        });

        logger.debug("[useDeleteAttachment] Response status:", response.status);

        if (!response.ok) {
          let message = "";
          try {
            const data = await response.json();
            if (data && typeof data.message === "string") {
              message = data.message;
            }
          } catch {
            // fall back to status text if body is not JSON
          }
          if (!message) {
            message = response.statusText || "Failed to delete attachment";
          }
          throw new Error(message);
        }
      } catch (error) {
        logger.error("[useDeleteAttachment] Error:", error);
        throw error;
      }
    },
    onSuccess: (_data, { caseId, deploymentId }) => {
      if (caseId) {
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.CASE_ATTACHMENTS, caseId],
        });
      }
      if (deploymentId) {
        queryClient.invalidateQueries({
          queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId],
        });
      }
    },
  });
}
