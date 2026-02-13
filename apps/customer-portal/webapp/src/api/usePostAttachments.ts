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

import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useMockConfig } from "@providers/MockConfigProvider";
import { useLogger } from "@hooks/useLogger";
import { addApiHeaders } from "@utils/apiUtils";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { PostCaseAttachmentRequest } from "@models/requests";

export interface PostAttachmentsVariables {
  caseId: string;
  body: PostCaseAttachmentRequest;
}

/**
 * Posts an attachment to a case (POST /cases/:caseId/attachments).
 * When mock is enabled, the mutation throws; the upload modal should disable the Save button.
 * On success, invalidates case-attachments queries so the list refetches.
 *
 * @returns {UseMutationResult<void, Error, PostAttachmentsVariables>} Mutation result.
 */
export function usePostAttachments(): UseMutationResult<
  void,
  Error,
  PostAttachmentsVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { getIdToken, isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const { isMockEnabled } = useMockConfig();

  return useMutation<void, Error, PostAttachmentsVariables>({
    mutationFn: async ({
      caseId,
      body,
    }: PostAttachmentsVariables): Promise<void> => {
      try {
        logger.debug("[usePostAttachments] Request:", {
          caseId,
          name: body.name,
          type: body.type,
          contentLength: body.content?.length ?? 0,
        });

        if (isMockEnabled) {
          throw new Error(
            "Upload attachment is not available when mock is enabled. Disable mock to upload.",
          );
        }

        if (!isSignedIn || isAuthLoading) {
          throw new Error("User must be signed in to upload an attachment");
        }

        const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
        if (!baseUrl) {
          throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
        }

        const idToken = await getIdToken();
        const requestUrl = `${baseUrl}/cases/${caseId}/attachments`;
        const response = await fetch(requestUrl, {
          method: "POST",
          headers: addApiHeaders(idToken),
          body: JSON.stringify(body),
        });

        logger.debug("[usePostAttachments] Response status:", response.status);

        if (!response.ok) {
          const text = await response.text();
          throw new Error(
            `Error uploading attachment: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
          );
        }
      } catch (error) {
        logger.error("[usePostAttachments] Error:", error);
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
