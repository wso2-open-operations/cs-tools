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
  keepPreviousData,
  useQuery,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  AnnouncementRequestState,
  SearchAnnouncementRequestsPayload,
  SearchAnnouncementRequestsResponse,
} from "@features/csm-announcements/types/announcementRequests";

/**
 * Backs the registry page's "Pending" tab — `POST /announcement-requests/search`
 * filtered to the not-yet-published states, so `published` requests (already
 * visible as real cases in the "Announcements" tab) don't show twice. Passing
 * `state` narrows to one specific state instead (e.g. a future "my drafts"
 * view); omit it to show every state.
 */
export function useSearchAnnouncementRequests(
  state: AnnouncementRequestState | undefined,
  page: number,
  pageSize: number,
): UseQueryResult<SearchAnnouncementRequestsResponse, Error> {
  const api = useBackendApi();
  const offset = page * pageSize;

  return useQuery<SearchAnnouncementRequestsResponse, Error>({
    queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH, state ?? "", page, pageSize],
    queryFn: (): Promise<SearchAnnouncementRequestsResponse> =>
      api.post<SearchAnnouncementRequestsPayload, SearchAnnouncementRequestsResponse>(
        "/announcement-requests/search",
        {
          ...(state && { state }),
          pagination: { offset, limit: pageSize },
        },
      ),
    placeholderData: keepPreviousData,
    staleTime: 10_000,
  });
}
