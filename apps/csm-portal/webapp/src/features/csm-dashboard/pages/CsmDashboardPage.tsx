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
import { useCallback, useEffect, useMemo, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import AgentsLandingPagePilot from "@features/csm-dashboard/components/AgentsLandingPagePilot";
import { useDashboardList } from "@features/csm-dashboard/api/useDashboardList";
import {
  abtFamilyForDashboardType,
  dashboardTypeForTeamFamily,
  useTeams,
} from "@features/csm-dashboard/api/useTeams";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import type { DashboardKey } from "@features/csm-dashboard/types/abtDashboard";
import { ALL_TEAMS_SENTINEL } from "@features/csm-dashboard/utils/teamFilterPlaceholder";

/**
 * Top-level CSM dashboard. The dashboard list is BE-driven (`GET
 * /dashboards`), and the initial selection depends on the signed-in user's
 * own ABT team membership (`GET /users/me`'s `team`, via `useCurrentUser`)
 * through two independent tiers, checked in this order:
 *
 * 1. `defaultForTeamKeys` (see `BeDashboardListItem`): a dashboard can name
 *    the exact team keys that should land on it outright, regardless of its
 *    own `isDefault`/`isTeamBased`/`type` — for specialist, non-team-based
 *    dashboards tier 2 below can never reach (e.g. `onboarding-engineer` for
 *    a `customer_onboarding`-team user, `migration-engineer` for
 *    `cs_migrations_team`). A team key naming a dashboard id that isn't in
 *    the BE-loaded list (not yet registered, or a stale config entry) falls
 *    straight through to tier 2 rather than erroring — see
 *    `defaultForTeamKeyEntry` below.
 * 2. The `isDefault`/`isTeamBased`/`type` predicate (`preferredEntry`
 *    below): a user WITH a resolved team defaults to the dashboard with
 *    BOTH `isDefault` and `isTeamBased` set, further narrowed by `type` when
 *    the user's own team's `family` resolves to one (cre-abt/cre families to
 *    the `cre` type, sre-abt/sre to `sre` — see `dashboardTypeForTeamFamily`
 *    in `useTeams.ts`) — an unresolved family falls back to matching on
 *    `isDefault && isTeamBased` alone, so this tier never regresses to
 *    matching nothing. A user with no team defaults to the dashboard with
 *    `isDefault` set and `isTeamBased` NOT set.
 *
 * If neither tier matches, this falls back to the BE's own (any)
 * `isDefault` entry, then to the first dashboard in the list — never to an
 * empty selection. The URL always wins over all of the above when it names
 * a (valid) dashboard.
 *
 * The selection is a real path segment — `/dashboard/:dashboardId`, and for a
 * team-based dashboard `/dashboard/:dashboardId/:teamId` — rather than a
 * query param or fragment, matched by three sibling routes in App.tsx all
 * rendering this same page (bare `/dashboard`, `/dashboard/:dashboardId`,
 * `/dashboard/:dashboardId/:teamId`). Selecting a dashboard/team is a
 * genuinely different content set each time, not a same-page panel switch, so
 * it earns a path segment under this app's URL-shape rule rather than
 * `?tab=`. `writePath` below replaces the URL with the canonical one- or
 * two-segment path once the selection is known — on the bare `/dashboard`
 * entry that means the very first render after the defaults resolve, so a
 * refresh or share always lands on an explicit dashboard id, never the bare
 * index.
 *
 * Dashboards are selected purely by dropdown — there is no other
 * per-dashboard scoping control. Every dashboard in the registry has at
 * least one real (config-driven) widget, so this always renders the real
 * widget grid, via `AgentsLandingPagePilot`.
 *
 * The Wallboard-styled full-screen dashboard (matching `digiops-cs`'s
 * Wallboard.tsx — see `CS_Dashboard.png`) is NOT rendered here — it lives
 * at its own route, `/cs-monitor-dashboard`
 * (`CsMonitorDashboardPage.tsx`), outside this page and outside the
 * normal CSM Portal chrome entirely. This page always renders the
 * standard widget grid for every dashboard, cs-overview included.
 */

export default function CsmDashboardPage(): JSX.Element {
  const navigate = useNavigate();
  const { dashboardId: urlDashboardId, teamId: urlTeamIdRaw } = useParams<{
    dashboardId?: string;
    teamId?: string;
  }>();

  const dashboardList = useDashboardList();
  const list = dashboardList.data;
  const currentUser = useCurrentUser();

  const urlEntry = list?.find((d) => d.id === urlDashboardId);

  const userHasTeam = Boolean(currentUser.user?.team);
  // Every team, unfiltered — needed for three independent reasons: (1)
  // resolving the signed-in user's OWN team's `family`, to derive
  // `preferredDashboardType` below BEFORE the initial dashboard is even
  // picked; (2) resolving the selected team's `creGroupId`/`sreGroupId` (the
  // `__current_team__` filter placeholder's real values) once a dashboard
  // IS picked; (3) "All ABTs" family filtering further down. Deliberately
  // NOT scoped to any one family the way AbtDashboardHeader's own picker
  // query is (see abtFamilyForDashboardType): the signed-in user's own team
  // can be outside the current (or eventual) dashboard's family (e.g. a
  // `cre` non-ABT team member viewing a `cre` dashboard, whose picker only
  // offers `cre-abt` teams). Enabled unconditionally rather than gated on
  // the current dashboard's `isTeamBased` — that used to be fine because
  // this query was only needed once a dashboard was already selected, but
  // it's now also an input to selecting one, so it can't wait on a value it
  // helps produce. A separate, differently-scoped query from the header's —
  // react-query no longer dedupes these into one fetch. Cheap: a 5-minute
  // stale time and this list rarely changes mid-session.
  const teams = useTeams(true);

  // The user's own team's family, and the dashboard `type` it prefers (see
  // `dashboardTypeForTeamFamily`) — `undefined` for a user with no team, an
  // unresolved team (not yet in the teams list), or a family that doesn't
  // map to a known dashboard type.
  const userTeamFamily = teams.data?.find(
    (t) => t.id === currentUser.user?.team?.teamKey,
  )?.family;
  const preferredDashboardType = dashboardTypeForTeamFamily(userTeamFamily);

  // Tier 1 (see module doc comment): a dashboard naming the signed-in
  // user's own teamKey in its own `defaultForTeamKeys` wins outright over
  // the isDefault/isTeamBased/type-based `preferredEntry` below — but only
  // when that dashboard id is actually in the BE-loaded list; otherwise
  // this is `undefined` and falls through untouched.
  const defaultForTeamKeyEntry = currentUser.user?.team?.teamKey
    ? list?.find((d) => d.defaultForTeamKeys?.includes(currentUser.user!.team!.teamKey))
    : undefined;
  // Tier 2: the preferred predicate per the user's own team membership.
  // BOTH isDefault and isTeamBased must match (not isTeamBased alone, and
  // not isDefault alone) — see the module doc comment above — further
  // narrowed by `type` when the user's own team's family resolves to one.
  // An unresolved `preferredDashboardType` falls back to matching on
  // isDefault && isTeamBased alone, so this predicate must never regress to
  // matching nothing just because the type couldn't be resolved.
  //
  // The backend loader (registry.go's `validate`) still only permits ONE
  // isDefault dashboard in the WHOLE registry, not one per type — so in
  // today's real config this type check is inert (there is only ever one
  // isDefault+isTeamBased dashboard to begin with) until that validation is
  // loosened to one-per-type. It's added here now so this predicate
  // doesn't need a second frontend change when that lands.
  const preferredEntry = userHasTeam
    ? list?.find(
        (d) =>
          d.isDefault &&
          d.isTeamBased &&
          (preferredDashboardType ? d.type === preferredDashboardType : true),
      )
    : list?.find((d) => d.isDefault && !d.isTeamBased);
  // Fallback 1: the BE's own (any) isDefault entry, regardless of
  // isTeamBased — covers a registry that has no isDefault+isTeamBased (or
  // isDefault+!isTeamBased) combination configured at all.
  const anyDefaultEntry = list?.find((d) => d.isDefault);
  // Fallback 2: the first dashboard in the list — never render nothing just
  // because the registry has no isDefault entry configured.
  const firstEntry = list && list.length > 0 ? list[0] : undefined;
  // True only while we genuinely don't know yet whether this user has a
  // team, or (when they do) what that team's family is — a failed fetch
  // (isError) on EITHER query must not hang this forever, so it falls
  // straight through to the defaults below instead.
  const userProfilePending =
    (currentUser.isLoading || (userHasTeam && teams.isLoading)) &&
    !currentUser.isError &&
    !teams.isError;

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
      currentEntry = defaultForTeamKeyEntry ?? preferredEntry ?? anyDefaultEntry ?? firstEntry;
    }
  }

  const dashboardKey = currentEntry?.id as DashboardKey | undefined;
  const isTeamBased = currentEntry?.isTeamBased ?? false;

  // Only apply a URL team id when the CURRENT dashboard is team-based — a
  // stale suffix left over from a previously selected team-based dashboard
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

  // "All ABTs" resolves to every team in the CURRENT DASHBOARD's own family
  // specifically (not the signed-in user's own team's family, which is what
  // the unscoped `teams` query above is for) — filtering the same
  // unscoped `teams.data` client-side by family, rather than firing a
  // second, family-scoped query, since `teams.data` already has every
  // team's `family` on it.
  const currentDashboardFamily = abtFamilyForDashboardType(currentEntry?.type);
  const allTeamsInFamilyCreGroupIds = useMemo(
    () =>
      (teams.data ?? [])
        .filter((t) => t.family === currentDashboardFamily)
        .map((t) => t.creGroupId)
        .filter((groupId): groupId is string => Boolean(groupId)),
    [teams.data, currentDashboardFamily],
  );
  const allTeamsInFamilySreGroupIds = useMemo(
    () =>
      (teams.data ?? [])
        .filter((t) => t.family === currentDashboardFamily)
        .map((t) => t.sreGroupId)
        .filter((groupId): groupId is string => Boolean(groupId)),
    [teams.data, currentDashboardFamily],
  );

  const selectedTeam = teams.data?.find((t) => t.id === selectedTeamId);
  const selectedTeamCreGroupId: string | string[] | undefined =
    selectedTeamId === ALL_TEAMS_SENTINEL ? allTeamsInFamilyCreGroupIds : selectedTeam?.creGroupId;
  const selectedTeamSreGroupId: string | string[] | undefined =
    selectedTeamId === ALL_TEAMS_SENTINEL ? allTeamsInFamilySreGroupIds : selectedTeam?.sreGroupId;
  // Human-readable label for the selected team, threaded down for the
  // `{{currentTeam}}` widget text placeholder (see
  // `widgetTextPlaceholder.ts`) — never the opaque group ids above, which
  // are useless for display.
  const selectedTeamLabel: string | undefined =
    selectedTeamId === ALL_TEAMS_SENTINEL ? "All ABTs" : selectedTeam?.name;

  const writePath = useCallback(
    (nextDashboardId: string, nextTeamId: string | undefined) => {
      navigate(
        nextTeamId ? `/dashboard/${nextDashboardId}/${nextTeamId}` : `/dashboard/${nextDashboardId}`,
        { replace: true },
      );
    },
    [navigate],
  );

  // Canonicalizes the URL to the resolved selection whenever it doesn't
  // already match, so a refresh or a share always lands on an explicit
  // dashboard id rather than staying on a non-canonical URL — three cases
  // land here: a bare `/dashboard` that resolved to a default, an
  // invalid/unknown dashboard id in the URL that fell back to a valid one,
  // and a non-team dashboard's URL carrying a stale leftover team-id suffix
  // (see `urlTeamId` above, which drops it). Deliberately compares against
  // `urlTeamId`, not `selectedTeamId`: a user's own derived default team
  // (`defaultTeamId`) is never written here — only a team id already
  // present in the URL is preserved (or stripped, if it's stale) — so the
  // team selector stays "derived, until the user or a shared URL actually
  // names one" per the class doc comment above.
  useEffect(() => {
    if (!dashboardKey) return;
    const dashboardIdStale = urlDashboardId !== dashboardKey;
    const teamIdStale = urlTeamIdRaw !== urlTeamId;
    if (dashboardIdStale || teamIdStale) {
      writePath(dashboardKey, urlTeamId);
    }
  }, [dashboardKey, urlDashboardId, urlTeamId, urlTeamIdRaw, writePath]);

  const handleDashboardChange = useCallback(
    (key: DashboardKey) => {
      const nextEntry = list?.find((d) => d.id === key);
      // Switching to a dashboard that isn't team-based: clear any stale
      // team selection rather than leaving an inapplicable one in the URL.
      // Switching between two team-based dashboards keeps the current
      // selection instead of resetting it.
      const nextTeamId = nextEntry?.isTeamBased ? selectedTeamId : undefined;
      writePath(key, nextTeamId);
    },
    [list, selectedTeamId, writePath],
  );

  const handleTeamChange = useCallback(
    (teamId: string | undefined) => {
      if (!dashboardKey) return;
      writePath(dashboardKey, teamId);
    },
    [dashboardKey, writePath],
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
        selectedTeamCreGroupId={selectedTeamCreGroupId}
        selectedTeamSreGroupId={selectedTeamSreGroupId}
        selectedTeamLabel={selectedTeamLabel}
      />
    </Box>
  );
}
