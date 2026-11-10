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
 * Whether anyone internal with a saved dashboard's link may view it, as its
 * owner sees it. `linkId` is the id in the link, absent while the link is off
 * (a new one each time it is turned on); `openedBy` is how many people opened
 * it since then. Who they are is never listed.
 */
export interface DashboardLink {
  enabled: boolean;
  linkId?: string;
  openedBy: number;
}

/**
 * The dashboard a link opened. `own` is `true` when the caller owns it (open
 * it under My Dashboards); otherwise it is under Shared Dashboards.
 */
export interface OpenedDashboardLink {
  dashboardId: string;
  own: boolean;
}

function linkPath(dashboardId: string): string {
  return `/dashboards/custom/${encodeURIComponent(dashboardId)}/link`;
}

function linkKey(dashboardId: string): unknown[] {
  return [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LINK, dashboardId];
}

/**
 * The link of one of the caller's dashboards; `null` when the caller has no
 * dashboard with this id. Disabled while `dashboardId` is `undefined` (e.g.
 * until the Share dialog opens).
 */
export function useGetCustomDashboardLink(
  dashboardId: string | undefined,
): UseQueryResult<DashboardLink | null, Error> {
  const api = useBackendApi();
  return useQuery<DashboardLink | null, Error>({
    queryKey: linkKey(dashboardId ?? ""),
    queryFn: async (): Promise<DashboardLink | null> => {
      if (!dashboardId) return null;
      return api.get<DashboardLink>(linkPath(dashboardId));
    },
    enabled: dashboardId !== undefined,
  });
}

/**
 * Turns on the link of one of the caller's dashboards. A link that is already
 * on keeps its id, so copies already sent still work. The answer replaces the
 * cached link.
 */
export function usePutCustomDashboardLink(): UseMutationResult<DashboardLink, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<DashboardLink, Error, string>({
    mutationFn: (dashboardId: string) =>
      api.put<Record<string, never>, DashboardLink>(linkPath(dashboardId), {}),
    onSuccess: (link: DashboardLink, dashboardId: string) => {
      qc.setQueryData(linkKey(dashboardId), link);
    },
    onSettled: (_data, _err, dashboardId) => {
      void qc.invalidateQueries({ queryKey: linkKey(dashboardId) });
    },
  });
}

/**
 * Turns off the link of one of the caller's dashboards: the link stops
 * working and everyone who had the dashboard only through it loses it. The
 * cached link reads as off at once.
 */
export function useDeleteCustomDashboardLink(): UseMutationResult<null, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, string>({
    mutationFn: (dashboardId: string) => api.del<null>(linkPath(dashboardId)),
    onSuccess: (_: null, dashboardId: string) => {
      qc.setQueryData<DashboardLink>(linkKey(dashboardId), { enabled: false, openedBy: 0 });
    },
    onSettled: (_data, _err, dashboardId) => {
      void qc.invalidateQueries({ queryKey: linkKey(dashboardId) });
    },
  });
}

/**
 * Opens a dashboard's link (the id from the link). The link id goes in the
 * body, not the path. A link that is off or unknown is an error whose message
 * says so. On success the Shared Dashboards list is read again, since the
 * dashboard may now be in it.
 */
export function useOpenDashboardLink(): UseMutationResult<OpenedDashboardLink, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<OpenedDashboardLink, Error, string>({
    mutationFn: (linkId: string) =>
      api.post<{ linkId: string }, OpenedDashboardLink>("/dashboards/shared/link", { linkId }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_SHARED_DASHBOARD_LIST] });
    },
  });
}
