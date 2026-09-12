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
  INCIDENT_PRIORITIES,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";

// URL params owned by the incident filter state. Prefixed (`inc...`) so they
// can't collide with the same-named params the shared cases view and the
// change-requests tab keep in the same `?tab=`-switched URL.
export const INCIDENT_FILTER_PARAM_KEYS = [
  "incQ",
  "incPriorities",
  "incSlaViolated",
  "incCreatedFrom",
  "incCreatedTo",
  "incProducts",
  "incSreTeams",
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
 * rather than passed through to the backend. Matches the change-requests
 * tab's own `parseDateOnly` (`changeRequestsFiltersUrl.ts`); this filter
 * treats the value as a UTC calendar date (see `IncidentsFilterBar`) rather
 * than a local one, but the shape validation is the same.
 */
function parseDateOnly(raw: string | null): string {
  const match = raw ? DATE_ONLY_RE.exec(raw) : null;
  if (!match) return "";
  const [, year, month, day] = match;
  const date = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
  const isRealDate =
    !Number.isNaN(date.getTime()) &&
    date.getUTCFullYear() === Number(year) &&
    date.getUTCMonth() === Number(month) - 1 &&
    date.getUTCDate() === Number(day);
  return isRealDate ? raw! : "";
}

/**
 * Comma-separated product names. Unlike `incPriorities`, this isn't a fixed
 * enum — arbitrary (non-empty, trimmed) values are accepted, since the
 * backend's own list is a ~43%-populated, uncontrolled catalogue (see
 * `IncidentProductMultiSelect`). Blank entries are dropped: the backend
 * rejects a blank/whitespace product name with a 400.
 */
function parseProductsCsv(raw: string | null): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * Comma-separated SRE team ids (`sreGroupId` UUIDs) — same shape as
 * `parseProductsCsv`: not a fixed enum, blank entries dropped. A stale/
 * hand-edited id that no longer matches a real team just yields an
 * unexpectedly narrow (possibly empty) result set rather than an error, same
 * as `parseProductsCsv`'s own tolerance.
 */
function parseTeamIdsCsv(raw: string | null): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

/**
 * Read incident filters from the URL. An unknown/malformed value (a
 * hand-edited or stale query string) is dropped rather than passed through,
 * so it falls back to the default (unfiltered) behaviour instead of being
 * silently sent to the backend.
 */
export function readIncidentFiltersFromUrl(
  params: URLSearchParams,
): IncidentFilters {
  const createdStartDate = parseDateOnly(params.get("incCreatedFrom"));
  let createdEndDate = parseDateOnly(params.get("incCreatedTo"));
  // Each bound parses independently, so a hand-edited or stale URL can invert
  // the range with two individually valid dates. Forwarded as-is that yields an
  // always-empty result with nothing explaining why, and leaves the date
  // pickers' minDate/maxDate contradicting the value they display. Drop the end
  // bound — the same "malformed input falls back to unfiltered" rule this
  // function already applies to every other param. Both dates are plain
  // YYYY-MM-DD, so a lexicographic compare is also a chronological one.
  if (createdStartDate && createdEndDate && createdStartDate > createdEndDate) {
    createdEndDate = "";
  }
  return {
    search: params.get("incQ") ?? "",
    priorities: parseCsv(params.get("incPriorities"), INCIDENT_PRIORITIES),
    slaViolated: params.get("incSlaViolated") === "1",
    createdStartDate,
    createdEndDate,
    products: parseProductsCsv(params.get("incProducts")),
    sreTeamIds: parseTeamIdsCsv(params.get("incSreTeams")),
  };
}

/**
 * Build the search-params representing these filters. Default values are
 * omitted so the URL stays clean.
 */
export function writeIncidentFiltersToUrl(f: IncidentFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("incQ", f.search);
  if (f.priorities.length) out.set("incPriorities", f.priorities.join(","));
  if (f.slaViolated) out.set("incSlaViolated", "1");
  if (f.createdStartDate) out.set("incCreatedFrom", f.createdStartDate);
  if (f.createdEndDate) out.set("incCreatedTo", f.createdEndDate);
  if (f.products.length) out.set("incProducts", f.products.join(","));
  if (f.sreTeamIds.length) out.set("incSreTeams", f.sreTeamIds.join(","));
  return out;
}
