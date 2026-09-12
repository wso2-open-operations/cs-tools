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
 * The "optional" columns `ChangeRequestsTab` can render between Subject and
 * State — everything a user can add, remove, or reorder via
 * `ColumnCustomizerButton`. Number, Subject, State, and Updated are never
 * optional: Number/Subject carry the row's identity and its real link,
 * State is the at-a-glance lifecycle signal, and Updated is the list's
 * implicit sort order.
 *
 * Fields on `BeChangeRequestSearchView` deliberately left off this list, and
 * why:
 * - `id` — a raw UUID, never human-facing.
 * - `description` — long-form text, not table-friendly; read on the detail page.
 * - `duration` — derived from `plannedStartOn`/`plannedEndOn`, both already
 *   offered individually; showing all three would be redundant.
 * - `deployment`, `deployedProduct` — deployment-level detail that's one more
 *   click away on the CR detail page; `product` and `project` already give
 *   enough at-a-glance context for a list row.
 * - `createdOn` vs `updatedOn` — `updatedOn` is the fixed "Updated" column;
 *   `createdOn` is still offered separately below since a CR's creation date
 *   and its last-touched date are genuinely different facts, same reasoning
 *   as `CsmCaseRow.createdAt`.
 */
export type ChangeRequestOptionalColumnId =
  | "project"
  | "impact"
  | "plannedStart"
  | "plannedEnd"
  | "product"
  | "assignedEngineer"
  | "assignedTeam"
  | "type"
  | "case"
  | "createdOn";

export const CHANGE_REQUEST_OPTIONAL_COLUMNS: Record<
  ChangeRequestOptionalColumnId,
  { label: string }
> = {
  project: { label: "Project" },
  impact: { label: "Impact" },
  plannedStart: { label: "Planned start" },
  plannedEnd: { label: "Planned end" },
  product: { label: "Product" },
  assignedEngineer: { label: "Assigned engineer" },
  assignedTeam: { label: "Assigned team" },
  type: { label: "Type" },
  case: { label: "Case" },
  createdOn: { label: "Created" },
};

/** The columns rendered before this feature existed — kept as the default
 * visible set so a returning user sees the exact same table until they open
 * the picker themselves. */
export const DEFAULT_VISIBLE_CHANGE_REQUEST_COLUMNS: ChangeRequestOptionalColumnId[] =
  ["project", "impact", "plannedStart"];
