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

// Announcements are cases of `type: "announcement"`, so this is a single
// `POST /cases/search` scoped to that type — read-only (create/target/unpublish
// needs a dedicated backend that isn't built yet). Paged via infinite scroll,
// newest-updated first, mirroring `services/cases.ts` `cases.infinite`.

import { infiniteQueryOptions } from "@tanstack/react-query";
import { CASES_SEARCH_ENDPOINT } from "@config/endpoints";
import type { CaseSearchPayloadDto, CaseSearchResponseDto } from "@src/types";
import { toCaseSummary, type CaseSummary } from "@src/types";
import type { AnnouncementFilters } from "@utils/announcements";
import apiClient from "./apiClient";

export interface AnnouncementSearchResult {
  items: CaseSummary[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
}

const ANNOUNCEMENTS_PAGE_LIMIT = 20;

// Empty filter arrays are omitted so the search defaults to every state across
// all projects. `searchQuery` matches subject/number. State is sent as-is (the
// backend accepts the lowercase UI states) and `toCaseSummary` normalizes the
// display-cased states the search view returns back to lowercase on the way out.
async function searchAnnouncements(filters: AnnouncementFilters, offset: number): Promise<AnnouncementSearchResult> {
  const q = filters.search.trim();
  const payload: CaseSearchPayloadDto = {
    pagination: { offset, limit: ANNOUNCEMENTS_PAGE_LIMIT },
    sortBy: { field: "updatedOn", order: "desc" },
    filters: {
      types: ["announcement"],
      ...(q ? { searchQuery: q } : {}),
      ...(filters.states.length ? { states: filters.states } : {}),
      ...(filters.projects.length ? { projectIds: filters.projects.map((p) => p.id) } : {}),
    },
  };
  const { data } = await apiClient.post<CaseSearchResponseDto>(CASES_SEARCH_ENDPOINT, payload);
  return {
    items: data.cases.map(toCaseSummary),
    total: data.total,
    offset: data.offset,
    limit: data.limit,
    hasMore: data.hasMore,
  };
}

export const announcements = {
  infinite: (filters: AnnouncementFilters) =>
    infiniteQueryOptions({
      queryKey: ["announcements", filters],
      queryFn: ({ pageParam }) => searchAnnouncements(filters, pageParam),
      initialPageParam: 0,
      getNextPageParam: (last) => (last.hasMore ? last.offset + last.limit : undefined),
    }),
};
