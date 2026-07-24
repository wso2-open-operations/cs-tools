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

import type { CaseSearchFiltersDto, CaseSeverity, CaseState, CaseType, CaseWorkState } from "@src/types";

export interface CaseFilters {
  severities: CaseSeverity[];
  states: CaseState[];
  workStates: NonNullable<CaseWorkState>[];
  assignedToMe: boolean;
  createdByMe: boolean;
}

export const EMPTY_FILTERS: CaseFilters = {
  severities: [],
  states: [],
  workStates: [],
  assignedToMe: false,
  createdByMe: false,
};

// Excludes `states` — state has its own dedicated tab row (not the filter sheet), so it
// shouldn't double-count against the sheet's "N filters active" badge.
export function countActiveFilters(filters: CaseFilters): number {
  return (
    filters.severities.length +
    filters.workStates.length +
    (filters.assignedToMe ? 1 : 0) +
    (filters.createdByMe ? 1 : 0)
  );
}
export function toCaseSearchFilters(
  type: CaseType,
  search: string,
  filters: CaseFilters,
  currentUserId: string | null,
): CaseSearchFiltersDto {
  return {
    types: [type],
    ...(search.length > 0 && { searchQuery: search }),
    ...(filters.severities.length > 0 && { severities: filters.severities }),
    ...(filters.states.length > 0 && { states: filters.states }),
    ...(filters.workStates.length > 0 && { workStates: filters.workStates }),
    ...(filters.createdByMe && { createdByMe: true }),
    ...(filters.assignedToMe && currentUserId && { assignedUserIds: [currentUserId] }),
  };
}

export function filtersToSearchParams(search: string, filters: CaseFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (search) params.set("q", search);
  if (filters.severities.length) params.set("severities", filters.severities.join(","));
  if (filters.states.length) params.set("states", filters.states.join(","));
  if (filters.workStates.length) params.set("workStates", filters.workStates.join(","));
  if (filters.assignedToMe) params.set("assignedToMe", "1");
  if (filters.createdByMe) params.set("createdByMe", "1");
  return params;
}

export function filtersFromSearchParams(params: URLSearchParams): { search: string; filters: CaseFilters } {
  const csv = (key: string) => params.get(key)?.split(",").filter(Boolean) ?? [];
  return {
    search: params.get("q") ?? "",
    filters: {
      severities: csv("severities") as CaseSeverity[],
      states: csv("states") as CaseState[],
      workStates: csv("workStates") as NonNullable<CaseWorkState>[],
      assignedToMe: params.get("assignedToMe") === "1",
      createdByMe: params.get("createdByMe") === "1",
    },
  };
}
