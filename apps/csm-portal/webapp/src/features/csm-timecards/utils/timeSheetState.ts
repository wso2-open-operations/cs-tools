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

import type { TimeCardState } from "@features/csm-timecards/types/timeCards";

/**
 * Actions that can be taken on a card. The backend only supports a
 * state-transition PATCH (`approved` / `rejected`) — there's no submit
 * (cards are created already submitted), no edit (reads never return the
 * fields needed to safely round-trip an edit), no recall, and no process.
 * There's also no bulk/sheet-level endpoint, so there are no sheet actions.
 */
export type TimecardAction = "approve" | "reject";

/** The capabilities of the current user relative to a card. */
export interface TimecardRoleCtx {
  /** The signed-in user owns the card. */
  isOwner: boolean;
  /** The signed-in user may approve (team-lead/approver, or admin). */
  isApprover: boolean;
  /** The signed-in user is a time-card admin. */
  isAdmin: boolean;
}

/**
 * Allowed actions on a single card given its state and the user's role.
 * Single source of truth for which buttons render. Only an approver/admin
 * acting on a `submitted` card (not their own) has any action — the owner
 * has none, matching what the backend actually supports today.
 */
export function cardActions(
  state: TimeCardState,
  role: TimecardRoleCtx,
): TimecardAction[] {
  if (state !== "submitted" || role.isOwner) return [];
  return role.isApprover || role.isAdmin ? ["approve", "reject"] : [];
}
