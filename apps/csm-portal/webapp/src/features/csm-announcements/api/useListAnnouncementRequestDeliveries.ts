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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { SearchAnnouncementRequestDeliveriesResponse } from "@features/csm-announcements/types/announcementRequests";

/**
 * `GET /announcement-requests/{id}/deliveries` — every delivery recorded so
 * far for this request. Enabled only once the request is `approved` (the
 * state Publish's own fan-out runs in — see
 * `usePublishAnnouncementRequest`, which uses this to resume a fan-out
 * that was interrupted by the dialog closing, rather than resending to
 * every resolved project from scratch) or already `published` (so a
 * published request's dialog can still show its own final delivery record).
 */
export function useListAnnouncementRequestDeliveries(
  id: string | undefined,
  enabled: boolean,
): UseQueryResult<SearchAnnouncementRequestDeliveriesResponse | null, Error> {
  const api = useBackendApi();
  return useQuery<SearchAnnouncementRequestDeliveriesResponse | null, Error>({
    queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_DELIVERIES, id ?? ""],
    queryFn: (): Promise<SearchAnnouncementRequestDeliveriesResponse | null> =>
      api.get<SearchAnnouncementRequestDeliveriesResponse>(
        `/announcement-requests/${encodeURIComponent(id as string)}/deliveries`,
      ),
    enabled: !!id && enabled,
    staleTime: 10_000,
  });
}
