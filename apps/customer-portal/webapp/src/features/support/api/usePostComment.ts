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
import { ApiQueryKeys, ApiMutationKeys } from "@constants/apiConstants";
import type {
  PostCommentRequest,
  PostCommentVariables,
} from "@features/support/types/supportApi";
import { parseApiResponseMessage } from "@utils/ApiError";

export type { PostCommentRequest, PostCommentVariables };

/**
 * Posts a comment to a case (POST /cases/:caseId/comments).
 * On success, invalidates case-comments queries so the list refetches.
 *
 * @returns {UseMutationResult<void, Error, PostCommentVariables>} Mutation result.
 */
export function usePostComment(): UseMutationResult<
  void,
  Error,
  PostCommentVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  return useMutation<void, Error, PostCommentVariables>({
    mutationKey: ApiMutationKeys.POST_COMMENT,
    mutationFn: async ({
      caseId,
      body,
    }: PostCommentVariables): Promise<void> => {
      logger.debug("[usePostComment] Request:", {
        caseId,
        contentLength: body.content?.length ?? 0,
      });

      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to post a comment");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const serializedBody = JSON.stringify({
        content: body.content,
        type: body.type,
      });
      const payloadBytes = new TextEncoder().encode(serializedBody).length;
      const MAX_PAYLOAD_BYTES = 10 * 1024 * 1024; // 10 MB
      if (payloadBytes > MAX_PAYLOAD_BYTES) {
        throw new Error(
          "The comment exceeds the 10 MB limit. Please reduce the size or the number of inline images and try again.",
        );
      }

      const requestUrl = `${baseUrl}/cases/${caseId}/comments`;
      const response = await authFetch(requestUrl, {
        method: "POST",
        body: serializedBody,
      });

      logger.debug("[usePostComment] Response status:", response.status);

      if (!response.ok) {
        const text = await response.text();
        throw new Error(parseApiResponseMessage(text, response.status, response.statusText));
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.CASE_COMMENTS] });
    },
  });
}
