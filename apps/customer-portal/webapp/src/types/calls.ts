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

import type { IdLabelRef, PaginationResponse } from "@/types/common";

// Request type for patching a call request.
export type PatchCallRequest = {
  reason?: string;
  cancellationReason?: string;
  stateKey: number;
  utcTimes?: string[];
  durationInMinutes?: number;
};

// Item type for a call request.
export type CallRequest = {
  id: string;
  number: string;
  case: IdLabelRef;
  reason: string | null;
  preferredTimes: string[];
  durationMin: number;
  scheduleTime: string | null;
  meetingLink: string | null;
  createdOn: string;
  updatedOn: string;
  state: IdLabelRef;
};

// Response type for get call requests.
export type CallRequestsResponse = PaginationResponse & {
  callRequests: CallRequest[];
};

// Response type for creating a call.
export type CreateCallResponse = {
  id: string;
};

// Request type for creating a call.
export type CreateCallRequest = {
  durationInMinutes: number;
  reason: string;
  utcTimes: string[];
};
