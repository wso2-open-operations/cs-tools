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

import type { BeCallRequestStateKey } from "@api/backend/types";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";

/** Human-readable label for each call request state key. */
export const CALL_REQUEST_STATE_LABEL: Record<BeCallRequestStateKey, string> = {
  pending_on_customer: "Pending on customer",
  pending_on_wso2: "Pending on WSO2",
  scheduled: "Scheduled",
  customer_rejected: "Rejected by customer",
  wso2_rejected: "Rejected by WSO2",
  canceled: "Canceled",
  notes_pending: "Notes pending",
  concluded: "Concluded",
};

/**
 * MUI Chip `color` for each state. Maps to the Oxygen UI palette.
 * Terminal states (rejected, canceled, concluded) use muted colours;
 * active/pending states use the action palette.
 */
export const CALL_REQUEST_STATE_COLOR: Record<
  BeCallRequestStateKey,
  "default" | "primary" | "secondary" | "error" | "info" | "success" | "warning"
> = {
  pending_on_customer: "warning",
  pending_on_wso2: "info",
  scheduled: "primary",
  customer_rejected: "error",
  wso2_rejected: "error",
  canceled: "default",
  notes_pending: "warning",
  concluded: "success",
};

/**
 * Integer choice keys the backing data source may return for `state.id`,
 * mapped to our enum keys. The data source passes its native state through
 * untranslated, so `state.id` can be an integer (e.g. 3) rather than a string
 * key (e.g. "scheduled").
 */
const NUMERIC_STATE_KEY: Record<string, BeCallRequestStateKey> = {
  "1": "pending_on_customer",
  "2": "pending_on_wso2",
  "3": "scheduled",
  "4": "customer_rejected",
  "5": "wso2_rejected",
  "6": "canceled",
  "7": "notes_pending",
  "8": "concluded",
};

/**
 * Resolve a backend call-request state to one of our enum keys from `state.id`,
 * which arrives either as our string enum key or as the data source's integer
 * choice key. Returns null otherwise, so callers must handle the null case
 * (never index a lookup with the raw id).
 *
 * `state.label` is intentionally NOT used for key resolution: the FE label table
 * (`CALL_REQUEST_STATE_LABEL`) is worded independently of the data source's
 * labels (e.g. our `customer_rejected` -> "Rejected by customer" vs the source's
 * "Customer Rejected"), so a reverse label lookup would resolve only some states
 * and silently miss others. The label is display-only (see `callRequestStateLabel`).
 * In practice the backend always sends a usable `state.id`.
 */
export function resolveCallRequestStateKey(
  state: { id: number | string; label?: string } | undefined,
): BeCallRequestStateKey | null {
  if (!state) return null;
  const raw = String(state.id);
  if (raw in CALL_REQUEST_STATE_LABEL) return raw as BeCallRequestStateKey;
  if (raw in NUMERIC_STATE_KEY) return NUMERIC_STATE_KEY[raw];
  return null;
}

/**
 * Resolve the display label for a call request state returned by the backend.
 * Prefers the backend-supplied `label`, else maps the id (string or integer).
 */
export function callRequestStateLabel(state: {
  id: number | string;
  label?: string;
} | undefined): string {
  if (!state) return "Unknown";
  if (state.label) return state.label;
  const key = resolveCallRequestStateKey(state);
  return key ? CALL_REQUEST_STATE_LABEL[key] : String(state.id);
}

/**
 * Resolve the MUI chip color for a call request state returned by the backend.
 * Same resolution logic as `callRequestStateLabel`.
 */
export function callRequestStateColor(
  state: { id: number | string; label?: string } | undefined,
): "default" | "primary" | "secondary" | "error" | "info" | "success" | "warning" {
  const key = resolveCallRequestStateKey(state);
  return key ? CALL_REQUEST_STATE_COLOR[key] : "default";
}

/**
 * The states a call request can still move on from: a case's call list shows only
 * these by default, so a call that has been completed, canceled or rejected stops
 * cluttering it (it stays one state-filter choice away).
 */
