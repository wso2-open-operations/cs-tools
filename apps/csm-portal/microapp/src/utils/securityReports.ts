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

import type { CaseState, CaseWorkState, Project } from "@src/types";

/** Lightweight assignee ref for the filter chip — mirrors how `projects` stores a
 * display-ready `Project[]` rather than bare ids (see utils/engagements.ts's own copy). */
export interface SecurityReportAssignee {
  id: string;
  name: string;
}

// UI-facing security report filters. `search` matches subject/number (server-side
// `searchQuery`); `states` → server-side `states`; `workStates` → server-side
// `workStates` (only meaningful while `states` includes "work_in_progress");
// `projects` → server-side `projectIds`; `assignees` → server-side
// `assignedUserIds`; `productNames` → server-side `productNames`. All empty by
// default, so the list shows every state across all projects. (No severity —
// security reports carry none, same as the webapp's CsmIssuesView locks it out
// for any non-"case" type. No engagement type either — that's engagement-only.)
export interface SecurityReportFilters {
  search: string;
  states: CaseState[];
  workStates: NonNullable<CaseWorkState>[];
  projects: Project[];
  assignees: SecurityReportAssignee[];
  productNames: string[];
}

export const EMPTY_SECURITY_REPORT_FILTERS: SecurityReportFilters = {
  search: "",
  states: [],
  workStates: [],
  projects: [],
  assignees: [],
  productNames: [],
};

/** Active filter groups — drives the "Filters (n)" badge. Search is excluded (it
 * has its own always-visible box). */
export function countActiveSecurityReportFilters(filters: SecurityReportFilters): number {
  let count = 0;
  if (filters.states.length > 0) count += 1;
  if (filters.workStates.length > 0) count += 1;
  if (filters.projects.length > 0) count += 1;
  if (filters.assignees.length > 0) count += 1;
  if (filters.productNames.length > 0) count += 1;
  return count;
}
