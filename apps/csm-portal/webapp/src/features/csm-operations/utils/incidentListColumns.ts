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
 * The "optional" columns `IncidentsTab` can render between Priority and
 * Opened — everything a user can add, remove, or reorder via
 * `ColumnCustomizerButton`. Number, Subject, Caller, State, Priority,
 * Opened, and Updated are never optional (the table's original fixed set,
 * unaffected by this feature).
 *
 * Every one of these was already present on `BeIncident` (the
 * `/incidents/search` response) but not rendered anywhere — `category`,
 * `assignmentGroup`, and `assignedTo` are already proven-renderable via
 * `IncidentPreviewDrawer`; `parent`, `createdBy`, and `updatedBy` get the
 * same treatment here for the first time. Fields deliberately left off:
 * - `id` — a raw id, never human-facing.
 * - `createdOn` vs `openedOn`/`updatedOn` — `openedOn`/`updatedOn` are
 *   already fixed columns; `createdOn` is still offered separately below
 *   since ServiceNow's record-creation timestamp and its own "opened at"
 *   can genuinely differ, same reasoning as `CsmCaseRow.createdAt`.
 */
export type IncidentOptionalColumnId =
  | "category"
  | "assignmentGroup"
  | "assignedTo"
  | "parent"
  | "createdBy"
  | "updatedBy"
  | "createdOn";

export const INCIDENT_OPTIONAL_COLUMNS: Record<IncidentOptionalColumnId, { label: string }> = {
  category: { label: "Category" },
  assignmentGroup: { label: "Assignment group" },
  assignedTo: { label: "Assigned to" },
  parent: { label: "Parent" },
  createdBy: { label: "Created by" },
  updatedBy: { label: "Updated by" },
  createdOn: { label: "Created" },
};

/** None of these existed as columns before this feature — the default
 * visible set is empty, so a returning user sees the exact same table until
 * they open the picker themselves. */
export const DEFAULT_VISIBLE_INCIDENT_COLUMNS: IncidentOptionalColumnId[] = [];
