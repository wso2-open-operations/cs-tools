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
import {
  registryPayloadFromFilters,
  type SearchAnnouncementRegistryPayload,
  type SearchAnnouncementRegistryResponse,
} from "@features/csm-announcements/types/announcementRegistry";
import type { AnnouncementFilters } from "@features/csm-announcements/types/csmAnnouncements";

/**
 * `POST /announcements/registry/search` — the Announcements tab's own
 * grouped list: one row per published announcement (a "batch" — every case
 * it created collapses into this one row) or one row per case with no known
 * owning announcement request (anything sent before this workflow existed,
 * shown individually rather than dropped). Replaces the old flat
 * `useSearchAnnouncements` (`POST /cases/search`), which showed one row per
 * case, repeating the same subject once per project.
 *
 * `page` is zero-based (MUI `TablePagination`); `pageSize` is the row
 * limit — applied server-side to the *grouped* row list, not the raw case
 * list underneath it (a batch of 50 cases collapsing into 1 row means raw
 * case offset/limit can't correspond to grouped-row offset/limit at all).
 */
export function useSearchAnnouncementRegistry(
  filters: AnnouncementFilters,
  page: number,
  pageSize: number,
): UseQueryResult<SearchAnnouncementRegistryResponse, Error> {
  const api = useBackendApi();
  const offset = page * pageSize;
  const payload = registryPayloadFromFilters(filters, offset, pageSize);

  return useQuery<SearchAnnouncementRegistryResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache.
    queryKey: [
      ApiQueryKeys.CSM_ANNOUNCEMENT_REGISTRY,
      payload.search ?? "",
      [...(payload.states ?? [])].sort(),
      [...(payload.projectIds ?? [])].sort(),
      page,
      pageSize,
    ],
    queryFn: (): Promise<SearchAnnouncementRegistryResponse> =>
      api.post<SearchAnnouncementRegistryPayload, SearchAnnouncementRegistryResponse>(
        "/announcements/registry/search",
        payload,
      ),
    // Keep the current page of rows on screen while the next page / a
    // changed filter loads, instead of blanking the table to skeletons —
    // same convention useSearchAnnouncements already uses.
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}
