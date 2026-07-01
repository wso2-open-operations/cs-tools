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

import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeCardState,
  TimeSheetState,
} from "@features/csm-timecards/types/timeCards";

/** Actions that can be taken on a card or sheet. */
export type TimecardAction =
  | "edit"
  | "delete"
  | "submit"
  | "resubmit"
  | "approve"
  | "reject"
  | "recall"
  | "process";

/** The capabilities of the current user relative to the target. */
export interface TimecardRoleCtx {
  /** The signed-in user owns the card/sheet. */
  isOwner: boolean;
  /** The signed-in user may approve (team-lead/approver, or admin). */
  isApprover: boolean;
  /** The signed-in user is a time-card admin. */
  isAdmin: boolean;
}

/** States in which the owner (or admin) may edit a card. */
export const EDITABLE_STATES: TimeCardState[] = ["pending", "rejected", "recalled"];

/**
 * Allowed actions on a single card given its state and the user's role. Single
 * source of truth for which buttons render (mirrors the cases `nextStates`
 * pattern). The owner edits/submits; an approver approves/rejects/recalls; an
 * admin can do both plus process.
 */
export function cardActions(
  state: TimeCardState,
  role: TimecardRoleCtx,
): TimecardAction[] {
  const canEdit = role.isOwner || role.isAdmin;
  const canApprove = role.isApprover || role.isAdmin;
  switch (state) {
    case "pending":
      return canEdit ? ["edit", "delete", "submit"] : [];
    case "rejected":
    case "recalled":
      return canEdit ? ["edit", "delete", "resubmit"] : [];
    case "submitted":
      return canApprove ? ["approve", "reject"] : [];
    case "approved":
      return [
        ...(canApprove ? (["recall"] as TimecardAction[]) : []),
        ...(role.isAdmin ? (["process"] as TimecardAction[]) : []),
      ];
    case "processed":
    default:
      return [];
  }
}

/** Roll a sheet's cards up into a single status for display. */
export function sheetStatus(cards: CsmTimeCard[]): TimeSheetState {
  if (cards.some((c) => c.state === "rejected")) return "rejected";
  if (cards.some((c) => c.state === "recalled")) return "recalled";
  if (cards.some((c) => c.state === "submitted")) return "submitted";
  if (
    cards.length > 0 &&
    cards.every((c) => c.state === "approved" || c.state === "processed")
  ) {
    return "approved";
  }
  return "open";
}

/** Sheet-level actions (a subset of {@link TimecardAction}). */
export type SheetAction = "submit" | "approve" | "reject" | "recall";

/** Sheet-level actions available to the user. */
export function sheetActions(
  sheet: CsmTimeSheet,
  role: TimecardRoleCtx,
): SheetAction[] {
  const actions: SheetAction[] = [];
  const canEdit = role.isOwner || role.isAdmin;
  const canApprove = role.isApprover || role.isAdmin;
  if (canEdit && sheet.cards.some((c) => EDITABLE_STATES.includes(c.state))) {
    actions.push("submit");
  }
  if (canApprove && sheet.cards.some((c) => c.state === "submitted")) {
    actions.push("approve"); // "approve remaining"
    actions.push("reject"); // "reject remaining"
  }
  if (canApprove && sheet.cards.some((c) => c.state === "approved")) {
    actions.push("recall");
  }
  return actions;
}
