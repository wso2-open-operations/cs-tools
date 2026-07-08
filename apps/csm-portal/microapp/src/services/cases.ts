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

import { queryOptions } from "@tanstack/react-query";
import { CASES_SEARCH_ENDPOINT, CASE_COMMENTS_SEARCH_ENDPOINT, CASE_DETAILS_ENDPOINT } from "@config/endpoints";
import type {
  CaseCommentSearchResponseDto,
  CaseSearchPayloadDto,
  CaseSearchResponseDto,
  CaseViewDto,
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

const getAllCases = async (payload: CaseSearchPayloadDto = {}): Promise<CaseSearchResult> => {
  const { data } = await apiClient.post<CaseSearchResponseDto>(CASES_SEARCH_ENDPOINT, payload);
  return {
    items: data.cases.map(toCaseSummary),
    total: data.total,
    limit: data.limit,
    offset: data.offset,
    hasMore: data.hasMore,
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

export const cases = {
  all: (payload: CaseSearchPayloadDto = {}) =>
    queryOptions({
      queryKey: ["cases", payload],
      queryFn: () => getAllCases(payload),
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
};
