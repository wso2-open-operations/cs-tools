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
import type {
  ChangeRequestDto,
  ChangeRequestsDto,
  ChangeRequestsStatsDto,
  GetChangeRequestsRequestDto,
} from "@features/changes/types/change.dto";
import type { ChangeRequest, ChangeRequestSummary } from "@features/changes/types/change.model";
import { toChangeRequest, toChangeRequestSummary } from "@features/changes/mappers/change.mapper";
import {
  CHANGE_REQUEST_DETAILS_ENDPOINT,
  CHANGE_REQUEST_STATS_ENDPOINT,
  PROJECT_CHANGE_REQUESTS_ENDPOINT,
} from "@config/endpoints";

export const getAllChangeRequests = async (
  id: string,
  body: GetChangeRequestsRequestDto = {},
): Promise<PaginatedArray<ChangeRequestSummary>> => {
  const response = (await apiClient.post<ChangeRequestsDto>(PROJECT_CHANGE_REQUESTS_ENDPOINT(id), body)).data;
  const result = response.changeRequests.map(toChangeRequestSummary) as PaginatedArray<ChangeRequestSummary>;
  result.pagination = { totalRecords: response.totalRecords, offset: response.offset, limit: response.limit };
  return result;
};

export const getChangeRequest = async (id: string): Promise<ChangeRequest> => {
  const response = (await apiClient.get<ChangeRequestDto>(CHANGE_REQUEST_DETAILS_ENDPOINT(id))).data;
  return toChangeRequest(response);
};

export const getChangeRequestsStats = async (id: string): Promise<ChangeRequestsStatsDto> => {
  return (await apiClient.get<ChangeRequestsStatsDto>(CHANGE_REQUEST_STATS_ENDPOINT(id))).data;
};
