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
import { BackendApiError, useBackendApi } from "@api/backend/client";

const BASE = "/dashboards/code";

/**
 * Query keys of the saved advanced dashboards. Kept here rather than in
 * ApiQueryKeys: only these hooks read or write them.
 */
export const CODE_DASHBOARD_LIST_KEY = "csm-code-dashboard-list";
export const CODE_DASHBOARD_DETAIL_KEY = "csm-code-dashboard-detail";

/** One entry of the caller's saved advanced dashboards (`GET /dashboards/code`): no source. */
export interface CodeDashboardSummary {
  id: string;
  name: string;
  createdOn: string;
  updatedOn: string;
}

interface CodeDashboardList {
  dashboards: CodeDashboardSummary[];
}

/** A saved advanced dashboard (`GET` / `PUT /dashboards/code/{id}`): its name and its source. */
export interface SavedCodeDashboard {
  id: string;
  name: string;
  source: string;
  createdOn: string;
  updatedOn: string;
}

/** What `usePutCodeDashboard` saves: the draft's id, the name the user gave it and its source. */
export interface SaveCodeDashboard {
  id: string;
  name: string;
  source: string;
}

/** What `useRenameCodeDashboard` takes: the saved advanced dashboard's id and its new name. */
export interface RenameCodeDashboard {
  id: string;
  name: string;
}

/** Where a saved advanced dashboard opens in the webapp: `/dashboard/my/code/:codeDashboardId` (MyDashboardsPage). */
export function savedCodeDashboardPath(id: string): string {
  return `/dashboard/my/code/${encodeURIComponent(id)}`;
}

function detailPath(id: string): string {
  return `${BASE}/${encodeURIComponent(id)}`;
}

/**
 * The caller's saved advanced dashboards, ordered by name. At most 50 per
 * user, so one request holds them all. The routes exist only while
 * DASHBOARD_GENERATE_CODE_ENABLED is on, so pass `enabled: false` for a user
 * who cannot use advanced dashboards.
 */
export function useGetCodeDashboards(enabled = true): UseQueryResult<CodeDashboardSummary[], Error> {
  const api = useBackendApi();
  return useQuery<CodeDashboardSummary[], Error>({
    queryKey: [CODE_DASHBOARD_LIST_KEY],
    queryFn: async (): Promise<CodeDashboardSummary[]> => {
      const res = await api.get<CodeDashboardList>(BASE);
      // The list route always answers 200, an empty list included; a null
      // (404) means the route itself is missing, not "nothing saved".
      if (res === null) {
        throw new Error("GET /dashboards/code returned 404");
      }
      return res.dashboards;
    },
    enabled,
    staleTime: 30_000,
  });
}

/**
 * One saved advanced dashboard, or `null` when the caller has none with this
 * id (deleted, or another user's). Disabled while `id` is `undefined`.
 */
export function useGetCodeDashboard(id: string | undefined): UseQueryResult<SavedCodeDashboard | null, Error> {
  const api = useBackendApi();
  return useQuery<SavedCodeDashboard | null, Error>({
    queryKey: [CODE_DASHBOARD_DETAIL_KEY, id ?? ""],
    queryFn: async (): Promise<SavedCodeDashboard | null> => {
      if (!id) return null;
      return api.get<SavedCodeDashboard>(detailPath(id));
    },
    enabled: id !== undefined,
  });
}

/**
 * Saves an advanced dashboard under its draft's id: creates it, or replaces
 * the caller's own one with that id. A name the caller already uses (409),
 * the 50-dashboard limit (400) and code over 60000 characters (413) are
 * thrown with the backend's message.
 */
export function usePutCodeDashboard(): UseMutationResult<SavedCodeDashboard, Error, SaveCodeDashboard> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SavedCodeDashboard, Error, SaveCodeDashboard>({
    mutationFn: ({ id, name, source }: SaveCodeDashboard) =>
      api.put<{ name: string; source: string }, SavedCodeDashboard>(detailPath(id), { name, source }),
    onSuccess: (saved: SavedCodeDashboard) => {
      qc.setQueryData([CODE_DASHBOARD_DETAIL_KEY, saved.id], saved);
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    },
  });
}

/**
 * Renames one of the caller's saved advanced dashboards, as
 * useRenameCustomDashboard does: reads it and saves it back under the same id
 * with the new name and its current source, so its shares, link and creation
 * time stay as they are. A dashboard that is gone is a 404 and a name the
 * caller already uses is the save's 409, each thrown as a BackendApiError.
 */
export function useRenameCodeDashboard(): UseMutationResult<SavedCodeDashboard, Error, RenameCodeDashboard> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<SavedCodeDashboard, Error, RenameCodeDashboard>({
    mutationFn: async ({ id, name }: RenameCodeDashboard) => {
      const current = await api.get<SavedCodeDashboard>(detailPath(id));
      if (current === null) {
        throw new BackendApiError(404, "this dashboard no longer exists");
      }
      return api.put<{ name: string; source: string }, SavedCodeDashboard>(detailPath(id), {
        name,
        source: current.source,
      });
    },
    onSuccess: (saved: SavedCodeDashboard) => {
      qc.setQueryData([CODE_DASHBOARD_DETAIL_KEY, saved.id], saved);
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    },
  });
}

/** Deletes one of the caller's saved advanced dashboards, by id; it leaves the cached list at once. */
export function useDeleteCodeDashboard(): UseMutationResult<null, Error, string> {
  const api = useBackendApi();
  const qc = useQueryClient();
  return useMutation<null, Error, string>({
    mutationFn: (id: string) => api.del<null>(detailPath(id)),
    onSuccess: (_: null, id: string) => {
      qc.removeQueries({ queryKey: [CODE_DASHBOARD_DETAIL_KEY, id] });
      qc.setQueryData<CodeDashboardSummary[]>([CODE_DASHBOARD_LIST_KEY], (list) => list?.filter((d) => d.id !== id));
    },
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: [CODE_DASHBOARD_LIST_KEY] });
    },
  });
}
