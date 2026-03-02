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
import { useLogger } from "@hooks/useLogger";
import { useAuthApiClient } from "@context/AuthApiContext";
import { ApiQueryKeys, ApiMutationKeys } from "@constants/apiConstants";
import { CommentType } from "@constants/supportConstants";

export interface PostChangeRequestCommentRequest {
  content: string;
  type: CommentType;
}

export interface PostChangeRequestCommentVariables {
  changeRequestId: string;
  body: PostChangeRequestCommentRequest;
}

export interface PostChangeRequestCommentResponse {
  id: string;
  createdOn: string;
  createdBy: string;
  content: string;
  hasInlineAttachments: boolean;
  inlineImageCount: number;
  inlineAttachments: unknown[];
}

/**
 * Posts a comment to a change request (POST /change-requests/:changeRequestId/comments).
 * On success, invalidates change-request-comments queries so the list refetches.
 *
 * @returns {UseMutationResult<PostChangeRequestCommentResponse, Error, PostChangeRequestCommentVariables>} Mutation result.
 */
export function usePostChangeRequestComment(): UseMutationResult<
  PostChangeRequestCommentResponse,
  Error,
  PostChangeRequestCommentVariables
> {
  const logger = useLogger();
  const queryClient = useQueryClient();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const fetchFn = useAuthApiClient();

  return useMutation<
    PostChangeRequestCommentResponse,
    Error,
    PostChangeRequestCommentVariables
  >({
    mutationKey: ApiMutationKeys.POST_CHANGE_REQUEST_COMMENT,
    mutationFn: async ({
      changeRequestId,
      body,
    }: PostChangeRequestCommentVariables): Promise<PostChangeRequestCommentResponse> => {
      logger.debug("[usePostChangeRequestComment] Request:", {
        changeRequestId,
        contentLength: body.content?.length ?? 0,
      });

      if (!isSignedIn || isAuthLoading) {
        throw new Error("User must be signed in to post a comment");
      }

      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const encodedChangeRequestId = encodeURIComponent(changeRequestId);
      const requestUrl = `${baseUrl}/change-requests/${encodedChangeRequestId}/comments`;
      const response = await fetchFn(requestUrl, {
        method: "POST",
        body: JSON.stringify({
          content: body.content,
          type: body.type,
        }),
      });

      logger.debug(
        "[usePostChangeRequestComment] Response status:",
        response.status,
      );

      if (!response.ok) {
        const text = await response.text();
        throw new Error(
          `Error posting change request comment: ${response.status} ${response.statusText}${text ? ` - ${text}` : ""}`,
        );
      }

      const data: PostChangeRequestCommentResponse = await response.json();
      logger.debug("[usePostChangeRequestComment] Comment posted:", {
        id: data.id,
        changeRequestId,
      });
      return data;
    },
    onSuccess: (_data, variables) => {
      // Invalidate only the active change request's comments
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUEST_COMMENTS, variables.changeRequestId],
      });
    },
  });
}
