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

import { ALL_TEAMS_SENTINEL } from "@features/csm-dashboard/utils/teamFilterPlaceholder";

export type TeamScopeState =
  /** Widgets may fetch: no team is selected, "All ABTs" is, or the team resolved with a usable group id. */
  | "ready"
  /** The teams list is still loading — a team-scoped widget would otherwise fetch org-wide. */
  | "loading"
  /** The teams list failed to load, so the selected team's scope is unknown. */
  | "error"
  /** The selected team is unknown or has no group id for this dashboard's discipline. */
  | "missing";

export interface TeamScopeInput {
  isTeamBased: boolean;
  selectedTeamId: string | undefined;
  /** `dashboard.type` — `cre` needs a CRE group id, `sre` an SRE group id. */
  dashboardType: string | undefined;
  teamsPending: boolean;
  teamsError: boolean;
  /** Whether the selected id matches a team in the loaded list. */
  selectedTeamFound: boolean;
  selectedTeamHasCreGroup: boolean;
  selectedTeamHasSreGroup: boolean;
}

/**
 * Whether a team-based dashboard's widgets may fetch yet. A `__current_team__`
 * filter whose group id cannot be resolved is dropped, which widens the query
 * to every team — so a single selected team must never reach the widgets
 * until its group id is known. "All ABTs" is a deliberate org-wide scope and
 * is always ready.
 */
export function resolveTeamScopeState({
  isTeamBased,
  selectedTeamId,
  dashboardType,
  teamsPending,
  teamsError,
  selectedTeamFound,
  selectedTeamHasCreGroup,
  selectedTeamHasSreGroup,
}: TeamScopeInput): TeamScopeState {
  if (!isTeamBased || !selectedTeamId || selectedTeamId === ALL_TEAMS_SENTINEL) {
    return "ready";
  }
  if (teamsPending) return "loading";
  if (teamsError) return "error";
  if (!selectedTeamFound) return "missing";
  const hasCre = selectedTeamHasCreGroup;
  const hasSre = selectedTeamHasSreGroup;
  if (dashboardType === "cre") return hasCre ? "ready" : "missing";
  if (dashboardType === "sre") return hasSre ? "ready" : "missing";
  return hasCre || hasSre ? "ready" : "missing";
}
