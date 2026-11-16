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
import { useBackendApi } from "@api/backend/client";
import type {
  DashboardPerson,
  DashboardShare,
  DashboardShareTarget,
  SharedDashboardSummary,
  SharedViaTeam,
} from "@features/csm-custom-dashboards/api/useDashboardShares";
import type { DashboardTeamShare, DashboardTeamShareTarget } from "@features/csm-custom-dashboards/api/useDashboardTeamShares";
import type { DashboardLink, OpenedDashboardLink } from "@features/csm-custom-dashboards/api/useDashboardLink";
import type { DashboardShareHooks } from "@features/csm-custom-dashboards/components/ShareCustomDashboardDialog";
import { codeDashboardLinkUrl } from "@features/csm-custom-dashboards/utils/dashboardLinkUrl";

/*
 * Sharing saved advanced dashboards: the custom dashboard share, team share
 * and link hooks, with the same signatures and answers, on the backend's
 * /dashboards/code/{id}/... and /dashboards/shared/code routes. Those routes
 * exist only while DASHBOARD_GENERATE_CODE_ENABLED is on. The query keys are
 * their own, so nothing here touches the custom dashboards' cache.
 */

const SHARES_KEY = "csm-code-dashboard-shares";
const TEAM_SHARES_KEY = "csm-code-dashboard-team-shares";
const LINK_KEY = "csm-code-dashboard-link";
export const SHARED_CODE_DASHBOARD_LIST_KEY = "csm-shared-code-dashboard-list";
const SHARED_CODE_DASHBOARD_DETAIL_KEY = "csm-shared-code-dashboard-detail";

/**
 * One advanced dashboard shared with the caller (`GET /dashboards/shared/code/{id}`):
 * the owner's current source and who shared it. The webapp checks the source
 * and runs it in its sandbox, as for the caller's own.
 */
export interface SharedCodeDashboard {
  id: string;
  name: string;
  source: string;
  /** Who shared it (its owner); `null` once that user has been deleted. */
  owner: DashboardPerson | null;
  /** The caller's team it was shared with; `null` when shared with the caller directly or only by link. */
  sharedViaTeam: SharedViaTeam | null;
  /** `true` when the caller has it only because they opened its link. */
  sharedViaLink: boolean;
  sharedOn: string;
  updatedOn: string;
}

/** Where an advanced dashboard shared with the user opens: `/dashboard/shared/code/:dashboardId` (SharedDashboardsPage). */
export function sharedCodeDashboardPath(dashboardId: string): string {
  return `/dashboard/shared/code/${encodeURIComponent(dashboardId)}`;
}

function ownPath(dashboardId: string): string {
  return `/dashboards/code/${encodeURIComponent(dashboardId)}`;
}

/** Who one of the caller's advanced dashboards is shared with; `null` when it is not the caller's. */
export function useGetCodeDashboardShares(dashboardId: string | undefined): UseQueryResult<DashboardShare[] | null, Error> {
  const api = useBackendApi();
  return useQuery<DashboardShare[] | null, Error>({
    queryKey: [SHARES_KEY, dashboardId ?? ""],
    queryFn: async (): Promise<DashboardShare[] | null> => {
      if (!dashboardId) return null;
      const res = await api.get<{ shares: DashboardShare[] }>(`${ownPath(dashboardId)}/shares`);
      return res === null ? null : res.shares;
    },
    enabled: dashboardId !== undefined,
  });
}

/** Shares one of the caller's advanced dashboards with a user; the answer replaces the cached list. */
export function usePutCodeDashboardShare(): UseMutationResult<DashboardShare[], Error, DashboardShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<DashboardShare[], Error, DashboardShareTarget>({
    mutationFn: async ({ dashboardId, userId }: DashboardShareTarget): Promise<DashboardShare[]> =>
      (
        await api.put<Record<string, never>, { shares: DashboardShare[] }>(
          `${ownPath(dashboardId)}/shares/${encodeURIComponent(userId)}`,
          {},
        )
      ).shares,
    onSuccess: (shares, target) => {
      qc.setQueryData([SHARES_KEY, target.dashboardId], shares);
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: [SHARES_KEY, target.dashboardId] });
    },
  });
}

