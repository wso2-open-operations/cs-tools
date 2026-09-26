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
 * `POST /announcement-requests/{id}/schedule` — sets or clears (`null`) an
 * approved request's automatic-publish time; `actorId` is always the
 * authenticated caller. Once set, `operations/csm-scheduled-tasks`'
 * `publish_scheduled_announcements` sub-cron publishes it automatically once
 * `scheduledFor` arrives — the manual Publish button (`usePublishAnnouncementRequest`)
 * is completely unaffected and still works at any time, before or after a
 * schedule is set, as an explicit override. Rejected (409) unless the
 * request is `approved`; rejected (400) if `scheduledFor` isn't strictly in
 * the future.
 */
export function useScheduleAnnouncementRequest(): UseMutationResult<
  AnnouncementRequest,
  Error,
  { id: string; scheduledFor: string | null }
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequest, Error, { id: string; scheduledFor: string | null }>({
    mutationFn: ({ id, scheduledFor }): Promise<AnnouncementRequest> =>
      api.post<{ scheduledFor: string | null }, AnnouncementRequest>(
        `/announcement-requests/${encodeURIComponent(id)}/schedule`,
        { scheduledFor },
      ),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, variables.id],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
    },
  });
}
