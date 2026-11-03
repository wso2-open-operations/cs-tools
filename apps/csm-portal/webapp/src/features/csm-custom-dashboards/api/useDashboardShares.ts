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
import type { BeDashboard } from "@api/backend/types";

/** A person on a dashboard share: the recipient (owner's view) or the owner (recipient's view). */
export interface DashboardPerson {
  id: string | null;
  email: string;
  name: string;
}

/** One person a saved dashboard is shared with (view only), as its owner sees it. */
export interface DashboardShare {
  user: DashboardPerson;
  sharedOn: string;
}

interface DashboardShareList {
  shares: DashboardShare[];
}

/**
 * The caller's team a dashboard was shared with (entity-service's team id,
 * name and type, e.g. `cre-abt`).
 */
export interface SharedViaTeam {
  id: string;
  name: string;
  type: string;
}

/** One dashboard another user shared with the caller (`GET /dashboards/shared`): no widgets. */
export interface SharedDashboardSummary {
  id: string;
  name: string;
  /** Who shared it (its owner); `null` once that user has been deleted. */
  owner: DashboardPerson | null;
  /** The caller's team it was shared with; `null` when shared with the caller directly. */
  sharedViaTeam: SharedViaTeam | null;
  sharedOn: string;
  /** When the owner last changed it. */
  updatedOn: string;
}

interface SharedDashboardList {
  dashboards: SharedDashboardSummary[];
}

/**
 * One dashboard shared with the caller (`GET /dashboards/shared/{id}`): the
 * owner's current version, in the same shape as `GET /dashboards/{id}`, so it
 * is drawn with the same components, and who shared it.
 */
export interface SharedDashboard {
  dashboard: BeDashboard;
  /** Who shared it (its owner); `null` once that user has been deleted. */
  owner: DashboardPerson | null;
  /** The caller's team it was shared with; `null` when shared with the caller directly. */
  sharedViaTeam: SharedViaTeam | null;
  sharedOn: string;
  updatedOn: string;
}

/** Which dashboard to share, and with whom (a user id from the user search). */
export interface DashboardShareTarget {
  dashboardId: string;
  userId: string;
}

function sharesPath(dashboardId: string): string {
  return `/dashboards/custom/${encodeURIComponent(dashboardId)}/shares`;
}

function sharePath({ dashboardId, userId }: DashboardShareTarget): string {
  return `${sharesPath(dashboardId)}/${encodeURIComponent(userId)}`;
}

function sharesKey(dashboardId: string): unknown[] {
  return [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_SHARES, dashboardId];
}

/**
 * Who one of the caller's dashboards is shared with, ordered by name; `null`
 * when the caller has no dashboard with this id. Disabled while `dashboardId`
 * is `undefined` (e.g. until the Share dialog opens).
 */
export function useGetCustomDashboardShares(
  dashboardId: string | undefined,
): UseQueryResult<DashboardShare[] | null, Error> {
  const api = useBackendApi();
  return useQuery<DashboardShare[] | null, Error>({
    queryKey: sharesKey(dashboardId ?? ""),
    queryFn: async (): Promise<DashboardShare[] | null> => {
      if (!dashboardId) return null;
      const res = await api.get<DashboardShareList>(sharesPath(dashboardId));
      return res === null ? null : res.shares;
    },
    enabled: dashboardId !== undefined,
  });
}

/**
 * Shares one of the caller's dashboards with a user, view only; sharing again
 * is not an error. The answer is the dashboard's shares after the change,
 * which replace the cached list. The user being the caller, or not internal
 * staff, is thrown with the backend's message.
 */
export function usePutCustomDashboardShare(): UseMutationResult<DashboardShare[], Error, DashboardShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<DashboardShare[], Error, DashboardShareTarget>({
    mutationFn: async (target: DashboardShareTarget): Promise<DashboardShare[]> =>
      (await api.put<Record<string, never>, DashboardShareList>(sharePath(target), {})).shares,
    onSuccess: (shares: DashboardShare[], target: DashboardShareTarget) => {
      qc.setQueryData(sharesKey(target.dashboardId), shares);
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: sharesKey(target.dashboardId) });
    },
  });
}

/** Stops sharing one of the caller's dashboards with a user; they leave the cached list at once. */
export function useDeleteCustomDashboardShare(): UseMutationResult<null, Error, DashboardShareTarget> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, DashboardShareTarget>({
    mutationFn: (target: DashboardShareTarget) => api.del<null>(sharePath(target)),
    onSuccess: (_: null, target: DashboardShareTarget) => {
      qc.setQueryData<DashboardShare[] | null>(sharesKey(target.dashboardId), (shares) =>
        shares?.filter((s) => s.user.id !== target.userId),
      );
    },
    onSettled: (_data, _err, target) => {
      void qc.invalidateQueries({ queryKey: sharesKey(target.dashboardId) });
    },
  });
}

/**
 * The dashboards other users shared with the caller, for the Shared
 * Dashboards switcher, ordered by name.
 */
export function useGetSharedDashboards(): UseQueryResult<SharedDashboardSummary[], Error> {
  const api = useBackendApi();
  return useQuery<SharedDashboardSummary[], Error>({
    queryKey: [ApiQueryKeys.CSM_SHARED_DASHBOARD_LIST],
    queryFn: async (): Promise<SharedDashboardSummary[]> => {
      const res = await api.get<SharedDashboardList>("/dashboards/shared");
      // The list route always answers 200, an empty list included; a null
      // (404) means the route itself is missing, not "nothing shared".
      if (res === null) {
        throw new Error("GET /dashboards/shared returned 404");
      }
      return res.dashboards;
    },
    staleTime: 30_000,
  });
}

/**
 * One dashboard shared with the caller, or `null` when it is not (unshared,
 * deleted, or never shared). One saved under rules that have since changed is
 * an error (422) whose message asks to have its owner update it. Disabled
 * while `id` is `undefined`.
 */
export function useGetSharedDashboard(id: string | undefined): UseQueryResult<SharedDashboard | null, Error> {
  const api = useBackendApi();
  return useQuery<SharedDashboard | null, Error>({
    queryKey: [ApiQueryKeys.CSM_SHARED_DASHBOARD_DETAIL, id ?? ""],
    queryFn: async (): Promise<SharedDashboard | null> => {
      if (!id) return null;
      return api.get<SharedDashboard>(`/dashboards/shared/${encodeURIComponent(id)}`);
    },
    enabled: id !== undefined,
  });
}
