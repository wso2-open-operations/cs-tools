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
  AnnouncementRequest,
  UpdateAnnouncementRequestPayload,
} from "@features/csm-announcements/types/announcementRequests";

export interface UpdateAnnouncementRequestVariables extends UpdateAnnouncementRequestPayload {
  id: string;
}

/**
 * `PATCH /announcement-requests/{id}`. Takes `id` as part of the mutation's
 * own variables rather than as a hook argument bound at render time — a
 * caller that creates the request and immediately wants to update it (e.g.
 * the create forms, which lazily create a draft then re-sync its content)
 * would otherwise capture a stale `id` from the render before the id existed,
 * since a `useMutation` bound to a hook argument closes over that render's
 * value. Passing `id` per call sidesteps that entirely.
 *
 * Behaves differently depending on the request's *current* state (enforced
 * server-side, not here — see entity-service's `AnnouncementRequestService.Update`
 * doc comment): editing a `draft` just updates fields; editing a
 * `pending_approval` request also reverts it to `draft` and clears the
 * dry-run/submit snapshot; editing an `approved` request updates
 * subject/description in place with no state change (and rejects an
 * audience change with 400 — the approved snapshot is frozen); a
 * `published` request rejects any edit outright (409). The response always
 * reflects the request's state *after* the edit, so callers don't need to
 * guess which branch fired.
 */
export function useUpdateAnnouncementRequest(): UseMutationResult<
  AnnouncementRequest,
  Error,
  UpdateAnnouncementRequestVariables
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequest, Error, UpdateAnnouncementRequestVariables>({
    mutationFn: ({ id, ...input }): Promise<AnnouncementRequest> =>
      api.patch<UpdateAnnouncementRequestPayload, AnnouncementRequest>(
        `/announcement-requests/${encodeURIComponent(id)}`,
        input,
      ),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, variables.id],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
    },
  });
}
