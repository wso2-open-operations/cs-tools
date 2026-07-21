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

// Engagements are cases of `type: "engagement"`, so this is a single
// `POST /cases/search` scoped to that type — read-only (create isn't in scope for
// this pass, mirrors the webapp's CsmEngagementsPage which only lists/filters).
// Paged via infinite scroll, newest-updated first, mirroring `services/cases.ts`
// `cases.infinite` and `services/announcements.ts`'s `announcements.infinite`.

import { infiniteQueryOptions } from "@tanstack/react-query";
import { CASES_SEARCH_ENDPOINT } from "@config/endpoints";
import type { CaseSearchPayloadDto, CaseSearchResponseDto } from "@src/types";
import { toCaseSummary, type CaseSummary } from "@src/types";
import type { EngagementFilters } from "@utils/engagements";
import apiClient from "./apiClient";

export interface EngagementSearchResult {
  items: CaseSummary[];
  total: number;
  offset: number;
  limit: number;
  hasMore: boolean;
}

const ENGAGEMENTS_PAGE_LIMIT = 20;

// Empty filter arrays are omitted so the search defaults to every state/type
// across all projects. `searchQuery` matches subject/number.
async function searchEngagements(filters: EngagementFilters, offset: number): Promise<EngagementSearchResult> {
  const q = filters.search.trim();
  const payload: CaseSearchPayloadDto = {
    pagination: { offset, limit: ENGAGEMENTS_PAGE_LIMIT },
    sortBy: { field: "updatedOn", order: "desc" },
    filters: {
      types: ["engagement"],
      ...(q ? { searchQuery: q } : {}),
      ...(filters.states.length ? { states: filters.states } : {}),
      ...(filters.workStates.length ? { workStates: filters.workStates } : {}),
      ...(filters.projects.length ? { projectIds: filters.projects.map((p) => p.id) } : {}),
      ...(filters.engagementTypes.length ? { engagementTypes: filters.engagementTypes } : {}),
      ...(filters.assignees.length ? { assignedUserIds: filters.assignees.map((a) => a.id) } : {}),
      ...(filters.productNames.length ? { productNames: filters.productNames } : {}),
    },
  };
  const { data } = await apiClient.post<CaseSearchResponseDto>(CASES_SEARCH_ENDPOINT, payload);
  const items = data.cases.map(toCaseSummary);
  return {
    items,
    total: data.total,
    offset: data.offset,
    limit: data.limit,
    // Some data sources omit hasMore from the search envelope (see cases.ts's
    // getAllCases and adminUsers.ts's searchUsers, which hit the same quirk);
    // derive it from offset/total when that happens instead of treating a
    // missing/false field as "no more pages" after the first page. Also require
    // a non-empty page: an empty page with a stale/inconsistent total should
    // never report hasMore, or the infinite query would keep requesting
    // further pages forever.
    hasMore: data.hasMore ?? (data.offset + items.length < data.total && items.length > 0),
  };
}

export const engagements = {
  infinite: (filters: EngagementFilters) =>
    infiniteQueryOptions({
      queryKey: ["engagements", filters],
      queryFn: ({ pageParam }) => searchEngagements(filters, pageParam),
      initialPageParam: 0,
      getNextPageParam: (last) => (last.hasMore ? last.offset + last.limit : undefined),
    }),
};
