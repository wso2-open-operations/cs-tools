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
import { BackendApiError, useBackendApi } from "@api/backend/client";
import type { AnnouncementRequestAttachment } from "@features/csm-announcements/types/announcementRequests";

/**
 * `GET /announcement-requests/{id}/attachments` — the files attached to a
 * request, oldest first. A request carries at most five, so there is no
 * paging. `enabled` lets a caller that only needs the list in some states (the
 * dialog's read-only view of a published request still does) skip it entirely.
 *
 * A 404 reads as "no files": the request itself was just read, so it can only
 * mean a backend that predates this feature, which cannot hold any. Without it a
 * web app deployed ahead of its backend would report every request's attachments
 * as unloadable, and Publish (which waits for this list) would stay blocked.
 */
export function useListAnnouncementRequestAttachments(
  id: string | undefined,
  enabled = true,
): UseQueryResult<AnnouncementRequestAttachment[], Error> {
  const api = useBackendApi();
  return useQuery<AnnouncementRequestAttachment[], Error>({
    queryKey: [ApiQueryKeys.ANNOUNCEMENT_REQUEST_ATTACHMENTS, id ?? ""],
    queryFn: async (): Promise<AnnouncementRequestAttachment[]> => {
      try {
        const response = await api.get<{ attachments?: AnnouncementRequestAttachment[] }>(
          `/announcement-requests/${encodeURIComponent(id as string)}/attachments`,
        );
        return response?.attachments ?? [];
      } catch (error) {
        if (error instanceof BackendApiError && error.status === 404) return [];
        throw error;
      }
    },
    enabled: !!id && enabled,
    staleTime: 10_000,
  });
}
