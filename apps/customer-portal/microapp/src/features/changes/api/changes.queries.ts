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
import type { GetChangeRequestsRequestDto } from "@features/changes/types/change.dto";
import { getAllChangeRequests, getChangeRequest, getChangeRequestsStats } from "@features/changes/api/changes.api";

export const changeRequests = {
  get: (id: string) => queryOptions({ queryKey: ["change-request", id], queryFn: () => getChangeRequest(id) }),

  all: (id: string, body: GetChangeRequestsRequestDto = {}) =>
    queryOptions({ queryKey: ["change-requests", id, body], queryFn: () => getAllChangeRequests(id, body) }),

  paginated: (id: string, body: GetChangeRequestsRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["change-requests", "paginated", id, body],
      queryFn: ({ pageParam }) =>
        getAllChangeRequests(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        return nextOffset >= Math.ceil(totalRecords / limit) ? undefined : nextOffset;
      },
    }),

  stats: (id: string) =>
    queryOptions({ queryKey: ["change-requests-stats", id], queryFn: () => getChangeRequestsStats(id) }),
};
