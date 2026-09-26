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
  AnnouncementRequestUpdate,
  CreateAnnouncementRequestUpdatePayload,
} from "@features/csm-announcements/types/announcementRequests";

/**
 * `POST /announcement-requests/{id}/updates` — records a new dated
 * follow-up; `actorId` is always the authenticated caller, forced
 * server-side (never read from this payload). Rejected (409) unless the
 * request is `published`; (403) unless the caller is its own creator (same
 * restriction as Publish — an approver's job is only to approve).
 *
 * This only records the update — it does not itself apply content as a
 * comment anywhere. The caller's own fan-out
 * (`usePostAnnouncementUpdateComments`) does that, separately, once this
 * succeeds.
 */
export function useCreateAnnouncementRequestUpdate(): UseMutationResult<
  AnnouncementRequestUpdate,
  Error,
  { id: string; payload: CreateAnnouncementRequestUpdatePayload }
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequestUpdate, Error, { id: string; payload: CreateAnnouncementRequestUpdatePayload }>({
    mutationFn: ({ id, payload }): Promise<AnnouncementRequestUpdate> =>
      api.post<CreateAnnouncementRequestUpdatePayload, AnnouncementRequestUpdate>(
        `/announcement-requests/${encodeURIComponent(id)}/updates`,
        payload,
      ),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_UPDATES, variables.id],
      });
    },
  });
}
