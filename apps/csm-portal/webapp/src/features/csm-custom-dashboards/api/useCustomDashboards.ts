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

const BASE = "/dashboards/custom";

/** One entry of the caller's saved dashboards (`GET /dashboards/custom`): no widgets. */
export interface CustomDashboardSummary {
  id: string;
  name: string;
  createdOn: string;
  updatedOn: string;
}

interface CustomDashboardList {
  dashboards: CustomDashboardSummary[];
}

/**
 * A saved dashboard (`GET` / `PUT /dashboards/custom/{id}`): the dashboard in
 * the same shape as `GET /dashboards/{id}`, so it is drawn with the same
 * components, and when it was saved.
 */
export interface SavedCustomDashboard {
  dashboard: BeDashboard;
  createdOn: string;
  updatedOn: string;
}

function detailPath(id: string): string {
  return `${BASE}/${encodeURIComponent(id)}`;
}

/**
 * The caller's saved dashboards for the My Dashboards dropdown, ordered by
 * name. At most 50 per user, so one request holds them all.
 */
export function useGetCustomDashboards(): UseQueryResult<CustomDashboardSummary[], Error> {
  const api = useBackendApi();
  return useQuery<CustomDashboardSummary[], Error>({
    queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST],
    queryFn: async (): Promise<CustomDashboardSummary[]> => {
      const res = await api.get<CustomDashboardList>(BASE);
      // The list route always answers 200, an empty list included; a null
      // (404) means the route itself is missing, not "nothing saved".
      if (res === null) {
        throw new Error("GET /dashboards/custom returned 404");
      }
      return res.dashboards;
    },
    staleTime: 30_000,
  });
}

/**
 * One saved dashboard, or `null` when the caller has none with this id
 * (deleted, or another user's). A dashboard saved under rules that have since
 * changed is an error (422) whose message asks the user to generate it again.
 * Disabled while `id` is `undefined`.
 */
export function useGetCustomDashboard(
  id: string | undefined,
): UseQueryResult<SavedCustomDashboard | null, Error> {
  const api = useBackendApi();
  return useQuery<SavedCustomDashboard | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, id ?? ""],
    queryFn: async (): Promise<SavedCustomDashboard | null> => {
      if (!id) return null;
      return api.get<SavedCustomDashboard>(detailPath(id));
    },
    enabled: id !== undefined,
  });
}

/**
 * Saves a dashboard to My Dashboards under its own id: creates it, or replaces
 * it when the caller already saved one with that id. The name is the
 * dashboard's `displayName`. A name the caller already uses (409) and the
 * 50-dashboard limit (400) are thrown with the backend's message.
 */
export function usePutCustomDashboard(): UseMutationResult<SavedCustomDashboard, Error, BeDashboard> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SavedCustomDashboard, Error, BeDashboard>({
    mutationFn: (dashboard: BeDashboard) =>
      api.put<{ dashboard: BeDashboard }, SavedCustomDashboard>(detailPath(dashboard.id), { dashboard }),
    onSuccess: (saved: SavedCustomDashboard) => {
      qc.setQueryData([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, saved.dashboard.id], saved);
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    },
  });
}

/** Deletes one of the caller's saved dashboards, by id; it leaves the cached list at once. */
export function useDeleteCustomDashboard(): UseMutationResult<null, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, string>({
    mutationFn: (id: string) => api.del<null>(detailPath(id)),
    onSuccess: (_: null, id: string) => {
      qc.removeQueries({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_DETAIL, id] });
      // Drop it from the cached list now, so a page reading the list before
      // the re-read lands never opens the dashboard that is gone.
      qc.setQueryData<CustomDashboardSummary[]>([ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST], (list) =>
        list?.filter((d) => d.id !== id),
      );
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LIST] });
    },
  });
}