/** Stops sharing one of the caller's advanced dashboards with a user; they leave the cached list at once. */
export function useDeleteCodeDashboardShare(): UseMutationResult<null, Error, DashboardShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, DashboardShareTarget>({
    mutationFn: ({ dashboardId, userId }: DashboardShareTarget) =>
      api.del<null>(`${ownPath(dashboardId)}/shares/${encodeURIComponent(userId)}`),
    onSuccess: (_, target) => {
      qc.setQueryData<DashboardShare[] | null>([SHARES_KEY, target.dashboardId], (shares) =>
        shares?.filter((s) => s.user.id !== target.userId),
      );
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: [SHARES_KEY, target.dashboardId] });
    },
  });
}

/** The teams one of the caller's advanced dashboards is shared with; `null` when it is not the caller's. */
export function useGetCodeDashboardTeamShares(
  dashboardId: string | undefined,
): UseQueryResult<DashboardTeamShare[] | null, Error> {
  const api = useBackendApi();
  return useQuery<DashboardTeamShare[] | null, Error>({
    queryKey: [TEAM_SHARES_KEY, dashboardId ?? ""],
    queryFn: async (): Promise<DashboardTeamShare[] | null> => {
      if (!dashboardId) return null;
      const res = await api.get<{ shares: DashboardTeamShare[] }>(`${ownPath(dashboardId)}/team-shares`);
      return res === null ? null : res.shares;
    },
    enabled: dashboardId !== undefined,
  });
}

/** Shares one of the caller's advanced dashboards with a registry team; the answer replaces the cached list. */
export function usePutCodeDashboardTeamShare(): UseMutationResult<DashboardTeamShare[], Error, DashboardTeamShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<DashboardTeamShare[], Error, DashboardTeamShareTarget>({
    mutationFn: async ({ dashboardId, teamKey }: DashboardTeamShareTarget): Promise<DashboardTeamShare[]> =>
      (
        await api.put<Record<string, never>, { shares: DashboardTeamShare[] }>(
          `${ownPath(dashboardId)}/team-shares/${encodeURIComponent(teamKey)}`,
          {},
        )
      ).shares,
    onSuccess: (shares, target) => {
      qc.setQueryData([TEAM_SHARES_KEY, target.dashboardId], shares);
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: [TEAM_SHARES_KEY, target.dashboardId] });
    },
  });
}

/** Stops sharing one of the caller's advanced dashboards with a team; it leaves the cached list at once. */
export function useDeleteCodeDashboardTeamShare(): UseMutationResult<null, Error, DashboardTeamShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, DashboardTeamShareTarget>({
    mutationFn: ({ dashboardId, teamKey }: DashboardTeamShareTarget) =>
      api.del<null>(`${ownPath(dashboardId)}/team-shares/${encodeURIComponent(teamKey)}`),
    onSuccess: (_, target) => {
      qc.setQueryData<DashboardTeamShare[] | null>([TEAM_SHARES_KEY, target.dashboardId], (shares) =>
        shares?.filter((s) => s.team.key !== target.teamKey),
      );
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: [TEAM_SHARES_KEY, target.dashboardId] });
    },
  });
}

/** The link of one of the caller's advanced dashboards; `null` when it is not the caller's. */
export function useGetCodeDashboardLink(dashboardId: string | undefined): UseQueryResult<DashboardLink | null, Error> {
  const api = useBackendApi();
  return useQuery<DashboardLink | null, Error>({
    queryKey: [LINK_KEY, dashboardId ?? ""],
    queryFn: async (): Promise<DashboardLink | null> => {
      if (!dashboardId) return null;
      return api.get<DashboardLink>(`${ownPath(dashboardId)}/link`);
    },
    enabled: dashboardId !== undefined,
  });
}

