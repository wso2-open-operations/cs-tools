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

import { Box, Card, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useDashboard } from "@features/csm-dashboard/api/useDashboard";
import DashboardWidgetGrid from "@features/csm-dashboard/components/DashboardWidgetGrid";
import DateRangeFilter, {
  type DateRangeFilterValue,
} from "@features/csm-dashboard/components/DateRangeFilter";
import SectionCard from "@features/csm-dashboard/components/SectionCard";
import { WIDGET_GRID_SX } from "@features/csm-dashboard/utils/dashboardWidgetGridLayout";
import { hasDateRangeFilterPlaceholder } from "@features/csm-dashboard/utils/dateRangeFilterPlaceholder";

/** Placeholder tile count while the dashboard detail is in flight. */
const PILOT_TILE_COUNT = 3;

interface AgentsLandingPagePilotProps {
  /** Id of the dashboard to render (e.g. "agents_pilot"). */
  dashboardId: string;
  /** The currently selected team's own `creGroupId` (see
   * `BeTeam.creGroupId`), or an array of every team's `creGroupId` in the
   * current dashboard's family when the "All ABTs" option is selected (see
   * `ALL_TEAMS_SENTINEL` in `teamFilterPlaceholder.ts`) — only meaningful
   * for an `isTeamBased` dashboard, threaded straight through to every tile
   * so each can resolve its own `__current_team__` filter placeholder for a
   * `creTeam` filter entry. `undefined` for a non-team-based dashboard, or
   * while the team isn't resolved yet. */
  selectedTeamCreGroupId?: string | string[];
  /** The currently selected team's own `sreGroupId` (see
   * `BeTeam.sreGroupId`), or an array of every team's `sreGroupId` in the
   * current dashboard's family when the "All ABTs" option is selected — the
   * `sreTeam`-filter counterpart of {@link selectedTeamCreGroupId}, resolved
   * independently. `undefined` in the same cases `selectedTeamCreGroupId`
   * is. */
  selectedTeamSreGroupId?: string | string[];
  /** Human-readable label for the selected team (its own display `name`,
   * or the literal `"All ABTs"`) — threaded down for each tile's own
   * `{{currentTeam}}` widget text placeholder (see
   * `widgetTextPlaceholder.ts`). `undefined` in the same cases
   * `selectedTeamCreGroupId` is. */
  selectedTeamLabel?: string;
}

/**
 * Pilot section for the config-driven dashboard widget system: renders
 * whichever dashboard's real `single_score` widgets are passed in via
 * `dashboardId`. The dashboard's metadata plus its widget templates —
 * display metadata and each widget's filter criteria — are fetched once via
 * {@link useDashboard}; each rendered tile then resolves its own data
 * independently. Today only the "agents_pilot" dashboard has real widgets
 * (see CsmDashboardPage.tsx), but this component is generic over any
 * dashboard id with widgets.
 */
export default function AgentsLandingPagePilot({
  dashboardId,
  selectedTeamCreGroupId,
  selectedTeamSreGroupId,
  selectedTeamLabel,
}: AgentsLandingPagePilotProps): JSX.Element {
  const { data, isLoading, isError } = useDashboard(dashboardId);

  // Whether ANY widget on this loaded dashboard actually uses the
  // date-range placeholder (see `dateRangeFilterPlaceholder.ts`) — derived
  // purely from the loaded widget list, not a dashboard-level config flag:
  // no such flag exists (or is needed), the same "derive from what's
  // actually configured" approach the chart-vs-list-tile grouping above
  // already uses. Checks both a widget's own `query` (every shape) and, for
  // a `groupBy`-configured pie/bar widget, `groupBy` itself carries no
  // filters of its own to check — its base filters are `query`, same as
  // every other shape (see `BeDashboardWidget.query`'s own doc comment) — so
  // `query` alone is the complete check. Defaults to `false` (no control
  // rendered) while the dashboard is still loading/erroring, same as
  // rendering no grid at all in those states below.
  const showDateRangeFilter = useMemo(
    () => (data?.widgets ?? []).some((w) => hasDateRangeFilterPlaceholder(w.query ?? {})),
    [data?.widgets],
  );
  // Page-local UI state, not URL/query-param-backed: unlike the team
  // picker (a real selection that should survive a refresh/share — see
  // `CsmDashboardPage`), a date range here is closer to an in-page filter
  // tweak. `undefined`/`undefined` (both fields) is "no range selected",
  // which `resolveDateRangeFilterPlaceholder` treats as "drop the filter
  // entirely" — i.e. all time, matching this dashboard's own real
  // unfiltered totals (961 feedback records, 229 in the last ~22 months) as
  // the default view rather than an arbitrary pre-selected window.
  const [dateRange, setDateRange] = useState<DateRangeFilterValue>({});

  return (
    <SectionCard>
      {isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load the widget pilot.
        </Typography>
      ) : isLoading ? (
        <Box sx={WIDGET_GRID_SX}>
          {Array.from({ length: PILOT_TILE_COUNT }, (_, i) => (
            <Card key={i} variant="outlined" sx={{ p: 1.75, gridColumn: "span 4" }}>
              <Skeleton variant="rounded" height={48} />
            </Card>
          ))}
        </Box>
      ) : (
        <>
          {showDateRangeFilter && (
            <Box sx={{ mb: 2 }}>
              <DateRangeFilter value={dateRange} onChange={setDateRange} />
            </Box>
          )}
          <DashboardWidgetGrid
            widgets={data?.widgets ?? []}
            selectedTeamCreGroupId={selectedTeamCreGroupId}
            selectedTeamSreGroupId={selectedTeamSreGroupId}
            selectedTeamLabel={selectedTeamLabel}
            dateRangeFrom={dateRange.from}
            dateRangeTo={dateRange.to}
          />
        </>
      )}
    </SectionCard>
  );
}
