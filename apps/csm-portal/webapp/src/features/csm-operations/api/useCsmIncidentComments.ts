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
 * Load all comments on an incident. Calls
 * `POST /incidents/{id}/comments/search` with a single wide page (limit
 * capped at BE_MAX_PAGE_LIMIT). Reuses the same generic BeComment shape as
 * case comments — only the referenceType widened upstream.
 */
export function useGetCsmIncidentComments(
  incidentId: string | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const api = useBackendApi();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.INCIDENT_COMMENTS, incidentId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!incidentId) return [];

      const payload: BeCaseCommentSearchPayload = {
        pagination: { offset: 0, limit: COMMENTS_PAGE_LIMIT },
      };
      const response = await api.post<
        BeCaseCommentSearchPayload,
        BeCommentSearchResponse
      >(
        `/incidents/${encodeURIComponent(incidentId)}/comments/search`,
        payload,
      );
      return (response.comments ?? []).map((comment) =>
        uiCommentFromBe(comment, { context: "case" }),
      );
    },
    enabled: !!incidentId,
    staleTime: 10_000,
  });
}

export interface PostCsmIncidentCommentInput {
  incidentId: string;
  /** Plain-text body. HTML is escaped on render. */
  bodyHtml: string;
  /** If true, the entry is an internal work note (not customer-visible). */
  internal?: boolean;
}

/**
 * Create a comment on an incident via `POST /incidents/{id}/comments`.
 */
export function usePostCsmIncidentComment(): UseMutationResult<
  CsmCaseComment,
  Error,
  PostCsmIncidentCommentInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, PostCsmIncidentCommentInput>({
    mutationKey: ApiMutationKeys.POST_INCIDENT_COMMENT,
    mutationFn: async (input): Promise<CsmCaseComment> => {
      const payload: BeCaseCommentCreatePayload = {
        type: commentTypeFromInternal(input.internal ?? false),
        content: input.bodyHtml,
      };
      const created = await api.post<BeCaseCommentCreatePayload, BeComment>(
        `/incidents/${encodeURIComponent(input.incidentId)}/comments`,
        payload,
      );
      return uiCommentFromBe(created, { context: "case" });
    },
    onSuccess: (_newComment, variables) => {
      // Same thin-ack limitation as case comments: refetch to hydrate the
      // author name and rendered content from the search endpoint.
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.INCIDENT_COMMENTS, variables.incidentId],
      });
    },
  });
}
