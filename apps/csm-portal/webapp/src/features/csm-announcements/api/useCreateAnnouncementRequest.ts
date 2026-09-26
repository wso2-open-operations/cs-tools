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
  CreateAnnouncementRequestPayload,
} from "@features/csm-announcements/types/announcementRequests";

/**
 * Create a draft via `POST /announcement-requests`. The backend forces
 * `createdBy` to the authenticated caller — the payload never carries it (see
 * that handler's own doc comment on why an inbound struct with no such field
 * at all, not just an ignored one, is the deliberate shape).
 */
export function useCreateAnnouncementRequest(): UseMutationResult<
  AnnouncementRequest,
  Error,
  CreateAnnouncementRequestPayload
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<AnnouncementRequest, Error, CreateAnnouncementRequestPayload>({
    mutationFn: (input): Promise<AnnouncementRequest> =>
      api.post<CreateAnnouncementRequestPayload, AnnouncementRequest>(
        "/announcement-requests",
        input,
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH] });
    },
  });
}
