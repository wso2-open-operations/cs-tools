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
import { ApiQueryKeys } from "@constants/apiConstants";
import { isMockMode, useBackendApi } from "@api/backend/client";
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import type {
  BeAccountSearchPayload,
  BeAccountSearchResponse,
  BeCaseSearchPayload,
  BeCaseSearchResponse,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";
import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CsmCaseRow,
  CsmCasesListResponse,
} from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 200;

/** Page size for the cross-project case list (server-side filtering TBD). */
const CASES_PAGE_LIMIT = 100;
/** Page size for the project / account name lookups (customer column). */
const LOOKUP_PAGE_LIMIT = 200;

/**
 * Cross-project CSM cases list.
 *
 * LIVE mode does a single `POST /cases/search` (the flat, cross-project search)
 * and maps each rich `CaseSearchView` — which embeds project / deployment /
 * deployed-product — to the UI `CsmCaseRow`. The account (customer) name is the
 * only field not embedded, so it's resolved once via `projects/search`
 * (projectId → accountId) + `accounts/search` (accountId → name) rather than
 * the old per-project fan-out.
 *
 * Filtering is still applied client-side in `CsmCasesPage`; pushing filters
 * into the search payload (searchQuery / priorityKeys / stateKeys / projectIds)
 * and true pagination are the follow-up. The BE has no assignee field, so the
 * `scope` parameter remains a no-op in LIVE.
 */
export function useGetCsmCases(
  scope: DashboardScope,
): UseQueryResult<CsmCasesListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmCasesListResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_CASES, scope],
    queryFn: async (): Promise<CsmCasesListResponse> => {
      if (isMockMode()) {
        logger.debug(`[useGetCsmCases] Returning mock cases for scope=${scope}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCases(scope);
      }

      // One cross-project case search, plus project/account lookups for the
      // customer column (cases embed the project, but not its account).
      const [casesResponse, projectsResponse, accountsResponse] =
        await Promise.all([
          api.post<BeCaseSearchPayload, BeCaseSearchResponse>("/cases/search", {
            pagination: { offset: 0, limit: CASES_PAGE_LIMIT },
          }),
          api
            .post<BeProjectSearchPayload, BeProjectSearchResponse>(
              "/projects/search",
              { pagination: { offset: 0, limit: LOOKUP_PAGE_LIMIT } },
            )
            .catch((err) => {
              logger.warn(
                `[useGetCsmCases] /projects/search failed: ${(err as Error).message}`,
              );
              return {
                projects: [],
                total: 0,
                limit: 0,
                offset: 0,
                hasMore: false,
              } satisfies BeProjectSearchResponse;
            }),
          api
            .post<BeAccountSearchPayload, BeAccountSearchResponse>(
              "/accounts/search",
              { pagination: { offset: 0, limit: LOOKUP_PAGE_LIMIT } },
            )
            .catch((err) => {
              logger.warn(
                `[useGetCsmCases] /accounts/search failed: ${(err as Error).message}`,
              );
              return {
                accounts: [],
                total: 0,
                limit: 0,
                offset: 0,
                hasMore: false,
              } satisfies BeAccountSearchResponse;
            }),
        ]);

      const projectAccount = new Map<string, string>(
        (projectsResponse.projects ?? [])
          .filter((p) => p.accountId)
          .map((p) => [p.id, p.accountId as string]),
      );
      const accountName = new Map<string, string>(
        (accountsResponse.accounts ?? [])
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
          createdAt: c.createdAt ?? "",
          updatedAt: c.updatedAt ?? c.createdAt ?? "",
        };
      });

      return { scope, cases };
    },
    staleTime: 30_000,
  });
}
