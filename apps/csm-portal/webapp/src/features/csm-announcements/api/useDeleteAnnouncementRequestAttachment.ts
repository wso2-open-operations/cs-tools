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

import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";

export interface DeleteAnnouncementRequestAttachmentVariables {
  id: string;
  attachmentId: string;
}

/**
 * `DELETE /announcement-requests/{id}/attachments/{attachmentId}` — removes one
 * file from a request. Like attaching one, it is a content edit: a request
 * waiting for approval goes back to draft, and a published one refuses (409).
 */
export function useDeleteAnnouncementRequestAttachment(): UseMutationResult<
  void,
  Error,
  DeleteAnnouncementRequestAttachmentVariables
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<void, Error, DeleteAnnouncementRequestAttachmentVariables>({
    mutationFn: ({ id, attachmentId }): Promise<void> =>
      api.delete(
        `/announcement-requests/${encodeURIComponent(id)}/attachments/${encodeURIComponent(attachmentId)}`,
      ),
    onSettled: async (_result, _error, variables) => {
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_ATTACHMENTS, variables.id] }),
        queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, variables.id] }),
        queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] }),
      ]);
    },
  });
}
