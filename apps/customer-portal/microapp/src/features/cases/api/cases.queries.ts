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

import { infiniteQueryOptions, mutationOptions, queryOptions } from "@tanstack/react-query";
import type { Pagination } from "@shared/types";
import type {
  CaseClassificationRequestDto,
  CreateCaseRequestDto,
  CreateCommentRequestDto,
  EditCaseRequestDto,
  GetCasesRequestDto,
  GetCasesStatsRequestDto,
} from "@features/cases/types/case.dto";
import {
  classifyCase,
  createCase,
  createComment,
  editCase,
  getAllCases,
  getAttachment,
  getAttachments,
  getCase,
  getCasesFilters,
  getCasesStats,
  getComments,
} from "@features/cases/api/cases.api";

export const cases = {
  get: (id: string) => queryOptions({ queryKey: ["case", id], queryFn: () => getCase(id) }),

  edit: (id: string) => mutationOptions({ mutationFn: (body: EditCaseRequestDto) => editCase(id, body) }),

  all: (id: string, body: GetCasesRequestDto = {}) =>
    queryOptions({ queryKey: ["cases", id, body], queryFn: () => getAllCases(id, body) }),

  paginated: (id: string, body: GetCasesRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["cases", "paginated", id, body],
      queryFn: ({ pageParam }) => getAllCases(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        return nextOffset >= Math.ceil(totalRecords / limit) ? undefined : nextOffset;
      },
    }),

  filters: (id: string) => queryOptions({ queryKey: ["filters", id], queryFn: () => getCasesFilters(id) }),

  create: mutationOptions({ mutationFn: (body: CreateCaseRequestDto) => createCase(body) }),

  classify: mutationOptions({
    mutationFn: (body: Omit<CaseClassificationRequestDto, "region" | "tier">) => classifyCase(body),
  }),

  stats: (id: string, body: Partial<GetCasesStatsRequestDto> = {}) =>
    queryOptions({ queryKey: ["cases-stats", id, body], queryFn: () => getCasesStats(id, body) }),

  comments: (id: string) => queryOptions({ queryKey: ["comments", id], queryFn: () => getComments(id) }),

  createComment: (id: string) =>
    mutationOptions({ mutationFn: (body: CreateCommentRequestDto) => createComment(id, body) }),

  attachments: (id: string, body: Partial<Omit<Pagination, "totalRecords">> = {}) =>
    queryOptions({ queryKey: ["cases", id, "attachments"], queryFn: () => getAttachments(id, body) }),

  attachment: (id: string) => queryOptions({ queryKey: ["attachment", id], queryFn: () => getAttachment(id) }),
};
