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
import { ApiQueryKeys } from "@constants/apiConstants";
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
import type {
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 200;

/** Page size for the cross-project case list (server-side filtering TBD). */
const CASES_PAGE_LIMIT = 100;
/** Page size for the account name lookup (customer column). */
const LOOKUP_PAGE_LIMIT = 100; // backend caps pagination limit at 100
// Cap the account scan the same way useProjectOptions caps the project scan.
const LOOKUP_MAX_PAGES = 20;

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
 * Severity / state filters are pushed into the search payload (priorityKeys /
 * stateKeys); the remaining filters stay client-side in `CsmCasesPage`
 * (`applyFilters`), which also drives MOCK mode. The BE has no assignee field,
 * so `scope` / assignee filters remain inert in LIVE.
 */
export function useGetCsmCases(
  filters: CasesFilters,
): UseQueryResult<CsmCasesListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useQuery<CsmCasesListResponse, Error>({
    // Sort the array filters so selection order doesn't fragment the cache
    // (["S1","S2"] and ["S2","S1"] are the same query).
    queryKey: [
      ApiQueryKeys.CSM_CASES,
      filters.scope,
      [...filters.severities].sort(),
      [...filters.states].sort(),
      [...filters.projects].sort(),
    ],
    queryFn: async (): Promise<CsmCasesListResponse> => {
      if (isMockMode()) {
        logger.debug(
          `[useGetCsmCases] Returning mock cases for scope=${filters.scope}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCases(filters.scope);
      }

      // One cross-project case search, plus project/account lookups for the
      // customer column (cases embed the project, but not its account). The
      // lookups go through `fetchQuery` with their own stable keys, so they
      // hit the network only when their cache is stale — not on every filter
      // change. Lookup failures degrade to "—" names, not a failed list.
      const [casesResponse, projects, accounts] = await Promise.all([
        api.post<BeCaseSearchPayload, BeCaseSearchResponse>("/cases/search", {
          pagination: { offset: 0, limit: CASES_PAGE_LIMIT },
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
          caseNumber: c.number ?? c.id,
          wso2CaseId: c.wso2Id ?? c.id,
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
          createdAt: c.createdAt ?? "",
          updatedAt: c.updatedAt ?? c.createdAt ?? "",
        };
      });

      return { scope: filters.scope, cases };
    },
    staleTime: 30_000,
  });
}
