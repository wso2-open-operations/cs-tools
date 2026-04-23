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
 * True when the user entered search text or any filter field has a value.
 * Used to choose empty-state copy (refined vs default list).
 *
 * @param searchTerm - Current search string.
 * @param filters - Filter object (string or numeric values).
 * @returns {boolean} True if search or any filter is active.
 */
export function hasListSearchOrFilters(
  searchTerm: string,
  filters: object,
): boolean {
  return countListSearchAndFilters(searchTerm, filters) > 0;
}

/**
 * Counts active refinements: non-empty search counts as one, plus each non-empty filter field.
 *
 * @param searchTerm - Search string.
 * @param filters - Filter key/value object.
 * @returns {number} Number of active filters (minimum 0).
 */
export function countListSearchAndFilters(
  searchTerm: string,
  filters: object,
): number {
  let n = 0;
  if (searchTerm.trim().length > 0) n += 1;
  for (const v of Object.values(
    filters as Record<string, string | number | undefined | null>,
  )) {
    if (v !== undefined && v !== null && String(v).trim() !== "") {
      n += 1;
    }
  }
  return n;
}
