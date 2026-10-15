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
import { readFileAsDataUrl } from "@features/csm-announcements/utils/announcementAttachments";
import type {
  AnnouncementRequestAttachment,
  CreateAnnouncementRequestAttachmentPayload,
} from "@features/csm-announcements/types/announcementRequests";

export interface UploadAnnouncementRequestAttachmentVariables {
  id: string;
  file: File;
}

/**
 * `POST /announcement-requests/{id}/attachments` — attaches one file to a
 * request. The file is sent as a base64 data URI in the JSON body (the same way a
 * case attachment is on a data source with no direct-upload storage); the
 * entity service decides whether it is accepted and says why when it is not,
 * which surfaces as the mutation's error message.
 *
 * Changing a request's files is a content edit: one that is waiting for approval
 * goes back to draft, so the request's own queries are refreshed as well as the
 * file list. Takes `id` per call, not as a hook argument, for the reason
 * `useUpdateAnnouncementRequest` documents: a create form attaches to a draft it
 * has only just made.
 */
export function useUploadAnnouncementRequestAttachment(): UseMutationResult<
  AnnouncementRequestAttachment,
  Error,
  UploadAnnouncementRequestAttachmentVariables
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequestAttachment, Error, UploadAnnouncementRequestAttachmentVariables>({
    mutationFn: async ({ id, file }): Promise<AnnouncementRequestAttachment> => {
      const dataUri = await readFileAsDataUrl(file);
      return api.post<CreateAnnouncementRequestAttachmentPayload, AnnouncementRequestAttachment>(
        `/announcement-requests/${encodeURIComponent(id)}/attachments`,
        { name: file.name, type: file.type || undefined, file: dataUri },
      );
    },
    onSettled: async (_result, _error, variables) => {
      // Settled, not just success: a refused file leaves the list as it was, but a
      // request whose state moved underneath the caller should be re-read either way.
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_ATTACHMENTS, variables.id] }),
        queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, variables.id] }),
        queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] }),
      ]);
    },
  });
}
