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
 * Backs the registry page's "Requests" tab -- `POST /announcement-requests/search`
 * narrowed to the given states (any-of, merged into one paginated list). An
 * empty list means every state.
 *
 * Exactly one state is sent as the older single `state` field rather than a
 * one-element `states` list: both mean the same thing to the service, but
 * entity-service rejects unknown request fields outright, so this keeps the
 * default single-state view working even if the webapp is ever rolled out
 * ahead of an entity-service that doesn't know `states` yet. Only a genuine
 * multi-state selection depends on the newer field.
 *
 * `enabled: false` sends nothing: the backend only serves this search to
 * announcement creators, so a caller who is not one must not fire a request
 * that is certain to be refused.
 */
export function useSearchAnnouncementRequests(
  states: AnnouncementRequestState[],
  page: number,
  pageSize: number,
  options: { enabled?: boolean } = {},
): UseQueryResult<SearchAnnouncementRequestsResponse, Error> {
  const api = useBackendApi();
  const offset = page * pageSize;
  // Selection order is irrelevant to the result, so sort + dedupe for a
  // stable cache key and request body: re-toggling the same set of states in
  // a different order must hit the same cache entry, not refetch.
  const normalizedStates = [...new Set(states)].sort();

  return useQuery<SearchAnnouncementRequestsResponse, Error>({
    queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUESTS_SEARCH, normalizedStates.join(","), page, pageSize],
    queryFn: (): Promise<SearchAnnouncementRequestsResponse> =>
      api.post<SearchAnnouncementRequestsPayload, SearchAnnouncementRequestsResponse>(
        "/announcement-requests/search",
        {
          ...(normalizedStates.length === 1 && { state: normalizedStates[0] }),
          ...(normalizedStates.length > 1 && { states: normalizedStates }),
          pagination: { offset, limit: pageSize },
        },
      ),
    placeholderData: keepPreviousData,
    staleTime: 10_000,
    enabled: options.enabled ?? true,
  });
}
