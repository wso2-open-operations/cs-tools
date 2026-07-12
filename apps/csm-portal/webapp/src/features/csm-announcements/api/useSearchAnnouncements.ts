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
import { beStateFromUi, uiStateFromBe } from "@api/backend/mappers";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";
import type {
  AnnouncementFilters,
  CsmAnnouncementRow,
  CsmAnnouncementsListResponse,
} from "@features/csm-announcements/types/csmAnnouncements";

/**
 * Read-only announcements list. Announcements are cases of
 * `type: "announcement"`, so this is a single `POST /cases/search` scoped to
 * that type, mapping each search view to a trimmed {@link CsmAnnouncementRow}.
 * A free-text `search` is pushed into the payload's `searchQuery` (matches
 * subject / number); the backend paginates and sorts by last-updated
 * descending, so the most recent announcements load on arrival.
 *
 * The state / project filters are pushed into the search payload: UI
 * `CaseState` → `beStateFromUi`, project ids as-is. Empty filter arrays are
 * omitted, so the list shows every state across all projects by default.
 * (No severity filter — announcements don't carry a severity of their own.)
 *
 * Creating / targeting / unpublishing announcements is not covered here — it
 * needs the dedicated announcement backend (digiops-cs#2053), which isn't
 * built yet. `page` is zero-based (MUI `TablePagination`); `pageSize` is the
 * row limit.
 */
export function useSearchAnnouncements(
  filters: AnnouncementFilters,
  page: number,
  pageSize: number,
): UseQueryResult<CsmAnnouncementsListResponse, Error> {
  const api = useBackendApi();
  const q = filters.search.trim();
  const offset = page * pageSize;

  return useQuery<CsmAnnouncementsListResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache.
    queryKey: [
      ApiQueryKeys.CSM_ANNOUNCEMENTS,
      q,
      [...filters.states].sort(),
      [...filters.projectIds].sort(),
      page,
      pageSize,
    ],
    queryFn: async (): Promise<CsmAnnouncementsListResponse> => {
      const res = await api.post<BeCaseSearchPayload, BeCaseSearchResponse>(
        "/cases/search",
        {
          pagination: { offset, limit: pageSize },
          sortBy: { field: "updatedOn", order: "desc" },
          filters: {
            types: ["announcement"],
            ...(q.length > 0 && { searchQuery: q }),
            ...(filters.states.length > 0 && {
              states: filters.states.map(beStateFromUi),
            }),
            ...(filters.projectIds.length > 0 && {
              projectIds: filters.projectIds,
            }),
          },
        },
      );

      const announcements: CsmAnnouncementRow[] = (res.cases ?? []).map((c) => ({
        id: c.id,
        number: c.number,
        subject: c.subject ?? "(no subject)",
        projectName: c.project?.name ?? "—",
        // The search view returns display-cased states (e.g. "Closed"); normalize
        // to the canonical lowercase UI state so StateChip picks the right
        // label/colour (matches the cases list — see useGetCsmCases).
        state: c.state ? uiStateFromBe(c.state) : undefined,
        createdBy: c.createdBy,
        createdAt: c.createdOn ?? "",
        updatedAt: c.updatedOn ?? c.createdOn ?? "",
      }));

      return {
        announcements,
        total: res.total ?? announcements.length,
        limit: res.limit ?? pageSize,
        offset: res.offset ?? offset,
        hasMore: res.hasMore ?? false,
      };
    },
    // Keep the current page of rows on screen while the next page / a changed
    // filter loads, instead of blanking the table to skeletons each time.
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}
