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
import {
  severityFromPriority,
  uiStateFromBe,
} from "@api/backend/mappers";
import type {
  BeAccountSearchPayload,
  BeAccountSearchResponse,
  BeCaseSearchResponse,
  BeProject,
  BeProjectCaseSearchPayload,
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

/** Max number of projects we fan out across to assemble the cross-project list. */
const PROJECT_FAN_OUT_LIMIT = 20;
/** Max cases pulled per project in the cross-project fan-out. */
const CASES_PER_PROJECT_LIMIT = 30;

/**
 * Cross-project CSM cases list.
 *
 * The backend exposes only a project-scoped case search
 * (`POST /projects/{id}/cases/search`). To preserve the cross-project list
 * UX, LIVE mode performs a bounded fan-out: pulls the first
 * {@link PROJECT_FAN_OUT_LIMIT} projects from `/projects/search`, then runs
 * the case search in parallel against each (limit
 * {@link CASES_PER_PROJECT_LIMIT}). Results are concatenated and mapped to
 * the UI's `CsmCaseRow` shape. Note: the BE doesn't yet support filtering
 * by assignee, so the `scope` parameter is currently a no-op in LIVE.
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

      const [projectsResponse, accountsResponse] = await Promise.all([
        api.post<BeProjectSearchPayload, BeProjectSearchResponse>(
          "/projects/search",
          { pagination: { offset: 0, limit: PROJECT_FAN_OUT_LIMIT } },
        ),
        api
          .post<BeAccountSearchPayload, BeAccountSearchResponse>(
            "/accounts/search",
            { pagination: { offset: 0, limit: PROJECT_FAN_OUT_LIMIT } },
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
      const projects: BeProject[] = projectsResponse.projects ?? [];
      const projectName = new Map<string, string>(
        projects.map((p) => [p.id, p.name ?? p.projectKey ?? p.id]),
      );
      const projectAccount = new Map<string, string>(
        projects
          .filter((p) => p.accountId)
          .map((p) => [p.id, p.accountId as string]),
      );
      const accountName = new Map<string, string>(
        (accountsResponse.accounts ?? [])
          .filter((a) => a.name)
          .map((a) => [a.id, a.name as string]),
      );

      // Fan out: one search per project in parallel.
      const perProject = await Promise.all(
        projects.map((p) =>
          api
            .post<BeProjectCaseSearchPayload, BeCaseSearchResponse>(
              `/projects/${encodeURIComponent(p.id)}/cases/search`,
              {
                pagination: { offset: 0, limit: CASES_PER_PROJECT_LIMIT },
              },
            )
            .catch((err) => {
              logger.warn(
                `[useGetCsmCases] cases/search for project ${p.id} failed: ${
                  (err as Error).message
                }`,
              );
              return null;
            }),
        ),
      );

      const cases: CsmCaseRow[] = [];
      for (const r of perProject) {
        if (!r) continue;
        for (const c of r.cases ?? []) {
          const projectId = c.projectId ?? "";
          const accountId = projectAccount.get(projectId) ?? "";
          cases.push({
            id: c.id,
            caseNumber: c.number ?? c.id,
            wso2CaseId: c.wso2Id ?? c.id,
            subject: c.subject ?? "(no subject)",
            customer: accountName.get(accountId) ?? "—",
            accountId,
            projectId,
            projectName: projectName.get(projectId) ?? "—",
            product: "—",
            severity: severityFromPriority(c.priority),
            state: uiStateFromBe(c.state),
            assignee: c.createdBy ?? "Unassigned",
            assigneeIsMe: false,
            slaClockType: "ack",
            minutesToBreach: 0,
            createdAt: c.createdAt ?? "",
            updatedAt: c.updatedAt ?? c.createdAt ?? "",
          });
        }
      }
      return { scope, cases };
    },
    staleTime: 30_000,
  });
}
