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

// Mirrors backend openapi.yaml's CallRequestView / CreateCallRequestPayload /
// SearchCallRequestsPayload. ServiceNow data source only.

export interface CallRequestStateDto {
  /** State key — int or string depending on upstream data source. */
  id: string | number;
  label: string;
}

export interface CallRequestCaseRefDto {
  id: string;
  name: string | null;
}

export interface CallRequestViewDto {
  id: string;
  number: string;
  case: CallRequestCaseRefDto | null;
  reason: string;
  preferredTimes: string[];
  durationMin: number;
  scheduleTime: string | null;
  meetingLink: string | null;
  createdOn: string;
  updatedOn: string | null;
  state: CallRequestStateDto | null;
  cancellationReason: string | null;
}

export interface SearchCallRequestsPayloadDto {
  filters?: {
    states?: string[];
  };
  pagination?: {
    offset?: number;
    limit?: number;
  };
}

export interface SearchCallRequestsResponseDto {
  callRequests: CallRequestViewDto[];
  total: number;
  offset: number;
  limit: number;
}

export interface CreateCallRequestPayloadDto {
  reason: string;
  /** Preferred UTC times for the call, as ISO strings. */
  utcTimes: string[];
  durationInMinutes: number;
}

export interface CreateCallRequestResponseDto {
  message?: string;
  callRequest: {
    id: string;
    createdOn: string;
    createdBy: string;
    state: CallRequestStateDto;
  };
}
