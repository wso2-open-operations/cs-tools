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

import type { ChangeRequestFilterableState, ChangeRequestImpact, ChangeRequestSearchPayloadDto } from "@src/types";

export interface ChangeRequestFilters {
  states: ChangeRequestFilterableState[];
  impacts: ChangeRequestImpact[];
  /** YYYY-MM-DD, or empty. */
  closedStartDate: string;
  /** YYYY-MM-DD, or empty. */
  closedEndDate: string;
}

export const EMPTY_CR_FILTERS: ChangeRequestFilters = {
  states: [],
  impacts: [],
  closedStartDate: "",
  closedEndDate: "",
};

export function countActiveCRFilters(filters: ChangeRequestFilters): number {
  return (
    (filters.states.length > 0 ? 1 : 0) +
    (filters.impacts.length > 0 ? 1 : 0) +
    (filters.closedStartDate ? 1 : 0) +
    (filters.closedEndDate ? 1 : 0)
  );
}

export function toChangeRequestSearchFilters(
  search: string,
  filters: ChangeRequestFilters,
): ChangeRequestSearchPayloadDto["filters"] {
  return {
    ...(search.length > 0 && { searchQuery: search }),
    ...(filters.states.length > 0 && { states: filters.states }),
    ...(filters.impacts.length > 0 && { impacts: filters.impacts }),
    // Closed-date filters are UTC datetimes on the wire; a plain YYYY-MM-DD local date is widened
    // to cover the whole day in UTC, mirroring the webapp's toISOStart/toISOEnd.
    ...(filters.closedStartDate && { closedStartDate: `${filters.closedStartDate}T00:00:00.000Z` }),
    ...(filters.closedEndDate && { closedEndDate: `${filters.closedEndDate}T23:59:59.999Z` }),
  };
}
