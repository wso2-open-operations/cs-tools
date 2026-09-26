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
 * `POST /announcement-requests/{id}/submit` — takes no body. The backend
 * resolves the real audience server-side (exclusion denylist applied) and
 * freezes it as `resolvedProjectIds`; the caller only needs the id. Rejected
 * (409) unless the request is a `draft` with a dry run already recorded.
 *
 * Takes `id` per call, not as a hook argument — see
 * `useUpdateAnnouncementRequest`'s doc comment for why: a caller that just
 * created the draft/recorded its dry run in the same flow would otherwise
 * close over a stale, pre-creation `id`.
 */
export function useSubmitAnnouncementRequest(): UseMutationResult<AnnouncementRequest, Error, { id: string }> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequest, Error, { id: string }>({
    mutationFn: ({ id }): Promise<AnnouncementRequest> =>
      api.postEmpty<AnnouncementRequest>(`/announcement-requests/${encodeURIComponent(id)}/submit`),
    onSuccess: (_result, variables) => {
      queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DETAIL, variables.id],
      });
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
    },
  });
}
