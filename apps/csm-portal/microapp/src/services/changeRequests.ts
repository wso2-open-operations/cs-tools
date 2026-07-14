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
import { CHANGE_REQUESTS_SEARCH_ENDPOINT, CHANGE_REQUEST_ENDPOINT } from "@config/endpoints";
import type {
  ChangeRequestDetailDto,
  ChangeRequestSearchPayloadDto,
  ChangeRequestSearchResponseDto,
  PatchChangeRequestPayloadDto,
  PatchChangeRequestResponseDto,
} from "@src/types";
import { toChangeRequestDetail, toChangeRequestSummary, type ChangeRequestSummary } from "@src/types";
import apiClient from "./apiClient";

export interface ChangeRequestSearchResult {
  items: ChangeRequestSummary[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}

// The search response has no `hasMore` field (unlike /cases/search) — derive it from offset/total,
// same defensive pattern as adminUsers.ts's searchUsers for the same reason.
const searchChangeRequests = async (
  payload: ChangeRequestSearchPayloadDto = {},
): Promise<ChangeRequestSearchResult> => {
  const { data } = await apiClient.post<ChangeRequestSearchResponseDto>(CHANGE_REQUESTS_SEARCH_ENDPOINT, payload);
  const items = data.changeRequests.map(toChangeRequestSummary);
  return {
    items,
    total: data.total,
    limit: data.limit,
    offset: data.offset,
    // Guard against an empty page still reporting hasMore (e.g. a stale/inconsistent total) —
    // an empty page always means there's nothing further to fetch, regardless of total.
    hasMore: items.length > 0 && data.offset + items.length < data.total,
  };
};

const getChangeRequest = async (id: string) => {
  const { data } = await apiClient.get<ChangeRequestDetailDto>(CHANGE_REQUEST_ENDPOINT(id));
  return toChangeRequestDetail(data);
};

const patchChangeRequest = async (id: string, payload: PatchChangeRequestPayloadDto) => {
  const { data } = await apiClient.patch<PatchChangeRequestResponseDto>(CHANGE_REQUEST_ENDPOINT(id), payload);
  return data;
};

const CHANGE_REQUEST_PAGE_LIMIT = 20;

export const changeRequests = {
  infinite: (payload: Omit<ChangeRequestSearchPayloadDto, "pagination"> = {}) =>
    infiniteQueryOptions({
      queryKey: ["change-requests", "infinite", payload],
      queryFn: ({ pageParam }) =>
        searchChangeRequests({ ...payload, pagination: { offset: pageParam, limit: CHANGE_REQUEST_PAGE_LIMIT } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => (lastPage.hasMore ? lastPage.offset + lastPage.limit : undefined),
    }),

  get: (id: string) =>
    queryOptions({
      queryKey: ["change-request", id],
      queryFn: () => getChangeRequest(id),
    }),

  patch: patchChangeRequest,
};
