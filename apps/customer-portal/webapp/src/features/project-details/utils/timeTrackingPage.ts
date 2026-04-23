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
 * Resolves the time card state id whose label is "Approved" (case-insensitive).
 *
 * @param states - Filter metadata time card states.
 * @returns State id when found.
 */
export function findApprovedTimeCardStateId(
  states: { id: string; label: string }[] | undefined,
): string | undefined {
  if (!states?.length) {
    return undefined;
  }
  const approved = states.find((s) => s.label.toLowerCase() === "approved");
  return approved?.id;
}

/**
 * Client-side page slice for a flattened list.
 *
 * @param items - Full list.
 * @param page - One-based page index.
 * @param pageSize - Page size.
 * @returns Items for the current page.
 */
export function paginateList<T>(
  items: T[],
  page: number,
  pageSize: number,
): T[] {
  const startIndex = (page - 1) * pageSize;
  return items.slice(startIndex, startIndex + pageSize);
}
