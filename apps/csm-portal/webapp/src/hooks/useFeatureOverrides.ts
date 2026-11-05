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

import { useEffect } from "react";
import { useMutation, useQuery, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { setSharedFeatureOverrides } from "@config/featureOverridesStore";

/** One saved row, as `GET /feature-overrides` returns it. */
export interface FeatureOverrideRow {
  navNodeId: string;
  state: string;
  updatedOn: string;
  updatedBy: string;
}

interface FeatureOverrideList {
  overrides: FeatureOverrideRow[];
  savedOnce: boolean;
}

/** The saved map, shaped for the flag engine and the Settings page. */
export interface FeatureOverridesData {
  /** `nav-node id -> state`, exactly as saved (enabled rows included). */
  rows: Record<string, string>;
  /** Who saved each row and when, for the "Saved by" line. */
  meta: Record<string, { updatedBy: string; updatedOn: string }>;
  /** True once an admin has finished a first save; until then `config.js` still applies. */
  savedOnce: boolean;
}

/** Shared by the shell (which feeds the flag engine) and the Settings page. */
export const FEATURE_OVERRIDES_KEY = ["feature-overrides"] as const;

/** Write body of `PUT /feature-overrides`: one page, plus the first-save mark on the last write. */
export interface SaveFeatureOverride {
  navNodeId: string;
  state: "enabled" | "preview";
  markSaved?: boolean;
}

/**
 * The shared per-page state. One request for the whole map. It is not retried:
 * the shell must fail open to `config.js` quickly when the endpoint or its
 * table is missing, an old backend is running, or the caller is not staff,
 * instead of holding the page behind a retry loop.
 */
export function useFeatureOverridesQuery(enabled = true): UseQueryResult<FeatureOverridesData, Error> {
  const api = useBackendApi();
  return useQuery<FeatureOverridesData, Error>({
    queryKey: FEATURE_OVERRIDES_KEY,
    enabled,
    retry: false,
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
    queryFn: async () => {
      const body = await api.get<FeatureOverrideList>("/feature-overrides");
      const rows: Record<string, string> = {};
      const meta: FeatureOverridesData["meta"] = {};
      for (const row of body?.overrides ?? []) {
        rows[row.navNodeId] = row.state;
        meta[row.navNodeId] = { updatedBy: row.updatedBy, updatedOn: row.updatedOn };
      }
      return { rows, meta, savedOnce: body?.savedOnce === true };
    },
  });
}

/**
 * Loads the shared map and hands it to the flag engine, so the sidebar, tab
 * chips, route guard and quick nav follow it. Mount once in the signed-in
 * shell. A failed load clears the store, which leaves `config.js` in charge
 * exactly as it was before this setting existed.
 */
export function useSharedFeatureOverrides(enabled: boolean): void {
  const query = useFeatureOverridesQuery(enabled);
  const { data, isError } = query;
  useEffect(() => {
    if (data) setSharedFeatureOverrides({ rows: data.rows, savedOnce: data.savedOnce });
    else if (isError) setSharedFeatureOverrides(undefined);
  }, [data, isError]);
}

/** Saves one page. The Settings page sends one of these per changed page. */
export function usePutFeatureOverride(): UseMutationResult<FeatureOverrideRow, Error, SaveFeatureOverride> {
  const api = useBackendApi();
  return useMutation<FeatureOverrideRow, Error, SaveFeatureOverride>({
    mutationFn: (input) => api.put<SaveFeatureOverride, FeatureOverrideRow>("/feature-overrides", input),
  });
}
