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
import { CASE_CALL_REQUESTS_ENDPOINT, CASE_CALL_REQUESTS_SEARCH_ENDPOINT } from "@config/endpoints";
import type {
  CreateCallRequestPayloadDto,
  CreateCallRequestResponseDto,
  SearchCallRequestsResponseDto,
} from "@src/types";
import { toCallRequest, type CallRequest } from "@src/types";
import apiClient from "./apiClient";

// ServiceNow data source only, mirroring the backend's own scoping.
const getCallRequests = async (caseId: string): Promise<CallRequest[]> => {
  const { data } = await apiClient.post<SearchCallRequestsResponseDto>(CASE_CALL_REQUESTS_SEARCH_ENDPOINT(caseId), {
    pagination: { limit: 50 },
  });
  return (data.callRequests ?? []).map(toCallRequest);
};

const createCallRequest = async (
  caseId: string,
  payload: CreateCallRequestPayloadDto,
): Promise<CreateCallRequestResponseDto> => {
  const { data } = await apiClient.post<CreateCallRequestResponseDto>(CASE_CALL_REQUESTS_ENDPOINT(caseId), payload);
  return data;
};

export const callRequests = {
  forCase: (caseId: string) =>
    queryOptions({
      queryKey: ["case", caseId, "call-requests"],
      queryFn: () => getCallRequests(caseId),
    }),
  create: createCallRequest,
};
