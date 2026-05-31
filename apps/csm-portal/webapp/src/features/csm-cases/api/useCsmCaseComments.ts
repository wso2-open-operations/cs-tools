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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  getMockCsmCaseComments,
  postMockCsmCaseComment,
} from "@features/csm-cases/api/mocks/commentsMocks";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 150;

export function useGetCsmCaseComments(
  caseId: string | undefined,
): UseQueryResult<CsmCaseComment[], Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmCaseComment[], Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseComment[]> => {
      if (!caseId) return [];

      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(
          `[useGetCsmCaseComments] Returning mock comments for ${caseId}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCaseComments(caseId);
      }

      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/cases/${encodeURIComponent(caseId)}/comments`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(
          `Error fetching comments for ${caseId}: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmCaseComment[];
    },
    enabled: !!caseId,
    staleTime: 10_000,
  });
}

export interface PostCsmCaseCommentInput {
  caseId: string;
  bodyHtml: string;
  /** Display name of the logged-in engineer. */
  authorName: string;
  /** If true, the entry is an internal work note (not customer-visible). */
  internal?: boolean;
}

export function usePostCsmCaseComment(): UseMutationResult<
  CsmCaseComment,
  Error,
  PostCsmCaseCommentInput
> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, PostCsmCaseCommentInput>({
    mutationFn: async (input): Promise<CsmCaseComment> => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
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

      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/cases/${encodeURIComponent(
        input.caseId,
      )}/comments`;
      const response = await authFetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          bodyHtml: input.bodyHtml,
          internal: input.internal ?? false,
        }),
      });
      if (!response.ok) {
        throw new Error(`Failed to post comment: ${response.statusText}`);
      }
      return (await response.json()) as CsmCaseComment;
    },
    onSuccess: (newComment, variables) => {
      queryClient.setQueryData<CsmCaseComment[] | undefined>(
        [ApiQueryKeys.CSM_CASE_COMMENTS, variables.caseId],
        (prev) => (prev ? [...prev, newComment] : [newComment]),
      );
    },
  });
}
