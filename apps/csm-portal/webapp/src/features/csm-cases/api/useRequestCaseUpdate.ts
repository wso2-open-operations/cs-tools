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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCaseUpdateRequestPayload,
  BeCaseUpdateRequestStage,
  BeCaseUpdateRequestTemplates,
  BeComment,
} from "@api/backend/types";
import { uiCommentFromBe } from "@api/backend/mappers";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

/**
 * Load the fixed reminder-message catalogue (both categories, all three
 * stages). Static content — same response every request, no case-specific
 * data — so it's cached for the life of the session rather than refetched.
 */
export function useGetCaseUpdateRequestTemplates(): UseQueryResult<
  BeCaseUpdateRequestTemplates,
  Error
> {
  const api = useBackendApi();

  return useQuery<BeCaseUpdateRequestTemplates, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_UPDATE_REQUEST_TEMPLATES],
    queryFn: async (): Promise<BeCaseUpdateRequestTemplates> => {
      const res = await api.get<BeCaseUpdateRequestTemplates>(
        "/case-update-request-templates",
      );
      // The endpoint is static/always-200 in normal operation; a null (404)
      // response would mean the route itself is missing, which is a real
      // failure to surface rather than paper over with empty templates.
      if (!res) throw new Error("Update request templates are unavailable.");
      return res;
    },
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

export interface RequestCaseUpdateInput {
  caseId: string;
  stage: BeCaseUpdateRequestStage;
  /** Required (non-empty) iff `stage` is `"custom"`. */
  customContent?: string;
}

/**
 * Post a "request update" nudge: `POST /cases/{id}/request-update`. Creates a
 * customer-visible comment on the case (same response shape as
 * `POST /cases/{id}/comments`), so the comments list is invalidated on
 * success exactly like `usePostCsmCaseComment` — the new entry then appears
 * in the activity feed without a manual refetch.
 */
export function useRequestCaseUpdate(): UseMutationResult<
  CsmCaseComment,
  Error,
  RequestCaseUpdateInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<CsmCaseComment, Error, RequestCaseUpdateInput>({
    mutationFn: async (input): Promise<CsmCaseComment> => {
      const payload: BeCaseUpdateRequestPayload = {
        stage: input.stage,
        ...(input.stage === "custom" && input.customContent
          ? { customContent: input.customContent }
          : {}),
      };
      const created = await api.post<BeCaseUpdateRequestPayload, BeComment>(
        `/cases/${encodeURIComponent(input.caseId)}/request-update`,
        payload,
      );
      return uiCommentFromBe(created, { context: "case" });
    },
    onSuccess: (_newComment, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, variables.caseId],
      });
    },
  });
}
