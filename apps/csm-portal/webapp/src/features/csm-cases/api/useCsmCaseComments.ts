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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { isMockMode, useBackendApi } from "@api/backend/client";
import type {
  BeCaseComment,
  BeCaseCommentCreatePayload,
  BeCaseCommentSearchPayload,
  BeCaseCommentSearchResponse,
} from "@api/backend/types";
import {
  commentTypeFromInternal,
  uiCommentFromBe,
} from "@api/backend/mappers";
import {
  getMockCsmCaseComments,
  postMockCsmCaseComment,
} from "@features/csm-cases/api/mocks/commentsMocks";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 150;

/** Page size used by the comments list. Capped at 100 by the BE. */
const COMMENTS_PAGE_LIMIT = 100;

/**
 * Load all comments on a case. In LIVE mode calls
 * `POST /cases/{id}/comments/search` with a wide page (limit=100). If a case
 * exceeds that, switch consumers to an explicit pagination wrapper rather
 * than chasing pages here.
 */
export function useGetCsmCaseComments(
  caseId: string | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!caseId) return [];

      if (isMockMode()) {
        logger.debug(
          `[useGetCsmCaseComments] Returning mock comments for ${caseId}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCaseComments(caseId);
      }

      const payload: BeCaseCommentSearchPayload = {
        pagination: { offset: 0, limit: COMMENTS_PAGE_LIMIT },
      };
      const response = await api.post<
        BeCaseCommentSearchPayload,
        BeCaseCommentSearchResponse
      >(`/cases/${encodeURIComponent(caseId)}/comments/search`, payload);
      return response.comments.map(uiCommentFromBe);
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
  const logger = useLogger();
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, PostCsmCaseCommentInput>({
    mutationFn: async (input): Promise<CsmCaseComment> => {
      if (isMockMode()) {
        logger.debug(
          `[usePostCsmCaseComment] Posting mock comment for ${input.caseId}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return postMockCsmCaseComment({
          caseId: input.caseId,
          bodyHtml: input.bodyHtml,
          authorName: input.authorName,
          authorRole: "wso2_engineer",
          internal: input.internal ?? false,
        });
      }

      const payload: BeCaseCommentCreatePayload = {
        commentType: commentTypeFromInternal(input.internal ?? false),
        // BE stores plain text. Strip simple HTML so we don't double-escape on read.
        body: input.bodyHtml.replace(/<\/?[^>]+>/g, "").trim(),
      };
      const created = await api.post<
        BeCaseCommentCreatePayload,
        BeCaseComment
      >(`/cases/${encodeURIComponent(input.caseId)}/comments`, payload);
      return uiCommentFromBe(created);
    },
    onSuccess: (newComment, variables) => {
      queryClient.setQueryData<CsmCaseComment[] | undefined>(
        [ApiQueryKeys.CSM_CASE_COMMENTS, variables.caseId],
        (prev) => (prev ? [...prev, newComment] : [newComment]),
      );
    },
  });
}
