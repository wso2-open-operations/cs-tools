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

import type { ChipProps } from "@wso2/oxygen-ui";
import type { CaseState, Project } from "@src/types";

// Announcement lifecycle states offered in the filter. `reopened` is excluded —
// it only appears as a `nextStates` signal, never a case's own state (mirrors the
// webapp's announcement state options).
export const ANNOUNCEMENT_FILTER_STATES: CaseState[] = [
  "open",
  "work_in_progress",
  "solution_proposed",
  "awaiting_info",
  "waiting_on_wso2",
  "closed",
];

// UI-facing announcement filters. `search` matches subject/number (server-side
// `searchQuery`); `states` → server-side `states`; `projects` → server-side
// `projectIds`. All empty by default, so the list shows every state across all
// projects. (No severity — announcements carry none.)
export interface AnnouncementFilters {
  search: string;
  states: CaseState[];
  projects: Project[];
}

export const EMPTY_ANNOUNCEMENT_FILTERS: AnnouncementFilters = { search: "", states: [], projects: [] };

/** Active filter groups — drives the "Filters (n)" badge. Search is excluded (it
 * has its own always-visible box). */
export function countActiveAnnouncementFilters(filters: AnnouncementFilters): number {
  let count = 0;
  if (filters.states.length > 0) count += 1;
  if (filters.projects.length > 0) count += 1;
  return count;
}

/**
 * Chip colour for an announcement's state — announcement-specific, NOT the
 * case-state palette (which paints a closed case green). An Open announcement is
 * live/published → success (green), a Closed one is inactive → default (grey),
 * anything else → info.
 */
export function announcementStateColor(state: CaseState): ChipProps["color"] {
  if (state === "open") return "success";
  if (state === "closed") return "default";
  return "info";
}
