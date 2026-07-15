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

import type { CallRequestViewDto } from "./callRequest.dto";
import { parseBackendTimestamp, parseOptionalBackendTimestamp } from "@utils/dateTime";

export interface CallRequest {
  id: string;
  number: string;
  reason: string;
  preferredTimes: string[];
  durationMin: number;
  scheduleTime: Date | null;
  meetingLink: string | null;
  createdOn: Date;
  stateLabel: string;
  cancellationReason: string | null;
}

export function toCallRequest(dto: CallRequestViewDto): CallRequest {
  return {
    id: dto.id,
    number: dto.number,
    reason: dto.reason,
    preferredTimes: dto.preferredTimes ?? [],
    durationMin: dto.durationMin,
    scheduleTime: parseOptionalBackendTimestamp(dto.scheduleTime),
    meetingLink: dto.meetingLink,
    createdOn: parseBackendTimestamp(dto.createdOn),
    stateLabel: dto.state?.label || "Unknown",
    cancellationReason: dto.cancellationReason,
  };
}
