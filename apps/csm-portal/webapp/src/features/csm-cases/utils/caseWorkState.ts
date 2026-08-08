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
 *
 * `assigneeIsMe` is checked first and unconditionally: the backend now also
 * rejects a public comment from anyone other than the case's assigned
 * engineer, on top of the existing state/work-state gate. A non-assignee gets
 * the ownership reason regardless of what state/workState say — even a
 * perfectly "ongoing" case is still not theirs to reply on — rather than
 * layering ownership on top of the state check, because
 * `canResumeToUnlockPublicReply` already only offers its "Resume work"
 * quick-fix to the assignee. If a non-assignee's paused case fell through to
 * the state reason instead, the composer would show a resumable-sounding
 * hint for an action that isn't actually available to them. Ownership first
 * keeps the messaging and the quick-fix in agreement: a non-assignee never
 * sees a resumable-sounding reason, only the ownership block.
 */
export function publicCommentGateReason(
  state: CaseState | undefined,
  workState: CaseWorkState | null | undefined,
  assigneeIsMe: boolean,
): string | null {
  if (!assigneeIsMe) {
    return "Only the case's assigned engineer can reply to the customer.";
  }
  if (caseAcceptsPublicComments(state, workState)) return null;
  if (state === "work_in_progress" && workState === "paused") {
    return "This case is paused — public replies are disabled. Resume work to reply to the customer.";
  }
  return "Customer replies are disabled unless the case is actively in progress.";
}

/**
 * Whether resuming work — a single-field PATCH, no reassignment or state
 * change — would be enough to unlock a public reply right now. True only for
 * the one lock reason that's actually a single click away: the case is
 * already `work_in_progress` and assigned to the signed-in engineer, just not
 * `ongoing`. `assigneeIsMe` matters because pausing/resuming is only offered
 * to the case's own assignee elsewhere in the UI (see CaseActionBar's own
 * `assigneeIsMe && state === "work_in_progress"` gate on the same action) —
 * without this check, an engineer viewing someone else's paused case would
 * see a "Resume work" quick-fix that isn't actually theirs to use. The other
 * lock reason (case not started at all) needs the full assign/start flow
 * instead, so it's never resumable this way regardless of assignee.
 *
 * Deliberately `workState !== "ongoing"`, not `workState === "paused"`: a
 * null/undefined work state on a work_in_progress case is real (e.g. data
 * predating the work-state feature) and is resumable too, matching
 * CaseActionBar's own handling of the exact same case — see its "anything
 * else (paused OR a null work-state in-progress case) is resumable" comment
 * in `buildSecondaryItems`. Narrowing this to `=== "paused"` would hide the
 * quick-fix for a case where the action bar's own "Resume work" item is
 * still shown and functional.
 */
export function canResumeToUnlockPublicReply(
  state: CaseState | undefined,
  workState: CaseWorkState | null | undefined,
  assigneeIsMe: boolean,
): boolean {
  return (
    assigneeIsMe && state === "work_in_progress" && workState !== "ongoing"
  );
}

/**
 * The work sub-state a `work_in_progress` case should be *shown* as having,
 * treating a never-set `workState` (`null`/`undefined`) the same as
 * `paused` — the same "anything but ongoing behaves like paused" rule
 * `canResumeToUnlockPublicReply` already applies to behavior (resuming,
 * unlocking public replies). Without this, the work-state chip on the cases
 * list / case header / preview drawer silently renders nothing at all for
 * such a case, reading as "no status" rather than the paused state it
 * actually is. Only meaningful once the caller already knows `state` is
 * `work_in_progress` — callers still gate on that themselves, same as the
 * other functions here.
 */
export function effectiveWorkState(
  workState: CaseWorkState | null | undefined,
): CaseWorkState {
  return workState ?? "paused";
}

/** Short label for the work sub-state chip on the case header / list. */
export const WORK_STATE_LABEL: Record<CaseWorkState, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};
