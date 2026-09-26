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
  RecordAnnouncementRequestDryRunPayload,
} from "@features/csm-announcements/types/announcementRequests";

export interface RecordAnnouncementRequestDryRunVariables extends RecordAnnouncementRequestDryRunPayload {
  id: string;
}

/**
 * `POST /announcement-requests/{id}/dry-run` — persists the case id from an
 * already-run `useAnnouncementDryRun` onto the draft, which is what actually
 * unblocks `useSubmitAnnouncementRequest` (rejected 409 without this). Only
 * valid while the request is a `draft`; rejected (409) otherwise.
 *
 * Takes `id` per call, not as a hook argument — see
 * `useUpdateAnnouncementRequest`'s doc comment for why: a caller that just
 * created the draft this same mutation needs to attach to would otherwise
 * close over a stale, pre-creation `id`.
 */
export function useRecordAnnouncementRequestDryRun(): UseMutationResult<
  AnnouncementRequest,
  Error,
  RecordAnnouncementRequestDryRunVariables
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequest, Error, RecordAnnouncementRequestDryRunVariables>({
    mutationFn: ({ id, ...input }): Promise<AnnouncementRequest> =>
      api.post<RecordAnnouncementRequestDryRunPayload, AnnouncementRequest>(
        `/announcement-requests/${encodeURIComponent(id)}/dry-run`,
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
