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
  Severity,
} from "@features/csm-dashboard/types/abtDashboard";
import { parseBackendTimestamp } from "@utils/dateTime";

export const SEVERITY_COLOR: Record<
  Severity,
  "error" | "warning" | "info" | "success" | "default"
> = {
  S0: "error",
  S1: "error",
  S2: "warning",
  S3: "info",
  S4: "default",
};

export const SEVERITY_LABEL: Record<Severity, string> = {
  S0: "Catastrophic",
  S1: "Critical",
  S2: "High",
  S3: "Medium",
  S4: "Low / Query",
};

export const STATE_LABEL: Record<CaseState, string> = {
  open: "Open",
  work_in_progress: "Work in progress",
  solution_proposed: "Solution proposed",
  awaiting_info: "Awaiting info",
  waiting_on_wso2: "Waiting on WSO2",
  closed: "Closed",
  reopened: "Reopened",
};

// Status chip colour by "whose move is it", so colour carries information when
// an engineer scans a queue — instead of every open case rendering the same
// brand orange (the old `isClosed ? "success" : "primary"`, which also failed
// WCAG contrast). The four buckets:
//   info (blue)    = active, on us, normal      -> open, work_in_progress
//   warning (amber)= active, on us, elevated    -> waiting_on_wso2
//   default (grey) = waiting on the customer     -> solution_proposed, awaiting_info
//   success (green)= done                        -> closed
// All four roles have a dark `contrastText` in this theme, so filled chips pass
// AA. `primary` (orange) is deliberately not used here: it is reserved for the
// one real action button, selected nav, and links.
export const STATE_COLOR: Record<
  CaseState,
  "info" | "warning" | "success" | "default"
> = {
  open: "info",
  work_in_progress: "info",
  waiting_on_wso2: "warning",
  solution_proposed: "default",
  awaiting_info: "default",
  closed: "success",
  // Defensive: `reopened` only appears in a closed case's `nextStates` (the
  // "Create related case" signal, see CaseState's doc) — never as a case's
  // own state, so this entry should be unreachable via a real case's state.
  // Kept in the "info" bucket rather than omitted, so the exhaustive Record
  // still compiles and any incidental rendering doesn't look broken.
  reopened: "info",
};

/**
 * Title-case an unknown backend state key for display, e.g.
 * `pending_review` -> "Pending review". This is the fallback that lets a state
 * the frontend has not been taught about still render with a readable label,
 * so introducing a new case state on the backend needs no frontend change.
 */
export function humanizeState(state: string): string {
  if (!state) return "Unknown";
  const words = state.split("_").filter(Boolean);
  return words
    .map((w, i) => (i === 0 ? w.charAt(0).toUpperCase() + w.slice(1) : w))
    .join(" ");
}

/**
 * Display label for a case state. Uses the curated label for known states and
 * gracefully degrades to a humanized key for any state the frontend does not
 * recognize (backend/frontend rollout skew, or a newly added state).
 */
export function stateLabel(state: string): string {
  return STATE_LABEL[state as CaseState] ?? humanizeState(state);
}

/**
 * Status-chip colour for a case state, defaulting to the neutral `default`
 * (grey) for any unrecognized state so a new state renders without styling
 * having to be added on the frontend first.
 */
export function stateColor(
  state: string,
): "info" | "warning" | "success" | "default" {
  return STATE_COLOR[state as CaseState] ?? "default";
}

export function formatRelativeTime(
  iso: string | null | undefined,
  now: number = Date.now(),
): string {
  if (!iso) return "—";
  // Backend timestamps are UTC. parseBackendTimestamp treats unzoned ISO
  // strings (and the slash/space formats some SN APIs return) as UTC.
  const parsed = parseBackendTimestamp(iso);
  const epoch = parsed ? parsed.getTime() : new Date(iso).getTime();
  if (Number.isNaN(epoch)) return "—";
  const diffMs = now - epoch;
  const futureSuffix = diffMs < 0 ? " from now" : " ago";
  const abs = Math.abs(diffMs);
  // Floor each unit so we don't round up across a boundary (e.g. show "1h ago"
  // at 30 minutes, or "1d ago" at 12 hours).
  const diffMin = Math.floor(abs / 60_000);
  if (diffMin < 1) return "just now";
  if (diffMin < 60) return `${diffMin}m${futureSuffix}`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h${futureSuffix}`;
  const diffDay = Math.floor(diffHr / 24);
  return `${diffDay}d${futureSuffix}`;
}