export const OPEN_CALL_REQUEST_STATES: BeCallRequestStateKey[] = [
  "pending_on_customer",
  "pending_on_wso2",
  "scheduled",
  "notes_pending",
];

/** All 8 state keys, used to drive filter dropdowns. */
export const ALL_CALL_REQUEST_STATES: BeCallRequestStateKey[] = [
  "pending_on_customer",
  "pending_on_wso2",
  "scheduled",
  "customer_rejected",
  "wso2_rejected",
  "canceled",
  "notes_pending",
  "concluded",
];

/**
 * Agent-facing actions available on a call request, keyed by its current
 * state. This mirrors exactly what the backend can fulfil today (see the
 * cross-layer contract) -- do not add an action here unless there is a
 * corresponding backend endpoint, otherwise the UI offers a transition that
 * fails on submit.
 *
 * - `pending_on_wso2`: agent can schedule the call or reject it.
 * - `scheduled`: agent can reschedule, mark it completed, or cancel. Sending
 *   notes from this state is normally gated by the backend's post-due
 *   automation moving the request to `notes_pending` first; it is not offered
 *   here -- "Mark as completed" is the way to finish a call without waiting for
 *   (or writing) notes.
 * - `notes_pending`: agent can send call notes (concludes the request) or just
 *   mark it completed with no notes.
 * - `pending_on_customer`: the customer owns scheduling/rejecting; the agent
 *   can only cancel on their behalf.
 * - Terminal states (`customer_rejected`, `wso2_rejected`, `canceled`,
 *   `concluded`): no actions.
 *
 * `complete` ("Mark as completed") is a one-click
 * `state: concluded` with no notes -- the backend only accepts that from
 * `scheduled` / `notes_pending`, which is exactly where it is offered.
 */
export type CallRequestAgentAction =
  | "schedule"
  | "reschedule"
  | "reject"
  | "sendNotes"
  | "complete"
  | "cancel";

export const CALL_REQUEST_AGENT_ACTIONS: Record<
  BeCallRequestStateKey,
  CallRequestAgentAction[]
> = {
  pending_on_customer: ["cancel"],
  pending_on_wso2: ["schedule", "reject"],
  scheduled: ["reschedule", "complete", "cancel"],
  customer_rejected: [],
  wso2_rejected: [],
  canceled: [],
  notes_pending: ["sendNotes", "complete"],
  concluded: [],
};

// ---------------------------------------------------------------------------
// Case-state gate for creating a call request
// ---------------------------------------------------------------------------

/**
 * Case states in which a call request may be created (or rescheduled). This
 * mirrors the data source's own gate (`CallRequestUtils.scheduleCall`), which
 * rejects the schedule/reschedule attempt with a generic error unless the
 * case is in one of these 5 states. The FE check exists purely to surface a
 * clear reason before submit -- the backend remains the source of truth and
 * is still enforced there; if the two ever drift, the backend wins.
 */
export const CALL_REQUEST_ALLOWED_CASE_STATES: readonly CaseState[] = [
  "work_in_progress",
  "awaiting_info",
  "waiting_on_wso2",
  "solution_proposed",
  "reopened",
];

/**
 * Whether a call request may be created for a case in `state`. An `undefined`
 * state (case not loaded / not passed by the caller) is treated as
 * permissive -- we only block once we positively know the state is
 * disallowed, and the backend still enforces the rule either way.
 */
export function caseAcceptsCallRequests(state: CaseState | undefined): boolean {
  if (!state) return true;
  return CALL_REQUEST_ALLOWED_CASE_STATES.includes(state);
}

/**
 * Human-readable reason a call request can't be created for the case's
 * current state, or `null` when it can (including when `state` is unknown).
 */
export function callRequestCaseStateBlockReason(
  state: CaseState | undefined,
): string | null {
  if (caseAcceptsCallRequests(state)) return null;
  const labels = CALL_REQUEST_ALLOWED_CASE_STATES.map(stateLabel);
  const last = labels[labels.length - 1];
  const rest = labels.slice(0, -1).join(", ");
  return `Calls can only be requested while the case is ${rest}, or ${last}.`;
}
