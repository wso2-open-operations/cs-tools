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

/**
 * URL (de)serialization for the users-list filter bar, following the same
 * `read*FiltersFromUrl` / `write*FiltersToUrl` convention as the cases list
 * (`casesFiltersUrl.ts`) so a filtered user list is shareable and survives a
 * reload. Deliberately no project/account filter here — "who is on this
 * project" is answered by the project-contacts search instead.
 */

export type ActiveFilter = "all" | "active" | "inactive";

export interface UsersFilters {
  search: string;
  groupIds: string[];
  teamIds: string[];
  active: ActiveFilter;
}

export const DEFAULT_USERS_FILTERS: UsersFilters = {
  search: "",
  groupIds: [],
  teamIds: [],
  active: "all",
};

const VALID_ACTIVE_VALUES: ActiveFilter[] = ["all", "active", "inactive"];

/**
 * Parse a CSV of free-form ids (group ids, team keys — neither a fixed
 * client-side enum). Empties stripped, length-capped per entry to avoid
 * pathological URL growth from a hand-edited link.
 */
function parseIdsCsv(raw: string | null, maxEntryLen = 120): string[] {
  if (!raw) return [];
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0 && s.length <= maxEntryLen);
}

export function readUsersFiltersFromUrl(params: URLSearchParams): UsersFilters {
  const activeRaw = params.get("active");
  const active: ActiveFilter = (VALID_ACTIVE_VALUES as string[]).includes(activeRaw ?? "")
    ? (activeRaw as ActiveFilter)
    : "all";

  return {
    search: params.get("search") ?? "",
    groupIds: parseIdsCsv(params.get("groups")),
    teamIds: parseIdsCsv(params.get("teams")),
    active,
  };
}

/**
 * Build the search-params object representing these filters. Default values
 * are omitted so the URL stays clean.
 */
export function writeUsersFiltersToUrl(f: UsersFilters): URLSearchParams {
  const out = new URLSearchParams();
  if (f.search) out.set("search", f.search);
  if (f.groupIds.length) out.set("groups", f.groupIds.join(","));
  if (f.teamIds.length) out.set("teams", f.teamIds.join(","));
  if (f.active !== "all") out.set("active", f.active);
  return out;
}
