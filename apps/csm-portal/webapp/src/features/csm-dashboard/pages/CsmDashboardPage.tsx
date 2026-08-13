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

import { Box, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useCallback, useMemo, type JSX } from "react";
import { useParams } from "react-router";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import { abtFamilyForDashboardType, useTeams } from "@features/csm-dashboard/api/useTeams";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import type { DashboardKey } from "@features/csm-dashboard/types/abtDashboard";
import { useNavTransition } from "@hooks/useNavTransition";
import { ALL_TEAMS_SENTINEL } from "@features/csm-dashboard/utils/teamFilterPlaceholder";

/**
 * Top-level CSM dashboard. The dashboard list is BE-driven (`GET
 * /dashboards`), and the initial selection depends on the signed-in user's
 * own ABT team membership (`GET /users/me`'s `team`, via `useCurrentUser`):
 * a user WITH a resolved team defaults to the first dashboard with BOTH
 * `isDefault` and `isTeamBased` set (with that team auto-selected — see
 * `selectedTeamId` below); a user with no team defaults to the first
 * dashboard with `isDefault` set and `isTeamBased` NOT set. If no dashboard
 * matches that preferred predicate, this falls back to the BE's own (any)
 * `isDefault` entry, then to the first dashboard in the list — never to an
 * empty selection. The URL always wins over all of that when it names a
 * (valid) dashboard — see the `:dashboardId`/`:teamId` route params this
 * page is mounted under (`/dashboard`, `/dashboard/:dashboardId`,
 * `/dashboard/:dashboardId/:teamId` — see `App.tsx`).
 *
 * Dashboards are selected purely by dropdown — there is no other
 * per-dashboard scoping control. Every dashboard in the registry has at
 * least one real (config-driven) widget, so this always renders the real
 * widget grid.
 */
