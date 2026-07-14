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

import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import {
  CASES_ENDPOINT,
  CASES_SEARCH_ENDPOINT,
  CASE_COMMENTS_SEARCH_ENDPOINT,
  CASE_DETAILS_ENDPOINT,
} from "@config/endpoints";
import type {
  CaseCommentSearchResponseDto,
  CaseCreatePayloadDto,
  CaseSearchFiltersDto,
  CaseSearchPayloadDto,
  CaseSearchResponseDto,
  CaseViewDto,
  CreatedCaseDto,
} from "@src/types";
import { toCaseDetail, toCaseSummary, toComment, type CaseDetail, type CaseSummary, type Comment } from "@src/types";
import apiClient from "./apiClient";

export interface CaseSearchResult {
  items: CaseSummary[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

// Exported (not just used internally) so the Home dashboard's composition query can fan out
// count-only searches (`pagination: { limit: 1 }`, read `.total`) without going through the
// `cases.all` query-options wrapper — mirrors the webapp's useCaseComposition.ts, which calls its
// api client directly for the same reason.
export const getAllCases = async (payload: CaseSearchPayloadDto = {}): Promise<CaseSearchResult> => {
  const { data } = await apiClient.post<CaseSearchResponseDto>(CASES_SEARCH_ENDPOINT, payload);
  const items = data.cases.map(toCaseSummary);
  return {
    items,
    total: data.total,
    limit: data.limit,
    offset: data.offset,
    // Some data sources omit hasMore from the search envelope (see adminUsers.ts's searchUsers,
    // which hits the same quirk on /users/search); derive it from offset/total when that happens
    // instead of treating a missing field as "no more pages".
    hasMore: data.hasMore ?? data.offset + items.length < data.total,
  };
};

const getCase = async (id: string): Promise<CaseDetail> => {
  const { data } = await apiClient.get<CaseViewDto>(CASE_DETAILS_ENDPOINT(id));
  return toCaseDetail(data);
};

const getCaseComments = async (id: string): Promise<Comment[]> => {
  const { data } = await apiClient.post<CaseCommentSearchResponseDto>(CASE_COMMENTS_SEARCH_ENDPOINT(id), {
    pagination: { limit: 50 },
  });
  return data.comments.map(toComment);
};

const createCase = async (payload: CaseCreatePayloadDto): Promise<CreatedCaseDto> => {
  const { data } = await apiClient.post<CreatedCaseDto>(CASES_ENDPOINT, payload);
  return data;
};

const CASES_PAGE_LIMIT = 20;

export const cases = {
  all: (payload: CaseSearchPayloadDto = {}) =>
    queryOptions({
      queryKey: ["cases", payload],
      queryFn: () => getAllCases(payload),
    }),

  // The full "View All" list page: same filters as the recent-5 view but paged via infinite
  // scroll, mirroring the webapp's useGetCsmCases.ts (updatedOn desc, page-by-offset).
  infinite: (filters: CaseSearchFiltersDto) =>
    infiniteQueryOptions({
      queryKey: ["cases", "infinite", filters],
      queryFn: ({ pageParam }) =>
        getAllCases({
          filters,
          sortBy: { field: "updatedOn", order: "desc" },
          pagination: { offset: pageParam, limit: CASES_PAGE_LIMIT },
        }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => (lastPage.hasMore ? lastPage.offset + lastPage.limit : undefined),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["case", id],
      queryFn: () => getCase(id),
    }),

  comments: (id: string) =>
    queryOptions({
      queryKey: ["case", id, "comments"],
      queryFn: () => getCaseComments(id),
    }),

  create: createCase,
};
