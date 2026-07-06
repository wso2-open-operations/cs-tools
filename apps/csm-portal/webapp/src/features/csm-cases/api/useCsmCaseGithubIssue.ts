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
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeCreateCaseGithubIssuePayload,
  BeCreateCaseGithubIssueResponse,
} from "@api/backend/types";

export interface PostCaseGithubIssueInput extends BeCreateCaseGithubIssuePayload {
  /** UUID of the case the issue is filed against (path param, not body). */
  caseId: string;
}

/**
 * File an internal GitHub issue from a case via `POST /cases/{id}/github-issues`
 * (ISSU-020). ServiceNow-only: the entity service exposes this endpoint only
 * when its data source is ServiceNow, and the SN scoped app both routes the
 * issue (by the case's product unit, unless `repoOverride` is given) and writes
 * the created issue URL back into the case's work notes.
 *
 * On success the case comments query is invalidated so that work-note entry
 * shows up in the activities feed without a manual refresh. There is no issues
 * list to refresh — SN does not track created issues (see the backend notes).
 */
export function usePostCaseGithubIssue(): UseMutationResult<
  BeCreateCaseGithubIssueResponse,
  Error,
  PostCaseGithubIssueInput
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    BeCreateCaseGithubIssueResponse,
    Error,
    PostCaseGithubIssueInput
  >({
    mutationFn: async ({
      caseId,
      ...payload
    }): Promise<BeCreateCaseGithubIssueResponse> => {
      return api.post<
        BeCreateCaseGithubIssuePayload,
        BeCreateCaseGithubIssueResponse
      >(`/cases/${encodeURIComponent(caseId)}/github-issues`, payload);
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CSM_CASE_COMMENTS, variables.caseId],
      });
    },
  });
}
