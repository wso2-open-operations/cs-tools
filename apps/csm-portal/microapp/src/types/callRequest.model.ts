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

import type { CallRequestStateDto, CallRequestStateKeyDto, CallRequestViewDto } from "./callRequest.dto";
import { parseBackendTimestamp, parseOptionalBackendTimestamp } from "@utils/dateTime";

export type CallRequestStateKey = CallRequestStateKeyDto;

export interface CallRequest {
  id: string;
  number: string;
  reason: string;
  preferredTimes: string[];
  durationMin: number;
  scheduleTime: Date | null;
  meetingLink: string | null;
  createdOn: Date;
  updatedOn: Date | null;
  stateLabel: string;
  /**
   * Raw state {id, label} — resolve to a CallRequestStateKey with
   * resolveCallRequestStateKey (in @utils/callRequestState) at the point of
   * use, e.g. to decide which agent actions are available. Kept raw here
   * rather than pre-resolved to avoid a model -> utils -> types import cycle.
   */
  state: CallRequestStateDto | null;
  cancellationReason: string | null;
  /** Agent (or team) assigned to run the call, once scheduled. */
  assignee: string | null;
  /** Call notes recorded after the call concludes. */
  notes: string | null;
}

export interface CallRequestUpdateInput {
  caseId: string;
  callRequestId: string;
  state: CallRequestStateKey;
  cancellationReason?: string;
  utcTimes?: string[];
  durationInMinutes?: number;
  meetingDate?: string;
  assignee?: string;
  notes?: string;
  plan?: string;
  attendees?: string;
  actionItems?: string;
  actualDurationMin?: number;
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
    updatedOn: parseOptionalBackendTimestamp(dto.updatedOn),
    stateLabel: dto.state?.label || "Unknown",
    state: dto.state,
    cancellationReason: dto.cancellationReason,
    assignee: dto.assignee,
    notes: dto.notes,
  };
}