export default function CsmDashboardPage(): JSX.Element {
  const navigate = useNavTransition();

  const dashboardList = useDashboardList();
  const list = dashboardList.data;
  const currentUser = useCurrentUser();

  // `:dashboardId`/`:teamId` are real URL path segments (not a query param
  // or hash fragment) — see the three `/dashboard...` routes in `App.tsx` —
  // so a link to a specific dashboard/team view is shareable/bookmarkable/
  // refresh-safe. `useParams` returns `undefined` for a segment this route
  // match didn't capture (e.g. the bare `/dashboard` route has neither), so
  // this never needs to parse anything by hand.
  const { dashboardId: urlDashboardId, teamId: urlTeamIdRaw } = useParams<{
    dashboardId?: string;
    teamId?: string;
  }>();

  const urlEntry = list?.find((d) => d.id === urlDashboardId);

  const userHasTeam = Boolean(currentUser.user?.team);
  // The preferred predicate per the user's own team membership: BOTH
  // isDefault and isTeamBased must match (not isTeamBased alone, and not
  // isDefault alone) — see the module doc comment above.
  //
  // Deliberately type-blind: `type` IS on `BeDashboardListItem` now (added
  // for the team picker's family filter, see abtFamilyForDashboardType in
  // useTeams.ts), but default-dashboard selection still isn't keyed off it.
  // The backend loader is held to ONE isDefault dashboard in total to match
  // — without that, a second typed default would be perfectly valid config
  // and which one a user landed on would come down to the backend's filename
  // ordering. Making this type-aware and loosening the loader to one default
  // per type are the same change; do not do either alone.
  const preferredEntry = userHasTeam
    ? list?.find((d) => d.isDefault && d.isTeamBased)
    : list?.find((d) => d.isDefault && !d.isTeamBased);
  // Fallback 1: the BE's own (any) isDefault entry, regardless of
  // isTeamBased — covers a registry that has no isDefault+isTeamBased (or
  // isDefault+!isTeamBased) combination configured at all.
  const anyDefaultEntry = list?.find((d) => d.isDefault);
  // Fallback 2: the first dashboard in the list — never render nothing just
  // because the registry has no isDefault entry configured.
  const firstEntry = list && list.length > 0 ? list[0] : undefined;
  // True only while we genuinely don't know yet whether this user has a
  // team — a failed profile fetch (isError) must not hang this forever, so
  // it falls straight through to the defaults below instead.
  const userProfilePending = currentUser.isLoading && !currentUser.isError;

  // The URL always wins when it names a dashboard actually in the loaded
  // list (stale/hand-edited hash falls through to the defaults below,
  // never crashes). Only when it doesn't do we need to pick a default —
  // and picking that default depends on the user's own team membership, so
  // hold off (skeleton) until that's resolved, unless it errored.
  let currentEntry = urlEntry;
  if (!currentEntry && list) {
    if (userProfilePending) {
      currentEntry = undefined;
    } else {
      currentEntry = preferredEntry ?? anyDefaultEntry ?? firstEntry;
    }
  }

  const dashboardKey = currentEntry?.id as DashboardKey | undefined;
  const isTeamBased = currentEntry?.isTeamBased ?? false;

  // Only apply a URL team id when the CURRENT dashboard is team-based — a
  // stale segment left over from a previously selected team-based dashboard
  // (or a hand-edited URL) must not leak into a non-team-based one.
  const urlTeamId = isTeamBased ? urlTeamIdRaw : undefined;
  // Default to the signed-in user's own team once their profile has
  // resolved, but only ever as a default: the moment the URL itself names a
  // team (including one written by the user's own pick — see
  // `handleTeamChange`), that value always wins over this one, so a manual
  // switch is never fought on re-render. A user with NO home team (or
  // whose profile hasn't resolved one yet) defaults to `ALL_TEAMS_SENTINEL`
  // ("All ABTs") rather than an empty selection — see `AbtDashboardHeader`.
  const defaultTeamId =
    isTeamBased && !urlTeamId
      ? userHasTeam
        ? currentUser.user?.team?.teamKey
        : ALL_TEAMS_SENTINEL
      : undefined;
  const selectedTeamId = urlTeamId ?? defaultTeamId;

  // Every team, unfiltered, for resolving the selected team's `groupId` (the
  // `__current_team__` filter placeholder's real value). Deliberately NOT
  // scoped to the current dashboard's family the way AbtDashboardHeader's own
  // picker query is (see abtFamilyForDashboardType): the signed-in user's own
  // team can be outside that family (e.g. a `cre` non-ABT team member viewing
  // a `cre` dashboard, whose picker only offers `cre-abt` teams), and
  // `defaultTeamId` still needs it resolved to a real groupId. A separate,
  // differently-scoped query from the header's — react-query no longer
  // dedupes these into one fetch.
  const teams = useTeams(isTeamBased);

  // "All ABTs" resolves to every team in the CURRENT DASHBOARD's own family
  // specifically (not the signed-in user's own team's family, which is what
  // the unscoped `teams` query above is for) — filtering the same
  // unscoped `teams.data` client-side by family, rather than firing a
  // second, family-scoped query, since `teams.data` already has every
  // team's `family` on it.
  const currentDashboardFamily = abtFamilyForDashboardType(currentEntry?.type);
  const allTeamsInFamilyGroupIds = useMemo(
    () =>
      (teams.data ?? [])
        .filter((t) => t.family === currentDashboardFamily)
        .map((t) => t.groupId)
        .filter((groupId): groupId is string => Boolean(groupId)),
    [teams.data, currentDashboardFamily],
  );

  const selectedTeam = teams.data?.find((t) => t.id === selectedTeamId);
  const selectedTeamGroupId: string | string[] | undefined =
    selectedTeamId === ALL_TEAMS_SENTINEL ? allTeamsInFamilyGroupIds : selectedTeam?.groupId;
  // Human-readable label for the selected team, threaded down for the
  // `{{currentTeam}}` widget text placeholder (see
  // `widgetTextPlaceholder.ts`) — never the opaque `groupId` above, which is
  // useless for display.
  const selectedTeamLabel: string | undefined =
    selectedTeamId === ALL_TEAMS_SENTINEL ? "All ABTs" : selectedTeam?.name;

  // Navigates to the real path for a dashboard/team selection —
  // `/dashboard/:dashboardId` with no team, `/dashboard/:dashboardId/:teamId`
  // with one — via `replace` so picking from the dropdown doesn't grow
  // history the way a genuine navigation should (same convention as
  // `usePathTabs.setActiveTab`).
  const writeSelection = useCallback(
    (nextDashboardId: string, nextTeamId: string | undefined) => {
      navigate(
        nextTeamId
          ? `/dashboard/${nextDashboardId}/${nextTeamId}`
          : `/dashboard/${nextDashboardId}`,
        { replace: true },
      );
    },
    [navigate],
  );

  const handleDashboardChange = useCallback(
    (key: DashboardKey) => {
      const nextEntry = list?.find((d) => d.id === key);
      // Switching to a dashboard that isn't team-based: clear any stale
      // team selection rather than leaving an inapplicable one in the URL.
      // Switching between two team-based dashboards keeps the current
      // selection instead of resetting it.
      const nextTeamId = nextEntry?.isTeamBased ? selectedTeamId : undefined;
      writeSelection(key, nextTeamId);
    },
    [list, selectedTeamId, writeSelection],
  );

  const handleTeamChange = useCallback(
    (teamId: string | undefined) => {
      if (!dashboardKey) return;
      writeSelection(dashboardKey, teamId);
    },
    [dashboardKey, writeSelection],
  );

  const dashboardListData = useMemo(() => dashboardList.data ?? [], [dashboardList.data]);

  if (dashboardList.isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Typography variant="h5">Dashboard</Typography>
        <Typography variant="body2" color="text.secondary">
          Could not load the dashboard list.
        </Typography>
      </Box>
    );
  }

  if (dashboardKey === undefined) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={200} />
      </Box>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AbtDashboardHeader
        dashboardKey={dashboardKey}
        onDashboardChange={handleDashboardChange}
        dashboardList={dashboardListData}
        selectedTeamId={selectedTeamId}
        onTeamChange={handleTeamChange}
      />
      <AgentsLandingPagePilot
        dashboardId={dashboardKey}
        selectedTeamGroupId={selectedTeamGroupId}
        selectedTeamLabel={selectedTeamLabel}
      />
    </Box>
  );
}
