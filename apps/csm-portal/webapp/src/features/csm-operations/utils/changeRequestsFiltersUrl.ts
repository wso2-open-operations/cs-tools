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

import {
  CHANGE_REQUEST_IMPACTS,
  CHANGE_REQUEST_STATES,
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";

// URL params owned by the change-request filter state. Prefixed (`cr...`) so
// they can't collide with the same-named params the shared cases view and the
// incidents tab keep in the same `?tab=`-switched URL (e.g. both this tab and
// the cases view would otherwise want a bare `q`/`states` key).
export const CR_FILTER_PARAM_KEYS = [
  "crQ",
  "crStates",
  "crImpacts",
  "crClosedFrom",
  "crClosedTo",
  "crSreTeams",
] as const;

const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

/**
 * Parse a comma-separated URL param into a list restricted to `allowed`. An
 * unrecognised entry is dropped rather than passed through, so a hand-edited or
 * stale query string can never send the backend a value outside the enum. The
 * type predicate is what narrows the result to `T[]` for the caller.
 */
function parseCsv<T extends string>(raw: string | null, allowed: T[]): T[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is T => (allowed as string[]).includes(s));
}

/**
 * Validate a `YYYY-MM-DD` value; anything else — wrong shape, or a shape that
 * parses to an out-of-range calendar date like `2026-13-99` — is dropped
 * rather than passed through to the backend.
 */
function parseDateOnly(raw: string | null): string {
  const match = raw ? DATE_ONLY_RE.exec(raw) : null;
  if (!match) return "";
  const [, year, month, day] = match;
  const date = new Date(Number(year), Number(month) - 1, Number(day));
  const isRealDate =
    !Number.isNaN(date.getTime()) &&
    date.getFullYear() === Number(year) &&
    date.getMonth() === Number(month) - 1 &&
    date.getDate() === Number(day);
  return isRealDate ? raw! : "";
}

/**
 * Comma-separated SRE team ids (`sreGroupId` UUIDs) — not a fixed enum,
 * blank entries dropped. Mirrors `parseTeamIdsCsv` in
 * `incidentsFiltersUrl.ts`.
 */
function parseTeamIdsCsv(raw: string | null): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * Read change-request filters from the URL. Unknown/malformed values (a
 * hand-edited or stale query string) are dropped rather than passed through,
 * so they fall back to the default (unfiltered) behaviour instead of being
 * silently sent to the backend.
 */
export function readChangeRequestFiltersFromUrl(
  params: URLSearchParams,
): ChangeRequestFilters {
  return {
    search: params.get("crQ") ?? "",
    states: parseCsv(params.get("crStates"), CHANGE_REQUEST_STATES),
    impacts: parseCsv(params.get("crImpacts"), CHANGE_REQUEST_IMPACTS),
    closedStartDate: parseDateOnly(params.get("crClosedFrom")),
    closedEndDate: parseDateOnly(params.get("crClosedTo")),
    sreTeamIds: parseTeamIdsCsv(params.get("crSreTeams")),
  };
}

/**
 * Build the search-params representing these filters. Default values are
 * omitted so the URL stays clean.
 */
export function writeChangeRequestFiltersToUrl(
  f: ChangeRequestFilters,
): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("crQ", f.search);
  if (f.states.length) out.set("crStates", f.states.join(","));
  if (f.impacts.length) out.set("crImpacts", f.impacts.join(","));
  if (f.closedStartDate) out.set("crClosedFrom", f.closedStartDate);
  if (f.closedEndDate) out.set("crClosedTo", f.closedEndDate);
  if (f.sreTeamIds.length) out.set("crSreTeams", f.sreTeamIds.join(","));
  return out;
}
