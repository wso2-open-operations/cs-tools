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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeDeployment,
  BeDeploymentSearchPayload,
  BeDeploymentSearchResponse,
} from "@api/backend/types";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/**
 * Deployments belonging to a project, via `POST /deployments/search`
 * (`projectIds: [projectId]`). Used by the case-create cascade once a project
 * is picked. Disabled until a project id is provided.
 */
export function useSearchDeployments(
  projectId: string | undefined,
): UseQueryResult<BeDeployment[], Error> {
  const api = useBackendApi();

  return useQuery<BeDeployment[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENTS, projectId ?? ""],
    queryFn: async (): Promise<BeDeployment[]> => {
      // Page through so deployments beyond the first page stay selectable.
      const all: BeDeployment[] = [];
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const res = await api.post<
          BeDeploymentSearchPayload,
          BeDeploymentSearchResponse
        >("/deployments/search", {
          projectIds: projectId ? [projectId] : [],
          pagination: { offset, limit: PAGE_LIMIT },
        });
        const page = res.deployments ?? [];
        all.push(...page);
        if (page.length < PAGE_LIMIT) break;
      }
      return all;
    },
    enabled: !!projectId,
    staleTime: 60_000,
  });
}
