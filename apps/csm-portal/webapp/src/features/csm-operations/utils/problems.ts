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

import type { BeProblemSearchFilters, BeProblemState } from "@api/backend/types";

type ChipColor = "default" | "info" | "warning" | "success" | "error";

const STATE_LABEL: Record<BeProblemState, string> = {
  NEW: "New",
  ASSESS: "Assess",
  ROOT_CAUSE_ANALYSIS: "Root Cause Analysis",
  FIX_IN_PROGRESS: "Fix In Progress",
  RESOLVED: "Resolved",
  CLOSED: "Closed",
};

// New/assess are unstarted-ish (default/info), the analysis/fix states are
// active work (info/warning), resolved/closed are terminal-good.
const STATE_COLOR: Record<BeProblemState, ChipColor> = {
  NEW: "default",
  ASSESS: "info",
  ROOT_CAUSE_ANALYSIS: "info",
  FIX_IN_PROGRESS: "warning",
  RESOLVED: "success",
  CLOSED: "success",
};

/** All problem states, for a filter control. */
export const PROBLEM_STATES = Object.keys(STATE_LABEL) as BeProblemState[];

function humanize(value: string): string {
  return value.replace(/_/g, " ");
}

export function problemStateLabel(state?: string | null): string {
  if (!state) return "—";
  return STATE_LABEL[state as BeProblemState] ?? humanize(state);
}

export function problemStateColor(state?: string | null): ChipColor {
  return STATE_COLOR[state as BeProblemState] ?? "default";
}

/**
 * The single forward transition legal from a given state, per the live
 * 6-state ServiceNow Problem Management engine (`CHANGES-problem-update.md`
 * §1/§3) — a strictly linear chain, unlike Incident/Change Request, which
 * both allow multiple legal next states from a given point. `null` once the
 * problem is `CLOSED` (terminal) or in a state this CSM-platform doesn't
 * expose a forward action for. `Mark Duplicate`/`Cancel`/`Re-Analyze` are
 * deliberately not modeled here — see the contract doc's "Not built" note.
 */
const PROBLEM_TRANSITIONS: Partial<
  Record<BeProblemState, { transition: string; target: BeProblemState }>
> = {
  NEW: { transition: "assess", target: "ASSESS" },
  ASSESS: { transition: "confirm", target: "ROOT_CAUSE_ANALYSIS" },
  ROOT_CAUSE_ANALYSIS: { transition: "fix", target: "FIX_IN_PROGRESS" },
  FIX_IN_PROGRESS: { transition: "resolve", target: "RESOLVED" },
  RESOLVED: { transition: "close", target: "CLOSED" },
};

export interface ProblemNextTransition {
  /** The `transition` key to send in the `PATCH /problems/{id}` body. */
  transition: string;
  /** The state this transition moves the problem into, for the button label. */
  target: BeProblemState;
}

/**
 * The one legal forward transition from `current`, or `null` if there isn't
 * one (terminal state, or an unrecognized value from a future backend
 * state this CSM-platform guardrail doesn't know about yet).
 */
export function getNextProblemTransition(
  current?: BeProblemState | null,
): ProblemNextTransition | null {
  if (!current) return null;
  return PROBLEM_TRANSITIONS[current] ?? null;
}

/**
 * Filters for the problems list. Deliberately just search + state + SRE
 * team — see the caveat on `BeProblemSearchFilters.states` in
 * `api/backend/types.ts` (not yet confirmed live against the backend).
 */
export interface ProblemFilters {
  search: string;
  states: BeProblemState[];
  /** Selected SRE team `sreGroupId`s — sent as a generic
   * `{ field: "assignmentGroupId", op: "in" }` filter entry (see
   * `buildProblemSearchFilters`), not a named payload field. */
  sreTeamIds: string[];
}

export const DEFAULT_PROBLEM_FILTERS: ProblemFilters = {
  search: "",
  states: [],
  sreTeamIds: [],
};

/** Count active (non-search) filters, used for the badge on the Filters button. */
export function countActiveProblemFilters(filters: ProblemFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) + (filters.sreTeamIds.length > 0 ? 1 : 0)
  );
}

/**
 * Build `ProblemSearchPayload.filters` from the UI's {@link ProblemFilters}
 * plus the (separately debounced) search text — mirrors
 * `buildIncidentSearchFilters` in `incidents.ts`.
 */
export function buildProblemSearchFilters(
  filters: ProblemFilters,
  debouncedSearch: string,
): BeProblemSearchFilters {
  return {
    ...(debouncedSearch.length > 0 && { searchQuery: debouncedSearch }),
    ...(filters.states.length > 0 && { states: filters.states }),
    ...(filters.sreTeamIds.length > 0 && {
      filters: [
        { field: "assignmentGroupId" as const, op: "in" as const, values: filters.sreTeamIds },
      ],
    }),
  };
}
