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

import type { CaseState, Severity } from "@features/csm-dashboard/types/abtDashboard";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import type { BeCaseWorkState, BeEngagementType } from "@api/backend/types";
import { ALL_ONBOARDING_STATUSES, ONBOARDING_STATUS_LABEL } from "./onboardingStatus";

/**
 * Fixed-enum option catalogues shared between `CasesFilterBar.tsx`'s Simple
 * grid controls and `advancedFilters.ts`'s Advanced-mode field catalogue —
 * one source of truth for each enum, since the same field now renders in
 * both modes (`filtersToAdvancedRows` reuses these `options` arrays so a
 * value picked in one mode is recognizable in the other). Lives in its own
 * (backend-client-free, component-free) module so neither `CasesFilterBar.tsx`
 * nor `advancedFilters.ts` has to import the other for these constants — both
 * already have their own real, `import type`-only circular reference to each
 * other (`CasesFilters`), and adding a *value*-level circular import here
 * (not type-only) would risk a real module-init-order hazard, unlike a
 * type-only one which is erased at compile time.
 */

export const ALL_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
export const SEVERITY_OPTIONS: { value: Severity; label: string }[] = ALL_SEVERITIES.map(
  (s) => ({ value: s, label: s }),
);

/** Excludes `reopened` — a valid backend enum value, but never a case's own
 * `state` (only ever a `nextStates` transition marker, see `CaseState`'s own
 * doc comment) — so offering it here would suggest a filter that can never
 * match anything. Mirrors `anyOfFilters.ts`'s own `ANY_OF_STATE_VALUES`. */
export const PRIMARY_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "awaiting_info",
  "solution_proposed",
  "waiting_on_wso2",
  "closed",
];
export const STATE_OPTIONS: { value: CaseState; label: string }[] = PRIMARY_STATES.map((s) => ({
  value: s,
  label: STATE_LABEL[s],
}));

export const ALL_WORK_STATES: BeCaseWorkState[] = ["ongoing", "paused"];
export const WORK_STATE_LABEL: Record<BeCaseWorkState, string> = {
  ongoing: "Ongoing",
  paused: "Paused",
};
export const WORK_STATE_OPTIONS: { value: BeCaseWorkState; label: string }[] =
  ALL_WORK_STATES.map((w) => ({ value: w, label: WORK_STATE_LABEL[w] }));

export const ALL_ENGAGEMENT_TYPES: BeEngagementType[] = [
  "migration",
  "consultancy",
  "new_feature_improvement",
  "follow_up",
  "onboarding",
];
export const ENGAGEMENT_TYPE_LABEL: Record<BeEngagementType, string> = {
  migration: "Migration",
  consultancy: "Consultancy",
  new_feature_improvement: "New feature / improvement",
  follow_up: "Follow-up",
  onboarding: "Onboarding",
};
export const ENGAGEMENT_TYPE_OPTIONS: { value: BeEngagementType; label: string }[] =
  ALL_ENGAGEMENT_TYPES.map((t) => ({ value: t, label: ENGAGEMENT_TYPE_LABEL[t] }));

export const ONBOARDING_STATUS_OPTIONS: { value: string; label: string }[] =
  ALL_ONBOARDING_STATUSES.map((value) => ({
    value,
    label: ONBOARDING_STATUS_LABEL[value],
  }));
