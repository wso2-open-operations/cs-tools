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

import type { BeProblemState } from "@api/backend/types";

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
 * Filters for the problems list. Deliberately just search + state — see the
 * caveat on `BeProblemSearchFilters.states` in `api/backend/types.ts` (not yet
 * confirmed live against the backend).
 */
export interface ProblemFilters {
  search: string;
  states: BeProblemState[];
}

export const DEFAULT_PROBLEM_FILTERS: ProblemFilters = {
  search: "",
  states: [],
};

/** Count active (non-search) filters, used for the badge on the Filters button. */
export function countActiveProblemFilters(filters: ProblemFilters): number {
  return filters.states.length > 0 ? 1 : 0;
}