/** Turns on the link of one of the caller's advanced dashboards (one that is on keeps its id). */
export function usePutCodeDashboardLink(): UseMutationResult<DashboardLink, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<DashboardLink, Error, string>({
    mutationFn: (dashboardId: string) => api.put<Record<string, never>, DashboardLink>(`${ownPath(dashboardId)}/link`, {}),
    onSuccess: (link, dashboardId) => {
      qc.setQueryData([LINK_KEY, dashboardId], link);
    },
    onSettled: (_data, _err, dashboardId) => {
      void qc.invalidateQueries({ queryKey: [LINK_KEY, dashboardId] });
    },
  });
}

/** Turns off the link of one of the caller's advanced dashboards; the cached link reads as off at once. */
export function useDeleteCodeDashboardLink(): UseMutationResult<null, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, string>({
    mutationFn: (dashboardId: string) => api.del<null>(`${ownPath(dashboardId)}/link`),
    onSuccess: (_, dashboardId) => {
      qc.setQueryData<DashboardLink>([LINK_KEY, dashboardId], { enabled: false, openedBy: 0 });
    },
    onSettled: (_data, _err, dashboardId) => {
      void qc.invalidateQueries({ queryKey: [LINK_KEY, dashboardId] });
    },
  });
}

/**
 * Opens an advanced dashboard's link (the id from the link, sent in the body).
 * On success the shared advanced dashboards are read again.
 */
export function useOpenCodeDashboardLink(): UseMutationResult<OpenedDashboardLink, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<OpenedDashboardLink, Error, string>({
    mutationFn: (linkId: string) =>
      api.post<{ linkId: string }, OpenedDashboardLink>("/dashboards/shared/code/link", { linkId }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [SHARED_CODE_DASHBOARD_LIST_KEY] });
    },
  });
}

/**
 * The advanced dashboards other users shared with the caller, ordered by name.
 * Pass `enabled: false` while the caller cannot view advanced dashboards (the
 * route is missing then).
 */
export function useGetSharedCodeDashboards(enabled = true): UseQueryResult<SharedDashboardSummary[], Error> {
  const api = useBackendApi();
  return useQuery<SharedDashboardSummary[], Error>({
    queryKey: [SHARED_CODE_DASHBOARD_LIST_KEY],
    queryFn: async (): Promise<SharedDashboardSummary[]> => {
      const res = await api.get<{ dashboards: SharedDashboardSummary[] }>("/dashboards/shared/code");
      // The list route always answers 200, an empty list included; a null
      // (404) means the route itself is missing, not "nothing shared".
      if (res === null) {
        throw new Error("GET /dashboards/shared/code returned 404");
      }
      return res.dashboards;
    },
    enabled,
    staleTime: 30_000,
  });
}

/** One advanced dashboard shared with the caller, or `null` when it is not. Disabled while `id` is `undefined`. */
export function useGetSharedCodeDashboard(id: string | undefined): UseQueryResult<SharedCodeDashboard | null, Error> {
  const api = useBackendApi();
  return useQuery<SharedCodeDashboard | null, Error>({
    queryKey: [SHARED_CODE_DASHBOARD_DETAIL_KEY, id ?? ""],
    queryFn: async (): Promise<SharedCodeDashboard | null> => {
      if (!id) return null;
      return api.get<SharedCodeDashboard>(`/dashboards/shared/code/${encodeURIComponent(id)}`);
    },
    enabled: id !== undefined,
  });
}

/**
 * The Share dialog's hooks for an advanced dashboard: people, teams and the
 * link, with the link at its own path (`/dashboard/shared/code-link/<linkId>`).
 */
export const CODE_DASHBOARD_SHARING: DashboardShareHooks = {
  useGetShares: useGetCodeDashboardShares,
  usePutShare: usePutCodeDashboardShare,
  useDeleteShare: useDeleteCodeDashboardShare,
  useGetTeamShares: useGetCodeDashboardTeamShares,
  usePutTeamShare: usePutCodeDashboardTeamShare,
  useDeleteTeamShare: useDeleteCodeDashboardTeamShare,
  linkHooks: {
    useGetLink: useGetCodeDashboardLink,
    usePutLink: usePutCodeDashboardLink,
    useDeleteLink: useDeleteCodeDashboardLink,
    linkUrl: codeDashboardLinkUrl,
  },
};
