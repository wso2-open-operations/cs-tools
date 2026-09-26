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
  RecordAnnouncementRequestDeliveriesPayload,
  SearchAnnouncementRequestDeliveriesResponse,
} from "@features/csm-announcements/types/announcementRequests";

/**
 * `POST /announcement-requests/{id}/deliveries` — upserts one Publish
 * fan-out pass's worth of per-project outcomes; `actorId` is always the
 * authenticated caller, forced server-side. Rejected (409) unless the
 * request is `approved`; (403) unless the caller is its own creator (same
 * restriction as Publish).
 *
 * This is the durable ledger `usePublishAnnouncementRequest` writes to
 * after each fan-out pass, so retry progress survives closing the dialog —
 * see that hook's own doc comment for the full "why".
 */
export function useRecordAnnouncementRequestDeliveries(): UseMutationResult<
  SearchAnnouncementRequestDeliveriesResponse,
  Error,
  { id: string; payload: RecordAnnouncementRequestDeliveriesPayload }
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<
    SearchAnnouncementRequestDeliveriesResponse,
    Error,
    { id: string; payload: RecordAnnouncementRequestDeliveriesPayload }
  >({
    mutationFn: ({ id, payload }): Promise<SearchAnnouncementRequestDeliveriesResponse> =>
      api.post<RecordAnnouncementRequestDeliveriesPayload, SearchAnnouncementRequestDeliveriesResponse>(
        `/announcement-requests/${encodeURIComponent(id)}/deliveries`,
        payload,
      ),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DELIVERIES, variables.id],
      });
    },
  });
}
