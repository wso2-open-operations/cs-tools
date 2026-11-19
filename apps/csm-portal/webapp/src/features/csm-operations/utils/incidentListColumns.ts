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
 * Every column `IncidentsTab` can render, in the default left-to-right
 * order. All of them can be hidden, shown or reordered through
 * `ColumnCustomizerButton` (at least one always stays), so a user can show
 * only what they need -- the original seven (Number, Subject, Caller,
 * State, Priority, Opened, Updated) are the default set, not a fixed one.
 *
 * Every one of these was already present on `BeIncident` (the
 * `/incidents/search` response). Fields deliberately left off:
 * - `id` -- a raw id, never human-facing.
 * - `createdOn` is offered separately from `openedOn`: the record-creation
 *   timestamp and the incident's own "opened at" can genuinely differ, same
 *   reasoning as `CsmCaseRow.createdAt`.
 */
export type IncidentColumnId =
  | "number"
  | "subject"
  | "caller"
  | "state"
  | "priority"
  | "category"
  | "assignmentGroup"
  | "assignedTo"
  | "parent"
  | "createdBy"
  | "updatedBy"
  | "createdOn"
  | "openedOn"
  | "updatedOn";

export const INCIDENT_COLUMNS: Record<IncidentColumnId, { label: string }> = {
  number: { label: "Number" },
  subject: { label: "Subject" },
  caller: { label: "Caller" },
  state: { label: "State" },
  priority: { label: "Priority" },
  category: { label: "Category" },
  assignmentGroup: { label: "Assignment group" },
  assignedTo: { label: "Assigned to" },
  parent: { label: "Parent" },
  createdBy: { label: "Created by" },
  updatedBy: { label: "Updated by" },
  createdOn: { label: "Created" },
  openedOn: { label: "Opened" },
  updatedOn: { label: "Updated" },
};

/** The table's original seven columns: what a user sees until they open the picker. */
export const DEFAULT_VISIBLE_INCIDENT_COLUMNS: IncidentColumnId[] = [
  "number",
  "subject",
  "caller",
  "state",
  "priority",
  "openedOn",
  "updatedOn",
];

/** The saved column choice's own key. A new one (not the earlier "incidents"
 * view id) because that saved only the optional columns: read as this full
 * list, it would leave the defaults after them in the wrong order. */
export const INCIDENT_COLUMNS_VIEW_ID = "incidents-columns";
