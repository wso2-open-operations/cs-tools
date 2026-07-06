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
  CaseState,
  CaseWorkState,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Whether a **customer-visible comment** may be posted right now. The backend
 * gates public comments on `work_in_progress` AND work sub-state `ongoing`.
 * Internal **work notes are NOT gated** by work-state — they may be added in
 * any state — so this must only be consulted for public replies, never work
 * notes. Keep in lockstep with the entity-service / BFF comment guard.
 */
export function caseAcceptsPublicComments(
  state: CaseState | undefined,
  workState: CaseWorkState | null | undefined,
): boolean {
  return state === "work_in_progress" && workState === "ongoing";
}

/**
 * Human-readable reason a **public comment** cannot be posted right now, or
 * `null` when it can. When set, the composer locks to work-note mode and shows
 * this as the hint so the engineer understands why a customer reply is
 * unavailable. Intentionally does not promise that a work note will save — that
 * depends on the backend exempting work notes from the in-progress guard
 * (pending follow-up); copy stays non-committal until then. Does not gate work
 * notes.
 */
export function publicCommentGateReason(
  state: CaseState | undefined,
  workState: CaseWorkState | null | undefined,
): string | null {
  if (caseAcceptsPublicComments(state, workState)) return null;
  if (state === "work_in_progress" && workState === "paused") {
    return "This case is paused — customer replies are disabled. Resume work to reply to the customer.";
  }
  return "Customer replies are disabled unless the case is actively in progress.";
}

/** Short label for the work sub-state chip on the case header / list. */
export const WORK_STATE_LABEL: Record<CaseWorkState, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};
