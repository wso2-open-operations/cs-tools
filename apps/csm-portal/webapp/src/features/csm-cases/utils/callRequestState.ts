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
 * - `scheduled`: agent can reschedule or cancel. Sending notes from this
 *   state is normally gated by the backend's post-due automation moving the
 *   request to `notes_pending` first; it is not offered here.
 * - `notes_pending`: agent can send call notes (concludes the request).
 * - `pending_on_customer`: the customer owns scheduling/rejecting; the agent
 *   can only cancel on their behalf.
 * - Terminal states (`customer_rejected`, `wso2_rejected`, `canceled`,
 *   `concluded`): no actions.
 */
export type CallRequestAgentAction = "schedule" | "reschedule" | "reject" | "sendNotes" | "cancel";

export const CALL_REQUEST_AGENT_ACTIONS: Record<
  BeCallRequestStateKey,
  CallRequestAgentAction[]
> = {
  pending_on_customer: ["cancel"],
  pending_on_wso2: ["schedule", "reject"],
  scheduled: ["reschedule", "cancel"],
  customer_rejected: [],
  wso2_rejected: [],
  canceled: [],
  notes_pending: ["sendNotes"],
  concluded: [],
};
