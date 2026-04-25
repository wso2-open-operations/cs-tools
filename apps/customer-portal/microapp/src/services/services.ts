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
import type { GetCasesRequestDto, PaginatedArray } from "@src/types";
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";
import type { ServiceRequest, ServiceRequestSummary } from "@src/types/service.model";
import type { ServiceRequestDto, ServiceRequestsDto } from "@src/types/service.dto";

import { CASE_DETAILS_ENDPOINT, PROJECT_CASES_ENDPOINT } from "@config/endpoints";

const getAllServiceRequests = async (
  id: string,
  body: GetCasesRequestDto = {},
): Promise<PaginatedArray<ServiceRequestSummary>> => {
  const response = (
    await apiClient.post<ServiceRequestsDto>(PROJECT_CASES_ENDPOINT(id), {
      ...body,
      filters: {
        ...(body?.filters ?? {}),
        caseTypes: ["service_request"],
      },
    })
  ).data;
  const result = response.cases.map(toServiceRequestSummary) as PaginatedArray<ServiceRequestSummary>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

const getServiceRequest = async (id: string): Promise<ServiceRequest> => {
  const response = (await apiClient.get<ServiceRequestDto>(CASE_DETAILS_ENDPOINT(id))).data;
  return toServiceRequest(response);
};

/* Mappers */
export function toServiceRequestSummary(dto: ServiceRequestsDto["cases"][number]): ServiceRequestSummary {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    title: dto.title,
    description: dto.description ?? "",
    assignee: dto.assignedTeam?.label,
    issueType: dto.issueType?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
  };
}

export function toServiceRequest(dto: ServiceRequestDto): ServiceRequest {
  return {
    id: dto.id,
    internalId: dto.internalId,
    number: dto.number,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    updatedOn: new Date(dto.updatedOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    title: dto.title,
    description: dto.description ?? "",
    assignee: dto.assignedTeam?.label,
    issueType: dto.issueType?.label,
    statusId: dto.status?.id,
    severityId: dto.severity?.id,
    deployment: dto.deployment?.label,
    product: dto.product?.label,
  };
}

/* Query Options */
export const serviceRequests = {
  get: (id: string) => queryOptions({ queryKey: ["service-request", id], queryFn: () => getServiceRequest(id) }),

  all: (id: string, body: GetCasesRequestDto = {}) =>
    queryOptions({
      queryKey: ["service-requests", id, body],
      queryFn: () => getAllServiceRequests(id, body),
    }),

  paginated: (id: string, body: GetCasesRequestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["service-requests", "paginated", id, body],
      queryFn: ({ pageParam }) =>
        getAllServiceRequests(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),
};
