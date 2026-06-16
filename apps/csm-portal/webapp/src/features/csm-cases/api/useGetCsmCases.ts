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
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { isMockMode, useBackendApi, type BackendApi } from "@api/backend/client";
import {
  beStateFromUi,
  priorityFromSeverity,
  severityFromPriority,
  uiStateFromBe,
} from "@api/backend/mappers";
import { projectOptionsQueryOptions } from "@features/csm-cases/api/useProjectOptions";
import type {
  BeAccount,
  BeAccountSearchPayload,
  BeAccountSearchResponse,
  BeCaseSearchPayload,
  BeCaseSearchResponse,
} from "@api/backend/types";
import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import {
  applyCasesFilters,
  sortBySlaUrgency,
} from "@features/csm-cases/utils/casesClientFilter";
import type {
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 200;
/** Page size for the account name lookup (customer column). */
const LOOKUP_PAGE_LIMIT = BE_MAX_PAGE_LIMIT;
// Cap the account scan the same way useProjectOptions caps the project scan.
// Doubled from 20 when the page limit halved to BE_MAX_PAGE_LIMIT, so the total
// scan ceiling (pages * limit) is unchanged.
const LOOKUP_MAX_PAGES = 40;

/**
 * Query options for the account-name lookup (`POST /accounts/search`). Kept as
 * its own stably-keyed query (resolved via `queryClient.fetchQuery`) so filter
 * changes on the cases list — which change the cases query key — reuse the
 * cached directory instead of re-fetching every account each time.
 */
function accountOptionsQueryOptions(api: BackendApi) {
  return {
    queryKey: [ApiQueryKeys.CSM_ACCOUNTS, "options"],
    queryFn: async (): Promise<BeAccount[]> => {
      const all: BeAccount[] = [];
      let offset = 0;
      for (let page = 0; page < LOOKUP_MAX_PAGES; page += 1) {
        const res = await api.post<
          BeAccountSearchPayload,
          BeAccountSearchResponse
        >("/accounts/search", {
          pagination: { offset, limit: LOOKUP_PAGE_LIMIT },
        });
        const accounts = res.accounts ?? [];
        all.push(...accounts);
        if (accounts.length < LOOKUP_PAGE_LIMIT) break;
        offset += LOOKUP_PAGE_LIMIT;
      }
      return all;
    },
    staleTime: 60_000,
  } as const;
}

/**
 * Cross-project CSM cases list.
 *
 * LIVE mode does a single `POST /cases/search` (the flat, cross-project search)
 * and maps each rich `CaseSearchView` — which embeds project / deployment /
 * deployed-product — to the UI `CsmCaseRow`. The account (customer) name is the
 * only field not embedded, so it's resolved via `projects/search`
 * (projectId → accountId) + `accounts/search` (accountId → name). Both lookups
 * live under their own stable query keys (resolved with
 * `queryClient.fetchQuery`), so changing list filters only re-runs
 * `/cases/search`; the project/account directories come from cache until they
 * go stale. The project lookup shares `useProjectOptions`' key, which the
 * cases page already mounts for its filter options.
 *
 * Search and the severity / state / project filters are pushed into the search
 * payload (searchQuery / priorityKeys / stateKeys / projectIds) and the BE
 * paginates the result (`pagination` → `total` / `limit` / `offset` /
 * `hasMore`). The remaining filters (assignee, SLA, product) have no BE support
 * and are disabled in LIVE; they only do anything in MOCK mode, where the whole
 * seeded dataset is filtered, sorted and sliced client-side here.
 *
 * `page` is zero-based (matching MUI `TablePagination`); `pageSize` is the row
 * limit (≤ {@link BE_MAX_PAGE_LIMIT}). With no filters the backend sorts by
 * last-updated descending, so the cases page loads the most recently updated
 * cases on arrival. `enabled` is an optional escape hatch to suspend the fetch.
 */
export function useGetCsmCases(
  filters: CasesFilters,
  page: number,
  pageSize: number,
  enabled = true,
): UseQueryResult<CsmCasesListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();
  const queryClient = useQueryClient();

  const offset = page * pageSize;
  const search = filters.search.trim();

  return useQuery<CsmCasesListResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache
    // (["S1","S2"] and ["S2","S1"] are the same query).
    queryKey: [
      ApiQueryKeys.CSM_CASES,
      filters.scope,
      search,
      [...filters.severities].sort(),
      [...filters.states].sort(),
      [...filters.projects].sort(),
      // MOCK-only filters still affect the sliced result in mock mode.
      filters.sla,
      [...filters.assignees].sort(),
      [...filters.products].sort(),
      page,
      pageSize,
    ],
    queryFn: async (): Promise<CsmCasesListResponse> => {
      if (isMockMode()) {
        logger.debug(
          `[useGetCsmCases] Returning mock cases for scope=${filters.scope}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        const all = getMockCsmCases(filters.scope);
        const matched = applyCasesFilters(all, filters)
          .slice()
          .sort(sortBySlaUrgency);
        const pageRows = matched.slice(offset, offset + pageSize);
        return {
          scope: filters.scope,
          cases: pageRows,
          total: matched.length,
          limit: pageSize,
          offset,
          hasMore: offset + pageRows.length < matched.length,
        };
      }

      // One cross-project case search, plus project/account lookups for the
      // customer column (cases embed the project, but not its account). The
      // lookups go through `fetchQuery` with their own stable keys, so they
      // hit the network only when their cache is stale — not on every filter
      // change. Lookup failures degrade to "—" names, not a failed list.
      const [casesResponse, projects, accounts] = await Promise.all([
        api.post<BeCaseSearchPayload, BeCaseSearchResponse>("/cases/search", {
          pagination: { offset, limit: pageSize },
          sortBy: { field: "updated_at", order: "desc" },
          ...(search.length > 0 && { searchQuery: search }),
          ...(filters.severities.length > 0 && {
            priorityKeys: filters.severities.map(priorityFromSeverity),
          }),
          ...(filters.states.length > 0 && {
            stateKeys: filters.states.map(beStateFromUi),
          }),
          // `filters.projects` holds project IDs (the filter is id-based).
          ...(filters.projects.length > 0 && {
            projectIds: filters.projects,
          }),
        }),
        queryClient.fetchQuery(projectOptionsQueryOptions(api)).catch((err) => {
          logger.warn(
            `[useGetCsmCases] project lookup failed: ${(err as Error).message}`,
          );
          return [];
        }),
        queryClient.fetchQuery(accountOptionsQueryOptions(api)).catch((err) => {
          logger.warn(
            `[useGetCsmCases] account lookup failed: ${(err as Error).message}`,
          );
          return [];
        }),
      ]);

      const projectAccount = new Map<string, string>(
        projects
          .filter((p) => p.accountId)
          .map((p) => [p.id, p.accountId as string]),
      );
      const accountName = new Map<string, string>(
        accounts
          .filter((a) => a.name)
          .map((a) => [a.id, a.name as string]),
      );

      const cases: CsmCaseRow[] = (casesResponse.cases ?? []).map((c) => {
        const projectId = c.project?.id ?? "";
        const accountId = projectAccount.get(projectId) ?? "";
        return {
          id: c.id,
          caseNumber: c.number,
          wso2CaseId: c.internalId,
          subject: c.subject ?? "(no subject)",
          customer: accountName.get(accountId) ?? "—",
          accountId,
          projectId,
          projectName: c.project?.name ?? "—",
          product: c.deployedProduct?.displayName ?? "—",
          severity: severityFromPriority(c.priority),
          state: uiStateFromBe(c.state),
          // No assignee field on the backend yet; surfaced as "Unassigned".
          assignee: "Unassigned",
          assigneeIsMe: false,
          slaClockType: "ack",
          minutesToBreach: 0,
          // No SLA data from the backend yet — keep the SLA column neutral
          // rather than painting every open row orange with a bogus "0m left".
          hasSla: false,
          createdAt: c.createdOn ?? "",
          updatedAt: c.updatedOn ?? c.createdOn ?? "",
        };
      });

      return {
        scope: filters.scope,
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
