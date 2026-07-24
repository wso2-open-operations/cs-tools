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

import type { BeIncidentPriority, BeIncidentState } from "@api/backend/types";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

const STATE_LABEL: Record<BeIncidentState, string> = {
  NEW: "New",
  IN_PROGRESS: "In Progress",
  ON_HOLD: "On Hold",
  RESOLVED: "Resolved",
  CLOSED: "Closed",
  CANCELLED: "Cancelled",
};

// New is unstarted (default), in-progress/on-hold are in-flight (info/warning),
// resolved/closed are terminal-good, cancelled is a terminal-problem state.
const STATE_COLOR: Record<BeIncidentState, ChipColor> = {
  NEW: "default",
  IN_PROGRESS: "info",
  ON_HOLD: "warning",
  RESOLVED: "success",
  CLOSED: "success",
  CANCELLED: "error",
};

const PRIORITY_LABEL: Record<BeIncidentPriority, string> = {
  CRITICAL: "Critical",
  HIGH: "High",
  MODERATE: "Moderate",
  LOW: "Low",
  PLANNING: "Planning",
};

const PRIORITY_COLOR: Record<BeIncidentPriority, ChipColor> = {
  CRITICAL: "error",
  HIGH: "error",
  MODERATE: "warning",
  LOW: "default",
  PLANNING: "default",
};

/** All incident priorities, for a filter control. */
export const INCIDENT_PRIORITIES = Object.keys(PRIORITY_LABEL) as BeIncidentPriority[];

/** All incident states, for the Edit dialog's State select. */
export const INCIDENT_STATES = Object.keys(STATE_LABEL) as BeIncidentState[];

/**
 * Legal forward transitions per state, modeled on the standard ITSM incident
 * lifecycle. This is a net-new CSM-platform guardrail, not a port of SN
 * behavior — ServiceNow enforces no old-state -> new-state legality check
 * for incidents in this org (only role-gating), so there's no SN precedent
 * to mirror here. `CLOSED` and `CANCELLED` are terminal: no outgoing edges.
 */
const STATE_TRANSITIONS: Record<BeIncidentState, BeIncidentState[]> = {
  NEW: ["IN_PROGRESS", "CANCELLED"],
  IN_PROGRESS: ["ON_HOLD", "RESOLVED", "CANCELLED"],
  ON_HOLD: ["IN_PROGRESS", "CANCELLED"],
  RESOLVED: ["CLOSED", "IN_PROGRESS"],
  CLOSED: [],
  CANCELLED: [],
};

/**
 * States the Edit dialog's State select should offer for an incident
 * currently in `current`: `current` itself (so "no change" always stays
 * selectable) plus its legal next states. Returns just `[current]` for the
 * two terminal states, which the dialog uses to render the select as
 * effectively non-actionable for state changes.
 */
export function getLegalNextIncidentStates(current: BeIncidentState): BeIncidentState[] {
  return [current, ...STATE_TRANSITIONS[current]];
}

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function incidentStateLabel(state?: string | null): string {
  if (!state) return "—";
  return STATE_LABEL[state as BeIncidentState] ?? humanize(state);
}

export function incidentStateColor(state?: string | null): ChipColor {
  return STATE_COLOR[state as BeIncidentState] ?? "default";
}

/**
 * Human-readable reason a comment cannot be posted on this incident right now,
 * or `null` when it can. Unlike the case comment gate, incidents don't share
 * the work-in-progress/ongoing model — the only gate is terminal state.
 */
export function incidentCommentGateReason(
  state?: string | null,
): string | null {
  if (state === "CLOSED" || state === "CANCELLED") {
    return "Comments are disabled on a closed or cancelled incident.";
  }
  return null;
}

export function incidentPriorityLabel(priority?: string | null): string {
  if (!priority) return "—";
  return PRIORITY_LABEL[priority as BeIncidentPriority] ?? humanize(priority);
}

export function incidentPriorityColor(priority?: string | null): ChipColor {
  return PRIORITY_COLOR[priority as BeIncidentPriority] ?? "default";
}

/**
 * Filters for the incidents list. Deliberately just search + priority — the
 * backend's `IncidentSearchPayload.filters` only supports `searchQuery`,
 * `priorities`, and `parentIds` (see openapi.yaml); there's no server-side
 * state/category filter to build a control for.
 */
export interface IncidentFilters {
  search: string;
  priorities: BeIncidentPriority[];
}

export const DEFAULT_INCIDENT_FILTERS: IncidentFilters = {
  search: "",
  priorities: [],
};

/** Count non-search active filters (used for the badge on the Filters button). */
export function countActiveIncidentFilters(filters: IncidentFilters): number {
  return filters.priorities.length > 0 ? 1 : 0;
}
