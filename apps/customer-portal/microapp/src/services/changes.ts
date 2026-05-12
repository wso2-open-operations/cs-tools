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
import type {
  PaginatedArray,
  GetChangeRequestsRquestDto,
  ChangeRequestSummary,
  ChangeRequestsDto,
  ChangeRequestDto,
  ChangeRequest,
  ChangeRequestsStatsDto,
} from "@src/types";
import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import {
  CHANGE_REQUEST_DETAILS_ENDPOINT,
  CHANGE_REQUEST_STATS_ENDPOINT,
  PROJECT_CHANGE_REQUESTS_ENDPOINT,
} from "@src/config/endpoints";

const getAllChangeRequests = async (
  id: string,
  body: GetChangeRequestsRquestDto = {},
): Promise<PaginatedArray<ChangeRequestSummary>> => {
  const response = (await apiClient.post<ChangeRequestsDto>(PROJECT_CHANGE_REQUESTS_ENDPOINT(id), body)).data;
  const result = response.changeRequests.map(toChangeRequestSummary) as PaginatedArray<ChangeRequestSummary>;
  result.pagination = {
    totalRecords: response.totalRecords,
    offset: response.offset,
    limit: response.limit,
  };

  return result;
};

const getChangeRequest = async (id: string): Promise<ChangeRequest> => {
  const response = (await apiClient.get<ChangeRequestDto>(CHANGE_REQUEST_DETAILS_ENDPOINT(id))).data;
  return toChangeRequest(response);
};

const getChangeRequestsStats = async (id: string): Promise<ChangeRequestsStatsDto> => {
  return (await apiClient.get<ChangeRequestsStatsDto>(CHANGE_REQUEST_STATS_ENDPOINT(id))).data;
};

/* Mappers */
function toChangeRequestSummary(dto: ChangeRequestsDto["changeRequests"][number]): ChangeRequestSummary {
  return {
    id: dto.id,
    internalId: dto.case?.internalId,
    number: dto.number,
    title: dto.title,
    description: dto.case?.label ?? "",
    requestType: dto.type?.label,
    impactId: dto.impact?.id,
    statusId: dto.state?.id,
    assignedTeam: dto.assignedTeam?.label,
    endDate: dto.endDate ? new Date(dto.endDate.replace(" ", "T")) : undefined,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    updatedOn: new Date(dto.updatedOn.replace(" ", "T")),
  };
}

function toChangeRequest(dto: ChangeRequestDto): ChangeRequest {
  return {
    id: dto.id,
    internalId: dto.case?.internalId,
    number: dto.number,
    title: dto.title,
    description: dto.case?.label ?? "",
    requestType: dto.type?.label,
    impactId: dto.impact?.id,
    statusId: dto.state?.id,
    endDate: dto.endDate ? new Date(dto.endDate.replace(" ", "T")) : undefined,
    createdOn: new Date(dto.createdOn.replace(" ", "T")),
    updatedOn: new Date(dto.updatedOn.replace(" ", "T")),
    createdBy: dto.createdBy,
    approvedOn: dto.approvedOn ? new Date(dto.approvedOn.replace(" ", "T")) : undefined,
    approvedBy: dto.approvedBy?.label ?? undefined,
    duration: dto.duration ?? undefined,
    hasCustomerApproved: dto.hasCustomerApproved,
    hasCustomerReviewed: dto.hasCustomerReviewed,
    assignedTeam: dto.assignedTeam?.label,
    serviceOutage: dto.serviceOutage ?? undefined,
    rollbackPlan: dto.rollbackPlan ?? undefined,
    communicationPlan: dto.communicationPlan ?? undefined,
    testPlan: dto.testPlan ?? undefined,
    deployment: dto.deployment?.label,
  };
}

/* Query Options */
export const changeRequests = {
  get: (id: string) => queryOptions({ queryKey: ["change-request", id], queryFn: () => getChangeRequest(id) }),

  all: (id: string, body: GetChangeRequestsRquestDto = {}) =>
    queryOptions({
      queryKey: ["change-requests", id, body],
      queryFn: () => getAllChangeRequests(id, body),
    }),

  paginated: (id: string, body: GetChangeRequestsRquestDto = {}) =>
    infiniteQueryOptions({
      queryKey: ["change-requests", "paginated", id, body],
      queryFn: ({ pageParam }) =>
        getAllChangeRequests(id, { ...body, pagination: { ...body.pagination, offset: pageParam } }),
      initialPageParam: 0,
      getNextPageParam: (lastPage) => {
        const { offset, limit, totalRecords } = lastPage.pagination;
        const nextOffset = offset + 1;
        const totalPages = Math.ceil(totalRecords / limit);
        return nextOffset >= totalPages ? undefined : nextOffset;
      },
    }),

  stats: (id: string) =>
    queryOptions({
      queryKey: ["change-requests-stats", id],
      queryFn: () => getChangeRequestsStats(id),
    }),
};
