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

import type { CaseState, CaseWorkState } from "@src/types";

// Both rules below are transcribed directly from the portal backend's actual guard
// (backend/internal/handler/cases.go, CreateCaseComment) — not the openapi.yaml doc (which
// doesn't mention either rule) or the webapp's own comments (one of which turned out to be
// stale/hedged). Two independent, non-public state gates:
//   - `type !== "work_note"` (i.e. a public comment): requires state === work_in_progress AND
//     workState === ongoing, else 409 "Comments can only be added when the case is in progress
//     and the work state is ongoing."
//   - `type === "work_note"`: exempt from that gate entirely — allowed in any state except
//     state === closed, where it 409s separately ("work note on closed case").

/**
 * Whether a **customer-visible comment** may be posted right now.
 */
export function caseAcceptsPublicComments(state: CaseState | undefined, workState: CaseWorkState): boolean {
  return state === "work_in_progress" && workState === "ongoing";
}

/** Whether an internal **work note** may be posted right now — blocked only once closed. */
export function caseAcceptsWorkNotes(state: CaseState | undefined): boolean {
  return state !== "closed";
}

/**
 * Human-readable reason a **public comment** can't be posted right now, or `null` when it can.
 * When set, the composer should lock to work-note mode (unless caseAcceptsWorkNotes is also
 * false) and show this hint.
 */
export function publicCommentGateReason(state: CaseState | undefined, workState: CaseWorkState): string | null {
  if (caseAcceptsPublicComments(state, workState)) return null;
  if (state === "work_in_progress" && workState === "paused") {
    return "This case is paused — customer replies are disabled. Resume work to reply to the customer.";
  }
  return "Customer replies are disabled unless the case is actively in progress.";
}

/** Human-readable reason a **work note** can't be posted right now, or `null` when it can. */
export function workNoteGateReason(state: CaseState | undefined): string | null {
  if (caseAcceptsWorkNotes(state)) return null;
  return "This case is closed — it's read-only.";
}
