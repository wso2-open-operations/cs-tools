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
  BeProjectSearchPayload,
  BeProjectSearchResponse,
  BeSubscriptionType,
} from "@api/backend/types";

/** Matches the entity service's own per-page ceiling (see that repo's `maxLimit`). */
const AUDIENCE_PAGE_LIMIT = 50;

/**
 * Safety bound against a wrong/always-true `hasMore`, mirroring the paged-loop
 * convention used elsewhere in this app (e.g. `useGetCsmCaseComments`). 100
 * pages * 50 rows/page = 5,000 projects, comfortably above the ~97-project
 * scale the announcement-enhancement brief describes.
 */
const MAX_AUDIENCE_PAGES = 100;

export interface AudienceFilters {
  /** e.g. ["Restricted", "Suspended"]. */
  excludeClosureStates: string[];
  excludeSubscriptionTypes: BeSubscriptionType[];
}

/**
 * `key`/`accountName` are optional so this shape can also carry the
 * EOL/product-version flow's resolved audience (`BeEntityRef`-only — id/name,
 * no key or account) through the same {@link ResolvedAudienceList}, rather
 * than duplicating that component for a narrower shape.
 */
export interface ResolvedAudienceProject {
  id: string;
  name: string;
  key?: string;
  accountName?: string | null;
}

export interface ResolvedAudience {
  projects: ResolvedAudienceProject[];
  /** The definitive resolved count — every matching project has been fetched, not estimated from one page. */
  total: number;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
}

/**
 * Resolves the full "all customer projects" audience for an announcement:
 * pages through every match of `POST /announcements/audience/search` under
 * the given exclusion filters (there is no single call that returns
 * "everything" — see `useProjectSearch.ts`'s own paged hooks for the same
 * shape) and returns the complete, definitive list plus its count.
 *
 * This dedicated endpoint — not the general `/projects/search` also used by
 * the type-ahead pickers in `useProjectSearch.ts` — additionally, always,
 * excludes a backend-configured denylist of project keys that must never
 * receive an automated customer announcement (mirrors a hardcoded condition
 * in the real ServiceNow flow this replaces). The caller cannot see or
 * override that list; it's applied unconditionally server-side, the same
 * way the EOL/product-version flow's own mandatory exclusion is.
 *
 * Unlike the type-ahead pickers in `useProjectSearch.ts`, this is meant to
 * be enabled only once the engineer has actually chosen the "All customer
 * projects" scope — resolving the whole audience on every keystroke of a
 * narrower search would be wasteful and is never what that scope needs.
 */
export function useResolveAnnouncementAudience(
  enabled: boolean,
  filters: AudienceFilters,
): ResolvedAudience {
  const api = useBackendApi();

  const query: UseQueryResult<ResolvedAudienceProject[], Error> = useQuery({
    queryKey: [ApiQueryKeys.CSM_PROJECTS, "resolve-announcement-audience", filters],
    queryFn: async (): Promise<ResolvedAudienceProject[]> => {
      const collected: ResolvedAudienceProject[] = [];
      let offset = 0;

      for (let page = 0; ; page++) {
        // Fail loudly rather than truncate: silently returning `collected`
        // here would report an incomplete list as the complete, definitive
        // audience (the caller derives `total` directly from its length) —
        // an announcement could then under-count real recipients with no
        // indication anything was cut off. Mirrors the same bound-and-throw
        // convention the entity service's own paged-exclude-filter loop
        // uses (see sn_project_service.go's fetchAllProjectsFiltered).
        if (page === MAX_AUDIENCE_PAGES) {
          throw new Error(
            `Too many matching projects to resolve the audience safely (exceeded ${MAX_AUDIENCE_PAGES} pages of ${AUDIENCE_PAGE_LIMIT}) — narrow the exclusion filters and try again.`,
          );
        }
        const res = await api.post<BeProjectSearchPayload, BeProjectSearchResponse>(
          "/announcements/audience/search",
          {
            pagination: { offset, limit: AUDIENCE_PAGE_LIMIT },
            excludeClosureStates:
              filters.excludeClosureStates.length > 0 ? filters.excludeClosureStates : undefined,
            excludeSubscriptionTypes:
              filters.excludeSubscriptionTypes.length > 0
                ? filters.excludeSubscriptionTypes
                : undefined,
          },
        );
        const page_ = res.projects ?? [];
        for (const p of page_) {
          collected.push({
            id: p.id,
            name: p.name ?? p.id,
            key: p.key ?? "",
            accountName: p.account?.name ?? null,
          });
        }
        // Stop if the last page came back empty (offset wouldn't advance) or
        // the backend says there's nothing more.
        if (!res.hasMore || page_.length === 0) break;
        offset += page_.length;
      }

      return collected;
    },
    enabled,
    staleTime: 30_000,
  });

  return {
    projects: query.data ?? [],
    total: query.data?.length ?? 0,
    isLoading: query.isFetching,
    isError: query.isError,
    error: query.error ?? null,
  };
}
