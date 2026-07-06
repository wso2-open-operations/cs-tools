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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
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
 * Load all comments on a case. In LIVE mode calls
 * `POST /cases/{id}/comments/search` with a single wide page (limit capped at
 * BE_MAX_PAGE_LIMIT). If a case exceeds that, switch consumers to an explicit
 * pagination wrapper rather than chasing pages here.
 */
export function useGetCsmCaseComments(
  caseId: string | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const api = useBackendApi();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!caseId) return [];

      const payload: BeCaseCommentSearchPayload = {
        pagination: { offset: 0, limit: COMMENTS_PAGE_LIMIT },
      };
      const response = await api.post<
        BeCaseCommentSearchPayload,
        BeCommentSearchResponse
      >(`/cases/${encodeURIComponent(caseId)}/comments/search`, payload);
      return (response.comments ?? []).map((comment) =>
        uiCommentFromBe(comment, { context: "case" }),
      );
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}

export interface PostCsmCaseCommentInput {
  caseId: string;
  /** Plain-text body. HTML is escaped on render. */
  bodyHtml: string;
  /** Display name of the logged-in engineer. */
  authorName: string;
  /** If true, the entry is an internal work note (not customer-visible). */
  internal?: boolean;
}

/**
 * Create a comment on a case. The backend accepts a plain-text body; the FE
 * still passes a `bodyHtml` field for parity with the mock, but it's stored
 * as plain text and re-rendered on read.
 */
export function usePostCsmCaseComment(): UseMutationResult<
  CsmCaseComment,
  Error,
  PostCsmCaseCommentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, PostCsmCaseCommentInput>({
    mutationFn: async (input): Promise<CsmCaseComment> => {
      const payload: BeCaseCommentCreatePayload = {
        type: commentTypeFromInternal(input.internal ?? false),
        // BE stores rich-text HTML; send the editor output as-is.
        content: input.bodyHtml,
      };
      const created = await api.post<
        BeCaseCommentCreatePayload,
        BeComment
      >(`/cases/${encodeURIComponent(input.caseId)}/comments`, payload);
      return uiCommentFromBe(created, { context: "case" });
    },
    onSuccess: (_newComment, variables) => {
      // The BE create response is a thin ack — it echoes only {id, createdOn,
      // createdBy(email string)}, not the author's display name, the rendered
      // content, or the comment type. Appending that partial object renders the
      // new entry as "Unknown WSO2 —" with no body. Until the BE echoes the full
      // comment on POST, refetch the list so the new comment is hydrated from
      // `comments/search`, which returns the full shape. One extra round-trip
      // per post, but the entry renders correctly.
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, variables.caseId],
      });
    },
  });
}
