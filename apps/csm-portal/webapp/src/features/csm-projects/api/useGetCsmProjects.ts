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
import type {
  BeProjectSearchPayload,
  BeProjectSearchResponse,
} from "@api/backend/types";
import { getMockCsmProjects } from "@features/csm-projects/api/mocks/projectsMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type {
  CsmProjectRow,
  CsmProjectsListResponse,
} from "@features/csm-projects/types/csmProjects";

const MOCK_LATENCY_MS = 200;
/** BE caps `/projects/search` at 100 per page. */
const PROJECTS_PAGE_LIMIT = 100;

/**
 * Cross-customer projects list. Backed by `POST /projects/search` in LIVE
 * mode. The `scope` argument is currently a no-op against the backend
 * because the search payload has no concept of "mine vs all" yet — every
 * call returns the first page of every project the auth context can see.
 *
 * Fields the UI shows (customer name, tier, status, update level, case
 * counts) aren't yet surfaced by the BE; they default to placeholders.
 */
export function useGetCsmProjects(
  scope: DashboardScope,
): UseQueryResult<CsmProjectsListResponse, Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmProjectsListResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_PROJECTS, scope],
    queryFn: async (): Promise<CsmProjectsListResponse> => {
      if (isMockMode()) {
        logger.debug(
          `[useGetCsmProjects] Returning mock projects for scope=${scope}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmProjects(scope);
      }
      const response = await api.post<
        BeProjectSearchPayload,
        BeProjectSearchResponse
      >("/projects/search", {
        pagination: { offset: 0, limit: PROJECTS_PAGE_LIMIT },
      });
      const projects: CsmProjectRow[] = (response.projects ?? []).map((p) => ({
        id: p.id,
        name: p.name ?? p.projectKey ?? p.id,
        customer: "—",
        accountId: p.accountId ?? "",
        tier: "Silver",
        productType: "—",
        status: "Active",
        updateLevel: "—",
        openCaseCount: 0,
        s0s1Count: 0,
        breachedCount: 0,
        lastActivityAt: p.updatedAt ?? p.createdAt ?? "",
      }));
      return { scope, projects };
    },
    staleTime: 30_000,
  });
}
