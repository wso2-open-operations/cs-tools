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
import { CASE_DETAILS_ENDPOINT, PROJECT_CASES_ENDPOINT } from "@config/endpoints";

import apiClient from "@infrastructure/api/client";

import type { GetCasesRequestDto } from "@features/case-types/cases/types/case.dto";
import {
  toServiceRequest,
  toServiceRequestSummary,
} from "@features/case-types/service-requests/mappers/service-request.mapper";
import type {
  ServiceRequestDto,
  ServiceRequestsDto,
} from "@features/case-types/service-requests/types/service-request.dto";
import type {
  ServiceRequest,
  ServiceRequestSummary,
} from "@features/case-types/service-requests/types/service-request.model";

import { CASE_TYPES } from "@shared/constants";
import type { PaginatedArray } from "@shared/types";

export const getAllServiceRequests = async (
  id: string,
  body: GetCasesRequestDto = {},
): Promise<PaginatedArray<ServiceRequestSummary>> => {
  const response = (
    await apiClient.post<ServiceRequestsDto>(PROJECT_CASES_ENDPOINT(id), {
      ...body,
      filters: { ...(body?.filters ?? {}), caseTypes: [CASE_TYPES.SERVICE_REQUEST] },
    })
  ).data;
  const result = response.cases.map(toServiceRequestSummary) as PaginatedArray<ServiceRequestSummary>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};

export const getServiceRequest = async (id: string): Promise<ServiceRequest> => {
  const response = (await apiClient.get<ServiceRequestDto>(CASE_DETAILS_ENDPOINT(id))).data;
  return toServiceRequest(response);
};
