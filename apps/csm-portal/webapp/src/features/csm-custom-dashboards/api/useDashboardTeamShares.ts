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
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";

/**
 * A team a dashboard is shared with: its registry key (what the team-share
 * routes take), name and family (`cre-abt`, `cre`, `sre-abt`, ...). `key` is
 * empty for a team no longer in the team registry.
 */
export interface DashboardShareTeam {
  key: string;
  name: string;
  family?: string;
}

/** One team a saved dashboard is shared with (view only), as its owner sees it. */
export interface DashboardTeamShare {
  team: DashboardShareTeam;
  sharedOn: string;
}

interface DashboardTeamShareList {
  shares: DashboardTeamShare[];
}

/** Which dashboard to share, and with which team (a registry team key, e.g. `rigel`). */
export interface DashboardTeamShareTarget {
  dashboardId: string;
  teamKey: string;
}

function teamSharesPath(dashboardId: string): string {
  return `/dashboards/custom/${encodeURIComponent(dashboardId)}/team-shares`;
}

function teamSharePath({ dashboardId, teamKey }: DashboardTeamShareTarget): string {
  return `${teamSharesPath(dashboardId)}/${encodeURIComponent(teamKey)}`;
}

function teamSharesKey(dashboardId: string): unknown[] {
  return [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_TEAM_SHARES, dashboardId];
}

/**
 * The teams one of the caller's dashboards is shared with, ordered by name;
 * `null` when the caller has no dashboard with this id. Disabled while
 * `dashboardId` is `undefined` (e.g. until the Share dialog opens).
 */
export function useGetCustomDashboardTeamShares(
  dashboardId: string | undefined,
): UseQueryResult<DashboardTeamShare[] | null, Error> {
  const api = useBackendApi();
  return useQuery<DashboardTeamShare[] | null, Error>({
    queryKey: teamSharesKey(dashboardId ?? ""),
    queryFn: async (): Promise<DashboardTeamShare[] | null> => {
      if (!dashboardId) return null;
      const res = await api.get<DashboardTeamShareList>(teamSharesPath(dashboardId));
      return res === null ? null : res.shares;
    },
    enabled: dashboardId !== undefined,
  });
}

/**
 * Shares one of the caller's dashboards with a team, view only; sharing again
 * is not an error. Whoever is in the team sees it under Shared Dashboards. The
 * answer is the dashboard's team shares after the change, which replace the
 * cached list.
 */
export function usePutCustomDashboardTeamShare(): UseMutationResult<
  DashboardTeamShare[],
  Error,
  DashboardTeamShareTarget
> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<DashboardTeamShare[], Error, DashboardTeamShareTarget>({
    mutationFn: async (target: DashboardTeamShareTarget): Promise<DashboardTeamShare[]> =>
      (await api.put<Record<string, never>, DashboardTeamShareList>(teamSharePath(target), {})).shares,
    onSuccess: (shares: DashboardTeamShare[], target: DashboardTeamShareTarget) => {
      qc.setQueryData(teamSharesKey(target.dashboardId), shares);
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: teamSharesKey(target.dashboardId) });
    },
  });
}

/** Stops sharing one of the caller's dashboards with a team; it leaves the cached list at once. */
export function useDeleteCustomDashboardTeamShare(): UseMutationResult<null, Error, DashboardTeamShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, DashboardTeamShareTarget>({
    mutationFn: (target: DashboardTeamShareTarget) => api.del<null>(teamSharePath(target)),
    onSuccess: (_: null, target: DashboardTeamShareTarget) => {
      qc.setQueryData<DashboardTeamShare[] | null>(teamSharesKey(target.dashboardId), (shares) =>
        shares?.filter((s) => s.team.key !== target.teamKey),
      );
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: teamSharesKey(target.dashboardId) });
    },
  });
}
