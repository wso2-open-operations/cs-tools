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
import { useAuthApiClient } from "@hooks/useAuthApiClient";
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { getMockCsmProjects } from "@features/csm-projects/api/mocks/projectsMocks";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";
import type { CsmProjectsListResponse } from "@features/csm-projects/types/csmProjects";

const MOCK_LATENCY_MS = 200;

export function useGetCsmProjects(
  scope: DashboardScope,
): UseQueryResult<CsmProjectsListResponse, Error> {
  const logger = useLogger();
  const authFetch = useAuthApiClient();

  return useQuery<CsmProjectsListResponse, Error>({
    queryKey: [ApiQueryKeys.CSM_PROJECTS, scope],
    queryFn: async (): Promise<CsmProjectsListResponse> => {
      if (window.config?.CSM_PORTAL_USE_MOCKS) {
        logger.debug(
          `[useGetCsmProjects] Returning mock projects for scope=${scope}`,
        );
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmProjects(scope);
      }

      const baseUrl = window.config?.CSM_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CSM_PORTAL_BACKEND_BASE_URL is not configured");
      }

      const url = `${baseUrl}/csm/projects?scope=${encodeURIComponent(scope)}`;
      const response = await authFetch(url, { method: "GET" });
      if (!response.ok) {
        throw new Error(`Error fetching CSM projects: ${response.statusText}`);
      }
      return (await response.json()) as CsmProjectsListResponse;
    },
    staleTime: 30_000,
  });
}
