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
import type { AnnouncementRequest } from "@features/csm-announcements/types/announcementRequests";

/**
 * `POST /announcement-requests/{id}/approve` — takes no body; `actorId` is
 * always the authenticated caller. There is no approver-role check: the real
 * approval decision happens over email, outside the platform, so this call
 * only records that whoever's working the request says it's been approved.
 * Rejected (409) unless the request is `pending_approval`.
 *
 * Takes `id` per call, not as a hook argument — matches the other
 * announcement-request mutation hooks (see `useUpdateAnnouncementRequest`'s
 * doc comment) for a consistent calling convention across all of them, even
 * though this particular one is only ever called against an already-known,
 * stable id (the dialog opens for an existing row).
 */
export function useApproveAnnouncementRequest(): UseMutationResult<AnnouncementRequest, Error, { id: string }> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequest, Error, { id: string }>({
    mutationFn: ({ id }): Promise<AnnouncementRequest> =>
      api.postEmpty<AnnouncementRequest>(`/announcement-requests/${encodeURIComponent(id)}/approve`),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, variables.id],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
    },
  });
}
