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

import type { CallRequestStateKey } from "@src/types";

/** Human-readable label for each call request state key. */
export const CALL_REQUEST_STATE_LABEL: Record<CallRequestStateKey, string> = {
  pending_on_customer: "Pending on customer",
  pending_on_wso2: "Pending on WSO2",
  scheduled: "Scheduled",
  customer_rejected: "Rejected by customer",
  wso2_rejected: "Rejected by WSO2",
  canceled: "Canceled",
  notes_pending: "Notes pending",
  concluded: "Concluded",
};

/** Chip `color` for each state — terminal states are muted, active ones use the action palette. */
export const CALL_REQUEST_STATE_COLOR: Record<
  CallRequestStateKey,
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

// Integer choice keys the backing data source may return for `state.id`,
// mapped to our enum keys (mirrors the webapp's callRequestState.ts).
const NUMERIC_STATE_KEY: Record<string, CallRequestStateKey> = {
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
 * choice key. Returns null otherwise — callers must handle that case rather
 * than indexing a lookup table with the raw id.
 */
export function resolveCallRequestStateKey(
  state: { id: string | number; label?: string } | null | undefined,
): CallRequestStateKey | null {
  if (!state) return null;
  const raw = String(state.id);
  if (raw in CALL_REQUEST_STATE_LABEL) return raw as CallRequestStateKey;
  if (raw in NUMERIC_STATE_KEY) return NUMERIC_STATE_KEY[raw];
  return null;
}

/**
 * Agent-facing actions available on a call request, keyed by its current
 * state — mirrors the webapp's CALL_REQUEST_AGENT_ACTIONS exactly (same
 * backend, same contract). Do not add an action here unless there is a
 * corresponding backend transition, otherwise the UI offers one that fails.
 *
 * - `pending_on_wso2`: agent can schedule the call or reject it.
 * - `scheduled`: agent can reschedule or cancel. Sending notes from here is
 *   gated by the backend's post-due automation moving the request to
 *   `notes_pending` first, so it isn't offered directly from `scheduled`.
 * - `notes_pending`: agent can send call notes (concludes the request).
 * - `pending_on_customer`: the customer owns scheduling/rejecting; the agent
 *   can only cancel on their behalf.
 * - Terminal states (`customer_rejected`, `wso2_rejected`, `canceled`,
 *   `concluded`): no actions.
 */
export type CallRequestAgentAction = "schedule" | "reschedule" | "reject" | "sendNotes" | "cancel";

export const CALL_REQUEST_AGENT_ACTIONS: Record<CallRequestStateKey, CallRequestAgentAction[]> = {
  pending_on_customer: ["cancel"],
  pending_on_wso2: ["schedule", "reject"],
  scheduled: ["reschedule", "cancel"],
  customer_rejected: [],
  wso2_rejected: [],
  canceled: [],
  notes_pending: ["sendNotes"],
  concluded: [],
};

export const CALL_REQUEST_ACTION_LABEL: Record<CallRequestAgentAction, string> = {
  schedule: "Schedule",
  reschedule: "Reschedule",
  reject: "Reject",
  sendNotes: "Send call notes",
  cancel: "Cancel",
};
