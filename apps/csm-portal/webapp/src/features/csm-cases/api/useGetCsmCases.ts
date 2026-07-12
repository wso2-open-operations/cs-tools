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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import {
  beStateFromUi,
  priorityFromSeverity,
  severityFromPriority,
  uiStateFromBe,
} from "@api/backend/mappers";
import { ASSIGNEE_ME_TOKEN } from "@features/csm-cases/utils/assignee";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import type {
  BeCaseSearchPayload,
  BeCaseSearchResponse,
  BeUserSearchResponse,
} from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import type {
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";
import {
  DEFAULT_CASES_SORT,
  type CasesSortOrder,
} from "@features/csm-cases/utils/casesSort";

/**
 * Cross-project CSM cases list.
 *
 * Does a single `POST /cases/search` (the flat, cross-project search)
 * and maps each rich `CaseSearchView` — which embeds project / deployment /
 * deployed-product — to the UI `CsmCaseRow`. The list has no Customer column,
 * so the customer (account) name is deliberately not resolved: doing so used to
 * page the entire account directory (`/accounts/search` has no ID filter, and
 * the case search view doesn't embed the account) to fill a field nothing
 * renders. If a Customer column is added, resolve the name from the search view
 * once it carries the account, not by scanning the directory.
 *
 * Search and the severity / state / case-type / project filters are pushed
 * into the search payload (searchQuery / severities / states / types /
 * projectIds) and the BE paginates the result (`pagination` → `total` /
 * `limit` / `offset` / `hasMore`).
 *
 * `page` is zero-based (matching MUI `TablePagination`); `pageSize` is the row
 * limit (≤ {@link BE_MAX_PAGE_LIMIT}). Cases are always sorted by `updatedOn`;
 * `sortOrder` (default `"desc"`) controls direction, so the cases page loads
 * the most recently updated cases on arrival by default but can be flipped
 * to oldest-updated-first. `enabled` is an optional escape hatch to suspend
 * the fetch.
 */
export function useGetCsmCases(
  filters: CasesFilters,
  page: number,
  pageSize: number,
  enabled = true,
  sortOrder: CasesSortOrder = DEFAULT_CASES_SORT.order,
): UseQueryResult<CsmCasesListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();
  // Signed-in email, to resolve `assigneeIsMe` per row against the assigned
  // engineer's email. In the key so a late-arriving claim recomputes.
  const currentUserEmail = useIdTokenClaims()?.email;
  // The caller's platform UUID, fetched once app-wide (CurrentUserProvider) and
  // used to resolve the `@me` assignee filter. In the key so a late-arriving id
  // re-resolves an `@me` selection.
  const currentUserId = useCurrentUser().user?.id;

  const offset = page * pageSize;
  const search = filters.search.trim();

  return useQuery<CsmCasesListResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache
    // (["S1","S2"] and ["S2","S1"] are the same query). `assignees` holds
    // engineer emails (+ the `@me` sentinel); it's resolved to UUIDs in the
    // queryFn, but keying on the raw selection is enough since resolution is
    // deterministic. `currentUserEmail` is already in the key, covering `@me`.
    queryKey: [
      ApiQueryKeys.CSM_CASES,
      search,
      [...filters.severities].sort(),
      [...filters.states].sort(),
      [...filters.caseTypes].sort(),
      [...filters.workStates].sort(),
      [...filters.assignees].sort(),
      [...filters.projects].sort(),
      [...filters.engagementTypes].sort(),
      [...filters.productNames].sort(),
      currentUserEmail ?? "",
      currentUserId ?? "",
      page,
      pageSize,
      sortOrder,
    ],
    queryFn: async (): Promise<CsmCasesListResponse> => {
      // Resolve the assignee filter (engineer emails + the `@me` sentinel) to
      // the UUIDs `/cases/search` expects. Named engineers → ids from a
      // targeted `/users/search` by email (selectable emails always come from
      // the directory, so this resolves them all); `@me` → the caller's id,
      // taken from the app-wide CurrentUserProvider (no per-query `/users/me`).
      // Runs only when the assignee filter is active. A user whose id can't be
      // resolved (e.g. `@me` when `/users/me` omits `id` — entity service down
      // — or an email with no match) is dropped. If an active assignee filter
      // resolves to NO ids, the result is definitionally empty (cases assigned
      // to engineers we couldn't identify), so we return an empty list rather
      // than omitting the filter — omitting it would broaden the search to ALL
      // cases, the opposite of what the user asked for. A transport failure of
      // the lookup is NOT swallowed — it throws so the query errors (the list
      // shows an error) instead of silently broadening to all cases.
      let assignedUserIds: string[] | undefined;
      if (filters.assignees.length > 0) {
        const wantsMe = filters.assignees.includes(ASSIGNEE_ME_TOKEN);
        const emails = filters.assignees.filter((a) => a !== ASSIGNEE_ME_TOKEN);
        let byEmail: BeUserSearchResponse | null = null;
        if (emails.length > 0) {
          try {
            byEmail = await api.post<
              { filters: { emails: string[] }; pagination: { limit: number } },
              BeUserSearchResponse
            >("/users/search", {
              filters: { emails },
              pagination: { limit: BE_MAX_PAGE_LIMIT },
            });
          } catch (err) {
            logger.warn(
              `[useGetCsmCases] assignee lookup failed: ${(err as Error).message}`,
            );
            throw new Error("Failed to resolve the assignee filter");
          }
        }
        const ids = new Set<string>();
        (byEmail?.users ?? []).forEach((u) => {
          if (u.id) ids.add(u.id);
        });
        if (wantsMe && currentUserId) ids.add(currentUserId);
        if (ids.size > 0) assignedUserIds = [...ids];
      }

      // Active assignee filter that resolved to nothing → empty result, not a
      // broadened (filter-less) search. See the resolution note above.
      if (filters.assignees.length > 0 && !assignedUserIds) {
        return { cases: [], total: 0, limit: pageSize, offset, hasMore: false };
      }

      // One cross-project case search. No account/project directory scan: the
      // list has no Customer column, and the old scan paged the entire account
      // directory to resolve a name nothing renders.
      const casesResponse = await api.post<
        BeCaseSearchPayload,
        BeCaseSearchResponse
      >("/cases/search", {
          pagination: { offset, limit: pageSize },
          sortBy: { field: "updatedOn", order: sortOrder },
          // Filter fields are nested under `filters` (BE payload restructure).
          filters: {
            ...(search.length > 0 && { searchQuery: search }),
            ...(filters.severities.length > 0 && {
              severities: filters.severities.map(priorityFromSeverity),
            }),
            ...(filters.states.length > 0 && {
              states: filters.states.map(beStateFromUi),
            }),
            ...(filters.caseTypes.length > 0 && {
              types: filters.caseTypes,
            }),
            // Work sub-state filter (ongoing/paused); only meaningful when
            // `states` includes `work_in_progress`, but the BE accepts it
            // independently so we forward it as-is.
            ...(filters.workStates.length > 0 && {
              workStates: filters.workStates,
            }),
            ...(filters.engagementTypes.length > 0 && {
              engagementTypes: filters.engagementTypes,
            }),
            // `filters.projects` holds project IDs (the filter is id-based).
            ...(filters.projects.length > 0 && {
              projectIds: filters.projects,
            }),
            // Assignee filter, resolved to engineer UUIDs above.
            ...(assignedUserIds && assignedUserIds.length > 0 && {
              assignedUserIds,
            }),
            // Product family names; SN matches `product.name` (all versions).
            ...(filters.productNames.length > 0 && {
              productNames: filters.productNames,
            }),
          },
      });

      const myEmail = currentUserEmail?.toLowerCase();
      const cases: CsmCaseRow[] = (casesResponse.cases ?? []).map((c) => {
        const projectId = c.project?.id ?? "";
        const assigneeEmail = c.assignedEngineer?.email;
        const assignee =
          c.assignedEngineer?.name?.trim() || assigneeEmail || "Unassigned";
        const assigneeIsMe =
          !!assigneeEmail && !!myEmail && assigneeEmail.toLowerCase() === myEmail;
        return {
          id: c.id,
          caseNumber: c.number,
          wso2CaseId: c.internalId,
          subject: c.subject ?? "(no subject)",
          // Not shown in the list; kept on the row shape but no longer resolved
          // (see the account-scan removal note above).
          customer: "",
          accountId: "",
          projectId,
          projectName: c.project?.name ?? "-",
          // Search embeds deployedProduct as { id, name } (name includes the
          // version); the GET view uses a displayName-shaped ref instead.
          product: c.deployedProduct?.name ?? c.product?.name ?? "-",
          severity: severityFromPriority(c.severity),
          state: uiStateFromBe(c.state),
          caseType: c.type,
          workState: c.workState ?? null,
          assignee,
          assigneeIsMe,
          slaClockType: "ack",
          minutesToBreach: 0,
          // No SLA data from the backend yet — keep the SLA column neutral
          // rather than painting every open row orange with a bogus "0m left".
          hasSla: false,
          createdAt: c.createdOn ?? "",
          updatedAt: c.updatedOn ?? c.createdOn ?? "",
          updatedAtIsCreatedFallback: !c.updatedOn && !!c.createdOn,
        };
      });

      return {
        cases,
        total: casesResponse.total ?? cases.length,
        limit: casesResponse.limit ?? pageSize,
        offset: casesResponse.offset ?? offset,
        hasMore: casesResponse.hasMore ?? false,
      };
    },
    enabled,
    staleTime: 30_000,
  });
}
