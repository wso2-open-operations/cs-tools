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

// Wire shapes for the ServiceNow-native time-card endpoints, mirroring the
// csm-portal webapp's `api/backend/types.ts`. Time is in whole MINUTES on the
// wire (see `totalTime`), not hours.

export type BeTimeCardState = "pending" | "submitted" | "approved" | "rejected" | "processed" | "recalled";

export interface BeTimeCardRef {
  id: string;
  name: string;
}

export interface BeTimeCardCaseRef {
  id: string;
  name: string;
  number: string;
}

// A time card as returned by search and the mutation endpoints. The backend
// never echoes back category / issue complexity / work-log comment / the
// per-activity minute breakdown / lead comment, even though those are accepted
// on write.
export interface BeTimeCardView {
  id: string;
  totalTime: number;
  createdOn: string;
  hasBillable: boolean;
  state: BeTimeCardState;
  user?: BeTimeCardRef;
  approvedBy?: BeTimeCardRef;
  project?: BeTimeCardRef;
  case?: BeTimeCardCaseRef;
}

// `caseId` is deliberately omitted here: it's documented in entity-service's
// openapi.yaml and implemented end-to-end, but confirmed non-functional live
// (always returns total: 0). Case scoping goes through `projectIds` plus a
// client-side filter instead. `states` is filtered client-side too — combining
// it with a large `projectIds` array 500s the backend.
export interface BeSearchTimeCardsFilters {
  projectIds?: string[];
  /** Only time cards submitted by this user. */
  userId?: string;
  /** Only time cards this user is eligible to approve (SN `approver_list`). */
  approverId?: string;
  /** Only time cards actually approved by this user (SN `approved_by`). */
  approvedById?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  startDate?: string;
  /** ISO 8601 date (YYYY-MM-DD). */
  endDate?: string;
  states?: BeTimeCardState[];
}

export interface BePagination {
  limit?: number;
  offset?: number;
}

export interface BeSearchTimeCardsPayload {
  filters?: BeSearchTimeCardsFilters;
  pagination?: BePagination;
}

export interface BeSearchTimeCardsResponse {
  timeCards: BeTimeCardView[];
  total: number;
  limit: number;
  offset: number;
}

// Either editable fields or a state transition — mutually exclusive, per the
// backend contract. The portal only ever sends the state-transition form.
export interface BeUpdateTimeCardPayload {
  state?: Extract<BeTimeCardState, "approved" | "rejected">;
  leadComment?: string;
}

export interface BeTimeCardMutationResponse {
  message?: string;
  timeCard: BeTimeCardView;
}
