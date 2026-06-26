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
import type { BeCaseType, BeEngagementType } from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import { ALL_CASE_TYPES } from "@features/csm-cases/utils/caseType";

export const DEFAULT_CASES_FILTERS: CasesFilters = {
  search: "",
  severities: [],
  states: [],
  caseTypes: [],
  assignees: [],
  projects: [],
  engagementTypes: [],
};

const VALID_SEVERITIES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const VALID_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "solution_proposed",
  "awaiting_info",
  "waiting_on_wso2",
  "closed",
];
const VALID_CASE_TYPES: BeCaseType[] = ALL_CASE_TYPES;
const VALID_ENGAGEMENT_TYPES: BeEngagementType[] = [
  "migration",
  "consultancy",
  "new_feature_improvement",
  "follow_up",
  "onboarding",
];

function parseCsv<T extends string>(raw: string | null, allowed: T[]): T[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is T => (allowed as string[]).includes(s));
}

/**
 * Parse a CSV of free-form strings (used for assignee / project values that
 * aren't part of a fixed enum). Empties stripped, length-capped per entry to
 * avoid pathological URL growth.
 */
function parseFreeFormCsv(raw: string | null, maxEntryLen = 120): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0 && s.length <= maxEntryLen);
}

export function readCasesFiltersFromUrl(
  params: URLSearchParams,
): CasesFilters {
  return {
    search: params.get("q") ?? "",
    severities: parseCsv(params.get("severities"), VALID_SEVERITIES),
    states: parseCsv(params.get("states"), VALID_STATES),
    caseTypes: parseCsv(params.get("types"), VALID_CASE_TYPES),
    assignees: parseFreeFormCsv(params.get("assignees")),
    projects: parseFreeFormCsv(params.get("projects")),
    engagementTypes: parseCsv(params.get("engagementTypes"), VALID_ENGAGEMENT_TYPES),
  };
}

/**
 * Build the search-params object representing these filters. Default values
 * are omitted so the URL stays clean.
 */
export function writeCasesFiltersToUrl(f: CasesFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("q", f.search);
  if (f.severities.length) out.set("severities", f.severities.join(","));
  if (f.states.length) out.set("states", f.states.join(","));
  if (f.caseTypes.length) out.set("types", f.caseTypes.join(","));
  if (f.assignees.length) out.set("assignees", f.assignees.join(","));
  if (f.projects.length) out.set("projects", f.projects.join(","));
  if (f.engagementTypes.length) out.set("engagementTypes", f.engagementTypes.join(","));
  return out;
}

/**
 * Count the filters that carry a non-default value (search counts as one).
 * Used for the filter-bar badge and by the cases page to tell whether the user
 * has expressed any intent yet — 0 means "show the search/filter prompt and
 * don't load anything".
 */
export function countActiveFilters(f: CasesFilters): number {
  let n = 0;
  if (f.search.trim()) n += 1;
  if (f.severities.length) n += 1;
  if (f.states.length) n += 1;
  if (f.caseTypes.length) n += 1;
  if (f.assignees.length) n += 1;
  if (f.projects.length) n += 1;
  if (f.engagementTypes.length) n += 1;
  return n;
}

/**
 * Convenience: build a `/cases?...` href from a partial filter override.
 * Anything not specified falls back to the defaults.
 */
export function casesHref(overrides: Partial<CasesFilters>): string {
  const full: CasesFilters = { ...DEFAULT_CASES_FILTERS, ...overrides };
  const qs = writeCasesFiltersToUrl(full).toString();
  return qs ? `/cases?${qs}` : "/cases";
}
