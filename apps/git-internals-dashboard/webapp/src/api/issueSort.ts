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

// GET /issues's `sort`/`order` values. Matches the backend's
// issueSortColumns whitelist (internal/handler/issue_sort.go) one-for-one —
// add a sortable field in both places together.
export type IssueSortField = "sla_consumption" | "created" | "updated";
export type IssueSortOrder = "asc" | "desc";

const ISSUE_SORT_FIELDS: readonly IssueSortField[] = ["sla_consumption", "created", "updated"];

export const DEFAULT_ISSUE_SORT: { field: IssueSortField; order: IssueSortOrder } = {
  field: "sla_consumption",
  order: "desc",
};

/** Maps an unknown or legacy URL `sort` value (e.g. a bookmarked `sort=age`) to the default field instead of sending it to the backend. */
export function parseIssueSortField(value: string | null): IssueSortField {
  return value != null && (ISSUE_SORT_FIELDS as readonly string[]).includes(value)
    ? (value as IssueSortField)
    : DEFAULT_ISSUE_SORT.field;
}

/** Maps an unknown URL `order` value to the default direction instead of sending it to the backend. */
export function parseIssueSortOrder(value: string | null): IssueSortOrder {
  return value === "asc" || value === "desc" ? value : DEFAULT_ISSUE_SORT.order;
}
