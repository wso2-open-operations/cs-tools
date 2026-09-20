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

import { PROBLEM_STATES, type ProblemFilters } from "@features/csm-operations/utils/problems";

// URL params owned by the problem filter state. Prefixed (`prob...`) so they
// can't collide with the same-named params the shared cases view and the
// change-requests/incidents tabs keep in the same `?tab=`-switched URL.
export const PROBLEM_FILTER_PARAM_KEYS = ["probQ", "probStates", "probSreTeams"] as const;

/**
 * Parse a comma-separated URL param into a list restricted to `allowed`. An
 * unrecognised entry is dropped rather than passed through, so a hand-edited or
 * stale query string can never send the backend a value outside the enum. The
 * type predicate is what narrows the result to `T[]` for the caller. Mirrors
 * `changeRequestsFiltersUrl.ts`/`incidentsFiltersUrl.ts`'s own `parseCsv`.
 */
function parseCsv<T extends string>(raw: string | null, allowed: T[]): T[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is T => (allowed as string[]).includes(s));
}

/**
 * Comma-separated SRE team ids (`sreGroupId` UUIDs) — not a fixed enum,
 * blank entries dropped. Mirrors `parseTeamIdsCsv` in
 * `incidentsFiltersUrl.ts`/`changeRequestsFiltersUrl.ts`.
 */
function parseTeamIdsCsv(raw: string | null): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * Read problem filters from the URL. Unknown/malformed values (a
 * hand-edited or stale query string) are dropped rather than passed through,
 * so they fall back to the default (unfiltered) behaviour instead of being
 * silently sent to the backend.
 */
export function readProblemFiltersFromUrl(params: URLSearchParams): ProblemFilters {
  return {
    search: params.get("probQ") ?? "",
    states: parseCsv(params.get("probStates"), PROBLEM_STATES),
    sreTeamIds: parseTeamIdsCsv(params.get("probSreTeams")),
  };
}

/**
 * Build the search-params representing these filters. Default values are
 * omitted so the URL stays clean.
 */
export function writeProblemFiltersToUrl(f: ProblemFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("probQ", f.search);
  if (f.states.length) out.set("probStates", f.states.join(","));
  if (f.sreTeamIds.length) out.set("probSreTeams", f.sreTeamIds.join(","));
  return out;
}
