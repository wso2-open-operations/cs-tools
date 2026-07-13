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

import type { BeTimeCardView } from "./timecard.dto";

// Approval lifecycle of a time card, per the entity-service contract. A card is
// created already `submitted` — there is no draft/pending step via the portal
// API. An approver then decides `approved` or `rejected`. `recalled`/`processed`
// exist in the backend enum but the portal exposes no endpoint to set them.
export type TimeCardState = "pending" | "submitted" | "approved" | "rejected" | "recalled" | "processed";

// A time card as returned by search and the mutation endpoints. `totalMinutes`
// is whole minutes (a direct passthrough of the wire's `totalTime`).
export interface CsmTimeCard {
  id: string;
  caseId: string;
  /** Case reference shown to humans (e.g. CS0352584). */
  caseNumber: string;
  projectId: string;
  projectName: string;
  /**
   * The work date (ISO, YYYY-MM-DD) despite the name — the backend returns
   * whatever date was submitted on create. Occasionally unparseable on real
   * records; `groupIntoSheets` skips such a card rather than losing its week.
   */
  createdOn: string;
  userId: string;
  userName: string;
  state: TimeCardState;
  /** Whether the logged time is billable to the customer. */
  billable: boolean;
  /** Whole minutes — the backend's own unit for this field. */
  totalMinutes: number;
  approvedById?: string;
  approvedByName?: string;
}

// Rolled-up status of a weekly sheet, derived from its cards. Purely a frontend
// display grouping — the backend has no "sheet" concept and no bulk endpoint.
export type TimeSheetState = "submitted" | "approved" | "rejected";

// A user's time cards for one ISO week (Mon–Sun), a display-only grouping.
export interface CsmTimeSheet {
  /** `${userId}:${weekStart}`. */
  id: string;
  userId: string;
  userName: string;
  /** Monday of the week (YYYY-MM-DD). */
  weekStart: string;
  /** Sunday of the week (YYYY-MM-DD). */
  weekEnd: string;
  state: TimeSheetState;
  cards: CsmTimeCard[];
  /** Whole minutes, summed from `cards`. */
  totalMinutes: number;
}

// Payload for an approver's accept/reject decision.
export interface TimeCardDecisionInput {
  cardId: string;
  state: Extract<TimeCardState, "approved" | "rejected">;
  leadComment?: string;
}

// Map the backend's `TimeCardView` to the portal's `CsmTimeCard`. `totalTime`
// is already whole minutes on the wire, which is also the unit shown — a direct
// passthrough, no conversion.
export function toTimeCard(v: BeTimeCardView): CsmTimeCard {
  return {
    id: v.id,
    caseId: v.case?.id ?? "",
    caseNumber: v.case?.number || v.case?.name || "—",
    projectId: v.project?.id ?? "",
    projectName: v.project?.name ?? "—",
    createdOn: v.createdOn,
    userId: v.user?.id ?? "",
    userName: v.user?.name ?? "—",
    state: v.state,
    billable: v.hasBillable,
    totalMinutes: v.totalTime,
    approvedById: v.approvedBy?.id,
    approvedByName: v.approvedBy?.name,
  };
}
