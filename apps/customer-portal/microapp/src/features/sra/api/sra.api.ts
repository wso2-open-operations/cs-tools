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

import apiClient from "@infrastructure/api/client";
import type { PaginatedArray } from "@shared/types";
import type { GetCasesRequestDto, CaseDto, CasesDto } from "@features/cases/types/case.dto";
import type { Case, CaseSummary } from "@features/cases/types/case.model";
import { toCase, toCaseSummary } from "@features/cases/mappers/case.mapper";
import { CASE_DETAILS_ENDPOINT, PROJECT_CASES_ENDPOINT } from "@config/endpoints";

export const getAllSecurityReportAnalysis = async (
  id: string,
  body: GetCasesRequestDto = {},
): Promise<PaginatedArray<CaseSummary>> => {
  const response = (
    await apiClient.post<CasesDto>(PROJECT_CASES_ENDPOINT(id), {
      ...body,
      filters: { ...(body?.filters ?? {}), caseTypes: ["security_report_analysis"] },
    })
  ).data;
  const result = response.cases.map(toCaseSummary) as PaginatedArray<CaseSummary>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};

export const getSecurityReportAnalysis = async (id: string): Promise<Case> => {
  const response = (await apiClient.get<CaseDto>(CASE_DETAILS_ENDPOINT(id))).data;
  return toCase(response);
};
