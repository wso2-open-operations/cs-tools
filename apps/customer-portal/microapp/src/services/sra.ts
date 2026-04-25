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

import apiClient from "@src/services/apiClient";
import type { Case, CaseDto, CasesDto, CaseSummary, GetCasesRequestDto, PaginatedArray } from "@src/types";
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import { toCase, toCaseSummary } from "./cases";

import { CASE_DETAILS_ENDPOINT, PROJECT_CASES_ENDPOINT } from "@config/endpoints";

const getAllSecurityRequestAnalysis = async (
  id: string,
  body: GetCasesRequestDto = {},
): Promise<PaginatedArray<CaseSummary>> => {
  const response = (
    await apiClient.post<CasesDto>(PROJECT_CASES_ENDPOINT(id), {
      ...body,
      filters: {
        ...(body?.filters ?? {}),
        caseTypes: ["security_report_analysis"],
      },
    })
  ).data;
  const result = response.cases.map(toCaseSummary) as PaginatedArray<CaseSummary>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

const getSecurityRequestAnalysis = async (id: string): Promise<Case> => {
  const response = (await apiClient.get<CaseDto>(CASE_DETAILS_ENDPOINT(id))).data;
  return toCase(response);
};

/* Query Options */
export const securityReportAnalysis = {
  get: (id: string) =>
    queryOptions({ queryKey: ["security-report-analysis", id], queryFn: () => getSecurityRequestAnalysis(id) }),

  all: (id: string, body: GetCasesRequestDto = {}) =>
    queryOptions({
      queryKey: ["security-report-analysis", id, body],
      queryFn: () => getAllSecurityRequestAnalysis(id, body),
    }),

  paginated: (id: string, body: GetCasesRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["security-report-analysis", "paginated", id, body],
      queryFn: ({ pageParam }) =>
        getAllSecurityRequestAnalysis(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),
};
