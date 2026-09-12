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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeDashboardFilterPreset,
  BeDashboardSharedSection,
} from "@api/backend/types";

/**
 * The two shared-config catalogues the dashboard builder authors against.
 *
 * Both are deployment configuration, not user data: a maintainer deploys
 * `_presets.json`/`_sections.json` alongside the dashboard definitions, and
 * neither has a write API (same story as the dashboards themselves — see
 * `dashboardDraftsStorage`). These queries are read-only and exist so the
 * builder can show what already exists to reference, and seed an edit of it.
 *
 * A deployment with neither file configured returns `[]` from both endpoints,
 * which is a legitimate state and not an error — the builder then simply
 * offers nothing to reference.
 */

/** Long stale time: this is deployment config that only changes on a
 * redeploy, so re-fetching it while an admin edits a draft is pure noise. */
const SHARED_CONFIG_STALE_TIME_MS = 5 * 60 * 1000;

export function useDashboardFilterPresets(): UseQueryResult<
  BeDashboardFilterPreset[],
  Error
> {
  const api = useBackendApi();

  return useQuery<BeDashboardFilterPreset[], Error>({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_FILTER_PRESETS],
    queryFn: async (): Promise<BeDashboardFilterPreset[]> => {
      const res = await api.get<BeDashboardFilterPreset[]>(
        "/dashboards/filter-presets",
      );
      // The endpoint always 200s with an array, empty when no presets file
      // is configured. `null` therefore means the endpoint itself 404'd — a
      // backend older than this feature, or a routing problem — which must
      // surface as an error rather than as "this deployment has no presets".
      if (res === null) {
        throw new Error("GET /dashboards/filter-presets returned 404");
      }
      return res;
    },
    staleTime: SHARED_CONFIG_STALE_TIME_MS,
  });
}

export function useDashboardSharedSections(): UseQueryResult<
  BeDashboardSharedSection[],
  Error
> {
  const api = useBackendApi();

  return useQuery<BeDashboardSharedSection[], Error>({
    queryKey: [ApiQueryKeys.CSM_DASHBOARD_SECTIONS],
    queryFn: async (): Promise<BeDashboardSharedSection[]> => {
      const res = await api.get<BeDashboardSharedSection[]>(
        "/dashboards/sections",
      );
      // Same reasoning as useDashboardFilterPresets.
      if (res === null) {
        throw new Error("GET /dashboards/sections returned 404");
      }
      return res;
    },
    staleTime: SHARED_CONFIG_STALE_TIME_MS,
  });
}
