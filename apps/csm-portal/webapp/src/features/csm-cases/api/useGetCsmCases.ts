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
import { useLogger } from "@hooks/useLogger";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import {
  ASSIGNEE_FILTER_RESOLVED_EMPTY,
  buildCaseSearchFilters,
  mapCaseSearchViewToRow,
  resolveAssignedUserIds,
} from "@features/csm-cases/utils/caseSearchPayload";
import { resolveRelativeDatePlaceholder } from "@utils/resolveRelativeDatePlaceholder";
import { resolveAdvancedFilterDateValues } from "@features/csm-cases/utils/advancedFilters";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";
import { classifyCaseQuery } from "@features/csm-cases/utils/caseQueryScope";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import type { BeCaseSearchPayload, BeCaseSearchResponse } from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type {
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";
import {
  DEFAULT_CASES_SORT,
  type CasesSortField,
  type CasesSortOrder,
} from "@features/csm-cases/utils/casesSort";

/**
 * How many exact identifier hits to pin above the first page. A case number is
 * unique, so this is realistically 1; the small headroom covers a WSO2 case id
 * that repeats across projects without ever pinning an unbounded block.
 */
const EXACT_MATCH_LIMIT = 5;

/**
 * Cross-project CSM cases list.
 *
 * Does a single `POST /cases/search` (the flat, cross-project search)
 * and maps each rich `CaseSearchView` — which embeds project / deployment /
 * deployed-product / account — to the UI `CsmCaseRow`. The optional Customer
 * column reads `account.name` straight off this response; no separate
 * `/accounts/search` directory scan is needed (that endpoint has no ID
 * filter anyway, so scanning it per row would have been the wrong approach
 * regardless).
 *
 * Search and the severity / state / case-type / project filters are pushed
 * into the search payload (searchQuery / severities / states / types /
 * projectIds) and the BE paginates the result (`pagination` → `total` /
 * `limit` / `offset` / `hasMore`).
 *
 * `page` is zero-based (matching MUI `TablePagination`); `pageSize` is the row
 * limit (≤ the backend's page-size cap, `BE_MAX_PAGE_LIMIT`). `sortField`
 * (default `"updatedOn"`) picks which column drives the server-side sort —
 * `createdOn`, `updatedOn`, `severity`, or `state` — and `sortOrder` (default
 * `"desc"`) controls direction, so the cases page loads the most recently
 * updated cases on arrival by default but can be flipped or repointed at a
 * different column. `enabled` is an optional escape hatch to suspend the
 * fetch.
 */
export function useGetCsmCases(
  filters: CasesFilters,
  page: number,
  pageSize: number,
  enabled = true,
  sortOrder: CasesSortOrder = DEFAULT_CASES_SORT.order,
  sortField: CasesSortField = DEFAULT_CASES_SORT.field,
): UseQueryResult<CsmCasesListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();
  // Signed-in email, to resolve `assigneeIsMe` per row against the assigned
  // engineer's email. In the key so a late-arriving claim recomputes — this
  // applies to every row regardless of the assignee filter, so it stays
  // unconditional.
  const currentUserEmail = useIdTokenClaims()?.email;
  // The caller's platform UUID, fetched once app-wide (CurrentUserProvider)
  // and used only to resolve an `@me` assignee filter (see
  // `resolveAssignedUserIds`) — nothing else reads it. Only fold it into the
  // key when `@me` is actually selected: `/users/me` is a real network call
  // that resolves after the id starts as `undefined`, and keying on it
  // unconditionally meant every page load re-fetched the exact same
  // unfiltered search a second time the moment it arrived, for every user,
  // regardless of whether any assignee filter was active at all.
  const currentUserId = useCurrentUser().user?.id;
  const wantsMe = filters.assignees.includes(ASSIGNEE_ME_TOKEN);

  const offset = page * pageSize;
  const search = filters.search.trim();

  return useQuery<CsmCasesListResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache
    // (["S1","S2"] and ["S2","S1"] are the same query). `assignees` holds
    // engineer emails (+ the `@me` sentinel); it's resolved to UUIDs in the
    // queryFn, but keying on the raw selection is enough since resolution is
    // deterministic. `currentUserEmail` is already in the key, covering `@me`.
    // Every field of `CasesFilters` that reaches `buildCaseSearchFilters`
    // below must appear here — one that doesn't (as `csTeams` didn't, until
    // its own bar control made that visible) makes React Query treat a
    // changed filter as the same query and skip the refetch entirely.
    queryKey: [
      ApiQueryKeys.CSM_CASES,
      search,
      [...filters.severities].sort(),
      [...filters.states].sort(),
      [...filters.excludeStates].sort(),
      [...filters.caseTypes].sort(),
      [...filters.workStates].sort(),
      [...filters.assignees].sort(),
      [...filters.projects].sort(),
      [...filters.engagementTypes].sort(),
      [...filters.productNames].sort(),
      [...filters.csTeams].sort(),
      [...filters.sreTeams].sort(),
      [...filters.tags].sort(),
      [...filters.excludeTags].sort(),
      [...filters.onboardingStatuses].sort(),
      [...filters.escalationLevels].sort(),
      [...filters.projectTypes].sort(),
      filters.slaElapsedPctGte,
      filters.slaElapsedPctLte,
      filters.hasEscalation,
      filters.createdOnGte,
      filters.createdOnLte,
      filters.updatedOnGte,
      filters.updatedOnLte,
      filters.closedOnGte,
      filters.closedOnLte,
      // JSON-stable-enough for a query key: row order is already
      // user-controlled (add/remove), and every field/op/values combination
      // that would otherwise collide is exactly the same shape the `/cases/
      // search` payload itself carries — no separate normalization needed
      // here beyond what `buildCaseSearchFilters` already applies.
      JSON.stringify(filters.advancedFilters),
      // Same reasoning as `advancedFilters` above — every field/values
      // combination an OR-group row carries is already the exact shape the
      // request payload itself sends.
      JSON.stringify(filters.anyOfBranches),
      currentUserEmail ?? "",
      wantsMe ? (currentUserId ?? "") : "",
      page,
      pageSize,
      sortField,
      sortOrder,
    ],
    queryFn: async (): Promise<CsmCasesListResponse> => {
      // Resolve the assignee filter (engineer emails + the `@me` sentinel) to
      // the UUIDs `/cases/search` expects — shared with the export action via
      // `resolveAssignedUserIds` so both apply the identical assignee filter.
      // A transport failure of the lookup is NOT swallowed — it throws so the
      // query errors (the list shows an error) instead of silently
      // broadening to all cases.
      let assignedUserIds: string[] | undefined;
      if (filters.assignees.length > 0) {
        let resolved: Awaited<ReturnType<typeof resolveAssignedUserIds>>;
        try {
          resolved = await resolveAssignedUserIds(api, filters.assignees, currentUserId);
        } catch (err) {
          logger.warn(
            `[useGetCsmCases] assignee lookup failed: ${(err as Error).message}`,
          );
          throw new Error("Failed to resolve the assignee filter");
        }
        // Active assignee filter that resolved to nothing → empty result, not
        // a broadened (filter-less) search. See `resolveAssignedUserIds`.
        if (resolved === ASSIGNEE_FILTER_RESOLVED_EMPTY) {
          return { cases: [], total: 0, limit: pageSize, offset, hasMore: false };
        }
        assignedUserIds = resolved;
      }

      // Resolve `createdOnGte`/`createdOnLte`/`updatedOnGte`/`updatedOnLte`/
      // `closedOnGte`/`closedOnLte` relative-date placeholders (`__today__`,
      // `__daysAgo:N__`, ... -- same grammar and resolver as the dashboard
      // widgets' own `resolveRelativeDateFilters`) against the viewer's own
      // browser-local "now", right here at query-build time -- no backend
      // change, and "today" correctly means whatever moment the link is
      // opened, not a moment baked into the URL. Anything that isn't a
      // recognized placeholder (a literal `YYYY-MM-DD`, or garbage) passes
      // through unchanged.
      const now = new Date();
      const resolveBound = (
        raw: string | null,
        op: "gte" | "lte",
      ): string | null =>
        raw === null ? null : (resolveRelativeDatePlaceholder(raw, op, now) ?? raw);
      const resolvedFilters: CasesFilters = {
        ...filters,
        createdOnGte: resolveBound(filters.createdOnGte, "gte"),
        createdOnLte: resolveBound(filters.createdOnLte, "lte"),
        updatedOnGte: resolveBound(filters.updatedOnGte, "gte"),
        updatedOnLte: resolveBound(filters.updatedOnLte, "lte"),
        closedOnGte: resolveBound(filters.closedOnGte, "gte"),
        closedOnLte: resolveBound(filters.closedOnLte, "lte"),
        // Same relative-date resolution, extended to any `createdOn`/
        // `updatedOn`/`closedOn` row from the "Advanced filters" builder —
        // see `resolveAdvancedFilterDateValues`.
        advancedFilters: resolveAdvancedFilterDateValues(filters.advancedFilters, now),
      };

      // One cross-project case search. No separate account/project directory
      // scan needed: the search response already embeds `account`, which
      // `mapCaseSearchViewToRow` reads directly for the optional Customer
      // column.
      const runSearch = (
        searchOptions?: { forceFreeText?: boolean; alsoFreeText?: boolean },
        pagination = { offset, limit: pageSize },
      ): Promise<BeCaseSearchResponse> =>
        api.post<BeCaseSearchPayload, BeCaseSearchResponse>("/cases/search", {
          pagination,
          sortBy: { field: sortField, order: sortOrder },
          filters: buildCaseSearchFilters(
            resolvedFilters,
            search,
            assignedUserIds,
            searchOptions,
          ),
        });

      // A query shaped like a case number / WSO2 id runs BOTH legs at once: the
      // exact indexed lookup and the free-text CONTAINS scan. Neither alone is
      // right — exact-only loses cases that merely reference the number in their
      // description, while free-text-only is what let an unrelated case outrank
      // (or hide) the one actually being looked up. Running them in parallel
      // costs no extra latency over the free-text leg it already ran.
      const isIdentifierQuery =
        search.length > 0 && classifyCaseQuery(search) !== "text";

      if (!isIdentifierQuery) {
        const casesResponse = await runSearch();
        const cases: CsmCaseRow[] = (casesResponse.cases ?? []).map((c) =>
          mapCaseSearchViewToRow(c, currentUserEmail),
        );
        return {
          cases,
          total: casesResponse.total ?? cases.length,
          limit: casesResponse.limit ?? pageSize,
          offset: casesResponse.offset ?? offset,
          hasMore: casesResponse.hasMore ?? false,
        };
      }

      // The merged sequence is [pinned exact hits] followed by the free-text
      // hits with those same rows removed. Pinning consumes slots at the front,
      // so from page 1 on, the free-text window starts one row early and is
      // over-fetched — otherwise the row displaced off page 0 would be skipped
      // entirely (page 1 would resume at `offset`, past it).
      //
      // The real hit count isn't known until the exact leg resolves, and both
      // legs are issued together, so the window is shaped for the realistic
      // case of a single hit (an identifier is unique) and `pinShift` below
      // reconciles the guess once both have landed.
      const ASSUMED_PIN_COUNT = 1;
      const textOffset = page === 0 ? 0 : Math.max(0, offset - ASSUMED_PIN_COUNT);
      const [exactResponse, textResponse, overlapResponse] = await Promise.all([
        // Page-independent: fetched once from the top rather than re-queried
        // per page, since an identifier match is a tiny, fixed result.
        runSearch(undefined, { offset: 0, limit: EXACT_MATCH_LIMIT }),
        runSearch(
          { forceFreeText: true },
          { offset: textOffset, limit: pageSize + EXACT_MATCH_LIMIT },
        ),
        // How many of the exact hits the free-text scan already counts. Only
        // its `total` is read, so this asks for a single row. Without it the
        // merged total has to be guessed, and guessing low makes the last row
        // unreachable: the paginator stops offering pages before it.
        runSearch({ alsoFreeText: true }, { offset: 0, limit: 1 }),
      ]);

      const exactRows = (exactResponse.cases ?? []).map((c) =>
        mapCaseSearchViewToRow(c, currentUserEmail),
      );
      const exactIds = new Set(exactRows.map((c) => c.id));
      // Dropped from every page of the free-text leg, not just the first, so a
      // pinned case can't also reappear further down its own result set.
      const textRows = (textResponse.cases ?? [])
        .map((c) => mapCaseSearchViewToRow(c, currentUserEmail))
        .filter((c) => !exactIds.has(c.id));

      // Nothing was pinned after all (no exact hit), so the extra leading row
      // this page fetched belongs to the previous page — drop it.
      const pinShift =
        page === 0
          ? 0
          : ASSUMED_PIN_COUNT - Math.min(exactRows.length, ASSUMED_PIN_COUNT);
      const cases = (
        page === 0 ? [...exactRows, ...textRows] : textRows.slice(pinShift)
      ).slice(0, pageSize);

      // Page-independent by construction: both terms are totals over the whole
      // result set, never over the page currently in hand. An earlier version
      // compared the pinned rows against the fetched page, which made the
      // reported total change as the user paged.
      //
      // Pinning usually reorders rather than adds, because the free-text scan
      // also covers the number/WSO2-id columns — so an exact hit is normally
      // already inside `textTotal`, and `overlapTotal` says exactly how many
      // are. Only the remainder is genuinely new: an identifier the scan misses
      // entirely, which is the anomaly this feature exists for. Undercounting
      // there is not cosmetic — the paginator stops offering pages at `total`,
      // so a row past that point can never be reached.
      const textTotal = textResponse.total ?? textRows.length;
      const overlapTotal = overlapResponse.total ?? 0;
      const pinnedNotCounted = Math.max(0, exactRows.length - overlapTotal);
      const total = textTotal + pinnedNotCounted;

      return {
        cases,
        total,
        limit: pageSize,
        offset,
        hasMore: offset + cases.length < total,
      };
    },
    enabled,
    staleTime: 30_000,
  });
}
