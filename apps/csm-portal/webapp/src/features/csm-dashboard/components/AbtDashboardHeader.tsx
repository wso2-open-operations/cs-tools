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

import { Box, FormControl, MenuItem, Select, Typography } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import type { BeDashboardListItem } from "@api/backend/types";
import type { DashboardKey } from "@features/csm-dashboard/types/abtDashboard";
import { abtFamilyForDashboardType, useTeams } from "@features/csm-dashboard/api/useTeams";
import { ALL_TEAMS_SENTINEL } from "@features/csm-dashboard/utils/teamFilterPlaceholder";

interface AbtDashboardHeaderProps {
  dashboardKey: DashboardKey;
  onDashboardChange: (key: DashboardKey) => void;
  /** Every dashboard in the BE registry (GET /dashboards), for the switcher. */
  dashboardList: BeDashboardListItem[];
  /** The currently selected team, controlled by the parent (URL-synced,
   * defaulted to the signed-in user's own team once resolved, or to
   * `ALL_TEAMS_SENTINEL` when the signed-in user has no home team — see
   * `CsmDashboardPage`). `undefined` when the current dashboard isn't
   * `isTeamBased` (the parent never passes a stale team id through in that
   * case) or, briefly, while the team list/user profile are still loading.
   * The Select also offers an `ALL_TEAMS_SENTINEL`-valued "All ABTs" entry
   * (see `teamFilterPlaceholder.ts`) — every team-based dashboard view now
   * has either a real team or "All ABTs" selected, never nothing. This is
   * narrower than, and unrelated to, the earlier "My ABT / All customers"
   * toggle removed 2026-08-02: that one spanned every team in the whole
   * registry, this one never leaves the current dashboard's own family
   * (`abtFamilyForDashboardType`). */
  selectedTeamId: string | undefined;
  onTeamChange: (teamId: string | undefined) => void;
}

/**
 * Dashboard header: title, the dashboard switcher, and (for a dashboard
 * flagged `isTeamBased`) a team selector sourced from `POST /teams/search`,
 * scoped to the current dashboard's family (`abtFamilyForDashboardType`) —
 * a `cre` dashboard's picker offers only `cre-abt` teams, `sre` only
 * `sre-abt`; a dashboard with no `type` gets every team, unfiltered.
 * Both the selected dashboard and the selected team are owned by the parent
 * (`CsmDashboardPage`), which keeps them in sync with the URL (a fragment,
 * not a query param) so a specific dashboard/team view is shareable. The
 * selected team scopes widget data client-side via the
 * `__current_team__` filter placeholder (see `teamFilterPlaceholder.ts`) —
 * the parent resolves the selected team's own `creGroupId`/`sreGroupId` and
 * threads both down to the widget grid. The earlier My ABT / All customers
 * toggle was removed
 * entirely — ABT scoping was never implemented and dashboards carry no
 * other special behavior beyond which one (and, for team-based ones, which
 * team) is selected. The picker's "All ABTs" entry (`ALL_TEAMS_SENTINEL`)
 * scopes to every team in the CURRENT dashboard's own family, not every
 * team in the registry — see `selectedTeamId`'s own doc comment above.
 * The dashboard switcher itself is hidden when `dashboardList` has one entry
 * or fewer — a caller with only one dashboard available (e.g. every
 * portal role but CS engineer/admin, once the restricted ones are
 * filtered server-side) has nothing to switch between, so the control would
 * only ever show its own single option.
 */
export default function AbtDashboardHeader({
  dashboardKey,
  onDashboardChange,
  dashboardList,
  selectedTeamId,
  onTeamChange,
}: AbtDashboardHeaderProps): JSX.Element {
  const currentOption = dashboardList.find((o) => o.id === dashboardKey);
  const isTeamBased = currentOption?.isTeamBased ?? false;
  const family = abtFamilyForDashboardType(currentOption?.type);

  const teams = useTeams(isTeamBased, family);

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        flexWrap: "wrap",
      }}
    >
      <Box>
        <Typography variant="h5">Dashboard</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          {currentOption?.displayName ?? ""}
        </Typography>
      </Box>
      <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
        {dashboardList.length > 1 && (
          <FormControl size="small" sx={{ minWidth: 200 }}>
            <Select
              value={dashboardKey}
              onChange={(e) => onDashboardChange(e.target.value as DashboardKey)}
              displayEmpty
              aria-label="Select dashboard"
            >
              {dashboardList.map((o) => (
                <MenuItem key={o.id} value={o.id}>
                  {o.displayName}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
        {isTeamBased && (
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <Select
              value={selectedTeamId ?? ""}
              onChange={(e) => onTeamChange(e.target.value || undefined)}
              displayEmpty
              aria-label="Select team"
            >
              <MenuItem value={ALL_TEAMS_SENTINEL}>All ABTs</MenuItem>
              {(teams.data ?? []).map((t) => (
                <MenuItem key={t.id} value={t.id}>
                  {t.name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
      </Box>
    </Box>
  );
}
