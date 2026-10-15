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

import { useMutation, type UseMutationResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { AttachAnnouncementRequestFilesToCaseResponse } from "@features/csm-announcements/types/announcementRequests";

export interface AttachAnnouncementRequestFilesToCaseVariables {
  id: string;
  caseId: string;
}

/**
 * `POST /announcement-requests/{id}/cases/{caseId}/attachments` — asks the server
 * to copy the request's files onto one case. Called once for each case a publish
 * creates (and for the dry-run case), so a publish to many projects does not
 * upload the same file from the browser once per project: the bytes never leave
 * the server. Only a case the request owns is accepted. Idempotent, so a retry
 * after a partial failure attaches only what is missing; its refusals carry
 * the entity service's reason as the error message.
 *
 * Deliberately has no cache invalidation: it changes a case's attachments, not
 * anything this feature's own queries hold.
 */
export function useAttachAnnouncementRequestFilesToCase(): UseMutationResult<
  AttachAnnouncementRequestFilesToCaseResponse,
  Error,
  AttachAnnouncementRequestFilesToCaseVariables
> {
  const api = useBackendApi();
  return useMutation<AttachAnnouncementRequestFilesToCaseResponse, Error, AttachAnnouncementRequestFilesToCaseVariables>({
    mutationFn: ({ id, caseId }): Promise<AttachAnnouncementRequestFilesToCaseResponse> =>
      api.postEmpty<AttachAnnouncementRequestFilesToCaseResponse>(
        `/announcement-requests/${encodeURIComponent(id)}/cases/${encodeURIComponent(caseId)}/attachments`,
      ),
  });
}
