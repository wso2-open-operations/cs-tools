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
import type { PatchCaseAttachmentVariables } from "@features/support/types/supportApi";

export type { PatchCaseAttachmentVariables };

/**
 * Updates a case attachment (name/description) via PATCH /cases/:caseId/attachments/:attachmentId.
 * On success, invalidates the case attachments query.
 *
 * @returns {UseMutationResult<void, Error, PatchCaseAttachmentVariables>} Mutation result.
 */
export function usePatchCaseAttachment(): UseMutationResult<
  void,
  Error,
  PatchCaseAttachmentVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, PatchCaseAttachmentVariables>({
    mutationFn: async ({
      caseId,
      attachmentId,
      body,
    }: PatchCaseAttachmentVariables): Promise<void> => {
      try {
        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to update an attachment");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const requestUrl = `${baseUrl}/cases/${caseId}/attachments/${attachmentId}`;
        const response = await authFetch(requestUrl, {
          method: "PATCH",
          body: JSON.stringify(body),
        });

        logger.debug(
          "[usePatchCaseAttachment] Response status:",
          response.status,
        );

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
            message = response.statusText || "Failed to update attachment";
          }
          throw new Error(message);
        }
      } catch (error) {
        logger.error("[usePatchCaseAttachment] Error:", error);
        throw error;
      }
    },
    onSuccess: (_data, { caseId }) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CASE_ATTACHMENTS, caseId],
      });
    },
  });
}
