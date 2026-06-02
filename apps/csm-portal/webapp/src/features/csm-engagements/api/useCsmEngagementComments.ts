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
import { ApiMutationKeys, ApiQueryKeys } from "@constants/apiConstants";
import {
  getMockCsmEngagementComments,
  postMockCsmEngagementComment,
} from "@features/csm-engagements/api/mocks/engagementsMocks";
import type { CsmEngagementComment } from "@features/csm-engagements/types/csmEngagements";

export function useGetCsmEngagementComments(
  engagementId: string | undefined,
): UseQueryResult<CsmEngagementComment[], Error> {
  const authFetch = useAuthApiClient();
  return useQuery<CsmEngagementComment[], Error>({
    queryKey: [ApiQueryKeys.CSM_ENGAGEMENT_COMMENTS, engagementId],
    enabled: !!engagementId,
    queryFn: async () => {
      if (!engagementId) return [];
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        await new Promise((r) => setTimeout(r, 150));
        return getMockCsmEngagementComments(engagementId);
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/engagements/${encodeURIComponent(engagementId)}/comments`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(
          `Error fetching engagement comments: ${response.statusText}`,
        );
      }
      return (await response.json()) as CsmEngagementComment[];
    },
    staleTime: 10_000,
  });
}

export interface PostCsmEngagementCommentVariables {
  engagementId: string;
  authorName: string;
  bodyHtml: string;
  internal?: boolean;
}

export function usePostCsmEngagementComment(): UseMutationResult<
  CsmEngagementComment,
  Error,
  PostCsmEngagementCommentVariables
> {
  const authFetch = useAuthApiClient();
  const qc = useQueryClient();
  return useMutation<CsmEngagementComment, Error, PostCsmEngagementCommentVariables>({
    mutationKey: ApiMutationKeys.POST_ENGAGEMENT_COMMENT,
    mutationFn: async (vars) => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        await new Promise((r) => setTimeout(r, 150));
        return postMockCsmEngagementComment(vars);
      }
      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const url = `${baseUrl}/csm/engagements/${encodeURIComponent(vars.engagementId)}/comments`;
      const response = await authFetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          authorName: vars.authorName,
          bodyHtml: vars.bodyHtml,
          internal: vars.internal,
        }),
      });
      if (!response.ok) {
        throw new Error(`Error posting engagement comment: ${response.statusText}`);
      }
      return (await response.json()) as CsmEngagementComment;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_ENGAGEMENT_COMMENTS, vars.engagementId],
      });
    },
  });
}
