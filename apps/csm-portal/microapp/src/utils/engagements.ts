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

import type { CaseState, CaseWorkState, EngagementType, Project } from "@src/types";

// Mirrors the webapp's ALL_ENGAGEMENT_TYPES/ENGAGEMENT_TYPE_LABEL
// (apps/csm-portal/webapp/src/features/csm-cases/components/CasesFilterBar.tsx).
export const ALL_ENGAGEMENT_TYPES: EngagementType[] = [
  "migration",
  "consultancy",
  "new_feature_improvement",
  "follow_up",
  "onboarding",
];

export const ENGAGEMENT_TYPE_LABEL: Record<EngagementType, string> = {
  migration: "Migration",
  consultancy: "Consultancy",
  new_feature_improvement: "New feature / improvement",
  follow_up: "Follow-up",
  onboarding: "Onboarding",
};

/** Lightweight assignee ref for the filter chip — mirrors how `projects` stores a
 * display-ready `Project[]` rather than bare ids. */
export interface EngagementAssignee {
  id: string;
  name: string;
}

// UI-facing engagement filters. `search` matches subject/number (server-side
// `searchQuery`); `states` → server-side `states`; `workStates` → server-side
// `workStates` (only meaningful while `states` includes "work_in_progress");
// `projects` → server-side `projectIds`; `engagementTypes` → server-side
// `engagementTypes`; `assignees` → server-side `assignedUserIds`; `productNames`
// → server-side `productNames`. All empty by default, so the list shows every
// state/type across all projects. (No severity — engagements carry none, same
// as the webapp's CsmIssuesView locks it out for any non-"case" type.)
export interface EngagementFilters {
  search: string;
  states: CaseState[];
  workStates: NonNullable<CaseWorkState>[];
  projects: Project[];
  engagementTypes: EngagementType[];
  assignees: EngagementAssignee[];
  productNames: string[];
}

export const EMPTY_ENGAGEMENT_FILTERS: EngagementFilters = {
  search: "",
  states: [],
  workStates: [],
  projects: [],
  engagementTypes: [],
  assignees: [],
  productNames: [],
};

/** Active filter groups — drives the "Filters (n)" badge. Search is excluded (it
 * has its own always-visible box). */
export function countActiveEngagementFilters(filters: EngagementFilters): number {
  let count = 0;
  if (filters.states.length > 0) count += 1;
  if (filters.workStates.length > 0) count += 1;
  if (filters.projects.length > 0) count += 1;
  if (filters.engagementTypes.length > 0) count += 1;
  if (filters.assignees.length > 0) count += 1;
  if (filters.productNames.length > 0) count += 1;
  return count;
}
