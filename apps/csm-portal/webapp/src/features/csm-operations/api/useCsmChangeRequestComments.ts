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
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  ApiMutationKeys,
  ApiQueryKeys,
  BE_MAX_PAGE_LIMIT,
} from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeComment,
  BeCaseCommentCreatePayload,
  BeCaseCommentSearchPayload,
  BeCommentSearchResponse,
} from "@api/backend/types";
import {
  commentTypeFromInternal,
  uiCommentFromBe,
} from "@api/backend/mappers";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

/** Page size used by the comments list. Capped by the BE; see BE_MAX_PAGE_LIMIT. */
const COMMENTS_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Load all comments on a change request. Calls
 * `POST /change-requests/{id}/comments/search` with a single wide page
 * (limit capped at BE_MAX_PAGE_LIMIT). Reuses the same generic BeComment
 * shape as case comments — only the referenceType widened upstream.
 */
export function useGetCsmChangeRequestComments(
  changeRequestId: string | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const api = useBackendApi();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUEST_COMMENTS, changeRequestId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!changeRequestId) return [];

      const payload: BeCaseCommentSearchPayload = {
        pagination: { offset: 0, limit: COMMENTS_PAGE_LIMIT },
      };
      const response = await api.post<
        BeCaseCommentSearchPayload,
        BeCommentSearchResponse
      >(
        `/change-requests/${encodeURIComponent(changeRequestId)}/comments/search`,
        payload,
      );
      return (response.comments ?? []).map((comment) =>
        uiCommentFromBe(comment, { context: "case" }),
      );
    },
    enabled: !!changeRequestId,
    staleTime: 10_000,
  });
}

export interface PostCsmChangeRequestCommentInput {
  changeRequestId: string;
  /** Plain-text body. HTML is escaped on render. */
  bodyHtml: string;
  /** If true, the entry is an internal work note (not customer-visible). */
  internal?: boolean;
}

/**
 * Create a comment on a change request via
 * `POST /change-requests/{id}/comments`.
 */
export function usePostCsmChangeRequestComment(): UseMutationResult<
  CsmCaseComment,
  Error,
  PostCsmChangeRequestCommentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    CsmCaseComment,
    Error,
    PostCsmChangeRequestCommentInput
  >({
    mutationKey: ApiMutationKeys.POST_CHANGE_REQUEST_COMMENT,
    mutationFn: async (input): Promise<CsmCaseComment> => {
      const payload: BeCaseCommentCreatePayload = {
        type: commentTypeFromInternal(input.internal ?? false),
        content: input.bodyHtml,
      };
      const created = await api.post<BeCaseCommentCreatePayload, BeComment>(
        `/change-requests/${encodeURIComponent(input.changeRequestId)}/comments`,
        payload,
      );
      return uiCommentFromBe(created, { context: "case" });
    },
    onSuccess: (_newComment, variables) => {
      // Same thin-ack limitation as case comments: refetch to hydrate the
      // author name and rendered content from the search endpoint.
      void queryClient.invalidateQueries({
        queryKey: [
          ApiQueryKeys.CHANGE_REQUEST_COMMENTS,
          variables.changeRequestId,
        ],
      });
    },
  });
}
