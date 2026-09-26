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

import { useCallback, useState } from "react";
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { apiConfig } from "@config/apiConfig";
import {
  ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
  settleWithConcurrencyLimit,
} from "@features/csm-announcements/utils/settleWithConcurrencyLimit";
import type { ResolvedAudienceProject } from "@features/csm-announcements/api/useResolveAnnouncementAudience";
import type { ProjectDetails } from "@features/csm-projects/types/csmProjects";

/**
 * Caps how many of an approved request's frozen `resolvedProjectIds` this
 * resolves to real names before Publish. Unlike `useResolveAnnouncementAudience`'s
 * up-to-5,000-project safety bound (a defensive ceiling, not the expected
 * case — see that hook's own doc comment: the real scale announcements
 * target is ~97 projects), this is a genuine display cap: each id here is
 * its own `GET /projects/{id}` round trip (no batch-by-ids endpoint exists
 * yet), so resolving thousands one at a time before a confirmation popup
 * can even render would be slow and needlessly load the backend for a
 * review step. 200 comfortably covers the real-world scale while keeping
 * this fast; a request past that still publishes to its full frozen
 * audience regardless — this cap only affects what the preview *shows*.
 */
export const AUDIENCE_PREVIEW_MAX_PROJECTS = 200;

export interface ResolvedAudiencePreview {
  projects: ResolvedAudienceProject[];
  /** The true total in resolvedProjectIds, which may exceed projects.length if capped. */
  total: number;
  isLoading: boolean;
  isError: boolean;
  /** True when total > AUDIENCE_PREVIEW_MAX_PROJECTS and the list below was truncated. */
  truncated: boolean;
  resolve: (projectIds: string[]) => Promise<void>;
}

/**
 * On-demand resolver from a flat `resolvedProjectIds` list (as stored on an
 * `AnnouncementRequest`, e.g. `["11111111-...", "22222222-..."]`) to real
 * project names/keys/accounts, for showing whoever is about to click
 * Publish exactly who that sends to — not just a count. Unlike
 * `useResolveAnnouncementAudience`, this doesn't run on mount: the caller
 * invokes `resolve` explicitly (when the confirmation popup opens), since
 * fetching up to 200 individual projects on every dialog render would be
 * wasteful for a request most viewers never click Publish on.
 */
export function useResolvedAudiencePreview(): ResolvedAudiencePreview {
  const authFetch = useAuthApiClient();
  const [projects, setProjects] = useState<ResolvedAudienceProject[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [isError, setIsError] = useState(false);

  const resolve = useCallback(
    async (projectIds: string[]): Promise<void> => {
      setIsLoading(true);
      setIsError(false);
      setTotal(projectIds.length);

      const toFetch = projectIds.slice(0, AUDIENCE_PREVIEW_MAX_PROJECTS);
      const results = await settleWithConcurrencyLimit(
        toFetch,
        ANNOUNCEMENT_CASE_CREATE_CONCURRENCY_LIMIT,
        async (id): Promise<ResolvedAudienceProject | null> => {
          const res = await authFetch(`${apiConfig.backendUrl}/projects/${encodeURIComponent(id)}`);
          if (res.status === 404) return null;
          if (!res.ok) throw new Error(`GET /projects/${id} failed: ${res.status}`);
          const detail = (await res.json()) as ProjectDetails;
          return { id: detail.id, name: detail.name, key: detail.key, accountName: detail.account?.name };
        },
      );

      // A single project failing to resolve (e.g. deleted since the
      // audience was frozen at submit time) shouldn't block seeing everyone
      // else — only a totally failed fetch blanks the list, so a real
      // network/auth problem is still visible rather than silently showing
      // "0 projects."
      const resolved = results
        .filter((r): r is PromiseFulfilledResult<ResolvedAudienceProject | null> => r.status === "fulfilled")
        .map((r) => r.value)
        .filter((p): p is ResolvedAudienceProject => p !== null);

      setProjects(resolved);
      setIsError(results.length > 0 && resolved.length === 0);
      setIsLoading(false);
    },
    [authFetch],
  );

  return {
    projects,
    total,
    isLoading,
    isError,
    truncated: total > AUDIENCE_PREVIEW_MAX_PROJECTS,
    resolve,
  };
}
